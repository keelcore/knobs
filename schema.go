package knobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

// The value types a property may declare.
const (
	TypeString   = "string"
	TypeInteger  = "integer"
	TypeNumber   = "number"
	TypeBoolean  = "boolean"
	TypeDuration = "duration" // a string with x-knobs-type: duration
)

// reservedModule holds the engine's own settings (the file-path knobs).
const reservedModule = "knobs"

const dialect = "https://json-schema.org/draft/2020-12/schema"

var (
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	envPattern  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// The only keywords knobs accepts; every one of them is enforced (spec/README.md, Schemas).
var (
	moduleKeywords = keywordSet("$schema", "$id", "title", "description", "x-knobs-module", "type",
		"additionalProperties", "required", "properties")
	propertyKeywords = keywordSet("type", "title", "description", "default", "enum", "minimum", "maximum",
		"minLength", "maxLength", "x-knobs-type", "x-knobs-secret", "x-knobs-connection", "x-knobs-env")
)

func keywordSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// property is one knob's rules, read from its schema.
type property struct {
	module, name string
	typ          string // TypeString, TypeInteger, TypeNumber, TypeBoolean or TypeDuration
	def          any
	hasDefault   bool
	enum         []any
	min, max     *float64
	minLen       *int64
	maxLen       *int64
	secret       bool
	connection   bool
	env          string
}

// module is one attached schema: its properties, required names, its own files, and the schema
// as written (for the master schema).
type module struct {
	name       string
	props      map[string]*property
	required   []string
	raw        map[string]any
	globalPath string
	userPath   string
}

// schemaSet is every attached module of one application.
type schemaSet struct {
	app     string
	modules map[string]*module
}

func (s *schemaSet) key(mod, name string) string { return s.app + "." + mod + "." + name }

// prop returns the property for module and name, or nil.
func (s *schemaSet) prop(mod, name string) *property {
	if m, ok := s.modules[mod]; ok {
		return m.props[name]
	}
	return nil
}

// sortedProps lists every property in key order.
func (s *schemaSet) sortedProps() []*property {
	var out []*property
	for _, m := range s.modules {
		for _, p := range m.props {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return s.key(out[i].module, out[i].name) < s.key(out[j].module, out[j].name)
	})
	return out
}

// parseModule reads and checks one module schema.
func parseModule(app string, data []byte) (*module, error) {
	raw, err := decodeObject(data)
	if err != nil {
		return nil, newError(CodeSchemaInvalid, "schema", "", "not a JSON object: "+err.Error())
	}
	name, _ := raw["x-knobs-module"].(string)
	if !namePattern.MatchString(name) || name == reservedModule {
		return nil, newError(CodeSchemaInvalid, "schema", "", fmt.Sprintf("x-knobs-module %q is not a usable module name", name))
	}
	modKey := app + "." + name
	if err := checkModuleShape(raw, modKey); err != nil {
		return nil, err
	}
	m := &module{name: name, props: map[string]*property{}, raw: raw}
	props, _ := raw["properties"].(map[string]any)
	for _, pname := range sortedKeys(props) {
		p, err := parseProperty(name, pname, props[pname], modKey+"."+pname)
		if err != nil {
			return nil, err
		}
		m.props[pname] = p
	}
	if err := parseRequired(m, raw["required"], modKey); err != nil {
		return nil, err
	}
	return m, nil
}

func checkModuleShape(raw map[string]any, modKey string) error {
	for _, k := range sortedKeys(raw) {
		if !moduleKeywords[k] {
			return newError(CodeSchemaInvalid, "schema", modKey, "unsupported keyword "+k)
		}
	}
	if raw["type"] != "object" || raw["additionalProperties"] != false {
		return newError(CodeSchemaInvalid, "schema", modKey, `a module is "type": "object" with "additionalProperties": false`)
	}
	if _, ok := raw["properties"].(map[string]any); !ok {
		return newError(CodeSchemaInvalid, "schema", modKey, "properties must be an object")
	}
	return nil
}

func parseRequired(m *module, v any, modKey string) error {
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		return newError(CodeSchemaInvalid, "schema", modKey, "required must be a list")
	}
	for _, r := range list {
		name, ok := r.(string)
		if !ok || m.props[name] == nil {
			return newError(CodeSchemaInvalid, "schema", modKey, fmt.Sprintf("required names an unknown property %v", r))
		}
		m.required = append(m.required, name)
	}
	return nil
}

