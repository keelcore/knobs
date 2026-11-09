package knobs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestZeroSecretRevealsEmpty(t *testing.T) {
	if (Secret{}).Reveal() != "" {
		t.Fatal("a zero Secret holds nothing")
	}
}

func TestSecretNeverPrints(t *testing.T) {
	s := NewSecret("hunter2")
	for _, out := range []string{
		s.String(), s.GoString(), fmt.Sprint(s), fmt.Sprintf("%v %+v %#v %s %q %x", s, s, s, s, s, s),
	} {
		if strings.Contains(out, "hunter2") || strings.Contains(out, fmt.Sprintf("%x", "hunter2")) {
			t.Fatalf("secret printed: %s", out)
		}
	}
	b, err := json.Marshal(struct{ P Secret }{s})
	if err != nil || strings.Contains(string(b), "hunter2") {
		t.Fatalf("secret marshalled: %s %v", b, err)
	}
	var back Secret
	if err := json.Unmarshal([]byte(`"x"`), &back); err == nil {
		t.Fatal("a secret must not be read from JSON")
	}
	if s.Reveal() != "hunter2" {
		t.Fatal("Reveal must return the value")
	}
}
