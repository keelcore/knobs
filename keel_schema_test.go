package knobs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// testdata/keel-schema.yaml is keelcore/keel pkg/config/schema.yaml (f7991c0), unchanged: a JSON
// Schema draft 2019-09 application schema with nested objects and arrays (issue #12).
func keelEngine(t *testing.T, layers ...Layer) *Engine {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "keel-schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New("keel")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Attach(Module{Schema: data}); err != nil {
		t.Fatalf("keel's schema does not attach: %v", err)
	}
	if err := e.Add(layers...); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestKeelSchemaResolvesEveryLayer(t *testing.T) {
	global := yamlFile(t, `keel:
  listeners:
    http:
      port: 8080
  sidecar:
    header_policy:
      forward: [x-request-id]
    retry:
      dial_attempts: 3
`)
	env := envOf(map[string]string{
		"KEEL_SIDECAR_HEADER_POLICY_STRIP": "cookie, authorization",
		"KEEL_LISTENERS_HTTP_ENABLED":      "true",
	})
	e := keelEngine(t, Defaults(), File(LayerGlobal, global, true), Env(env),
		CLI([]string{"keel.sidecar.retry.dial_attempts=5"}))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]Value{
		"keel.listeners.http.port":           {Value: int64(8080), Layer: LayerGlobal},
		"keel.listeners.http.enabled":        {Value: true, Layer: LayerEnv},
		"keel.sidecar.header_policy.forward": {Value: []any{"x-request-id"}, Layer: LayerGlobal},
		"keel.sidecar.header_policy.strip":   {Value: []any{"cookie", "authorization"}, Layer: LayerEnv},
		"keel.sidecar.retry.dial_attempts":   {Value: int64(5), Layer: LayerCLI},
	} {
		got, ok := c.Get(key)
		if !ok || !reflect.DeepEqual(got.Value, want.Value) || got.Layer != want.Layer {
			t.Errorf("%s = %#v from %q, want %#v from %q", key, got.Value, got.Layer, want.Value, want.Layer)
		}
	}
	if n, err := c.Int("sidecar", "retry.dial_attempts"); err != nil || n != 5 {
		t.Fatalf("Int = %d, %v", n, err)
	}
	if l, err := c.List("sidecar", "header_policy.forward"); err != nil || len(l) != 1 {
		t.Fatalf("List = %v, %v", l, err)
	}
}

func TestKeelSchemaRulesHold(t *testing.T) {
	for name, tc := range map[string]struct {
		knob, code string
	}{
		"minimum":    {"keel.sidecar.retry.dial_attempts=0", CodeOutOfRange},
		"enum":       {"keel.audit.remote_sink.protocol=smtp", CodeNotInEnum},
		"type":       {"keel.listeners.http.port=eighty", CodeInvalidType},
		"array item": {"keel.sidecar.header_policy.forward=[1]", CodeInvalidType},
		"nested key": {"keel.sidecar.retry.bogus=1", CodeUnknownKey},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := keelEngine(t, CLI([]string{tc.knob})).Resolve()
			wantCode(t, err, tc.code)
		})
	}
}

func TestKeelSchemaOwnFileHoldsModulesDirectly(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "keel-schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	own := yamlFile(t, "listeners:\n  http:\n    port: 9090\n")
	e, _ := New("keel")
	if err := e.Attach(Module{Schema: data, GlobalPath: own, Code: map[string]any{"tls.acme.cache_dir": "/var/cache/keel"}}); err != nil {
		t.Fatal(err)
	}
	e.Add(Code(nil), File(LayerGlobal, "", false))
	c, err := e.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get("keel.listeners.http.port"); v.Value != int64(9090) || v.Layer != LayerGlobal {
		t.Fatalf("own file: %+v", v)
	}
	if v, _ := c.Get("keel.tls.acme.cache_dir"); v.Value != "/var/cache/keel" || v.Layer != LayerCode {
		t.Fatalf("code keyed <module>.<name>: %+v", v)
	}
	if c.Dump()["keel"].(map[string]any)["listeners"].(map[string]any)["http"].(map[string]any)["port"] != int64(9090) {
		t.Fatalf("dump does not nest: %v", c.Dump())
	}
}
