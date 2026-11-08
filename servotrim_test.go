package knobs

// The tests of servotrim's two resolvers (pipeline/config and clients/go/knob), ported to this
// engine. Each test names the servotrim test it carries. Where the engine's contract differs on
// purpose, the test says so:
//   - a malformed value in a higher layer fails loading instead of falling back to the default
//     (fail-closed; spec/README.md, Layers);
//   - transport codecs (X-Servotrim-* header names, the outbound Authorization codec) are the
//     client's mapping: the engine takes connection values by key and never sees headers;
//   - the per-session header-parse cache is gone: connection scope keeps no state.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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

// gate builds the servotrim test engine: the given layers over the defaults.
func gate(t *testing.T, layers ...Layer) (*Config, error) {
	t.Helper()
	e, err := New("servotrim")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Attach([]byte(gateSchema), "", ""); err != nil {
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

func layerOf(t *testing.T, c *Config, key string) string {
	t.Helper()
	v, ok := c.Get(key)
	if !ok {
		t.Fatalf("%s is not set", key)
	}
	return v.Layer
}

// TestTierString (both resolvers): the layer names, lowest to highest.
func TestServotrimTierNames(t *testing.T) {
	want := []string{"default", "global", "user", "secrets", "env", "cli", "runtime", "connection"}
	got := []string{LayerDefault, LayerGlobal, LayerUser, LayerSecrets, LayerEnv, LayerCLI, LayerRuntime, LayerConnection}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("layer %d: %s, want %s", i, got[i], want[i])
		}
	}
}

// TestNewTierPrecedence, TestBasePrecedence: each layer wins over the lower ones, and the
// winning layer is recorded.
func TestServotrimPrecedence(t *testing.T) {
	file := yamlFile(t, "servotrim:\n  gate:\n    window: 20\n")
	env := envOf(map[string]string{"SERVOTRIM_GATE_WINDOW": "30"})
	for _, tc := range []struct {
		layers []Layer
		window int64
		layer  string
	}{
		{nil, 16, LayerDefault},
		{[]Layer{File(LayerGlobal, file, true)}, 20, LayerGlobal},
		{[]Layer{File(LayerGlobal, file, true), Env(env)}, 30, LayerEnv},
		{[]Layer{File(LayerGlobal, file, true), Env(env), CLI([]string{"servotrim.gate.window=40"})}, 40, LayerCLI},
	} {
		c := mustGate(t, tc.layers...)
		if n, _ := c.Int("gate", "window"); n != tc.window || layerOf(t, c, "servotrim.gate.window") != tc.layer {
			t.Fatalf("want %d from %s, got %d from %s", tc.window, tc.layer, n, layerOf(t, c, "servotrim.gate.window"))
		}
	}
}

// TestNilEnvSource: a nil environment lookup is tolerated.
func TestServotrimNilEnv(t *testing.T) {
	c := mustGate(t, Env(nil))
	if layerOf(t, c, "servotrim.gate.window") != LayerDefault {
		t.Fatal("a nil env sets nothing")
	}
}

// TestResolveUnknown: an unknown key resolves to nothing.
func TestServotrimUnknownKey(t *testing.T) {
	c := mustGate(t)
	if _, ok := c.Get("servotrim.gate.nope"); ok {
		t.Fatal("an unknown key is not set")
	}
	if _, err := c.Int("gate", "nope"); err == nil {
		t.Fatal("an unknown key has no value")
	}
}

// TestRuntimeTier: runtime beats cli, a connection beats runtime.
func TestServotrimRuntimeAndConnection(t *testing.T) {
	c := mustGate(t, CLI([]string{"servotrim.gate.window=40"}), Runtime(map[string]string{"servotrim.gate.window": "50"}))
	if n, _ := c.Int("gate", "window"); n != 50 || layerOf(t, c, "servotrim.gate.window") != LayerRuntime {
		t.Fatalf("runtime: %d", n)
	}
	r, _ := c.WithConnection(Connection{Values: map[string]string{"servotrim.gate.window": "60"}, Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 60 || layerOf(t, r, "servotrim.gate.window") != LayerConnection {
		t.Fatalf("connection: %d", n)
	}
}

// TestResolveNilSession, TestAbsentHeaderKeepsBase: no connection values leave the process
// configuration as it is, with no attempts.
func TestServotrimEmptyConnection(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 16 || len(attempts) != 0 {
		t.Fatalf("got %d, %v", n, attempts)
	}
}

// TestHeaderOverlay, TestResolveWithHeaders: an authorized override on an allowlisted key wins.
func TestServotrimConnectionOverlay(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"servotrim.gate.mode": "loose"}, Authorized: true})
	if s, _ := r.String("gate", "mode"); s != "loose" || attempts[0].Outcome != OutcomeApplied {
		t.Fatalf("got %q %v", s, attempts)
	}
}

