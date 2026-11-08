package knobstest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/keelcore/knobs"
)

// recorder is a testing.TB that records failures instead of failing the test that uses it.
type recorder struct {
	testing.TB
	failures []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

const schema = `{
  "x-knobs-module": "http",
  "type": "object",
  "additionalProperties": false,
  "properties": {"port": {"type": "integer", "default": 8080}}
}`

func engine(t *testing.T, s string) *knobs.Engine {
	t.Helper()
	e, err := knobs.New("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Attach(knobs.Module{Schema: []byte(s)}); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestSchemaWritesThenMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.json")
	t.Setenv(writeEnv, "1")
	r := &recorder{TB: t}
	Schema(r, engine(t, schema), path)
	if len(r.failures) != 0 {
		t.Fatalf("writing the schema: %v", r.failures)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.HasSuffix(string(b), "}\n") || !strings.Contains(string(b), `"port"`) {
		t.Fatalf("written schema: %q %v", b, err)
	}
	t.Setenv(writeEnv, "")
	r = &recorder{TB: t}
	Schema(r, engine(t, schema), path)
	if len(r.failures) != 0 {
		t.Fatalf("an unchanged schema matches: %v", r.failures)
	}
}

func TestSchemaReportsADifference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.json")
	t.Setenv(writeEnv, "1")
	Schema(&recorder{TB: t}, engine(t, schema), path)
	t.Setenv(writeEnv, "")
	changed := strings.Replace(schema, `"default": 8080`, `"default": 9090`, 1)
	r := &recorder{TB: t}
	Schema(r, engine(t, changed), path)
	if len(r.failures) != 1 || !strings.Contains(r.failures[0], writeEnv+"=1") {
		t.Fatalf("a changed schema fails, naming how to rewrite it: %v", r.failures)
	}
}

func TestSchemaMissingOrUnwritable(t *testing.T) {
	r := &recorder{TB: t}
	Schema(r, engine(t, schema), filepath.Join(t.TempDir(), "absent.json"))
	if len(r.failures) != 1 || !strings.Contains(r.failures[0], writeEnv+"=1") {
		t.Fatalf("a missing committed schema fails: %v", r.failures)
	}
	t.Setenv(writeEnv, "1")
	r = &recorder{TB: t}
	Schema(r, engine(t, schema), filepath.Join(t.TempDir(), "no", "such", "dir", "schema.json"))
	if len(r.failures) != 1 {
		t.Fatalf("an unwritable path fails: %v", r.failures)
	}
}

func TestSingleMajor(t *testing.T) {
	saved := readBuildInfo
	t.Cleanup(func() { readBuildInfo = saved })
	for _, tc := range []struct {
		info  *debug.BuildInfo
		fails bool
	}{
		{&debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/keelcore/knobs"}}}, false},
		{&debug.BuildInfo{Main: debug.Module{Path: "github.com/keelcore/knobs"},
			Deps: []*debug.Module{{Path: "github.com/keelcore/knobs/v2"}}}, true},
		{nil, true},
	} {
		readBuildInfo = func() (*debug.BuildInfo, bool) { return tc.info, tc.info != nil }
		r := &recorder{TB: t}
		SingleMajor(r)
		if (len(r.failures) > 0) != tc.fails {
			t.Fatalf("%+v: failures %v", tc.info, r.failures)
		}
	}
}

func TestSingleMajorInThisBinary(t *testing.T) {
	SingleMajor(t)
}

func TestSchemaEncodingFailure(t *testing.T) {
	saved := marshalSchema
	marshalSchema = func(any, string, string) ([]byte, error) { return nil, fmt.Errorf("no") }
	t.Cleanup(func() { marshalSchema = saved })
	r := &recorder{TB: t}
	Schema(r, engine(t, schema), filepath.Join(t.TempDir(), "schema.json"))
	if len(r.failures) != 1 {
		t.Fatalf("an encoding failure fails the test: %v", r.failures)
	}
}
