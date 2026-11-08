//go:build !windows

package conform

import "os"

// setMode gives a case file its mode.
func setMode(path string, mode os.FileMode) error { return os.Chmod(path, mode) }
