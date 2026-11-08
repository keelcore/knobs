package knobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const httpSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "x-knobs-module": "http",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "port": {"type": "integer", "default": 8080, "minimum": 1, "maximum": 65535, "x-knobs-connection": true},
    "ratio": {"type": "number", "default": 0.5, "minimum": 0, "maximum": 1},
    "debug": {"type": "boolean", "default": false},
    "host": {"type": "string", "default": "127.0.0.1"},
    "timeout": {"type": "string", "x-knobs-type": "duration", "default": "5s"}
  }
}`

const dbSchema = `{
  "x-knobs-module": "db",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "password": {"type": "string", "x-knobs-secret": true}
  }
}`

func engine(t testing.TB, schemas ...string) *Engine {
	t.Helper()
	e, err := New("demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range schemas {
		if err := e.Attach([]byte(s), "", ""); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != code {
		t.Fatalf("want error code %s, got %v", code, err)
	}
}

func TestNewRejectsBadAppName(t *testing.T) {
	for _, app := range []string{"", "Demo", "1demo", "de-mo"} {
		_, err := New(app)
		wantCode(t, err, CodeInvalidArgument)
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
		"unknown type":         `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "array"}}}`,
		"bounds on string":     `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "string", "minimum": 1}}}`,
		"length on integer":    `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {"p": {"type": "integer", "maxLength": 1}}}`,
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
		"module keyword":       `{"x-knobs-module": "m", "type": "object", "additionalProperties": false, "properties": {}, "patternProperties": {}}`,
	} {
		t.Run(name, func(t *testing.T) {
			e := engine(t)
			wantCode(t, e.Attach([]byte(s), "", ""), CodeSchemaInvalid)
		})
	}
}

func TestAttachAfterResolveFails(t *testing.T) {
	e := engine(t, httpSchema)
	e.Add(Defaults())
	if _, err := e.Resolve(); err != nil {
		t.Fatal(err)
	}
	wantCode(t, e.Attach([]byte(dbSchema), "", ""), CodeInvalidArgument)
	wantCode(t, e.Add(Env(nil)), CodeInvalidArgument)
}

func TestTypedAccessors(t *testing.T) {
	e := engine(t, httpSchema, dbSchema)
	e.Add(Defaults())
	e.Add(Env(func(name string) (string, bool) {
		if name == "DEMO_DB_PASSWORD" {
			return "hunter2", true
		}
		return "", false
	}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if n, err := c.Int("http", "port"); err != nil || n != 8080 {
		t.Fatalf("Int = %d, %v", n, err)
	}
	if f, err := c.Float("http", "ratio"); err != nil || f != 0.5 {
		t.Fatalf("Float = %v, %v", f, err)
	}
	if f, err := c.Float("http", "port"); err != nil || f != 8080 {
		t.Fatalf("Float of an integer = %v, %v", f, err)
	}
	if b, err := c.Bool("http", "debug"); err != nil || b {
		t.Fatalf("Bool = %v, %v", b, err)
	}
	if s, err := c.String("http", "host"); err != nil || s != "127.0.0.1" {
		t.Fatalf("String = %q, %v", s, err)
	}
	if d, err := c.Duration("http", "timeout"); err != nil || d != 5*time.Second {
		t.Fatalf("Duration = %v, %v", d, err)
	}
	s, err := c.Secret("db", "password")
	if err != nil || s.Reveal() != "hunter2" {
		t.Fatalf("Secret = %v, %v", s, err)
	}
	if _, err := c.String("db", "password"); err == nil {
		t.Fatal("String of a secret must fail: read it with Secret")
	}
	if _, err := c.Secret("http", "host"); err == nil {
		t.Fatal("Secret of a plain value must fail")
	}
	for _, f := range []func() error{
		func() error { _, err := c.Int("http", "host"); return err },
		func() error { _, err := c.Float("http", "host"); return err },
		func() error { _, err := c.Bool("http", "host"); return err },
		func() error { _, err := c.String("http", "port"); return err },
		func() error { _, err := c.Duration("http", "host"); return err },
		func() error { _, err := c.Int("http", "nope"); return err },
	} {
		if f() == nil {
			t.Fatal("an accessor of the wrong type or an unset key must fail")
		}
	}
	if v, ok := c.Get("demo.http.port"); !ok || v.Layer != "default" || v.Secret {
		t.Fatalf("Get = %+v, %v", v, ok)
	}
	if _, ok := c.Get("demo.http.nope"); ok {
		t.Fatal("Get of an unknown key must report false")
	}
}

func TestSecretNeverPrints(t *testing.T) {
	s := NewSecret("hunter2")
	for _, out := range []string{
		s.String(), s.GoString(), fmt.Sprint(s), fmt.Sprintf("%v %+v %#v %s %q %x", s, s, s, s, s, s),
	} {
		if strings.Contains(out, "hunter2") || strings.Contains(out, fmt.Sprintf("%x", "hunter2")) {
			t.Fatalf("secret printed: %s", out)
		}
	}
	b, err := json.Marshal(struct{ P Secret }{s})
	if err != nil || strings.Contains(string(b), "hunter2") {
		t.Fatalf("secret marshalled: %s %v", b, err)
	}
	var back Secret
	if err := json.Unmarshal([]byte(`"x"`), &back); err == nil {
		t.Fatal("a secret must not be read from JSON")
	}
	if s.Reveal() != "hunter2" {
		t.Fatal("Reveal must return the value")
	}
}

func TestErrorsNeverCarrySecretValues(t *testing.T) {
	e := engine(t, dbSchema)
	e.Add(Defaults())
	e.Add(CLI([]string{"demo.db.password=hunter2"}))
	_, err := e.Resolve()
	wantCode(t, err, CodeSecretOnCLI)
	if strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("error carries the secret: %v", err)
	}
}

func TestDefaultFileLocationsAreOptional(t *testing.T) {
	e := engine(t, httpSchema)
	e.Add(Defaults())
	e.Add(File(LayerGlobal, filepath.Join(t.TempDir(), "absent.yaml"), false))
	if _, err := e.Resolve(); err != nil {
		t.Fatalf("a missing file at its default location is skipped: %v", err)
	}
}

func TestFileUnreadable(t *testing.T) {
	e := engine(t, httpSchema)
	e.Add(File(LayerGlobal, t.TempDir(), true))
	_, err := e.Resolve()
	wantCode(t, err, CodeFileParse)
}

func TestEmptyFileIsEmptyLayer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.yaml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	e := engine(t, httpSchema)
	e.Add(Defaults())
	e.Add(File(LayerUser, path, true))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get("demo.http.port"); v.Layer != "default" {
		t.Fatalf("got %+v", v)
	}
}

func TestFileRootNotMapping(t *testing.T) {
	for _, body := range []string{"- 1\n", "demo: 5\n", "demo:\n  http:\n    - 1\n"} {
		path := filepath.Join(t.TempDir(), "f.yaml")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		e := engine(t, httpSchema)
		e.Add(File(LayerGlobal, path, true))
		_, err := e.Resolve()
		var ke *Error
		if !errors.As(err, &ke) {
			t.Fatalf("%q: want an error, got %v", body, err)
		}
	}
}

func TestStandardPaths(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.yaml")
	if err := os.WriteFile(global, []byte("demo:\n  http:\n    port: 9000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"DEMO_KNOBS_GLOBAL_PATH": global, "XDG_CONFIG_HOME": dir, "HOME": dir}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	e := engine(t, httpSchema)
	if err := e.AddStandard(Standard{Env: lookup}); err != nil {
		t.Fatal(err)
	}
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get("demo.http.port"); v.Layer != "global" {
		t.Fatalf("the global path comes from env: %+v", v)
	}
	// cli beats env for the path knobs; an explicit path that is missing fails.
	e = engine(t, httpSchema)
	if err := e.AddStandard(Standard{Env: lookup, Knobs: []string{"demo.knobs.global_path=" + filepath.Join(dir, "absent.yaml")}}); err != nil {
		t.Fatal(err)
	}
	_, err = e.Resolve()
	wantCode(t, err, CodeFileNotFound)
}

func TestStandardDefaultPaths(t *testing.T) {
	g, u := DefaultGlobalPath("demo"), DefaultUserPath("demo", func(k string) (string, bool) {
		if k == "HOME" {
			return "/home/x", true
		}
		return "", false
	})
	if g != "/etc/demo/config.yaml" || u != "/home/x/.config/demo/config.yaml" {
		t.Fatalf("default paths: %s %s", g, u)
	}
	if u := DefaultUserPath("demo", func(k string) (string, bool) { return "/xdg", k == "XDG_CONFIG_HOME" }); u != "/xdg/demo/config.yaml" {
		t.Fatalf("XDG path: %s", u)
	}
	if u := DefaultUserPath("demo", func(string) (string, bool) { return "", false }); u != "" {
		t.Fatalf("no home: %q", u)
	}
	e := engine(t, httpSchema)
	if err := e.AddStandard(Standard{Knobs: []string{"bad"}}); err == nil {
		t.Fatal("a malformed --knob fails AddStandard")
	}
}

func TestWithConnection(t *testing.T) {
	e := engine(t, httpSchema)
	e.Add(Defaults())
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"demo.http.port": "9000"}, Authorized: true})
	if n, _ := r.Int("http", "port"); n != 9000 || len(attempts) != 1 || attempts[0].Outcome != OutcomeApplied {
		t.Fatalf("connection: %d %+v", n, attempts)
	}
	if v, _ := r.Get("demo.http.port"); v.Layer != LayerConnection {
		t.Fatalf("layer %s", v.Layer)
	}
	if n, _ := c.Int("http", "port"); n != 8080 {
		t.Fatalf("WithConnection must not change the process config: %d", n)
	}
	for o, want := range map[Outcome]string{
		OutcomeApplied: "applied", OutcomeClamped: "clamped", OutcomeRejectedUnauthorized: "rejected_unauthorized",
		OutcomeRejectedNotOverridable: "rejected_not_overridable", OutcomeRejectedMalformed: "rejected_malformed",
		Outcome(99): "unknown",
	} {
		if o.String() != want {
			t.Fatalf("%d: %s", o, o.String())
		}
	}
}

func TestDumpAndSchema(t *testing.T) {
	e := engine(t, httpSchema, dbSchema)
	e.Add(Defaults())
	e.Add(Runtime(map[string]string{"demo.db.password": "hunter2"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	dump := c.Dump()["demo"].(map[string]any)["db"].(map[string]any)
	if dump["password"] != Redacted {
		t.Fatalf("dump: %v", dump)
	}
	s, err := json.Marshal(e.Schema())
	if err != nil || !strings.Contains(string(s), `"http"`) {
		t.Fatalf("schema: %s %v", s, err)
	}
	if len(c.Values()) != 6 {
		t.Fatalf("values: %+v", c.Values())
	}
}

type httpBinding struct {
	Port    int64         `knobs:"http.port"`
	Ratio   float64       `knobs:"http.ratio"`
	Debug   bool          `knobs:"http.debug"`
	Host    string        `knobs:"http.host"`
	Timeout time.Duration `knobs:"http.timeout"`
	Skip    string
}

type dbBinding struct {
	Password Secret `knobs:"db.password"`
}

func TestBind(t *testing.T) {
	e := engine(t, httpSchema, dbSchema)
	e.Add(Defaults())
	e.Add(Runtime(map[string]string{"demo.db.password": "hunter2"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var h httpBinding
	var d dbBinding
	if err := c.Bind(&h); err != nil {
		t.Fatal(err)
	}
	if err := c.Bind(&d); err != nil {
		t.Fatal(err)
	}
	if h.Port != 8080 || h.Ratio != 0.5 || h.Debug || h.Host != "127.0.0.1" || h.Timeout != 5*time.Second {
		t.Fatalf("bound %+v", h)
	}
	if d.Password.Reveal() != "hunter2" {
		t.Fatal("secret not bound")
	}
	for _, bad := range []any{
		h, nil, &struct {
			P string `knobs:"http.port"`
		}{}, &struct {
			P int64 `knobs:"http.nope"`
		}{}, &struct {
			P int64 `knobs:"nodot"`
		}{}, &struct {
			P []int `knobs:"http.port"`
		}{},
	} {
		if err := c.Bind(bad); err == nil {
			t.Fatalf("Bind(%T) must fail", bad)
		}
	}
}

func TestTableOf(t *testing.T) {
	rows, err := TableOf(&httpBinding{}, &dbBinding{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Row{
		{Key: "db.password", Type: "string", Secret: true},
		{Key: "http.debug", Type: "boolean"},
		{Key: "http.host", Type: "string"},
		{Key: "http.port", Type: "integer"},
		{Key: "http.ratio", Type: "number"},
		{Key: "http.timeout", Type: "duration"},
	}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("rows %v\nwant %v", rows, want)
	}
	if _, err := TableOf(httpBinding{}); err == nil {
		t.Fatal("TableOf needs pointers to structs")
	}
	if _, err := TableOf(&struct {
		P []int `knobs:"m.p"`
	}{}); err == nil {
		t.Fatal("TableOf rejects unsupported field types")
	}
}

func TestLayerNames(t *testing.T) {
	for _, l := range []Layer{Defaults(), File(LayerGlobal, "x", false), File(LayerSecrets, "x", false), Env(nil), CLI(nil), Runtime(nil)} {
		if l.Name() == "" {
			t.Fatalf("%T has no name", l)
		}
	}
	if (&Error{Code: CodeUnknownKey, Layer: "cli", Key: "demo.x.y", Msg: "m"}).Error() == "" {
		t.Fatal("Error() is empty")
	}
}
