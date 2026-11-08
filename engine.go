package knobs

import (
	"path/filepath"
	"sort"
	"strings"
)

// Engine resolves one application's configuration: attached schemas, then layers in
// precedence order, lowest first. Attach and Add are closed by the first Resolve.
type Engine struct {
	set    *schemaSet
	layers []Layer
	closed bool
}

// New creates the engine for an application. The name is lowercase snake case: it is the root
// of every file, the prefix of every key and environment variable.
func New(app string) (*Engine, error) {
	if !namePattern.MatchString(app) {
		return nil, newError(CodeInvalidArgument, "args", "", "the application name is lowercase snake case")
	}
	return &Engine{set: &schemaSet{app: app, modules: map[string]*module{}}}, nil
}

// Attach mounts one module schema at <app>.<module>, the module named by its x-knobs-module.
// globalPath and userPath name the module's own files, holding its mapping directly; each
// applies right after the layer of the same name (LayerGlobal, LayerUser), and either may be
// empty or missing.
func (e *Engine) Attach(schema []byte, globalPath, userPath string) error {
	if e.closed {
		return newError(CodeInvalidArgument, "schema", "", "Attach after Resolve")
	}
	m, err := parseModule(e.set.app, schema)
	if err != nil {
		return err
	}
	if _, dup := e.set.modules[m.name]; dup {
		return newError(CodeModuleConflict, "schema", e.set.app+"."+m.name, "two schemas for one module")
	}
	m.globalPath, m.userPath = globalPath, userPath
	e.set.modules[m.name] = m
	return nil
}

// Add appends layers above every layer added before them, in the order given.
func (e *Engine) Add(layers ...Layer) error {
	if e.closed {
		return newError(CodeInvalidArgument, "args", "", "Add after Resolve")
	}
	e.layers = append(e.layers, layers...)
	return nil
}

// Schema is the master schema: the application object with one property per attached module.
func (e *Engine) Schema() map[string]any { return e.set.master() }

// Resolve validates and merges every layer into the process configuration. It closes the
// engine: later Attach and Add calls fail.
func (e *Engine) Resolve() (*Config, error) {
	e.closed = true
	view := &Schemas{set: e.set}
	values := map[string]Value{}
	for _, l := range e.withModuleFiles() {
		settings, err := l.Load(view)
		if err != nil {
			return nil, err
		}
		if err := merge(e.set, l, sortSettings(settings), values); err != nil {
			return nil, err
		}
	}
	if err := checkRequired(e.set, values); err != nil {
		return nil, err
	}
	return &Config{set: e.set, values: values}, nil
}

// withModuleFiles is the layer list with each module's own files right after the layer of the
// same name, modules in name order.
func (e *Engine) withModuleFiles() []Layer {
	var out []Layer
	mods := sortedKeys(e.set.modules)
	for _, l := range e.layers {
		out = append(out, l)
		for _, name := range mods {
			m := e.set.modules[name]
			switch {
			case l.Name() == LayerGlobal && m.globalPath != "":
				out = append(out, moduleFile(LayerGlobal, name, m.globalPath))
			case l.Name() == LayerUser && m.userPath != "":
				out = append(out, moduleFile(LayerUser, name, m.userPath))
			}
		}
	}
	return out
}

// merge validates one layer's settings, in key order, and writes them over the values below.
func merge(set *schemaSet, l Layer, settings []Setting, values map[string]Value) error {
	pol := l.Policy()
	for _, st := range settings {
		if st.Err != "" {
			return newError(st.Err, l.Name(), st.Key, "")
		}
		p := set.prop(st.Module, st.Name)
		if p.secret && pol.RejectSecrets != "" {
			return newError(pol.RejectSecrets, l.Name(), st.Key, "a secret is set only in the secrets file, env or code")
		}
		v, code := convert(p, st)
		if code != "" {
			return newError(code, l.Name(), st.Key, "")
		}
		values[st.Key] = Value{Key: st.Key, Value: v, Layer: l.Name(), Secret: p.secret || pol.SecretValues}
	}
	return nil
}

