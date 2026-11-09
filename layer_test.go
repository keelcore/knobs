package knobs

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestModuleFileParseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.yaml")
	if err := os.WriteFile(path, []byte("port: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e, _ := New("demo")
	if err := e.Attach(Module{Schema: []byte(httpSchema), GlobalPath: path}); err != nil {
		t.Fatal(err)
	}
	e.Add(Defaults(), File(LayerGlobal, "", false))
	_, err := e.Resolve()
	wantCode(t, err, CodeFileParse)
}

func TestCodeLayer(t *testing.T) {
	e := engine(t, httpSchema, dbSchema)
	e.Add(Code(map[string]any{"demo.http.port": 7000, "demo.http.host": "0.0.0.0"}), Defaults())
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	// A schema default overrides the code value for the same key.
	if v, _ := c.Get("demo.http.port"); v.Layer != LayerSchema || v.Value != int64(8080) {
		t.Fatalf("schema over code: %+v", v)
	}
	e = engine(t, optionalSchema)
	e.Add(Code(map[string]any{"demo.opt.pin": 3, "demo.opt.wait": 90 * time.Second}), Defaults())
	c, err = e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get("demo.opt.pin"); v.Layer != LayerCode || v.Value != int64(3) {
		t.Fatalf("code baseline: %+v", v)
	}
	if d, _ := c.Duration("opt", "wait"); d != 90*time.Second {
		t.Fatalf("a time.Duration in code: %v", d)
	}
	for code, values := range map[string]map[string]any{
		CodeSecretInCode: {"demo.db.password": "hunter2"},
		CodeInvalidType:  {"demo.http.port": "7000"},
		CodeUnknownKey:   {"demo.http.nope": 1},
	} {
		e := engine(t, httpSchema, dbSchema)
		e.Add(Code(values))
		_, err := e.Resolve()
		wantCode(t, err, code)
	}
	if Code(nil).Name() != LayerCode {
		t.Fatal("name")
	}
}

func TestModuleCodeErrors(t *testing.T) {
	for code, mod := range map[string]Module{
		CodeUnknownKey:   {Schema: []byte(httpSchema), Code: map[string]any{"nope": 1}},
		CodeInvalidType:  {Schema: []byte(httpSchema), Code: map[string]any{"port": "7000"}},
		CodeSecretInCode: {Schema: []byte(dbSchema), Code: map[string]any{"password": "hunter2"}},
	} {
		e, _ := New("demo")
		if err := e.Attach(mod); err != nil {
			t.Fatal(err)
		}
		e.Add(Code(nil))
		_, err := e.Resolve()
		wantCode(t, err, code)
	}
	// An explicit Code layer applies after the modules' own values.
	e, _ := New("demo")
	if err := e.Attach(Module{Schema: []byte(optionalSchema), Code: map[string]any{"pin": 1}}); err != nil {
		t.Fatal(err)
	}
	e.Add(Code(map[string]any{"demo.opt.pin": 2}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := c.Int("opt", "pin"); n != 2 {
		t.Fatalf("explicit code over module code: %d", n)
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
	if v, _ := c.Get("demo.http.port"); v.Layer != LayerSchema {
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

func yamlFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func envOf(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

// TestTierString (both resolvers): the layer names, lowest to highest.
func TestLayerNameConstants(t *testing.T) {
	want := []string{"code", "schema", "global", "user", "secrets", "env", "cli", "runtime", "connection"}
	got := []string{LayerCode, LayerSchema, LayerGlobal, LayerUser, LayerSecrets, LayerEnv, LayerCLI, LayerRuntime, LayerConnection}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("layer %d: %s, want %s", i, got[i], want[i])
		}
	}
}

// TestNilEnvSource: a nil environment lookup is tolerated.
func TestEnvNilLookup(t *testing.T) {
	c := mustGate(t, Env(nil))
	if layerOf(t, c, "demo.gate.window") != LayerSchema {
		t.Fatal("a nil env sets nothing")
	}
}

// TestStringAccessor: an empty string set in a file is a value; an empty env value is unset.
func TestEmptyValues(t *testing.T) {
	c := mustGate(t, File(LayerGlobal, yamlFile(t, "demo:\n  gate:\n    mode: \"\"\n"), true),
		Env(envOf(map[string]string{"DEMO_GATE_LOCKED": ""})))
	if s, _ := c.String("gate", "mode"); s != "" || layerOf(t, c, "demo.gate.mode") != LayerGlobal {
		t.Fatalf("empty file string: %q", s)
	}
	if layerOf(t, c, "demo.gate.locked") != LayerSchema {
		t.Fatal("an empty env value is unset")
	}
}

// TestKeyCodecs, TestKeyDerivation, TestHeaderKeyEmptySegment: the key, file and env forms
// derive from app, module and name; header names are the client's mapping.
func TestSchemasKeyForms(t *testing.T) {
	e, _ := New("demo")
	if err := e.Attach(Module{Schema: []byte(gateSchema)}); err != nil {
		t.Fatal(err)
	}
	s := &Schemas{set: e.set}
	if s.App() != "demo" || s.EnvName("gate", "window") != "DEMO_GATE_WINDOW" {
		t.Fatalf("forms: %s %s", s.App(), s.EnvName("gate", "window"))
	}
	if mod, name, ok := s.Lookup("demo.gate.window"); !ok || mod != "gate" || name != "window" {
		t.Fatal("lookup")
	}
	for _, bad := range []string{"demo.gate", "other.gate.window", "demo..window", "demo.gate.window.x"} {
		if _, _, ok := s.Lookup(bad); ok {
			t.Fatalf("%q must not resolve", bad)
		}
	}
}

// FuzzFileLayer: any file content resolves or fails with a knobs Error; it never panics.
func FuzzFileLayer(f *testing.F) {
	for _, s := range []string{"demo:\n  http:\n    port: 9000\n", "demo: 5\n", "- 1\n", "{", "demo:\n  x:\n    y: [1]\n"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "f.yaml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		e := engine(t, httpSchema, dbSchema)
		e.Add(Defaults(), File(LayerGlobal, path, true))
		if _, err := e.Resolve(); err != nil {
			var ke *Error
			if !errors.As(err, &ke) {
				t.Fatalf("not a knobs error: %v", err)
			}
		}
	})
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
