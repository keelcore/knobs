package knobs

import (
	"errors"
	"runtime/debug"
	"strings"
	"testing"
)

const ownedSchema = `{
  "x-knobs-module": "sidecar",
  "x-knobs-owner": "example.com/keel",
  "type": "object",
  "additionalProperties": false,
  "properties": {"workers": {"type": "integer", "default": 2, "minimum": 1}}
}`

func withBuildInfo(t *testing.T, info *debug.BuildInfo) {
	t.Helper()
	saved := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return info, info != nil }
	t.Cleanup(func() { readBuildInfo = saved })
}

func ownedEngine(t *testing.T) *Engine {
	t.Helper()
	e, _ := New("demo")
	if err := e.Attach(Module{Schema: []byte(ownedSchema)}); err != nil {
		t.Fatal(err)
	}
	return e
}

func sidecarSchema(e *Engine) map[string]any {
	app := e.Schema()["properties"].(map[string]any)["demo"].(map[string]any)
	return app["properties"].(map[string]any)["sidecar"].(map[string]any)
}

func TestOwnerVersionFromBuildInfo(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{
		Main: debug.Module{Path: "example.com/app", Version: "(devel)"},
		Deps: []*debug.Module{{Path: "example.com/keel", Version: "v1.4.2"}},
	})
	m := sidecarSchema(ownedEngine(t))
	if m["x-knobs-owner"] != "example.com/keel" || m["x-knobs-owner-version"] != "v1.4.2" {
		t.Fatalf("owner annotations: %v %v", m["x-knobs-owner"], m["x-knobs-owner-version"])
	}
}

func TestOwnerVersionOfTheMainModule(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Path: "example.com/keel", Version: "v2.0.0"}})
	if v := sidecarSchema(ownedEngine(t))["x-knobs-owner-version"]; v != "v2.0.0" {
		t.Fatalf("main module version: %v", v)
	}
}

func TestOwnerVersionUnknown(t *testing.T) {
	withBuildInfo(t, nil)
	m := sidecarSchema(ownedEngine(t))
	if _, has := m["x-knobs-owner-version"]; has || m["x-knobs-owner"] != "example.com/keel" {
		t.Fatalf("an unresolvable version is left out: %v", m)
	}
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Path: "example.com/other"}})
	if _, has := sidecarSchema(ownedEngine(t))["x-knobs-owner-version"]; has {
		t.Fatal("an owner the binary does not link has no version")
	}
}

func TestErrorsNameTheOwner(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{Deps: []*debug.Module{{Path: "example.com/keel", Version: "v1.4.2"}}})
	e := ownedEngine(t)
	e.Add(Defaults(), CLI([]string{"demo.sidecar.workers=0"}))
	_, err := e.Resolve()
	var ke *Error
	if !errors.As(err, &ke) || ke.Owner != "example.com/keel@v1.4.2" || !strings.Contains(err.Error(), "example.com/keel@v1.4.2") {
		t.Fatalf("the error names the owner: %v", err)
	}
	if strings.Contains(sidecarSchemaOwnerless(t), "x-knobs-owner") {
		t.Fatal("a module without an owner has no owner annotations")
	}
}

func sidecarSchemaOwnerless(t *testing.T) string {
	t.Helper()
	e, _ := New("demo")
	if err := e.Attach(Module{Schema: []byte(strings.Replace(ownedSchema, `"x-knobs-owner": "example.com/keel",`, "", 1))}); err != nil {
		t.Fatal(err)
	}
	b := &strings.Builder{}
	for k := range sidecarSchema(e) {
		b.WriteString(k + " ")
	}
	return b.String()
}

func TestMajors(t *testing.T) {
	for _, tc := range []struct {
		paths []string
		want  int
	}{
		{[]string{"github.com/keelcore/knobs", "go.yaml.in/yaml/v3"}, 1},
		{[]string{"github.com/keelcore/knobs", "github.com/keelcore/knobs/v2"}, 2},
		{[]string{"github.com/keelcore/knobsx", "github.com/other/knobs"}, 0},
		{[]string{"github.com/keelcore/knobs/v3", "github.com/keelcore/knobs/v3"}, 1},
	} {
		if got := Majors(tc.paths); len(got) != tc.want {
			t.Fatalf("%v: %v", tc.paths, got)
		}
	}
}

func TestErrorNamesAnOwnerOfUnknownVersion(t *testing.T) {
	withBuildInfo(t, nil)
	e := ownedEngine(t)
	e.Add(Defaults(), CLI([]string{"demo.sidecar.workers=0"}))
	_, err := e.Resolve()
	var ke *Error
	if !errors.As(err, &ke) || ke.Owner != "example.com/keel" {
		t.Fatalf("the owner without a version: %v", err)
	}
}
