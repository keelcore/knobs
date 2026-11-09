package knobs

import (
	"reflect"
	"strings"
	"testing"
)

// The tests of schemas and values through the JSON Schema validator (issue #12).

const nestedSchema = `{
  "x-knobs-module": "svc",
  "type": "object",
  "additionalProperties": false,
  "dependentRequired": {"user": ["pass"]},
  "properties": {
    "user": {"type": "string"},
    "pass": {"type": "string"},
    "tags": {"type": "array", "items": {"type": "string"}, "minItems": 1},
    "ports": {"type": "array", "items": {"type": "integer"}},
    "loose": {"type": "array"},
    "labels": {"type": "object", "additionalProperties": false, "patternProperties": {"^[a-z]+$": {"type": "string"}}},
    "extra": {},
    "step": {"type": "number", "minimum": 0, "exclusiveMaximum": 10, "x-knobs-connection": true},
    "db": {
      "type": "object",
      "additionalProperties": false,
      "required": ["tls"],
      "properties": {
        "tls": {"type": "object", "additionalProperties": false, "required": ["ca"], "properties": {"ca": {"type": "string"}}}
      }
    }
  }
}`

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

func TestNestedValuesThroughEveryLayer(t *testing.T) {
	file := yamlFile(t, "demo:\n  svc:\n    tags: [a]\n    labels: {env: prod}\n    extra: 3\n    db: {tls: {ca: x}}\n")
	e := engine(t, nestedSchema)
	e.Add(File(LayerGlobal, file, true), Env(envOf(map[string]string{
		"DEMO_SVC_PORTS": "80, ,443",
		"DEMO_SVC_LOOSE": "1,two",
	})), CLI([]string{`demo.svc.labels={"team":"core"}`, `demo.svc.extra=plain text`}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"demo.svc.tags":      []any{"a"},
		"demo.svc.ports":     []any{int64(80), int64(443)},
		"demo.svc.loose":     []any{int64(1), "two"},
		"demo.svc.labels":    map[string]any{"team": "core"},
		"demo.svc.extra":     "plain text",
		"demo.svc.db.tls.ca": "x",
	} {
		if v, _ := c.Get(key); !reflect.DeepEqual(v.Value, want) {
			t.Errorf("%s = %#v, want %#v", key, v.Value, want)
		}
	}
	if _, err := c.List("svc", "user"); err == nil {
		t.Fatal("List of an unset key fails")
	}
	if _, err := c.List("svc", "db.tls.ca"); err == nil {
		t.Fatal("List of a string fails")
	}
}

func TestValueErrorCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		layer Layer
		code  string
	}{
		"array text not json":   {CLI([]string{"demo.svc.tags=[a"}), CodeInvalidType},
		"array item":            {CLI([]string{"demo.svc.ports=1,x"}), CodeInvalidType},
		"array too short":       {CLI([]string{"demo.svc.tags=[]"}), CodeInvalidLength},
		"object text":           {CLI([]string{"demo.svc.labels=x"}), CodeInvalidType},
		"object extra key":      {CLI([]string{`demo.svc.labels={"Bad":"x"}`}), CodeUnknownKey},
		"exclusive bound":       {CLI([]string{"demo.svc.step=10"}), CodeOutOfRange},
		"object from code":      {Code(map[string]any{"demo.svc.labels": map[string]string{"env": "x"}}), ""},
		"code value of no type": {Code(map[string]any{"demo.svc.extra": struct{}{}}), CodeInvalidType},
		"array of no type":      {Code(map[string]any{"demo.svc.loose": []any{struct{}{}}}), CodeInvalidType},
		"object of no type":     {Code(map[string]any{"demo.svc.labels": map[string]any{"a": struct{}{}}}), CodeInvalidType},
		"dependent required":    {CLI([]string{"demo.svc.user=u"}), CodeMissingRequired},
		"nested required":       {Code(map[string]any{"demo.svc.db.tls.ca": 1}), CodeInvalidType},
	} {
		t.Run(name, func(t *testing.T) {
			e := engine(t, nestedSchema)
			e.Add(Code(map[string]any{"demo.svc.db.tls.ca": "x"}), tc.layer)
			_, err := e.Resolve()
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			wantCode(t, err, tc.code)
		})
	}
}

