//go:build !windows

package knobs

import "testing"

func TestStandardDefaultPaths(t *testing.T) {
	g, u := defaultGlobalPath("demo", defaultFileName, nil), DefaultUserPath("demo", func(k string) (string, bool) {
		if k == "HOME" {
			return "/home/x", true
		}
		return "", false
	})
	if g != "/etc/demo/config.yaml" || u != "/home/x/.config/demo/config.yaml" || DefaultGlobalPath("demo") != g {
		t.Fatalf("default paths: %s %s", g, u)
	}
	if u := DefaultUserPath("demo", func(k string) (string, bool) { return "/xdg", k == "XDG_CONFIG_HOME" }); u != "/xdg/demo/config.yaml" {
		t.Fatalf("XDG path: %s", u)
	}
	if u := DefaultUserPath("demo", func(string) (string, bool) { return "", false }); u != "" {
		t.Fatalf("no home: %q", u)
	}
}

// The global and user files take one file name.
func TestDefaultPathsTakeTheFileName(t *testing.T) {
	if g := defaultGlobalPath("demo", "keel.yaml", nil); g != "/etc/demo/keel.yaml" {
		t.Fatalf("global: %s", g)
	}
	if u := defaultUserPath("demo", "keel.yaml", func(k string) (string, bool) { return "/xdg", k == "XDG_CONFIG_HOME" }); u != "/xdg/demo/keel.yaml" {
		t.Fatalf("user: %s", u)
	}
}
