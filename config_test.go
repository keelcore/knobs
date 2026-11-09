package knobs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	if v, ok := c.Get("demo.http.port"); !ok || v.Layer != LayerSchema || v.Secret {
		t.Fatalf("Get = %+v, %v", v, ok)
	}
	if _, ok := c.Get("demo.http.nope"); ok {
		t.Fatal("Get of an unknown key must report false")
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

func layerOf(t *testing.T, c *Config, key string) string {
	t.Helper()
	v, ok := c.Get(key)
	if !ok {
		t.Fatalf("%s is not set", key)
	}
	return v.Layer
}

// TestResolveUnknown: an unknown key resolves to nothing.
func TestGetUnknownKey(t *testing.T) {
	c := mustGate(t)
	if _, ok := c.Get("demo.gate.nope"); ok {
		t.Fatal("an unknown key is not set")
	}
	if _, err := c.Int("gate", "nope"); err == nil {
		t.Fatal("an unknown key has no value")
	}
}

// TestProvenance: every set value, sorted by key, with its winning layer, including a
// connection override.
func TestValuesProvenance(t *testing.T) {
	c := mustGate(t)
	r, _ := c.WithConnection(Connection{Values: map[string]string{"demo.gate.window": "20"}, Authorized: true})
	var keys, layers []string
	for _, v := range r.Values() {
		keys = append(keys, v.Key)
		layers = append(layers, v.Layer)
	}
	want := []string{"demo.gate.locked", "demo.gate.mode", "demo.gate.ratio", "demo.gate.window"}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys %v", keys)
		}
	}
	if layers[3] != LayerConnection || layers[0] != LayerSchema {
		t.Fatalf("layers %v", layers)
	}
}
