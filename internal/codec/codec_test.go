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
