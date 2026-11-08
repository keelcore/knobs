// Package knobstest holds the checks an application on knobs runs in its own tests: that its committed
// master schema is the one its code builds (Schema), and that its binary links one major version of
// knobs (SingleMajor).
package knobstest

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime/debug"
	"testing"

	"github.com/keelcore/knobs"
)

// writeEnv, set to 1, makes Schema write the committed file instead of comparing with it.
const writeEnv = "KNOBS_WRITE_SCHEMA"

// readBuildInfo is the test binary's build information; tests replace it.
var readBuildInfo = debug.ReadBuildInfo

// marshalSchema encodes the master schema; tests replace it.
var marshalSchema = json.MarshalIndent

// Schema fails the test when the master schema the engine builds differs from the committed file at
// path, so every knob change shows in review as a change to that file. With KNOBS_WRITE_SCHEMA=1 it
// writes the file instead.
func Schema(t testing.TB, e *knobs.Engine, path string) {
	t.Helper()
	built, err := marshalSchema(e.Schema(), "", "  ")
	if err != nil {
		t.Errorf("knobstest: the master schema does not encode: %v", err)
		return
	}
	built = append(built, '\n')
	if os.Getenv(writeEnv) == "1" {
		if err := os.WriteFile(path, built, 0o600); err != nil {
			t.Errorf("knobstest: writing %s: %v", path, err)
		}
		return
	}
	committed, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(committed, built) {
		t.Errorf("knobstest: %s is not the master schema the code builds; rewrite it with %s=1 go test and review "+
			"the change (%v)", path, writeEnv, err)
	}
}

// SingleMajor fails the test when the test binary links more than one major version of knobs
// (github.com/keelcore/knobs and github.com/keelcore/knobs/v2, say): one binary, one engine.
func SingleMajor(t testing.TB) {
	t.Helper()
	info, ok := readBuildInfo()
	if !ok {
		t.Errorf("knobstest: the binary carries no build information")
		return
	}
	paths := []string{info.Main.Path}
	for _, d := range info.Deps {
		paths = append(paths, d.Path)
	}
	if majors := knobs.Majors(paths); len(majors) > 1 {
		t.Errorf("knobstest: the binary links %d major versions of knobs: %v", len(majors), majors)
	}
}
