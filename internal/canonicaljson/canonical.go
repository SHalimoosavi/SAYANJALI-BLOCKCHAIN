package canonicaljson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf16"
)

// Marshal implements the frozen Python json.dumps(payload, sort_keys=True,
// separators=(",", ":"), default=str) behavior for protocol values.
// Protocol-critical code uses typed maps/slices containing strings, integers,
// floats, booleans and nil. Keys are sorted lexicographically.
func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := encode(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func String(v any) (string, error) {
	b, err := Marshal(v)
	return string(b), err
}

func encode(b *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		encodeString(b, x)
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int8:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int16:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case uint:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint8:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint16:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(x, 10))
	case float32:
		if err := encodeFloat(b, float64(x)); err != nil {
			return err
		}
	case float64:
		if err := encodeFloat(b, x); err != nil {
			return err
		}
	case json.Number:
		b.WriteString(string(x))
	case []any:
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := encode(b, item); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			encodeString(b, k)
			b.WriteByte(':')
			if err := encode(b, x[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		// Match the reference's default=str fallback for non-JSON values.
		encodeString(b, fmt.Sprint(v))
	}
	return nil
}

func encodeFloat(b *bytes.Buffer, f float64) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		// Python json.dumps allows NaN/Infinity by default. Preserve the token
		// spelling observed by the standard library rather than rejecting it.
		switch {
		case math.IsNaN(f):
			b.WriteString("NaN")
		case f > 0:
			b.WriteString("Infinity")
		default:
			b.WriteString("-Infinity")
		}
		return nil
	}
	// Python repr(float) retains .0 for integral finite floats.
	var s string
	af := math.Abs(f)
	// CPython repr/json uses fixed-point notation for 1e-4 <= |x| < 1e16
	// and scientific notation outside that interval.
	if af >= 1e-4 && af < 1e16 {
		s = strconv.FormatFloat(f, 'f', -1, 64)
	} else {
		s = strconv.FormatFloat(f, 'e', -1, 64)
	}
	if !containsExponent(s) && !containsDot(s) {
		s += ".0"
	}
	b.WriteString(s)
	return nil
}

func containsExponent(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 'e' || s[i] == 'E' {
			return true
		}
	}
	return false
}
func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

func encodeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r > 0x7f {
				if r <= 0xffff {
					writeU4(b, uint16(r))
				} else {
					for _, u := range utf16.Encode([]rune{r}) {
						writeU4(b, u)
					}
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

func writeU4(b *bytes.Buffer, u uint16) {
	const hex = "0123456789abcdef"
	b.WriteString(`\u`)
	b.WriteByte(hex[(u>>12)&0xf])
	b.WriteByte(hex[(u>>8)&0xf])
	b.WriteByte(hex[(u>>4)&0xf])
	b.WriteByte(hex[u&0xf])
}
