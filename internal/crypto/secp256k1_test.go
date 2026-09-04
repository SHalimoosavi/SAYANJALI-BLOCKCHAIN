package crypto

import (
	"encoding/hex"
	"testing"
)

func TestPrivateOnePublicKey(t *testing.T) {
	priv, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
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
