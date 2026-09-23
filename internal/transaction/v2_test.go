package transaction

import (
	"encoding/hex"
	"math"
	"testing"
)

const auditedNetworkID = "237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3"

func primaryV2(t *testing.T) Transaction {
	t.Helper()
	return Transaction{
		Version:         V2Version,
		NetworkID:       auditedNetworkID,
		Sender:          "SYJaf533e027d9d6ccc0958c470c26ea9d96a9b76fd",
		Receiver:        "SYJ1111111111111111111111111111111111111111",
		AmountBaseUnits: 123456789,
		Timestamp:       1735689601.25,
		Nonce:           0,
		SenderPublicKey: "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8",
		Signature:       "c70c07e257a8b5f45091dea4ec0ceee24db186e9fa3eb8ca2ed777b670e082d0540b1604a77a0da1499a5068a01c39314455d42c3d4ad5aa46d7417e6c26a0a9",
		TxID:            "f2d1b0239244c5c1b70805b15eba42a9e99ca32c764f17e267398136159136ac",
	}
}

func TestV2PrimaryVector(t *testing.T) {
	tx := primaryV2(t)
	if tx.SenderPublicKey != "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8" {
		t.Fatal("public key mismatch")
	}
	if tx.TxID != "f2d1b0239244c5c1b70805b15eba42a9e99ca32c764f17e267398136159136ac" {
		t.Fatalf("tx id = %s", tx.TxID)
	}
	digest, err := tx.V2SigningDigest()
	if err != nil {
		t.Fatal(err)
	}
	if digest != "edbfdb74b27736cb4e1d09f9f641b8304d4c2da560e8fa1adc4b767a52574a31" {
		t.Fatalf("digest = %s", digest)
	}
	if tx.Signature != "c70c07e257a8b5f45091dea4ec0ceee24db186e9fa3eb8ca2ed777b670e082d0540b1604a77a0da1499a5068a01c39314455d42c3d4ad5aa46d7417e6c26a0a9" {
		t.Fatalf("signature = %s", tx.Signature)
	}
	if err := tx.ValidateV2(auditedNetworkID); err != nil {
		t.Fatal(err)
	}
}

func TestV2TxIDIgnoresSignature(t *testing.T) {
	tx := primaryV2(t)
	want := tx.TxID
	tx.Signature = ""
	got, err := tx.ComputeV2TxID()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("tx id changed with signature: %s != %s", got, want)
	}
}

func TestV2RejectsWrongNetwork(t *testing.T) {
	tx := primaryV2(t)
	tx.NetworkID = "539059f933550ccd507fbd8d8791df0671a1e77ea54e7c2f857c808c47660e72"
	if err := tx.ValidateV2(auditedNetworkID); err == nil {
		t.Fatal("wrong network accepted")
	}
}

func TestV2RejectsWrongVersion(t *testing.T) {
	tx := primaryV2(t)
	tx.Version = 0
	if err := tx.ValidateV2(auditedNetworkID); err == nil {
		t.Fatal("wrong version accepted")
	}
}

func TestV2RejectsNonFiniteTimestamp(t *testing.T) {
	tx := primaryV2(t)
	tx.Timestamp = math.NaN()
	if err := tx.ValidateV2(auditedNetworkID); err == nil {
		t.Fatal("NaN timestamp accepted")
	}
	tx.Timestamp = math.Inf(1)
	if err := tx.ValidateV2(auditedNetworkID); err == nil {
		t.Fatal("Infinity timestamp accepted")
	}
}

func TestV2RejectsSenderMismatch(t *testing.T) {
	tx := primaryV2(t)
	tx.Sender = "SYJ1111111111111111111111111111111111111111"
	if err := tx.ValidateV2(auditedNetworkID); err == nil {
		t.Fatal("sender mismatch accepted")
	}
}

func TestV2RejectsSignatureMismatch(t *testing.T) {
	tx := primaryV2(t)
	raw, _ := hex.DecodeString(tx.Signature)
	raw[0] ^= 1
	tx.Signature = hex.EncodeToString(raw)
	if err := tx.ValidateV2(auditedNetworkID); err == nil {
		t.Fatal("signature mismatch accepted")
	}
}

func TestV2CoinbaseDoesNotConsumeNormalNonce(t *testing.T) {
	tx, err := NewV2Coinbase("SYJ1111111111111111111111111111111111111111", auditedNetworkID, 5_000_000_000, 1735689631.25)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Nonce != 0 || tx.Signature != "" || tx.SenderPublicKey != "" {
		t.Fatal("coinbase has normal transfer nonce/signature state")
	}
	want, err := tx.ComputeV2TxID()
	if err != nil {
		t.Fatal(err)
	}
	if tx.TxID != want {
		t.Fatal("coinbase tx id mismatch")
	}
}