// convert turns a setting into a value of its property's type that passes its rules.
func convert(p *property, st Setting) (any, string) {
	var v any
	var code string
	if text, ok := st.Value.(string); ok && st.Text {
		v, code = coerceText(p, text)
	} else {
		v, code = coerceTyped(p, st.Value)
	}
	if code != "" {
		return nil, code
	}
	return v, checkRules(p, v)
}

func checkRequired(set *schemaSet, values map[string]Value) error {
	var missing []string
	for _, m := range set.modules {
		for _, name := range m.required {
			if k := set.key(m.name, name); values[k].Layer == "" {
				missing = append(missing, k)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return newError(CodeMissingRequired, "resolve", missing[0], "no layer sets it")
}

// Standard configures AddStandard. Empty paths are resolved from the path knobs
// <app>.knobs.global_path, user_path and secrets_path (cli, then env), else the defaults; a
// secrets file is read only when a path is given.
type Standard struct {
	GlobalPath, UserPath, SecretsPath string
	Env                               func(string) (string, bool)
	Knobs                             []string
	Runtime                           map[string]string
}

// AddStandard adds the standard layers in precedence order: default, global, user, secrets,
// env, cli and, when given, runtime.
func (e *Engine) AddStandard(s Standard) error {
	paths, knobs, err := e.pathKnobs(s.Knobs)
	if err != nil {
		return err
	}
	env := s.Env
	if env == nil {
		env = func(string) (string, bool) { return "", false }
	}
	global, gx := e.pick(s.GlobalPath, "global_path", paths, env, DefaultGlobalPath(e.set.app))
	user, ux := e.pick(s.UserPath, "user_path", paths, env, DefaultUserPath(e.set.app, env))
	secrets, sx := e.pick(s.SecretsPath, "secrets_path", paths, env, "")
	layers := []Layer{
		Defaults(), File(LayerGlobal, global, gx), File(LayerUser, user, ux), File(LayerSecrets, secrets, sx),
		Env(s.Env), CLI(knobs),
	}
	if s.Runtime != nil {
		layers = append(layers, Runtime(s.Runtime))
	}
	return e.Add(layers...)
}

// pathKnobs splits the <app>.knobs.* arguments from the rest.
func (e *Engine) pathKnobs(args []string) (map[string]string, []string, error) {
	paths := map[string]string{}
	var rest []string
	prefix := e.set.app + "." + reservedModule + "."
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		if !ok {
			return nil, nil, newError(CodeInvalidArgument, LayerCLI, "", "want KEY=VALUE")
		}
		if name, isPath := strings.CutPrefix(k, prefix); isPath {
			paths[name] = v
			continue
		}
		rest = append(rest, a)
	}
	return paths, rest, nil
}

// pick chooses a file path: the given one, the cli path knob, the env path knob, else the
// default. Every source but the default names the file explicitly.
func (e *Engine) pick(given, knob string, paths map[string]string, env func(string) (string, bool),
	def string) (string, bool) {
	if given != "" {
		return given, true
	}
	if p, ok := paths[knob]; ok {
		return p, true
	}
	if p, ok := env(strings.ToUpper(e.set.app + "_" + reservedModule + "_" + knob)); ok && p != "" {
		return p, true
	}
	return def, false
}

// DefaultGlobalPath is the global file's default location: /etc/<app>/config.yaml.
func DefaultGlobalPath(app string) string { return filepath.Join("/etc", app, "config.yaml") }

// DefaultUserPath is the user file's default location: $XDG_CONFIG_HOME/<app>/config.yaml, else
// $HOME/.config/<app>/config.yaml, else none.
func DefaultUserPath(app string, env func(string) (string, bool)) string {
	if x, ok := env("XDG_CONFIG_HOME"); ok && x != "" {
		return filepath.Join(x, app, "config.yaml")
	}
	if h, ok := env("HOME"); ok && h != "" {
		return filepath.Join(h, ".config", app, "config.yaml")
	}
	return ""
}