// TestUnauthorizedCredentialIgnored: an allowlisted key is not overridden without authorization
// (fail-closed).
func TestServotrimUnauthorized(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"servotrim.gate.window": "40"}})
	if n, _ := r.Int("gate", "window"); n != 16 || attempts[0].Outcome != OutcomeRejectedUnauthorized {
		t.Fatalf("got %d %v", n, attempts)
	}
}

// TestUnauthorizedHeaderIgnored: a key not marked overridable keeps its value, authorized or not.
func TestServotrimNotOverridable(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{"servotrim.gate.locked": "x", "servotrim.gate.api_key": "k"}, Authorized: true})
	if s, _ := r.String("gate", "locked"); s != "fixed" {
		t.Fatalf("got %q", s)
	}
	for _, a := range attempts {
		if a.Outcome != OutcomeRejectedNotOverridable {
			t.Fatalf("%s: %v", a.Key, a.Outcome)
		}
	}
}

// TestHeaderClampAndReject, TestHeaderAttempt: out of range clamps to the bound; a non-numeric
// value on a bounded key keeps the base; every attempt is recorded.
func TestServotrimClampAndReject(t *testing.T) {
	c := mustGate(t)
	r, attempts := c.WithConnection(Connection{Values: map[string]string{
		"servotrim.gate.window": "9999", "servotrim.gate.ratio": "0.2", "servotrim.gate.mode": "ok",
	}, Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 64 {
		t.Fatalf("clamped to max: %d", n)
	}
	if f, _ := r.Float("gate", "ratio"); f != 1 {
		t.Fatalf("clamped to min (only minimum set): %v", f)
	}
	want := []Outcome{OutcomeApplied, OutcomeClamped, OutcomeClamped}
	for i, a := range attempts {
		if a.Outcome != want[i] {
			t.Fatalf("%s: %v, want %v", a.Key, a.Outcome, want[i])
		}
	}
	r, attempts = c.WithConnection(Connection{Values: map[string]string{"servotrim.gate.window": "abc"}, Authorized: true})
	if n, _ := r.Int("gate", "window"); n != 16 || attempts[0].Outcome != OutcomeRejectedMalformed {
		t.Fatalf("malformed keeps the base: %d %v", n, attempts)
	}
}

// TestClamp, TestClampBounds: the bound cases directly.
func TestServotrimClampNumber(t *testing.T) {
	one, sixtyFour, half := 1.0, 64.0, 1.5
	for _, tc := range []struct {
		typ      string
		min, max *float64
		in, want any
		clamped  bool
	}{
		{TypeInteger, &one, &sixtyFour, int64(40), int64(40), false},
		{TypeInteger, &one, &sixtyFour, int64(9999), int64(64), true},
		{TypeInteger, &one, &sixtyFour, int64(0), int64(1), true},
		{TypeInteger, &half, nil, int64(0), int64(2), true},
		{TypeNumber, &one, nil, 5.0, 5.0, false},
		{TypeNumber, &one, nil, 0.5, 1.0, true},
		{TypeNumber, nil, nil, 0.5, 0.5, false},
		{TypeString, nil, nil, "anything", "anything", false},
	} {
		p := &property{typ: tc.typ, min: tc.min, max: tc.max}
		got, clamped := clampNumber(p, tc.in)
		if got != tc.want || clamped != tc.clamped {
			t.Fatalf("%v: got %v %v, want %v %v", tc.in, got, clamped, tc.want, tc.clamped)
		}
	}
}

// TestTypedAccessorsFallback: servotrim fell back to the default on a malformed value; this
// engine fails loading instead, naming the layer and key.
func TestServotrimMalformedFailsClosed(t *testing.T) {
	_, err := gate(t, Env(envOf(map[string]string{"SERVOTRIM_GATE_WINDOW": "abc"})))
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != CodeInvalidType || ke.Layer != LayerEnv || ke.Key != "servotrim.gate.window" {
		t.Fatalf("got %v", err)
	}
}

// TestStringAccessor: an empty string set in a file is a value; an empty env value is unset.
func TestServotrimEmptyValues(t *testing.T) {
	c := mustGate(t, File(LayerGlobal, yamlFile(t, "servotrim:\n  gate:\n    mode: \"\"\n"), true),
		Env(envOf(map[string]string{"SERVOTRIM_GATE_LOCKED": ""})))
	if s, _ := c.String("gate", "mode"); s != "" || layerOf(t, c, "servotrim.gate.mode") != LayerGlobal {
		t.Fatalf("empty file string: %q", s)
	}
	if layerOf(t, c, "servotrim.gate.locked") != LayerDefault {
		t.Fatal("an empty env value is unset")
	}
}

// TestKeyCodecs, TestKeyDerivation, TestHeaderKeyEmptySegment: the key, file and env forms
// derive from app, module and name; header names are the client's mapping.
func TestServotrimKeyForms(t *testing.T) {
	e, _ := New("servotrim")
	if err := e.Attach([]byte(gateSchema), "", ""); err != nil {
		t.Fatal(err)
	}
	s := &Schemas{set: e.set}
	if s.App() != "servotrim" || s.EnvName("gate", "window") != "SERVOTRIM_GATE_WINDOW" {
		t.Fatalf("forms: %s %s", s.App(), s.EnvName("gate", "window"))
	}
	if mod, name, ok := s.Lookup("servotrim.gate.window"); !ok || mod != "gate" || name != "window" {
		t.Fatal("lookup")
	}
	for _, bad := range []string{"servotrim.gate", "other.gate.window", "servotrim..window", "servotrim.gate.window.x"} {
		if _, _, ok := s.Lookup(bad); ok {
			t.Fatalf("%q must not resolve", bad)
		}
	}
}

// TestPathSpec: the file paths are knobs of the reserved module, set by cli or env only; a file
// cannot name them.
func TestServotrimPathKnobs(t *testing.T) {
	_, err := gate(t, File(LayerGlobal, yamlFile(t, "servotrim:\n  knobs:\n    global_path: /x\n"), true))
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != CodeUnknownKey || ke.Key != "servotrim.knobs" {
		t.Fatalf("got %v", err)
	}
}

// TestSecretViolation, TestSecretViolationDeterministicOrder: a secret in a plain file or on the
// command line fails; with two, the first key in order is reported.
func TestServotrimSecretViolation(t *testing.T) {
	_, err := gate(t, File(LayerGlobal, yamlFile(t, "servotrim:\n  gate:\n    token: t\n    api_key: k\n"), true))
	var ke *Error
	if !errors.As(err, &ke) || ke.Code != CodeSecretInPlainFile || ke.Key != "servotrim.gate.api_key" {
		t.Fatalf("got %v", err)
	}
	_, err = gate(t, CLI([]string{"servotrim.gate.token=t"}))
	if !errors.As(err, &ke) || ke.Code != CodeSecretOnCLI {
		t.Fatalf("got %v", err)
	}
	c := mustGate(t, Env(envOf(map[string]string{"SERVOTRIM_GATE_API_KEY": "k"})))
	if s, err := c.Secret("gate", "api_key"); err != nil || s.Reveal() != "k" {
		t.Fatalf("a secret from env: %v", err)
	}
}

// TestProvenance: every set value, sorted by key, with its winning layer, including a
// connection override.
func TestServotrimProvenance(t *testing.T) {
	c := mustGate(t)
	r, _ := c.WithConnection(Connection{Values: map[string]string{"servotrim.gate.window": "20"}, Authorized: true})
	var keys, layers []string
	for _, v := range r.Values() {
		keys = append(keys, v.Key)
		layers = append(layers, v.Layer)
	}
	want := []string{"servotrim.gate.locked", "servotrim.gate.mode", "servotrim.gate.ratio", "servotrim.gate.window"}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys %v", keys)
		}
	}
	if layers[3] != LayerConnection || layers[0] != LayerDefault {
		t.Fatalf("layers %v", layers)
	}
}
