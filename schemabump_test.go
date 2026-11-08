package knobs

import (
	"reflect"
	"testing"
)

const bumpOld = `{
  "type": "object",
  "properties": {
    "demo": {"type": "object", "properties": {
      "http": {"type": "object", "properties": {"port": {"type": "integer"}, "host": {"type": "string"}}},
      "debug": {"type": "boolean"}
    }}
  }
}`

func TestSchemaFieldsWalksLeaves(t *testing.T) {
	got, err := SchemaFields([]byte(bumpOld))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"demo.debug", "demo.http.host", "demo.http.port"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields %v, want %v", got, want)
	}
}

func TestSchemaFieldsReadsYAML(t *testing.T) {
	yamlSchema := "type: object\nproperties:\n  tls:\n    type: object\n    properties:\n      cert: {type: string}\n  port: {type: integer}\n"
	got, err := SchemaFields([]byte(yamlSchema))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"port", "tls.cert"}) {
		t.Fatalf("fields %v", got)
	}
}

func TestSchemaFieldsOddShapes(t *testing.T) {
	for _, s := range []string{`{}`, `{"properties": []}`, `{"properties": {"a": 1}}`} {
		if _, err := SchemaFields([]byte(s)); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	if f, _ := SchemaFields([]byte(`{"properties": {"a": 1}}`)); !reflect.DeepEqual(f, []string{"a"}) {
		t.Fatalf("a non-object property is a leaf: %v", f)
	}
	if _, err := SchemaFields([]byte("- not\n- a mapping\n")); err == nil {
		t.Fatal("a schema that is not a mapping fails")
	}
	if _, err := SchemaFields([]byte("{")); err == nil {
		t.Fatal("a schema that does not parse fails")
	}
}

func TestSchemaBump(t *testing.T) {
	added := `{"properties": {"demo": {"properties": {
      "http": {"properties": {"port": {}, "host": {}, "timeout": {}}}, "debug": {}}}}}`
	removed := `{"properties": {"demo": {"properties": {"http": {"properties": {"port": {}}}, "debug": {}}}}}`
	for _, tc := range []struct {
		name     string
		old, new []byte
		level    string
		subject  string
		removed  []string
		added    []string
	}{
		{"no change", []byte(bumpOld), []byte(bumpOld), "patch", "internal improvements; no config surface changes", nil, nil},
		{"added", []byte(bumpOld), []byte(added), "minor", "added 1 config field(s)", nil, []string{"demo.http.timeout"}},
		{"removed", []byte(bumpOld), []byte(removed), "major", "removed 1 config field(s)", []string{"demo.http.host"}, nil},
		{"no previous schema", nil, []byte(bumpOld), "minor", "added 3 config field(s)", nil,
			[]string{"demo.debug", "demo.http.host", "demo.http.port"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := SchemaBump(tc.old, tc.new)
			if err != nil {
				t.Fatal(err)
			}
			if b.Level != tc.level || b.Subject() != tc.subject || !reflect.DeepEqual(b.Removed, tc.removed) ||
				!reflect.DeepEqual(b.Added, tc.added) {
				t.Fatalf("got %+v %q", b, b.Subject())
			}
		})
	}
	if _, err := SchemaBump([]byte("{"), []byte(bumpOld)); err == nil {
		t.Fatal("an old schema that does not parse fails")
	}
	if _, err := SchemaBump([]byte(bumpOld), []byte("{")); err == nil {
		t.Fatal("a new schema that does not parse fails")
	}
}
