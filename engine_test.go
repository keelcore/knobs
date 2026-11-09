package knobs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

// TestAttachModules: main attaches each module; every module brings its own schema, its code values (keyed
// by its own knob names) and its own files, and the engine aggregates them.
func TestAttachModules(t *testing.T) {
	dir := t.TempDir()
	optGlobal := filepath.Join(dir, "opt.yaml")
	if err := os.WriteFile(optGlobal, []byte("pin: 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	httpMod := Module{Schema: []byte(httpSchema), Code: map[string]any{"port": 7000, "host": "0.0.0.0"}}
	optMod := Module{Schema: []byte(optionalSchema), Code: map[string]any{"limit": 2, "wait": 3 * time.Second}, GlobalPath: optGlobal}
	e, _ := New("demo")
	if err := e.Attach(httpMod, optMod); err != nil {
		t.Fatal(err)
	}
	if err := e.AddStandard(Standard{}); err != nil {
		t.Fatal(err)
	}
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]Value{
		"demo.http.port": {Value: int64(8080), Layer: LayerSchema}, // a schema default overrides module code
		"demo.opt.limit": {Value: int64(2), Layer: LayerCode},      // module code is the baseline
		"demo.opt.wait":  {Value: "3s", Layer: LayerCode},
		"demo.opt.pin":   {Value: int64(5), Layer: LayerGlobal}, // the module's own file
	} {
		if v, _ := c.Get(key); v.Value != want.Value || v.Layer != want.Layer {
			t.Fatalf("%s: %+v, want %+v", key, v, want)
		}
	}
	if props := e.Schema()["properties"].(map[string]any)["demo"].(map[string]any)["properties"].(map[string]any); len(props) != 2 {
		t.Fatalf("both modules aggregate into the master schema: %v", props)
	}
}

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
		if err := e.Attach(Module{Schema: []byte(s)}); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func TestNewRejectsBadAppName(t *testing.T) {
	for _, app := range []string{"", "Demo", "1demo", "de-mo"} {
		_, err := New(app)
		wantCode(t, err, CodeInvalidArgument)
	}
}

func TestAttachAfterResolveFails(t *testing.T) {
	e := engine(t, httpSchema)
	e.Add(Defaults())
	if _, err := e.Resolve(); err != nil {
		t.Fatal(err)
	}
	wantCode(t, e.Attach(Module{Schema: []byte(dbSchema)}), CodeInvalidArgument)
	wantCode(t, e.Add(Env(nil)), CodeInvalidArgument)
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

func TestAddStandardRejectsAMalformedKnob(t *testing.T) {
	e := engine(t, httpSchema)
	if err := e.AddStandard(Standard{Knobs: []string{"bad"}}); err == nil {
		t.Fatal("a malformed --knob fails AddStandard")
	}
}

// gateSchema is a module with connection-overridable knobs (one with bounds, one with only a minimum),
// a knob that is not overridable, and two secrets.
const gateSchema = `{
  "x-knobs-module": "gate",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "window": {"type": "integer", "default": 16, "minimum": 1, "maximum": 64, "x-knobs-connection": true},
    "ratio": {"type": "number", "default": 2, "minimum": 1, "x-knobs-connection": true},
    "mode": {"type": "string", "default": "strict", "x-knobs-connection": true},
    "locked": {"type": "string", "default": "fixed"},
    "api_key": {"type": "string", "x-knobs-secret": true},
    "token": {"type": "string", "x-knobs-secret": true}
  }
}`

// gate builds an engine for gateSchema: the given layers over the defaults.
func gate(t *testing.T, layers ...Layer) (*Config, error) {
	t.Helper()
	e, err := New("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Attach(Module{Schema: []byte(gateSchema)}); err != nil {
		t.Fatal(err)
	}
	e.Add(Defaults())
	for _, l := range layers {
		e.Add(l)
	}
	return e.Resolve()
}

func mustGate(t *testing.T, layers ...Layer) *Config {
	t.Helper()
	c, err := gate(t, layers...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestNewTierPrecedence, TestBasePrecedence: each layer wins over the lower ones, and the
// winning layer is recorded.
func TestLayerPrecedence(t *testing.T) {
	file := yamlFile(t, "demo:\n  gate:\n    window: 20\n")
	env := envOf(map[string]string{"DEMO_GATE_WINDOW": "30"})
	for _, tc := range []struct {
		layers []Layer
		window int64
		layer  string
	}{
		{nil, 16, LayerSchema},
		{[]Layer{File(LayerGlobal, file, true)}, 20, LayerGlobal},
		{[]Layer{File(LayerGlobal, file, true), Env(env)}, 30, LayerEnv},
		{[]Layer{File(LayerGlobal, file, true), Env(env), CLI([]string{"demo.gate.window=40"})}, 40, LayerCLI},
	} {
		c := mustGate(t, tc.layers...)
		if n, _ := c.Int("gate", "window"); n != tc.window || layerOf(t, c, "demo.gate.window") != tc.layer {
			t.Fatalf("want %d from %s, got %d from %s", tc.window, tc.layer, n, layerOf(t, c, "demo.gate.window"))
		}
	}
}

// A malformed value in a higher layer fails loading, naming the layer and key: it never falls
// back to the value below it.
func TestMalformedValueFailsClosed(t *testing.T) {
	_, err := gate(t, Env(envOf(map[string]string{"DEMO_GATE_WINDOW": "abc"})))
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != CodeInvalidType || ke.Layer != LayerEnv || ke.Key != "demo.gate.window" {
		t.Fatalf("got %v", err)
	}
}

// TestPathSpec: the file paths are knobs of the reserved module, set by cli or env only; a file
// cannot name them.
func TestPathKnobsNotInFiles(t *testing.T) {
	_, err := gate(t, File(LayerGlobal, yamlFile(t, "demo:\n  knobs:\n    global_path: /x\n"), true))
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != CodeUnknownKey || ke.Key != "demo.knobs" {
		t.Fatalf("got %v", err)
	}
}

// TestSecretViolation, TestSecretViolationDeterministicOrder: a secret in a plain file or on the
// command line fails; with two, the first key in order is reported.
func TestSecretLayerPolicy(t *testing.T) {
	_, err := gate(t, File(LayerGlobal, yamlFile(t, "demo:\n  gate:\n    token: t\n    api_key: k\n"), true))
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != CodeSecretInPlainFile || ke.Key != "demo.gate.api_key" {
		t.Fatalf("got %v", err)
	}
	_, err = gate(t, CLI([]string{"demo.gate.token=t"}))
	if !errors.As(err, &ke) || ke.Code != CodeSecretOnCLI {
		t.Fatalf("got %v", err)
	}
	c := mustGate(t, Env(envOf(map[string]string{"DEMO_GATE_API_KEY": "k"})))
	if s, err := c.Secret("gate", "api_key"); err != nil || s.Reveal() != "k" {
		t.Fatalf("a secret from env: %v", err)
	}
}
