package knobs

import (
	"testing"
)

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
