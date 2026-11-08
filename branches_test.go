package knobs

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCoerceText(t *testing.T) {
	for _, tc := range []struct {
		typ, text string
		want      any
		code      string
	}{
		{TypeInteger, "42", int64(42), ""},
		{TypeInteger, "-7", int64(-7), ""},
		{TypeInteger, "99999999999999999999", nil, CodeInvalidType},
		{TypeInteger, "4.2", nil, CodeInvalidType},
		{TypeNumber, "4.25", 4.25, ""},
		{TypeNumber, "1e3", 1000.0, ""},
		{TypeNumber, "1e999", nil, CodeInvalidType},
		{TypeNumber, "four", nil, CodeInvalidType},
		{TypeBoolean, "true", true, ""},
		{TypeBoolean, "false", false, ""},
		{TypeBoolean, "TRUE", nil, CodeInvalidType},
		{TypeString, "x", "x", ""},
		{TypeDuration, "1h30m", "1h30m", ""},
		{TypeDuration, "90", nil, CodeInvalidType},
	} {
		got, code := coerceText(&property{typ: tc.typ}, tc.text)
		if got != tc.want || code != tc.code {
			t.Fatalf("%s %q: got %v %q, want %v %q", tc.typ, tc.text, got, code, tc.want, tc.code)
		}
	}
}

func TestCoerceTypedNumbers(t *testing.T) {
	integer, number := &property{typ: TypeInteger}, &property{typ: TypeNumber}
	for _, tc := range []struct {
		p    *property
		in   any
		want any
		ok   bool
	}{
		{integer, int(5), int64(5), true},
		{integer, uint64(5), int64(5), true},
		{integer, uint64(math.MaxUint64), nil, false},
		{integer, 5.0, int64(5), true},
		{integer, 5.5, nil, false},
		{integer, "5", nil, false},
		{number, int(5), 5.0, true},
		{number, int64(5), 5.0, true},
		{number, uint64(5), 5.0, true},
		{number, math.NaN(), nil, false},
		{number, math.Inf(1), nil, false},
		{number, true, nil, false},
	} {
		got, code := coerceTyped(tc.p, tc.in)
		if (code == "") != tc.ok || (tc.ok && got != tc.want) {
			t.Fatalf("%s %v: got %v %q", tc.p.typ, tc.in, got, code)
		}
	}
}

func TestEnumComparesNumbersByValue(t *testing.T) {
	p := &property{typ: TypeNumber, enum: []any{int64(1), 2.5}}
	if !inEnum(p, 1.0) || !inEnum(p, 2.5) || inEnum(p, 3.0) || inEnum(p, "1") {
		t.Fatal("enum membership")
	}
}

func TestZeroSecretRevealsEmpty(t *testing.T) {
	if (Secret{}).Reveal() != "" {
		t.Fatal("a zero Secret holds nothing")
	}
}

func TestSchemaShapeErrors(t *testing.T) {
	for name, s := range map[string]string{
		"properties not object": `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": []}`,
		"required not list":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "required": "p", "properties": {"p": {"type": "string"}}}`,
		"enum wrong type":       `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "enum": ["a"]}}}`,
		"top level not object":  `[1]`,
	} {
		t.Run(name, func(t *testing.T) {
			wantCode(t, engine(t).Attach([]byte(s), "", ""), CodeSchemaInvalid)
		})
	}
}

func TestModuleFileParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.yaml")
	if err := os.WriteFile(path, []byte("port: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _ := New("demo")
	if err := e.Attach([]byte(httpSchema), path, ""); err != nil {
		t.Fatal(err)
	}
	e.Add(Defaults(), File(LayerGlobal, "", false))
	_, err := e.Resolve()
	wantCode(t, err, CodeFileParse)
}

func TestAddStandardSources(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user.yaml")
	if err := os.WriteFile(user, []byte("demo:\n  http:\n    host: 10.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := engine(t, httpSchema)
	err := e.AddStandard(Standard{
		UserPath: user,
		Knobs:    []string{"demo.http.debug=true"},
		Runtime:  map[string]string{"demo.http.port": "7000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	for key, layer := range map[string]string{"demo.http.host": LayerUser, "demo.http.debug": LayerCLI, "demo.http.port": LayerRuntime} {
		if v, _ := c.Get(key); v.Layer != layer {
			t.Fatalf("%s from %s, want %s", key, v.Layer, layer)
		}
	}
	wantCode(t, e.AddStandard(Standard{}), CodeInvalidArgument)
}

const optionalSchema = `{
  "x-knobs-module": "opt",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "limit": {"type": "integer", "x-knobs-connection": true},
    "wait": {"type": "string", "x-knobs-type": "duration"},
    "pin": {"type": "integer"}
  }
}`

func TestConnectionOverUnsetKey(t *testing.T) {
	e := engine(t, optionalSchema)
	e.Add(Defaults())
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	r, _ := c.WithConnection(Connection{Values: map[string]string{"demo.opt.limit": "3"}, Authorized: true})
	if vs := r.Values(); len(vs) != 1 || vs[0].Key != "demo.opt.limit" {
		t.Fatalf("a connection value with no process value is listed: %+v", vs)
	}
	// A connection built from a connection starts again from the process configuration.
	again, _ := r.WithConnection(Connection{})
	if _, ok := again.Get("demo.opt.limit"); ok {
		t.Fatal("connections do not stack")
	}
}

func TestBindUnsetAndOverflow(t *testing.T) {
	e := engine(t, optionalSchema)
	e.Add(Defaults(), Runtime(map[string]string{"demo.opt.wait": "99999999999h"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	pinned := struct {
		Pin int64 `knobs:"opt.pin"`
	}{Pin: 9}
	if err := c.Bind(&pinned); err != nil || pinned.Pin != 9 {
		t.Fatalf("an unset key leaves its field: %v %d", err, pinned.Pin)
	}
	var waits struct {
		Wait time.Duration `knobs:"opt.wait"`
	}
	if err := c.Bind(&waits); err == nil {
		t.Fatal("a duration beyond time.Duration fails")
	}
}

func TestSecretOfNonString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, []byte("demo:\n  http:\n    port: 9000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := engine(t, httpSchema)
	e.Add(Defaults(), File(LayerSecrets, path, true))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.Secret("http", "port")
	if err != nil || s.Reveal() != "9000" {
		t.Fatalf("a secret integer reveals as text: %v %q", err, s.Reveal())
	}
}
