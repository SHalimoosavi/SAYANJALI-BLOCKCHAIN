package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
)

var (
	secpP, _  = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F", 16)
	secpN, _  = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	secpGx, _ = new(big.Int).SetString("79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798", 16)
	secpGy, _ = new(big.Int).SetString("483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8", 16)
)

func CurveName() string { return "SECP256k1" }
func P() *big.Int       { return new(big.Int).Set(secpP) }
func N() *big.Int       { return new(big.Int).Set(secpN) }

func PublicKeyFromPrivate(priv []byte) ([]byte, error) {
	if len(priv) != 32 {
		return nil, errors.New("private key must be 32 bytes")
	}
	d := new(big.Int).SetBytes(priv)
	if d.Sign() <= 0 || d.Cmp(secpN) >= 0 {
		return nil, errors.New("private key scalar out of range")
	}
	x, y := scalarBaseMult(d)
	out := make([]byte, 64)
	x.FillBytes(out[:32])
	y.FillBytes(out[32:])
	return out, nil
}

func GeneratePrivateKey() ([]byte, error) {
	for {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		d := new(big.Int).SetBytes(b)
		if d.Sign() > 0 && d.Cmp(secpN) < 0 {
			return b, nil
		}
	}
}

func ParsePublicKey(raw []byte) (*big.Int, *big.Int, error) {
	if len(raw) != 64 {
		return nil, nil, errors.New("public key must be 64 raw bytes")
	}
	x := new(big.Int).SetBytes(raw[:32])
	y := new(big.Int).SetBytes(raw[32:])
	if !isOnCurve(x, y) {
		return nil, nil, errors.New("public key is not on secp256k1")
	}
	return x, y, nil
}

func VerifyECDSA(publicKeyHex, message string, signature []byte) bool {
	pub, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return false
	}
	x, y, err := ParsePublicKey(pub)
	if err != nil {
		return false
	}
	if len(signature) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if r.Sign() <= 0 || r.Cmp(secpN) >= 0 || s.Sign() <= 0 || s.Cmp(secpN) >= 0 {
		return false
	}
	zBytes := SHA256Bytes([]byte(message))
	z := new(big.Int).SetBytes(zBytes[:])
	w := new(big.Int).ModInverse(s, secpN)
	if w == nil {
		return false
	}
	u1 := new(big.Int).Mul(z, w)
	u1.Mod(u1, secpN)
	u2 := new(big.Int).Mul(r, w)
	u2.Mod(u2, secpN)
	x1, y1 := scalarBaseMult(u1)
	x2, y2 := scalarMult(x, y, u2)
	xr, _ := pointAdd(x1, y1, x2, y2)
	if xr == nil {
		return false
	}
	xr.Mod(xr, secpN)
	return xr.Cmp(r) == 0
}

func VerifyECDSAHex(publicKeyHex, message, signatureHex string) bool {
	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	return VerifyECDSA(publicKeyHex, message, sig)
}

func scalarBaseMult(k *big.Int) (*big.Int, *big.Int) { return scalarMult(secpGx, secpGy, k) }

// ScalarBaseMultForSigning exposes the base-point multiplication needed by
// the wallet signer without exposing mutable curve parameters.
func ScalarBaseMultForSigning(k *big.Int) (*big.Int, *big.Int) { return scalarBaseMult(k) }

func scalarMult(px, py, k *big.Int) (*big.Int, *big.Int) {
	if k.Sign() == 0 {
		return nil, nil
	}
	x, y := new(big.Int).Set(px), new(big.Int).Set(py)
	rx, ry := (*big.Int)(nil), (*big.Int)(nil)
	for i := k.BitLen() - 1; i >= 0; i-- {
		if rx != nil {
			rx, ry = pointDouble(rx, ry)
		}
		if k.Bit(i) == 1 {
			if rx == nil {
				rx, ry = new(big.Int).Set(x), new(big.Int).Set(y)
			} else {
				rx, ry = pointAddFull(rx, ry, x, y)
			}
		}
	}
	return rx, ry
}

func pointDouble(x1, y1 *big.Int) (*big.Int, *big.Int) {
	if y1.Sign() == 0 {
		return nil, nil
	}
	threeX2 := new(big.Int).Mul(x1, x1)
	threeX2.Mul(threeX2, big.NewInt(3))
	threeX2.Mod(threeX2, secpP)
	den := new(big.Int).Mul(y1, big.NewInt(2))
	den.Mod(den, secpP)
	den.ModInverse(den, secpP)
	lam := new(big.Int).Mul(threeX2, den)
	lam.Mod(lam, secpP)
	x3 := new(big.Int).Mul(lam, lam)
	twoX := new(big.Int).Mul(x1, big.NewInt(2))
	x3.Sub(x3, twoX)
	x3.Mod(x3, secpP)
	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(lam, y3)
	y3.Sub(y3, y1)
	y3.Mod(y3, secpP)
	return x3, y3
}

func pointAdd(x1, y1, x2, y2 *big.Int) (*big.Int, error) {
	x, _ := pointAddFull(x1, y1, x2, y2)
	if x == nil {
		return nil, nil
	}
	return x, nil
}
func pointAddFull(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	if x1 == nil {
		if x2 == nil {
			return nil, nil
		}
		return new(big.Int).Set(x2), new(big.Int).Set(y2)
	}
	if x2 == nil {
		return new(big.Int).Set(x1), new(big.Int).Set(y1)
	}
	if x1.Cmp(x2) == 0 {
		if y1.Cmp(y2) == 0 {
			return pointDouble(x1, y1)
		}
		return nil, nil
	}
	num := new(big.Int).Sub(y2, y1)
	num.Mod(num, secpP)
	den := new(big.Int).Sub(x2, x1)
	den.Mod(den, secpP)
	den.ModInverse(den, secpP)
	lam := new(big.Int).Mul(num, den)
	lam.Mod(lam, secpP)
	x3 := new(big.Int).Mul(lam, lam)
	x3.Sub(x3, x1)
	x3.Sub(x3, x2)
	x3.Mod(x3, secpP)
	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(lam, y3)
	y3.Sub(y3, y1)
	y3.Mod(y3, secpP)
	return x3, y3
}

func isOnCurve(x, y *big.Int) bool {
	if x.Sign() < 0 || x.Cmp(secpP) >= 0 || y.Sign() < 0 || y.Cmp(secpP) >= 0 {
		return false
	}
	lhs := new(big.Int).Mul(y, y)
	lhs.Mod(lhs, secpP)
	rhs := new(big.Int).Mul(x, x)
	rhs.Mul(rhs, x)
	rhs.Add(rhs, big.NewInt(7))
	rhs.Mod(rhs, secpP)
	return lhs.Cmp(rhs) == 0
}
