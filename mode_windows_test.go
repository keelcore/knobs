//go:build windows

package knobs_test

import (
	"os"

	acl "github.com/hectane/go-acl"
	"github.com/hectane/go-acl/api"
	"golang.org/x/sys/windows"
)

// setMode gives a case file the access its mode states, as a DACL: the current user always, and Everyone read
// access when the mode grants group or other bits. Windows file modes carry no access rights.
func setMode(path string, mode os.FileMode) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	grants := []api.ExplicitAccess{acl.GrantSid(windows.GENERIC_ALL, user.User.Sid)}
	if mode&0o077 != 0 {
		everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
		if err != nil {
			return err
		}
		grants = append(grants, acl.GrantSid(windows.GENERIC_READ, everyone))
	}
	return acl.Apply(path, true, false, grants...)
}
