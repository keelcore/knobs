package knobs

import (
	"reflect"
	"testing"
)

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
