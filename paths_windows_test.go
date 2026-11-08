//go:build windows

package knobs

import "testing"

func TestStandardDefaultPathsOnWindows(t *testing.T) {
	env := func(vars map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
	}
	if g := defaultGlobalPath("demo", env(map[string]string{"ProgramData": `D:\Data`})); g != `D:\Data\demo\config.yaml` {
		t.Fatalf("global from ProgramData: %s", g)
	}
	if g := defaultGlobalPath("demo", env(nil)); g != `C:\ProgramData\demo\config.yaml` {
		t.Fatalf("global without ProgramData: %s", g)
	}
	if u := DefaultUserPath("demo", env(map[string]string{"APPDATA": `C:\Users\x\AppData\Roaming`})); u != `C:\Users\x\AppData\Roaming\demo\config.yaml` {
		t.Fatalf("user from APPDATA: %s", u)
	}
	if u := DefaultUserPath("demo", env(nil)); u != "" {
		t.Fatalf("no APPDATA: %q", u)
	}
	if DefaultGlobalPath("demo") == "" {
		t.Fatal("the process environment gives a global path")
	}
}
