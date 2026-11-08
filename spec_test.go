package knobs_test

import (
	"bytes"
	"encoding/json"
	"github.com/keelcore/knobs/internal/conform"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// caseFile is spec/cases/<name>/case.json (spec/README.md, Cases).
type caseFile struct {
	Description string            `json:"description"`
	Args        []string          `json:"args"`
	Modes       map[string]string `json:"modes"`
	Command     string            `json:"command"`
	Expect      json.RawMessage   `json:"expect"`
	ExpectError json.RawMessage   `json:"expect_error"`
}

// TestSpecCases runs every conformance case through the harness in process, so the engine package
// is measured against the whole contract (internal/conform repeats this for the harness package).
func TestSpecCases(t *testing.T) {
	spec, err := filepath.Abs("spec")
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := filepath.Glob(filepath.Join(spec, "cases", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no cases under %s: %v", spec, err)
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) { runCase(t, spec, dir) })
	}
}

func runCase(t *testing.T, spec, src string) {
	var c caseFile
	raw, err := os.ReadFile(filepath.Join(src, "case.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	dir := copyCase(t, src, c.Modes)
	args := make([]string, 0, len(c.Args)+1)
	for _, a := range c.Args {
		args = append(args, strings.ReplaceAll(a, "{schema}", filepath.Join(spec, "schema")))
	}
	args = append(args, c.Command)
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	status := conform.Main(args, &out, &errOut)
	want, wantStatus := c.Expect, 0
	if c.ExpectError != nil {
		want, wantStatus = c.ExpectError, 2
	}
	if status != wantStatus {
		t.Fatalf("status %d, want %d\nstdout: %s\nstderr: %s", status, wantStatus, out.String(), errOut.String())
	}
	assertJSONEqual(t, want, out.Bytes())
}

// copyCase copies the case directory to a scratch directory and applies the case's file modes.
func copyCase(t *testing.T, src string, modes map[string]string) string {
	dir := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, mode := range modes {
		m, err := strconv.ParseUint(mode, 8, 32)
		if err != nil {
			t.Fatal(err)
		}
		if err := setMode(filepath.Join(dir, name), os.FileMode(m)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func assertJSONEqual(t *testing.T, want, have []byte) {
	t.Helper()
	var w, h any
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("expectation: %v", err)
	}
	if err := json.Unmarshal(have, &h); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, have)
	}
	if !reflect.DeepEqual(w, h) {
		t.Fatalf("output differs\nwant: %s\nhave: %s", canonical(w), canonical(h))
	}
}

func canonical(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
