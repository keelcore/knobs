package knobs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSchemaShapeErrors(t *testing.T) {
	for name, s := range map[string]string{
		"properties not object": `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": []}`,
		"required not list":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "required": "p", "properties": {"p": {"type": "string"}}}`,
		"enum misspelled":       `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "enums": [1]}}}`,
		"top level not object":  `[1]`,
	} {
		t.Run(name, func(t *testing.T) {
			wantCode(t, engine(t).Attach(Module{Schema: []byte(s)}), CodeSchemaInvalid)
		})
	}
}

func TestAttachRejectsBadSchemas(t *testing.T) {
	for name, s := range map[string]string{
		"not json":             `{`,
		"no module":            `{"type": "object", "additionalProperties": false, "properties": {}}`,
		"reserved module":      `{"x-knobs-module": "knobs", "type": "object", "additionalProperties": false, "properties": {}}`,
		"bad module name":      `{"x-knobs-module": "Http", "type": "object", "additionalProperties": false, "properties": {}}`,
		"module not object":    `{"x-knobs-module": "m", "type": "string", "additionalProperties": false, "properties": {}}`,
		"open module":          `{"x-knobs-module": "m", "type": "object", "properties": {}}`,
		"bad property name":    `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"Port": {"type": "integer"}}}`,
		"property not object":  `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": 1}}`,
		"unknown type":         `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "strnig"}}}`,
		"misspelled bound":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "minimun": 1}}}`,
		"misspelled length":    `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "maxLenght": 1}}}`,
		"duration on integer":  `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "x-knobs-type": "duration"}}}`,
		"unknown knobs type":   `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "x-knobs-type": "ip"}}}`,
		"default out of range": `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "default": 9, "maximum": 5}}}`,
		"default wrong type":   `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "default": "9"}}}`,
		"secret connection":    `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "x-knobs-secret": true, "x-knobs-connection": true}}}`,
		"bad env name":         `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "x-knobs-env": "lower"}}}`,
		"required unknown":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "required": ["q"], "properties": {"p": {"type": "string"}}}`,
		"enum not list":        `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "enum": "a"}}}`,
		"flag not boolean":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "x-knobs-secret": "yes"}}}`,
		"min not number":       `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "minimum": "1"}}}`,
		"length not integer":   `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "minLength": 1.5}}}`,
		"module keyword":       `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {}, "patternProperty": {}}`,
	} {
		t.Run(name, func(t *testing.T) {
			e := engine(t)
			wantCode(t, e.Attach(Module{Schema: []byte(s)}), CodeSchemaInvalid)
		})
	}
}

func TestAttachRejectsBadApplicationSchemas(t *testing.T) {
	for name, s := range map[string]string{
		"open":              `{"type": "object", "properties": {"m": {"type": "object", "additionalProperties": false, "properties": {}}}}`,
		"empty owner":       `{"x-knobs-owner": "", "type": "object", "additionalProperties": false, "properties": {"m": {"type": "object", "additionalProperties": false, "properties": {}}}}`,
		"no modules":        `{"type": "object", "additionalProperties": false, "properties": {}}`,
		"module not object": `{"type": "object", "additionalProperties": false, "properties": {"m": true}}`,
		"reserved module":   `{"type": "object", "additionalProperties": false, "properties": {"knobs": {"type": "object", "additionalProperties": false, "properties": {}}}}`,
		"open module":       `{"type": "object", "additionalProperties": false, "properties": {"m": {"type": "object", "properties": {}}}}`,
		"no properties":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false}`,
		"nested module":     `{"type": "object", "additionalProperties": false, "properties": {"m": {"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {}}}}`,
		"knob keyword":      `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "x-knobs-secret": true, "properties": {}}`,
		"nested owner":      `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"o": {"x-knobs-owner": "x", "type": "object", "additionalProperties": false, "properties": {}}}}`,
		"nested bad name":   `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"o": {"type": "object", "additionalProperties": false, "properties": {"Bad": {}}}}}`,
		"metaschema":        `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"minLength": -1}}}`,
		"ref to a file":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"$ref": "file:///etc/passwd"}}}`,
		"yaml not object":   "- a\n- b\n",
		"yaml":              ":\n  - [",
		"trailing json":     `{"x-knobs-module": "m"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			wantCode(t, engine(t).Attach(Module{Schema: []byte(s)}), CodeSchemaInvalid)
		})
	}
}

func TestApplicationSchemaOwnerAndConflict(t *testing.T) {
	app := `x-knobs-owner: example.com/app
type: object
additionalProperties: false
properties:
  http:
    type: object
    additionalProperties: false
    properties:
      port: {type: integer}
  db:
    x-knobs-owner: example.com/db
    type: object
    additionalProperties: false
    properties:
      url: {type: string}
`
	e := engine(t, app)
	mods := e.Schema()["properties"].(map[string]any)["demo"].(map[string]any)["properties"].(map[string]any)
	if mods["http"].(map[string]any)["x-knobs-owner"] != "example.com/app" || mods["db"].(map[string]any)["x-knobs-owner"] != "example.com/db" {
		t.Fatalf("owners: %v", mods)
	}
	wantCode(t, e.Attach(Module{Schema: []byte(httpSchema)}), CodeModuleConflict)
	if _, ok := e.set.modules["http"]; !ok || len(e.set.sources) != 1 {
		t.Fatal("a conflicting schema attaches none of its modules")
	}
}

