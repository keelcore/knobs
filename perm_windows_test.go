//go:build windows

package knobs

import (
	"os"
	"path/filepath"
	"testing"

	acl "github.com/hectane/go-acl"
	"github.com/hectane/go-acl/api"
	"golang.org/x/sys/windows"
)

// secureFile writes a file whose DACL holds only the given grants, inheriting nothing.
func secureFile(t *testing.T, grants ...api.ExplicitAccess) (string, os.FileInfo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := acl.Apply(path, true, false, grants...); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, info
}

func me(t *testing.T) *windows.SID {
	t.Helper()
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return u.User.Sid
}

func wellKnown(t *testing.T, kind windows.WELL_KNOWN_SID_TYPE) *windows.SID {
	t.Helper()
	sid, err := windows.CreateWellKnownSid(kind)
	if err != nil {
		t.Fatal(err)
	}
	return sid
}

func TestOwnerOnlyAcceptsTheOwnerSystemAndAdministrators(t *testing.T) {
	path, info := secureFile(t,
		acl.GrantSid(windows.GENERIC_ALL, me(t)),
		acl.GrantSid(windows.GENERIC_ALL, wellKnown(t, windows.WinLocalSystemSid)),
		acl.GrantSid(windows.GENERIC_ALL, wellKnown(t, windows.WinBuiltinAdministratorsSid)),
	)
	if err := ownerOnly(path, info); err != nil {
		t.Fatalf("an owner-only DACL is accepted: %v", err)
	}
}

func TestOwnerOnlyRejectsBroadGrants(t *testing.T) {
	for name, kind := range map[string]windows.WELL_KNOWN_SID_TYPE{
		"Everyone":            windows.WinWorldSid,
		"Users":               windows.WinBuiltinUsersSid,
		"Authenticated Users": windows.WinAuthenticatedUserSid,
	} {
		path, info := secureFile(t,
			acl.GrantSid(windows.GENERIC_ALL, me(t)),
			acl.GrantSid(windows.GENERIC_READ, wellKnown(t, kind)),
		)
		if err := ownerOnly(path, info); err == nil {
			t.Fatalf("a grant to %s is refused", name)
		}
	}
}

func TestOwnerOnlyMissingFile(t *testing.T) {
	if err := ownerOnly(filepath.Join(t.TempDir(), "absent.yaml"), nil); err == nil {
		t.Fatal("a file whose security descriptor cannot be read is refused")
	}
}
