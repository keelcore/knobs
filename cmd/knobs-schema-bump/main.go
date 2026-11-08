// Command knobs-schema-bump reports the semver bump a configuration schema change calls for
// (internal/bumpcli), in the canonical release hook's form.
package main

import (
	"os"

	"github.com/keelcore/knobs/internal/bumpcli"
)

func main() {
	os.Exit(bumpcli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
