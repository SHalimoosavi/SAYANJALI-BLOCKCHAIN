package compatibility_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
)

type canonicalOracleCase struct {
	Name      string `json:"name"`
	ValueJSON string `json:"value_json"`
	Expected  string `json:"expected"`
}

func loadCanonicalOracle(t *testing.T) []canonicalOracleCase {
	t.Helper()
	b, err := os.ReadFile("fixtures/canonical_python_oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []canonicalOracleCase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func decodeOracleValue(raw string) (any, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return convertOracleNumbers(value)
}

func convertOracleNumbers(v any) (any, error) {
	switch x := v.(type) {
	case json.Number:
		text := string(x)
		if strings.ContainsAny(text, ".eE") {
			f, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, err
			}
			return f, nil
		}
		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			return i, nil
		}
		u, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return nil, err
		}
		return u, nil
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			converted, err := convertOracleNumbers(item)
			if err != nil {
				return nil, err
			}
			out[i] = converted
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for key, item := range x {
			converted, err := convertOracleNumbers(item)
			if err != nil {
				return nil, err
			}
			out[key] = converted
		}
		return out, nil
	default:
		return v, nil
	}
}

func TestCanonicalPythonOracle(t *testing.T) {
	cases := loadCanonicalOracle(t)
	matched := 0
	for _, tc := range cases {
		value, err := decodeOracleValue(tc.ValueJSON)
		if err != nil {
			t.Fatalf("%s decode: %v", tc.Name, err)
		}
		got, err := canonicaljson.String(value)
		if err != nil {
			t.Fatalf("%s encode: %v", tc.Name, err)
		}
		if got != tc.Expected {
			t.Fatalf("%s Python/Go mismatch\nPYTHON: %s\nGO:     %s", tc.Name, tc.Expected, got)
		}
		matched++
	}
	t.Logf("Python oracle differential serialization: %d/%d cases matched byte-for-byte", matched, len(cases))
}

func TestCanonicalSpecialFloatBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		want  string
	}{
		{"positive_zero", 0.0, "0.0"},
		{"negative_zero", math.Copysign(0, -1), "-0.0"},
		{"one", 1.0, "1.0"},
		{"minus_one", -1.0, "-1.0"},
		{"fixed_boundary", 1e-4, "0.0001"},
		{"fixed_below_boundary", 0.00009999999999999999, "9.999999999999999e-05"},
		{"scientific_boundary", 1e16, "1e+16"},
		{"fixed_below_scientific_boundary", 1e15, "1000000000000000.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicaljson.String(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestCanonicalUnsupportedTypeFails(t *testing.T) {
	_, err := canonicaljson.String(fmt.Errorf("not a protocol value"))
	if err == nil {
		t.Fatal("unsupported value type unexpectedly serialized")
	}
}
