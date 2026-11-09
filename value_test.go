package knobs

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestCoerceText(t *testing.T) {
	for _, tc := range []struct {
		typ, text string
		want      any
		code      string
	}{
		{TypeInteger, "42", int64(42), ""},
		{TypeInteger, "-7", int64(-7), ""},
		{TypeInteger, "99999999999999999999", nil, CodeInvalidType},
		{TypeInteger, "4.2", nil, CodeInvalidType},
		{TypeNumber, "4.25", 4.25, ""},
		{TypeNumber, "1e3", 1000.0, ""},
		{TypeNumber, "1e999", nil, CodeInvalidType},
		{TypeNumber, "four", nil, CodeInvalidType},
		{TypeBoolean, "true", true, ""},
		{TypeBoolean, "false", false, ""},
		{TypeBoolean, "TRUE", nil, CodeInvalidType},
		{TypeString, "x", "x", ""},
		{TypeDuration, "1h30m", "1h30m", ""},
		{TypeDuration, "90", nil, CodeInvalidType},
	} {
		got, code := coerceText(&property{typ: tc.typ}, tc.text)
		if got != tc.want || code != tc.code {
			t.Fatalf("%s %q: got %v %q, want %v %q", tc.typ, tc.text, got, code, tc.want, tc.code)
		}
	}
}

func TestCoerceTypedNumbers(t *testing.T) {
	integer, number := &property{typ: TypeInteger}, &property{typ: TypeNumber}
	for _, tc := range []struct {
		p    *property
		in   any
		want any
		ok   bool
	}{
		{integer, int(5), int64(5), true},
		{integer, uint64(5), int64(5), true},
		{integer, uint64(math.MaxUint64), nil, false},
		{integer, 5.0, int64(5), true},
		{integer, 5.5, nil, false},
		{integer, "5", nil, false},
		{number, int(5), 5.0, true},
		{number, int64(5), 5.0, true},
		{number, uint64(5), 5.0, true},
		{number, math.NaN(), nil, false},
		{number, math.Inf(1), nil, false},
		{number, true, nil, false},
	} {
		got, code := coerceTyped(tc.p, tc.in)
		if (code == "") != tc.ok || (tc.ok && got != tc.want) {
			t.Fatalf("%s %v: got %v %q", tc.p.typ, tc.in, got, code)
		}
	}
}

// TestClamp, TestClampBounds: the bound cases directly.
func TestClampNumber(t *testing.T) {
	one, sixtyFour, half := 1.0, 64.0, 1.5
	for _, tc := range []struct {
		typ      string
		min, max *float64
		in, want any
		clamped  bool
	}{
		{TypeInteger, &one, &sixtyFour, int64(40), int64(40), false},
		{TypeInteger, &one, &sixtyFour, int64(9999), int64(64), true},
		{TypeInteger, &one, &sixtyFour, int64(0), int64(1), true},
		{TypeInteger, &half, nil, int64(0), int64(2), true},
		{TypeNumber, &one, nil, 5.0, 5.0, false},
		{TypeNumber, &one, nil, 0.5, 1.0, true},
		{TypeNumber, nil, nil, 0.5, 0.5, false},
		{TypeString, nil, nil, "anything", "anything", false},
	} {
		p := &property{typ: tc.typ, min: tc.min, max: tc.max}
		got, clamped := clampNumber(p, tc.in)
		if got != tc.want || clamped != tc.clamped {
			t.Fatalf("%v: got %v %v, want %v %v", tc.in, got, clamped, tc.want, tc.clamped)
		}
	}
}

// FuzzCoerceText: any text converts to a value of the property's type or fails with
// invalid_type; it never panics.
func FuzzCoerceText(f *testing.F) {
	for _, s := range []string{"42", "-1", "4.5e3", "true", "1h30m", "", "µs", "99999999999999999999"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		for _, typ := range []string{TypeInteger, TypeNumber, TypeBoolean, TypeString, TypeDuration} {
			v, code := coerceText(&property{typ: typ}, text)
			if (code == "") == (v == nil) {
				t.Fatalf("%s %q: value %v with code %q", typ, text, v, code)
			}
		}
	})
}

func TestNormalizeValue(t *testing.T) {
	type label string
	type flag bool
	for _, tc := range []struct{ in, want any }{
		{float32(1.5), 1.5},
		{label("x"), "x"},
		{flag(true), true},
		{[2]int{1, 2}, []any{int64(1), int64(2)}},
		{map[int]string{1: "a"}, map[int]string{1: "a"}},
		{uint64(1 << 63), uint64(1 << 63)},
		{struct{}{}, struct{}{}},
	} {
		if got := normalizeValue(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("normalizeValue(%#v) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
	if f, ok := asFloat(uint64(1 << 63)); !ok || f != 1<<63 {
		t.Fatal("asFloat of an integer past int64")
	}
	if _, err := decodeJSON([]byte(`1 2`)); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing data: %v", err)
	}
}
