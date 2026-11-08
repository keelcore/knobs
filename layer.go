package knobs

import (
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Layer names (spec/README.md, Layers).
const (
	LayerCode       = "code"
	LayerSchema     = "schema"
	LayerGlobal     = "global"
	LayerUser       = "user"
	LayerSecrets    = "secrets"
	LayerEnv        = "env"
	LayerCLI        = "cli"
	LayerRuntime    = "runtime"
	LayerConnection = "connection"
)

// Setting is one value a layer sets. Key is the dotted key; Module and Name are empty when the
// key names nothing in the schemas. Text values are converted to the property's type; other
// values must already be of it. Err, when set, is the error code the setting raises.
type Setting struct {
	Key, Module, Name string
	Value             any
	Text              bool
	Err               string
}

// Policy is how a layer treats secrets: whether every value from it is secret, and the error a
// key marked secret raises when this layer sets it ("" when the layer may set secrets).
type Policy struct {
	SecretValues  bool
	RejectSecrets string
}

// Layer is one source of settings. The engine merges layers in the order they were added, each
// overriding the ones before it, key by key.
type Layer interface {
	Name() string
	Policy() Policy
	Load(s *Schemas) ([]Setting, error)
}

// Schemas is the read-only view of the attached schemas a layer loads against.
type Schemas struct{ set *schemaSet }

// App is the application name.
func (s *Schemas) App() string { return s.set.app }

// Lookup splits a dotted key into module and name when the schemas define it.
func (s *Schemas) Lookup(key string) (module, name string, ok bool) {
	parts := strings.Split(key, ".")
	if len(parts) != 3 || parts[0] != s.set.app || s.set.prop(parts[1], parts[2]) == nil {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// EnvName is the environment variable a property reads: its x-knobs-env, else the upper case
// <APP>_<MODULE>_<NAME>.
func (s *Schemas) EnvName(module, name string) string {
	if p := s.set.prop(module, name); p != nil && p.env != "" {
		return p.env
	}
	return strings.ToUpper(s.set.app + "_" + module + "_" + name)
}

// Keys lists every defined property as module and name, in key order.
func (s *Schemas) Keys() [][2]string {
	var out [][2]string
	for _, p := range s.set.sortedProps() {
		out = append(out, [2]string{p.module, p.name})
	}
	return out
}

type layer struct {
	name   string
	policy Policy
	load   func(s *Schemas) ([]Setting, error)
}

func (l *layer) Name() string                       { return l.name }
func (l *layer) Policy() Policy                     { return l.policy }
func (l *layer) Load(s *Schemas) ([]Setting, error) { return l.load(s) }

// Code is the layer of values set in code: the baseline under every other layer. It holds each attached
// module's own code values (Module.Code), then the given values by dotted key, which override them. Values
// are typed as file values are (int, int64, float64, bool, string; a time.Duration for a duration key). A
// key marked secret fails here: a credential in code is hardcoded.
func Code(values map[string]any) Layer {
	return &layer{name: LayerCode, policy: Policy{RejectSecrets: CodeSecretInCode}, load: func(s *Schemas) ([]Setting, error) {
		var out []Setting
		for _, mod := range sortedKeys(s.set.modules) {
			for _, name := range sortedKeys(s.set.modules[mod].code) {
				out = append(out, codeSetting(s, s.set.key(mod, name), s.set.modules[mod].code[name]))
			}
		}
		for _, k := range sortedKeys(values) {
			out = append(out, codeSetting(s, k, values[k]))
		}
		return out, nil
	}}
}

func codeSetting(s *Schemas, key string, v any) Setting {
	mod, name, known := s.Lookup(key)
	if !known {
		return Setting{Key: key, Err: CodeUnknownKey}
	}
	if d, isDuration := v.(time.Duration); isDuration {
		v = d.String()
	}
	return Setting{Key: key, Module: mod, Name: name, Value: v}
}

// Defaults is the layer of every property's schema default; it overrides Code.
func Defaults() Layer {
	return &layer{name: LayerSchema, load: func(s *Schemas) ([]Setting, error) {
		var out []Setting
		for _, p := range s.set.sortedProps() {
			if p.hasDefault {
				out = append(out, Setting{Key: s.set.key(p.module, p.name), Module: p.module, Name: p.name, Value: p.def})
			}
		}
		return out, nil
	}}
}

// File is a YAML file layer rooted at the application. Named LayerSecrets, every value from it is
// secret and the file must be readable by its owner only; any other name is a plain file, where a
// key marked secret fails. explicit says the path was named by the operator: missing, it fails;
// otherwise a missing file is skipped. An empty path is an empty layer.
func File(name, path string, explicit bool) Layer {
	l := &layer{name: name, policy: Policy{RejectSecrets: CodeSecretInPlainFile}}
	if name == LayerSecrets {
		l.policy = Policy{SecretValues: true}
	}
	l.load = func(s *Schemas) ([]Setting, error) {
		doc, err := readYAML(name, path, explicit)
		if err != nil || doc == nil {
			return nil, err
		}
		return appSettings(s, doc), nil
	}
	return l
}

// moduleFile is a module's own file in the layer named name: its mapping is the module's directly.
func moduleFile(name, module, path string) Layer {
	return &layer{name: name, policy: Policy{RejectSecrets: CodeSecretInPlainFile}, load: func(s *Schemas) ([]Setting, error) {
		doc, err := readYAML(name, path, false)
		if err != nil || doc == nil {
			return nil, err
		}
		return moduleSettings(s, module, doc), nil
	}}
}

// readYAML reads a YAML file; nil without error means an absent optional file or an empty one.
func readYAML(layerName, path string, explicit bool) (any, error) {
	if path == "" {
		return nil, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if explicit {
			return nil, newError(CodeFileNotFound, layerName, "", path+" does not exist")
		}
		return nil, nil
	}
	if err == nil && layerName == LayerSecrets {
		if permErr := ownerOnly(path, info); permErr != nil {
			return nil, newError(CodeSecretsFilePermissions, layerName, "", permErr.Error())
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, newError(CodeFileParse, layerName, "", err.Error())
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, newError(CodeFileParse, layerName, "", "not valid YAML")
	}
	if doc != nil {
		if _, ok := doc.(map[string]any); !ok {
			return nil, newError(CodeFileParse, layerName, "", "the document is not a mapping")
		}
	}
	return doc, nil
}

// appSettings flattens a document rooted at the application.
func appSettings(s *Schemas, doc any) []Setting {
	var out []Setting
	for root, v := range doc.(map[string]any) {
		if root != s.set.app {
			out = append(out, Setting{Key: root, Err: CodeUnknownKey})
			continue
		}
		mods, ok := v.(map[string]any)
		if !ok {
			out = append(out, Setting{Key: root, Err: CodeInvalidType})
			continue
		}
		for mod, mv := range mods {
			if _, known := s.set.modules[mod]; !known {
				out = append(out, Setting{Key: root + "." + mod, Err: CodeUnknownKey})
				continue
			}
			out = append(out, moduleSettings(s, mod, mv)...)
		}
	}
	return out
}

// moduleSettings flattens one module's mapping.
func moduleSettings(s *Schemas, mod string, v any) []Setting {
	names, ok := v.(map[string]any)
	if !ok {
		return []Setting{{Key: s.set.app + "." + mod, Err: CodeInvalidType}}
	}
	var out []Setting
	for name, value := range names {
		st := Setting{Key: s.set.key(mod, name), Module: mod, Name: name, Value: value}
		if s.set.prop(mod, name) == nil {
			st = Setting{Key: st.Key, Err: CodeUnknownKey}
		}
		out = append(out, st)
	}
	return out
}

// Env is the environment layer: each property's variable (Schemas.EnvName), read through
// lookup; an empty value counts as unset. A nil lookup reads nothing.
func Env(lookup func(string) (string, bool)) Layer {
	return &layer{name: LayerEnv, load: func(s *Schemas) ([]Setting, error) {
		if lookup == nil {
			return nil, nil
		}
		var out []Setting
		for _, k := range s.Keys() {
			if v, ok := lookup(s.EnvName(k[0], k[1])); ok && v != "" {
				out = append(out, Setting{Key: s.set.key(k[0], k[1]), Module: k[0], Name: k[1], Value: v, Text: true})
			}
		}
		return out, nil
	}}
}

// CLI is the command-line layer: each argument is KEY=VALUE, as given to --knob. A key marked
// secret fails here: other users can read a process's arguments.
func CLI(args []string) Layer {
	return &layer{name: LayerCLI, policy: Policy{RejectSecrets: CodeSecretOnCLI}, load: func(s *Schemas) ([]Setting, error) {
		return textSettings(s, LayerCLI, args)
	}}
}

// Runtime is the layer of values set in code, by dotted key.
func Runtime(values map[string]string) Layer {
	return &layer{name: LayerRuntime, load: func(s *Schemas) ([]Setting, error) {
		args := make([]string, 0, len(values))
		for _, k := range sortedKeys(values) {
			args = append(args, k+"="+values[k])
		}
		return textSettings(s, LayerRuntime, args)
	}}
}

func textSettings(s *Schemas, layerName string, args []string) ([]Setting, error) {
	var out []Setting
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return nil, newError(CodeInvalidArgument, layerName, "", "want KEY=VALUE")
		}
		mod, name, known := s.Lookup(key)
		if !known {
			out = append(out, Setting{Key: key, Err: CodeUnknownKey})
			continue
		}
		out = append(out, Setting{Key: key, Module: mod, Name: name, Value: value, Text: true})
	}
	return out, nil
}

// sortSettings orders settings by key; a key set twice in one layer keeps its last value.
func sortSettings(in []Setting) []Setting {
	sort.SliceStable(in, func(i, j int) bool { return in[i].Key < in[j].Key })
	return in
}
