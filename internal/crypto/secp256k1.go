package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	secp256k1ecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// CurveName returns the protocol curve name.
func CurveName() string { return "SECP256k1" }

// P and N expose immutable copies of the secp256k1 field prime and group order
// for protocol-boundary validation and tests. Curve arithmetic is delegated to
// the vetted secp256k1 implementation.
func P() *big.Int { return new(big.Int).Set(secp256k1.Params().P) }
func N() *big.Int { return new(big.Int).Set(secp256k1.Params().N) }

func validatePrivateKey(priv []byte) error {
	if len(priv) != secp256k1.PrivKeyBytesLen {
		return errors.New("private key must be 32 bytes")
	}
	var scalar secp256k1.ModNScalar
	if scalar.SetByteSlice(priv) || scalar.IsZero() {
		return errors.New("private key scalar out of range")
	}
	return nil
}

// PublicKeyFromPrivate returns the frozen SYJ raw X||Y public-key encoding.
func PublicKeyFromPrivate(priv []byte) ([]byte, error) {
	if err := validatePrivateKey(priv); err != nil {
		return nil, err
	}
	key := secp256k1.PrivKeyFromBytes(priv)
	defer key.Zero()
	serialized := key.PubKey().SerializeUncompressed()
	if len(serialized) != secp256k1.PubKeyBytesLenUncompressed || serialized[0] != secp256k1.PubKeyFormatUncompressed {
		return nil, errors.New("unexpected secp256k1 public-key encoding")
	}
	out := make([]byte, 64)
	copy(out, serialized[1:])
	return out, nil
}

func GeneratePrivateKey() ([]byte, error) {
	key, err := secp256k1.GeneratePrivateKeyFromRand(rand.Reader)
	if err != nil {
		return nil, err
	}
	defer key.Zero()
	return append([]byte(nil), key.Serialize()...), nil
}

// ParsePublicKey validates the frozen raw X||Y encoding and returns coordinate
// copies for callers that need protocol-boundary inspection. No curve arithmetic
// is performed here; validation is delegated to the vetted library.
func ParsePublicKey(raw []byte) (*big.Int, *big.Int, error) {
	if len(raw) != 64 {
		return nil, nil, errors.New("public key must be 64 raw bytes")
	}
	sec1 := make([]byte, 65)
	sec1[0] = secp256k1.PubKeyFormatUncompressed
	copy(sec1[1:], raw)
	pub, err := secp256k1.ParsePubKey(sec1)
	if err != nil {
		return nil, nil, err
	}
	return new(big.Int).Set(pub.X()), new(big.Int).Set(pub.Y()), nil
}

func VerifyECDSA(publicKeyHex, message string, signature []byte) bool {
	pub, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pub) != 64 {
		return false
	}
	if len(signature) != 64 {
		return false
	}
	sec1 := make([]byte, 65)
	sec1[0] = secp256k1.PubKeyFormatUncompressed
	copy(sec1[1:], pub)
	publicKey, err := secp256k1.ParsePubKey(sec1)
	if err != nil {
		return false
	}

	var r, s secp256k1.ModNScalar
	if r.SetByteSlice(signature[:32]) || r.IsZero() {
		return false
	}
	if s.SetByteSlice(signature[32:]) || s.IsZero() {
		return false
	}
	sig := secp256k1ecdsa.NewSignature(&r, &s)
	hash := SHA256Bytes([]byte(message))
	return sig.Verify(hash[:], publicKey)
}

func VerifyECDSAHex(publicKeyHex, message, signatureHex string) bool {
	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	return VerifyECDSA(publicKeyHex, message, sig)
}

// SignECDSA signs a message and returns the frozen raw R||S encoding. The
// underlying implementation uses RFC6979 deterministic nonces and canonical
// low-S signatures; this does not change the protocol representation or the
// verification rules for existing signatures.
func SignECDSA(priv []byte, message string) ([]byte, error) {
	if err := validatePrivateKey(priv); err != nil {
		return nil, err
	}
	key := secp256k1.PrivKeyFromBytes(priv)
	defer key.Zero()
	hash := SHA256Bytes([]byte(message))
	sig := secp256k1ecdsa.Sign(key, hash[:])
	var rBytes, sBytes [32]byte
	rScalar := sig.R()
	sScalar := sig.S()
	rScalar.PutBytes(&rBytes)
	sScalar.PutBytes(&sBytes)
	out := make([]byte, 64)
	copy(out[:32], rBytes[:])
	copy(out[32:], sBytes[:])
	return out, nil
}
