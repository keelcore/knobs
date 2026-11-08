package knobs

import (
	"math"
	"regexp"
	"strconv"
	"unicode/utf8"
)

var (
	integerText  = regexp.MustCompile(`^-?[0-9]+$`)
	numberText   = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?$`)
	durationText = regexp.MustCompile(`^[-+]?([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`)
)

// coerceText converts env, cli, runtime and header text to the property's type
// (spec/README.md, Layers). The second result is an error code, or "".
func coerceText(p *property, text string) (any, string) {
	switch p.typ {
	case TypeInteger:
		if !integerText.MatchString(text) {
			return nil, CodeInvalidType
		}
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, CodeInvalidType
		}
		return n, ""
	case TypeNumber:
		if !numberText.MatchString(text) {
			return nil, CodeInvalidType
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, CodeInvalidType
		}
		return f, ""
	case TypeBoolean:
		switch text {
		case "true":
			return true, ""
		case "false":
			return false, ""
		}
		return nil, CodeInvalidType
	}
	return coerceTyped(p, text)
}

// coerceTyped checks a typed value (from a file or a schema) against the property's type and
// normalizes it: integers to int64, numbers to float64.
func coerceTyped(p *property, v any) (any, string) {
	switch p.typ {
	case TypeInteger:
		if n, ok := asInt(v); ok {
			return n, ""
		}
	case TypeNumber:
		if f, ok := asFloat(v); ok {
			return f, ""
		}
	case TypeBoolean:
		if b, ok := v.(bool); ok {
			return b, ""
		}
	case TypeString:
		if s, ok := v.(string); ok {
			return s, ""
		}
	case TypeDuration:
		if s, ok := v.(string); ok && durationText.MatchString(s) {
			return s, ""
		}
	}
	return nil, CodeInvalidType
}

// asInt accepts any integer, and a float with no fractional part.
func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case uint64:
		if n <= math.MaxInt64 {
			return int64(n), true
		}
	case float64:
		if n == math.Trunc(n) && math.Abs(n) < 1<<63 {
			return int64(n), true
		}
	}
	return 0, false
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	}
	return 0, false
}

// checkRules applies enum, bounds and lengths to a value already of the property's type.
func checkRules(p *property, v any) string {
	if len(p.enum) > 0 && !inEnum(p, v) {
		return CodeNotInEnum
	}
	if f, ok := asFloat(v); ok && (p.typ == TypeInteger || p.typ == TypeNumber) {
		if (p.min != nil && f < *p.min) || (p.max != nil && f > *p.max) {
			return CodeOutOfRange
		}
	}
	if s, ok := v.(string); ok {
		n := int64(utf8.RuneCountInString(s))
		if (p.minLen != nil && n < *p.minLen) || (p.maxLen != nil && n > *p.maxLen) {
			return CodeInvalidLength
		}
	}
	return ""
}

func inEnum(p *property, v any) bool {
	for _, e := range p.enum {
		if e == v {
			return true
		}
		if a, ok := asFloat(e); ok {
			if b, ok := asFloat(v); ok && a == b {
				return true
			}
		}
	}
	return false
}

// clampNumber sets a number outside the property's bounds to the nearer bound; an integer
// property clamps to the nearest integer inside the bounds. It reports whether it clamped.
func clampNumber(p *property, v any) (any, bool) {
	f, ok := asFloat(v)
	if !ok {
		return v, false
	}
	lo, hi := math.Inf(-1), math.Inf(1)
	if p.min != nil {
		lo = *p.min
	}
	if p.max != nil {
		hi = *p.max
	}
	if p.typ == TypeInteger {
		lo, hi = math.Ceil(lo), math.Floor(hi)
	}
	switch {
	case f < lo:
		f = lo
	case f > hi:
		f = hi
	default:
		return v, false
	}
	if p.typ == TypeInteger {
		return int64(f), true
	}
	return f, true
}
