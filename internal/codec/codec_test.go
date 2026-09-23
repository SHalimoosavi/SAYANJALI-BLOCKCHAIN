package codec

import (
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"testing"
)

func TestBlockRoundTripPreservesConsensusFields(t *testing.T) {
	tx := transaction.Transaction{Sender: "SYJ-COINBASE-0000000000000000000000000000", Receiver: "SYJaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AmountBaseUnits: 5_000_000_000, Timestamp: 1735689631.25}
	b, err := block.New(1, "5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b", 1735689631.25, 104, 2, []transaction.Transaction{tx})
	if err != nil {
		t.Fatal(err)
	}
	if b.Transactions[0].TxHash == "" {
		t.Fatal("block constructor did not materialize transaction hash")
	}
	if err != nil {
		t.Fatal(err)
	}
	raw, err := BlockBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBlock(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hash != b.Hash || got.MerkleRoot != b.MerkleRoot || got.Index != b.Index || got.Transactions[0].AmountBaseUnits != tx.AmountBaseUnits {
		t.Fatal("block round-trip changed consensus fields")
	}
}
func TestHeaderBytesRoundTrip(t *testing.T) {
	b, _ := block.Genesis()
	raw, err := HeaderBytes(b.Header)
	if err != nil {
		t.Fatal(err)
	}
	h, err := DecodeHeader(raw)
	if err != nil {
		t.Fatal(err)
	}
	if h.Index != b.Index || h.PreviousHash != b.PreviousHash || h.Timestamp != b.Timestamp || h.Nonce != b.Nonce || h.Difficulty != b.Difficulty || h.MerkleRoot != b.MerkleRoot {
		t.Fatal("header round-trip mismatch")
	}
}

func TestV2TransactionRoundTrip(t *testing.T) {
	tx := transaction.Transaction{
		Version:         2,
		NetworkID:       "237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3",
		Nonce:           0,
		Sender:          "SYJaf533e027d9d6ccc0958c470c26ea9d96a9b76fd",
		Receiver:        "SYJ1111111111111111111111111111111111111111",
		AmountBaseUnits: 123456789,
		Timestamp:       1735689601.25,
		SenderPublicKey: "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8",
		Signature:       "c70c07e257a8b5f45091dea4ec0ceee24db186e9fa3eb8ca2ed777b670e082d0540b1604a77a0da1499a5068a01c39314455d42c3d4ad5aa46d7417e6c26a0a9",
		TxID:            "f2d1b0239244c5c1b70805b15eba42a9e99ca32c764f17e267398136159136ac",
	}
	raw, err := TransactionBytes(tx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeTransaction(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || got.NetworkID != tx.NetworkID || got.Nonce != tx.Nonce || got.TxID != tx.TxID {
		t.Fatalf("V2 transaction round-trip mismatch: %#v", got)
	}
}
