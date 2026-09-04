package wallet

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math/big"

	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
)

// Sign returns a raw 64-byte r||s ECDSA signature. The protocol permits a
// non-deterministic ECDSA nonce; signatures are therefore not vectorized.
func (k *KeyPair) Sign(message string) (string, error) {
	priv, err := hex.DecodeString(k.PrivateKeyHex)
	if err != nil {
		return "", err
	}
	d := new(big.Int).SetBytes(priv)
	n := corecrypto.N()
	zBytes := sha256.Sum256([]byte(message))
	z := new(big.Int).SetBytes(zBytes[:])
	for {
		kb := make([]byte, 32)
		if _, err = io.ReadFull(rand.Reader, kb); err != nil {
			return "", err
		}
		kk := new(big.Int).SetBytes(kb)
		if kk.Sign() == 0 || kk.Cmp(n) >= 0 {
			continue
		}
		rx, _ := corecrypto.ScalarBaseMultForSigning(kk)
		r := new(big.Int).Mod(rx, n)
		if r.Sign() == 0 {
			continue
		}
		kinv := new(big.Int).ModInverse(kk, n)
		if kinv == nil {
			continue
		}
		s := new(big.Int).Mul(r, d)
		s.Add(s, z)
		s.Mul(s, kinv)
		s.Mod(s, n)
		if s.Sign() == 0 {
			continue
		}
		out := make([]byte, 64)
		r.FillBytes(out[:32])
		s.FillBytes(out[32:])
		return hex.EncodeToString(out), nil
	}
}

func Verify(publicKeyHex, message, signatureHex string) bool {
	return corecrypto.VerifyECDSAHex(publicKeyHex, message, signatureHex)
}
