// Package bumpcli is knobs-schema-bump: the release-hook form of knobs.SchemaBump. Called as
// `knobs-schema-bump OLD NEW`, it prints the canonical bump hook's contract on stdout (the bump
// level, then the tag subject) and the removed and added fields on stderr. A missing OLD (no
// schema at the previous release) counts every field as added.
package bumpcli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/keelcore/knobs"
)

// Main runs the command and returns its exit status.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		_, _ = fmt.Fprintln(stderr, "usage: knobs-schema-bump OLD_SCHEMA NEW_SCHEMA")
		return 2
	}
	old, err := os.ReadFile(args[0])
	if errors.Is(err, fs.ErrNotExist) {
		_, _ = fmt.Fprintf(stderr, "WARNING: %s does not exist; treating all current fields as added\n", args[0])
		old, err = nil, nil
	}
	cur, curErr := os.ReadFile(args[1])
	if err = errors.Join(err, curErr); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	b, err := knobs.SchemaBump(old, cur)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	list(stderr, "Fields removed (BREAKING):", b.Removed)
	list(stderr, "Fields added:", b.Added)
	_, _ = fmt.Fprintf(stdout, "%s\n%s\n", b.Level, b.Subject())
	return 0
}

func list(w io.Writer, label string, fields []string) {
	if len(fields) == 0 {
		_, _ = fmt.Fprintln(w, label, "none")
		return
	}
	_, _ = fmt.Fprintln(w, label)
	for _, f := range fields {
		_, _ = fmt.Fprintln(w, " ", f)
	}
}
