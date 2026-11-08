//go:build !windows

package knobs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerOnlyByModeBits(t *testing.T) {
	for mode, ok := range map[os.FileMode]bool{0o600: true, 0o400: true, 0o640: false, 0o604: false, 0o610: false} {
		path := filepath.Join(t.TempDir(), "secrets.yaml")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := ownerOnly(path, info) == nil; got != ok {
			t.Fatalf("mode %04o: owner-only %v, want %v", mode, got, ok)
		}
	}
}