func TestWholeModuleErrorsNameTheirKey(t *testing.T) {
	e := engine(t, nestedSchema)
	e.Add(Code(map[string]any{"demo.svc.user": "u"}))
	_, err := e.Resolve()
	var ke *Error
	if !asError(err, &ke) || ke.Code != CodeMissingRequired || ke.Key != "demo.svc.pass" || ke.Layer != "resolve" {
		t.Fatalf("want missing_required on demo.svc.pass (dependentRequired), got %v", err)
	}
	e = engine(t, nestedSchema)
	e.Add(Code(map[string]any{"demo.svc.user": "u", "demo.svc.pass": "p", "demo.svc.db.tls.ca": "x"}))
	if _, err := e.Resolve(); err != nil {
		t.Fatal(err)
	}
	m := e.set.modules["svc"]
	if got := m.pathOf([]string{"tags", "0"}); got != "tags" {
		t.Fatalf("pathOf into an array = %q", got)
	}
	if got := m.pathOf([]string{"db", "tls"}); got != "db.tls" {
		t.Fatalf("pathOf a nested object = %q", got)
	}
}

func TestConnectionBoundBreakingARuleIsMalformed(t *testing.T) {
	e := engine(t, nestedSchema)
	e.Add(Code(map[string]any{"demo.svc.db.tls.ca": "x"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	_, attempts := c.WithConnection(Connection{Values: map[string]string{"demo.svc.step": "12"}, Authorized: true})
	if attempts[0].Outcome != OutcomeRejectedMalformed {
		t.Fatalf("clamped to an exclusive maximum: %v", attempts[0].Outcome)
	}
}

func TestNormalizeValue(t *testing.T) {
	type label string
	type flag bool
	for _, tc := range []struct{ in, want any }{
		{float32(1.5), 1.5},
		{label("x"), "x"},
		{flag(true), true},
		{[2]int{1, 2}, []any{int64(1), int64(2)}},
		{map[int]string{1: "a"}, map[int]string{1: "a"}},
		{uint64(1 << 63), uint64(1 << 63)},
		{struct{}{}, struct{}{}},
	} {
		if got := normalizeValue(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("normalizeValue(%#v) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
	if f, ok := asFloat(uint64(1 << 63)); !ok || f != 1<<63 {
		t.Fatal("asFloat of an integer past int64")
	}
	if _, err := decodeJSON([]byte(`1 2`)); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing data: %v", err)
	}
}

func asError(err error, target **Error) bool {
	e, ok := err.(*Error)
	*target = e
	return ok
}

func TestDependenciesKeyword(t *testing.T) {
	deps := `{
  "$schema": "https://json-schema.org/draft/2019-09/schema",
  "x-knobs-module": "old",
  "type": "object",
  "additionalProperties": false,
  "dependencies": {"user": ["pass"]},
  "properties": {"user": {"type": "string"}, "pass": {"type": "string"}}
}`
	e := engine(t, deps)
	e.Add(CLI([]string{"demo.old.user=u"}))
	_, err := e.Resolve()
	var ke *Error
	if !asError(err, &ke) || ke.Code != CodeMissingRequired || ke.Key != "demo.old.pass" {
		t.Fatalf("dependencies: %v", err)
	}
}

func TestUnrecognizedKeywords(t *testing.T) {
	for name, tc := range map[string]struct {
		schema string
		want   []Keyword
	}{
		"draft 2020-12 by default": {
			`{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "maxlength": 3}}}`,
			[]Keyword{{"maxlength", "#/properties/p"}},
		},
		"a nested typo hides its ancestors' until fixed": {
			`{"x-knobs-module": "m", "minimun": 1, "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "patern": "x"}}}`,
			[]Keyword{{"patern", "#/properties/p"}},
		},
		"a typo with none under it": {
			`{"x-knobs-module": "m", "minimun": 1, "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string"}}}`,
			[]Keyword{{"minimun", "#"}},
		},
		"location tokens escaped": {
			`{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "object", "patternProperties": {"^a/~b$": {"maxium": 1}}}}}`,
			[]Keyword{{"maxium", "#/properties/p/patternProperties/^a~1~0b$"}},
		},
		"another extension": {
			`{"x-knobs-module": "m", "x-go-type": "T", "type": "object", "additionalProperties": false, "properties": {}}`,
			[]Keyword{{"x-go-type", "#"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := engine(t).Attach(Module{Schema: []byte(tc.schema)})
			var ke *Error
			if !asError(err, &ke) || ke.Code != CodeSchemaInvalid || !reflect.DeepEqual(ke.Keywords, tc.want) {
				t.Fatalf("got %v, keywords %v; want %v", err, ke.Keywords, tc.want)
			}
		})
	}
}

func TestOnlyStrictDraftsLoad(t *testing.T) {
	for _, draft := range []string{"http://json-schema.org/draft-07/schema#", "http://json-schema.org/draft-04/schema#"} {
		s := `{"$schema": "` + draft + `", "x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {}}`
		wantCode(t, engine(t).Attach(Module{Schema: []byte(s)}), CodeSchemaInvalid)
	}
	engine(t, `{"$schema": "https://json-schema.org/draft/2019-09/schema#", "x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {}}`)
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
