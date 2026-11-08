package bumpcli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMainPrintsTheHookContract(t *testing.T) {
	old := write(t, `{"properties": {"a": {}, "b": {}}}`)
	cur := write(t, `{"properties": {"a": {}, "c": {}}}`)
	var out, errOut bytes.Buffer
	if status := Main([]string{old, cur}, &out, &errOut); status != 0 {
		t.Fatalf("status %d: %s", status, errOut.String())
	}
	if out.String() != "major\nremoved 1 config field(s)\n" {
		t.Fatalf("stdout %q", out.String())
	}
	for _, want := range []string{"Fields removed (BREAKING):", "  b", "Fields added:", "  c"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr lacks %q: %s", want, errOut.String())
		}
	}
}

func TestMainWithoutPreviousSchema(t *testing.T) {
	cur := write(t, `{"properties": {"a": {}}}`)
	var out, errOut bytes.Buffer
	if status := Main([]string{filepath.Join(t.TempDir(), "absent.json"), cur}, &out, &errOut); status != 0 {
		t.Fatalf("status %d", status)
	}
	if out.String() != "minor\nadded 1 config field(s)\n" || !strings.Contains(errOut.String(), "WARNING") {
		t.Fatalf("stdout %q stderr %q", out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "Fields removed (BREAKING): none") {
		t.Fatalf("an empty list says none: %s", errOut.String())
	}
}

func TestMainErrors(t *testing.T) {
	good := write(t, `{"properties": {"a": {}}}`)
	bad := write(t, `{`)
	for name, args := range map[string][]string{
		"usage":          {good},
		"current absent": {good, filepath.Join(t.TempDir(), "absent.json")},
		"current bad":    {good, bad},
	} {
		var out, errOut bytes.Buffer
		if status := Main(args, &out, &errOut); status == 0 || out.Len() != 0 {
			t.Fatalf("%s: status %d stdout %q", name, status, out.String())
		}
	}
}
