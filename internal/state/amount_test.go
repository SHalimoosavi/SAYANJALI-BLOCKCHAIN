package state

import "testing"

func TestBaseUnitConversions(t *testing.T) {
	cases := map[string]uint64{"1": 100000000, "50.0": 5000000000, "0.00000001": 1, "720000000": 72000000000000000}
	for in, want := range cases {
		got, err := ToBaseUnits(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Fatalf("%s: got %d want %d", in, got, want)
		}
	}
	if _, err := ToBaseUnits("0.000000001"); err == nil {
		t.Fatal("accepted >8 decimal places")
	}
	if _, err := ToBaseUnits("720000000.00000001"); err == nil {
		t.Fatal("accepted over-supply amount")
	}
	if FormatAmount(123456789) != "1.23456789" {
		t.Fatal("format mismatch")
	}
}
