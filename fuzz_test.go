package knobs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// FuzzCoerceText: any text converts to a value of the property's type or fails with
// invalid_type; it never panics.
func FuzzCoerceText(f *testing.F) {
	for _, s := range []string{"42", "-1", "4.5e3", "true", "1h30m", "", "µs", "99999999999999999999"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		for _, typ := range []string{TypeInteger, TypeNumber, TypeBoolean, TypeString, TypeDuration} {
			v, code := coerceText(&property{typ: typ}, text)
			if (code == "") == (v == nil) {
				t.Fatalf("%s %q: value %v with code %q", typ, text, v, code)
			}
		}
	})
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

// FuzzConnection: any connection key and text yield exactly one attempt, and the process
// configuration is unchanged.
func FuzzConnection(f *testing.F) {
	f.Add("demo.http.port", "9000", true)
	f.Add("demo.db.password", "x", true)
	f.Add("nope", "", false)
	e := engine(f, httpSchema, dbSchema)
	e.Add(Defaults())
	c, err := e.Resolve()
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, key, text string, authorized bool) {
		_, attempts := c.WithConnection(Connection{Values: map[string]string{key: text}, Authorized: authorized})
		if len(attempts) != 1 {
			t.Fatalf("attempts %v", attempts)
		}
		if n, _ := c.Int("http", "port"); n != 8080 {
			t.Fatalf("the process configuration changed: %d", n)
		}
	})
}