func parseProperty(mod, name string, v any, key string) (*property, error) {
	raw, ok := v.(map[string]any)
	if !namePattern.MatchString(name) || !ok {
		return nil, newError(CodeSchemaInvalid, "schema", key, "a property is an object with a lowercase snake case name")
	}
	for _, k := range sortedKeys(raw) {
		if !propertyKeywords[k] {
			return nil, newError(CodeSchemaInvalid, "schema", key, "unsupported keyword "+k)
		}
	}
	p := &property{module: mod, name: name}
	steps := []func(*property, map[string]any) string{readType, readFlags, readBounds, readLengths, readEnum, readDefault}
	for _, step := range steps {
		if msg := step(p, raw); msg != "" {
			return nil, newError(CodeSchemaInvalid, "schema", key, msg)
		}
	}
	return p, nil
}

func readType(p *property, raw map[string]any) string {
	t, _ := raw["type"].(string)
	switch t {
	case TypeString, TypeInteger, TypeNumber, TypeBoolean:
		p.typ = t
	default:
		return fmt.Sprintf("type %v is not string, integer, number or boolean", raw["type"])
	}
	if kt, ok := raw["x-knobs-type"]; ok {
		if kt != TypeDuration || t != TypeString {
			return `x-knobs-type must be "duration", on a string`
		}
		p.typ = TypeDuration
	}
	return ""
}

func readFlags(p *property, raw map[string]any) string {
	var ok bool
	for _, f := range []struct {
		word string
		dst  *bool
	}{{"x-knobs-secret", &p.secret}, {"x-knobs-connection", &p.connection}} {
		if v, present := raw[f.word]; present {
			if *f.dst, ok = v.(bool); !ok {
				return f.word + " must be a boolean"
			}
		}
	}
	if p.secret && p.connection {
		return "a secret is never connection-overridable"
	}
	if v, present := raw["x-knobs-env"]; present {
		if p.env, _ = v.(string); !envPattern.MatchString(p.env) {
			return "x-knobs-env must be an upper case environment variable name"
		}
	}
	return ""
}

func readBounds(p *property, raw map[string]any) string {
	for _, b := range []struct {
		word string
		dst  **float64
	}{{"minimum", &p.min}, {"maximum", &p.max}} {
		v, present := raw[b.word]
		if !present {
			continue
		}
		if p.typ != TypeInteger && p.typ != TypeNumber {
			return b.word + " applies to integer and number only"
		}
		f, ok := v.(float64)
		if n, isInt := v.(int64); isInt {
			f, ok = float64(n), true
		}
		if !ok {
			return b.word + " must be a number"
		}
		*b.dst = &f
	}
	return ""
}

func readLengths(p *property, raw map[string]any) string {
	for _, l := range []struct {
		word string
		dst  **int64
	}{{"minLength", &p.minLen}, {"maxLength", &p.maxLen}} {
		v, present := raw[l.word]
		if !present {
			continue
		}
		if p.typ != TypeString && p.typ != TypeDuration {
			return l.word + " applies to string only"
		}
		n, ok := v.(int64)
		if !ok || n < 0 {
			return l.word + " must be a non-negative integer"
		}
		*l.dst = &n
	}
	return ""
}

func readEnum(p *property, raw map[string]any) string {
	v, present := raw["enum"]
	if !present {
		return ""
	}
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return "enum must be a non-empty list"
	}
	for _, e := range list {
		val, code := coerceTyped(p, e)
		if code != "" {
			return fmt.Sprintf("enum value %v is not of type %s", e, p.typ)
		}
		p.enum = append(p.enum, val)
	}
	return ""
}

func readDefault(p *property, raw map[string]any) string {
	v, present := raw["default"]
	if !present {
		return ""
	}
	if p.secret {
		return "a secret has no default"
	}
	val, code := coerceTyped(p, v)
	if code == "" {
		code = checkRules(p, val)
	}
	if code != "" {
		return "default breaks its own rules: " + code
	}
	p.def, p.hasDefault = val, true
	return ""
}

// decodeObject decodes a JSON object, keeping integers as int64 and other numbers as float64.
func decodeObject(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	obj, ok := normalizeJSON(v).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("top level is %T", v)
	}
	return obj, nil
}

func normalizeJSON(v any) any {
	switch t := v.(type) {
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return n
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, e := range t {
			t[k] = normalizeJSON(e)
		}
	case []any:
		for i, e := range t {
			t[i] = normalizeJSON(e)
		}
	}
	return v
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// master is the master schema: the application object with one property per module, each the
// module schema as written without $schema and $id.
func (s *schemaSet) master() map[string]any {
	mods := map[string]any{}
	for name, m := range s.modules {
		cp := make(map[string]any, len(m.raw))
		for k, v := range m.raw {
			if k != "$schema" && k != "$id" {
				cp[k] = v
			}
		}
		mods[name] = cp
	}
	return map[string]any{
		"$schema":              dialect,
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			s.app: map[string]any{"type": "object", "additionalProperties": false, "properties": mods},
		},
	}
}
