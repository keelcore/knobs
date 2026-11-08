// Command knobs-conform is the Go conformance harness (spec/README.md).
package main

import (
	"os"

	"github.com/keelcore/knobs/internal/conform"
)

func main() {
	os.Exit(conform.Main(os.Args[1:], os.Stdout, os.Stderr))
}
