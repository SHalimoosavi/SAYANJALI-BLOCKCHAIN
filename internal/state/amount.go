package state

import (
	"errors"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
	"math/big"
	"strings"
)

func ToBaseUnits(value string) (uint64, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return 0, errors.New("SYJ amount is required")
	}
	if strings.HasPrefix(s, "-") {
		return 0, errors.New("SYJ amount must not be negative")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid SYJ amount")
	}
	whole := parts[0]
	if whole == "" {
		whole = "0"
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > 8 {
		return 0, errors.New("SYJ amount supports at most 8 decimal places")
	}
	for len(frac) < 8 {
		frac += "0"
	}
	if whole == "" {
		whole = "0"
	}
	for _, r := range whole + frac {
		if r < '0' || r > '9' {
			return 0, errors.New("invalid SYJ amount")
		}
	}
	n := new(big.Int)
	if _, ok := n.SetString(whole+frac, 10); !ok {
		return 0, errors.New("invalid SYJ amount")
	}
	if n.Sign() < 0 || n.BitLen() > 64 {
		return 0, errors.New("SYJ amount out of range")
	}
	u := n.Uint64()
	if u > protocol.MaxSupplyBaseUnits {
		return 0, errors.New("SYJ amount exceeds the maximum SYJ supply")
	}
	return u, nil
}
func FromBaseUnits(units uint64) string {
	whole := units / protocol.BaseUnitsPerSYJ
	frac := units % protocol.BaseUnitsPerSYJ
	if frac == 0 {
		return big.NewInt(0).SetUint64(whole).String()
	}
	return big.NewInt(0).SetUint64(whole).String() + "." + trimRightZeros(fmt8(frac))
}
func fmt8(v uint64) string {
	s := big.NewInt(0).SetUint64(v).String()
	for len(s) < 8 {
		s = "0" + s
	}
	return s
}
func FormatAmount(units uint64) string { return FromBaseUnits(units) }
func ValidateBaseUnits(units uint64, allowZero bool) error {
	if units == 0 && !allowZero {
		return errors.New("SYJ amount must be positive")
	}
	if units > protocol.MaxSupplyBaseUnits {
		return errors.New("SYJ amount exceeds the maximum SYJ supply")
	}
	return nil
}
func trimRightZeros(s string) string {
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	return s
}