// Schemas the subset parser before #12 rejected are valid JSON Schema, and load.
func TestAttachAcceptsEveryKeyword(t *testing.T) {
	for name, p := range map[string]string{
		"array":                `{"type": "array", "items": {"type": "string"}}`,
		"minimum on a string":  `{"type": "string", "minimum": 1}`,
		"maxLength on integer": `{"type": "integer", "maxLength": 1}`,
		"enum of another type": `{"type": "integer", "enum": ["a"]}`,
		"pattern":              `{"type": "string", "pattern": "^[a-z]+$"}`,
	} {
		t.Run(name, func(t *testing.T) {
			engine(t, `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "patternProperties": {}, "properties": {"p": `+p+`}}`)
		})
	}
}

// testdata/keel-schema.yaml is keelcore/keel pkg/config/schema.yaml (f7991c0), unchanged: a JSON
// Schema draft 2019-09 application schema with nested objects and arrays (issue #12).
func keelEngine(t *testing.T, layers ...Layer) *Engine {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "keel-schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New("keel")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Attach(Module{Schema: data}); err != nil {
		t.Fatalf("keel's schema does not attach: %v", err)
	}
	if err := e.Add(layers...); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestKeelSchemaResolvesEveryLayer(t *testing.T) {
	global := yamlFile(t, `keel:
  listeners:
    http:
      port: 8080
  sidecar:
    header_policy:
      forward: [x-request-id]
    retry:
      dial_attempts: 3
`)
	env := envOf(map[string]string{
		"KEEL_SIDECAR_HEADER_POLICY_STRIP": "cookie, authorization",
		"KEEL_LISTENERS_HTTP_ENABLED":      "true",
	})
	e := keelEngine(t, Defaults(), File(LayerGlobal, global, true), Env(env),
		CLI([]string{"keel.sidecar.retry.dial_attempts=5"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]Value{
		"keel.listeners.http.port":           {Value: int64(8080), Layer: LayerGlobal},
		"keel.listeners.http.enabled":        {Value: true, Layer: LayerEnv},
		"keel.sidecar.header_policy.forward": {Value: []any{"x-request-id"}, Layer: LayerGlobal},
		"keel.sidecar.header_policy.strip":   {Value: []any{"cookie", "authorization"}, Layer: LayerEnv},
		"keel.sidecar.retry.dial_attempts":   {Value: int64(5), Layer: LayerCLI},
	} {
		got, ok := c.Get(key)
		if !ok || !reflect.DeepEqual(got.Value, want.Value) || got.Layer != want.Layer {
			t.Errorf("%s = %#v from %q, want %#v from %q", key, got.Value, got.Layer, want.Value, want.Layer)
		}
	}
	if n, err := c.Int("sidecar", "retry.dial_attempts"); err != nil || n != 5 {
		t.Fatalf("Int = %d, %v", n, err)
	}
	if l, err := c.List("sidecar", "header_policy.forward"); err != nil || len(l) != 1 {
		t.Fatalf("List = %v, %v", l, err)
	}
}

func TestKeelSchemaRulesHold(t *testing.T) {
	for name, tc := range map[string]struct {
		knob, code string
	}{
		"minimum":    {"keel.sidecar.retry.dial_attempts=0", CodeOutOfRange},
		"enum":       {"keel.audit.remote_sink.protocol=smtp", CodeNotInEnum},
		"type":       {"keel.listeners.http.port=eighty", CodeInvalidType},
		"array item": {"keel.sidecar.header_policy.forward=[1]", CodeInvalidType},
		"nested key": {"keel.sidecar.retry.bogus=1", CodeUnknownKey},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := keelEngine(t, CLI([]string{tc.knob})).Resolve()
			wantCode(t, err, tc.code)
		})
	}
}

func TestKeelSchemaOwnFileHoldsModulesDirectly(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "keel-schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	own := yamlFile(t, "listeners:\n  http:\n    port: 9090\n")
	e, _ := New("keel")
	if err := e.Attach(Module{Schema: data, GlobalPath: own, Code: map[string]any{"tls.acme.cache_dir": "/var/cache/keel"}}); err != nil {
		t.Fatal(err)
	}
	e.Add(Code(nil), File(LayerGlobal, "", false))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get("keel.listeners.http.port"); v.Value != int64(9090) || v.Layer != LayerGlobal {
		t.Fatalf("own file: %+v", v)
	}
	if v, _ := c.Get("keel.tls.acme.cache_dir"); v.Value != "/var/cache/keel" || v.Layer != LayerCode {
		t.Fatalf("code keyed <module>.<name>: %+v", v)
	}
	if c.Dump()["keel"].(map[string]any)["listeners"].(map[string]any)["http"].(map[string]any)["port"] != int64(9090) {
		t.Fatalf("dump does not nest: %v", c.Dump())
	}
}

func TestOwnerMustBeAString(t *testing.T) {
	bad := strings.Replace(ownedSchema, `"x-knobs-owner": "example.com/keel"`, `"x-knobs-owner": 5`, 1)
	e, _ := New("demo")
	wantCode(t, e.Attach(Module{Schema: []byte(bad)}), CodeSchemaInvalid)
}
