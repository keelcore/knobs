package conform

import (
	"bytes"
	"encoding/json"
	"sort"
	"testing"
)

func TestTypesCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if status := Main([]string{"types"}, &out, &errOut); status != 0 {
		t.Fatalf("status %d: %s", status, out.String())
	}
	var rows []struct{ Key, Type string }
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, r.Key)
	}
	if !sort.StringsAreSorted(keys) || len(keys) != 9 {
		t.Fatalf("want 9 sorted rows for the http and db bindings, got %v", keys)
	}
}
