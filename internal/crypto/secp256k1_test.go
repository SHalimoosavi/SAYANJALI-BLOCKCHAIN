package crypto

import (
	"encoding/hex"
	"math/big"
	"testing"
)

const testPrivateKeyHex = "0000000000000000000000000000000000000000000000000000000000000001"

func TestPrivateOnePublicKey(t *testing.T) {
	priv, _ := hex.DecodeString(testPrivateKeyHex)
	pub, err := PublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(pub)
	want := "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8"
	if got != want {
		t.Fatalf("public key mismatch: got %s want %s", got, want)
	}
}

func TestPrivateKeyValidation(t *testing.T) {
	n := N()
	for _, tc := range []struct {
		name string
		key  []byte
	}{
		{"empty", nil},
		{"short", make([]byte, 31)},
		{"long", make([]byte, 33)},
		{"zero", make([]byte, 32)},
		{"one", mustDecodeHex(testPrivateKeyHex)},
		{"n_minus_one", pad32(new(big.Int).Sub(new(big.Int).Set(n), big.NewInt(1)))},
		{"n", pad32(n)},
		{"n_plus_one", pad32(new(big.Int).Add(new(big.Int).Set(n), big.NewInt(1)))},
		{"malformed_not_representable", []byte("not-a-private-key")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PublicKeyFromPrivate(tc.key)
			wantErr := tc.name != "one" && tc.name != "n_minus_one"
			if (err != nil) != wantErr {
				t.Fatalf("error=%v wantErr=%v", err, wantErr)
			}
		})
	}
}

func TestSignVerifyAndProtocolEncoding(t *testing.T) {
	priv := mustDecodeHex(testPrivateKeyHex)
	pub, err := PublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	message := "sayanjali deterministic crypto remediation"
	sig1, err := SignECDSA(priv, message)
	if err != nil {
		t.Fatal(err)
	}
	sig2, err := SignECDSA(priv, message)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig1) != 64 || len(sig2) != 64 {
		t.Fatalf("raw signatures must be 64 bytes: %d %d", len(sig1), len(sig2))
	}
	if hex.EncodeToString(sig1) != hex.EncodeToString(sig2) {
		t.Fatal("vetted signer is expected to be deterministic")
	}
	if !VerifyECDSA(hex.EncodeToString(pub), message, sig1) {
		t.Fatal("generated signature did not verify")
	}
	if VerifyECDSA(hex.EncodeToString(pub), message+"!", sig1) {
		t.Fatal("signature verified for modified message")
	}
	corrupt := append([]byte(nil), sig1...)
	corrupt[0] ^= 1
	if VerifyECDSA(hex.EncodeToString(pub), message, corrupt) {
		t.Fatal("corrupted signature verified")
	}
	badPub := append([]byte(nil), pub...)
	badPub[10] ^= 1
	if VerifyECDSA(hex.EncodeToString(badPub), message, sig1) {
		t.Fatal("corrupted public key verified")
	}
}

func TestSignatureBoundaryValidation(t *testing.T) {
	priv := mustDecodeHex(testPrivateKeyHex)
	pub, err := PublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	message := "boundary"
	valid, err := SignECDSA(priv, message)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		mut  func([]byte)
	}{
		{"empty", func(b []byte) {}},
		{"r_zero", func(b []byte) {
			for i := 0; i < 32; i++ {
				b[i] = 0
			}
		}},
		{"s_zero", func(b []byte) {
			for i := 32; i < 64; i++ {
				b[i] = 0
			}
		}},
		{"r_n", func(b []byte) { copy(b[:32], pad32(N())) }},
		{"s_n", func(b []byte) { copy(b[32:], pad32(N())) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "empty" {
				if VerifyECDSA(hex.EncodeToString(pub), message, nil) {
					t.Fatal("empty signature verified")
				}
				return
			}
			candidate := append([]byte(nil), valid...)
			tc.mut(candidate)
			if VerifyECDSA(hex.EncodeToString(pub), message, candidate) {
				t.Fatal("invalid boundary signature verified")
			}
		})
	}
}

func TestHighSAcceptedByProtocol(t *testing.T) {
	priv := mustDecodeHex(testPrivateKeyHex)
	pub, err := PublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	message := "high-s compatibility"
	sig, err := SignECDSA(priv, message)
	if err != nil {
		t.Fatal(err)
	}
	s := new(big.Int).SetBytes(sig[32:])
	highS := new(big.Int).Sub(N(), s)
	if highS.Sign() <= 0 || highS.Cmp(N()) >= 0 {
		t.Fatal("failed to construct high-S value")
	}
	copy(sig[32:], pad32(highS))
	if !VerifyECDSA(hex.EncodeToString(pub), message, sig) {
		t.Fatal("protocol verifier rejected a valid high-S ECDSA signature")
	}
}

func TestMalformedPublicKeysNeverPanic(t *testing.T) {
	message := "malformed public key"
	sig := make([]byte, 64)
	for _, pub := range [][]byte{
		nil,
		make([]byte, 1),
		make([]byte, 63),
		make([]byte, 65),
		make([]byte, 64),
	} {
		if VerifyECDSA(hex.EncodeToString(pub), message, sig) {
			t.Fatal("malformed public key unexpectedly verified")
		}
	}
}

func FuzzVerifyECDSAHex(f *testing.F) {
	priv := mustDecodeHex(testPrivateKeyHex)
	pub, err := PublicKeyFromPrivate(priv)
	if err != nil {
		f.Fatal(err)
	}
	valid, err := SignECDSA(priv, "fuzz-seed")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(hex.EncodeToString(pub), "fuzz-seed", hex.EncodeToString(valid))
	f.Fuzz(func(t *testing.T, publicKeyHex, message, signatureHex string) {
		_ = VerifyECDSAHex(publicKeyHex, message, signatureHex)
	})
}

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func pad32(n *big.Int) []byte {
	out := make([]byte, 32)
	n.FillBytes(out)
	return out
}
