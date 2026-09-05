package chain

import (
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
	"testing"
)

func TestOpenCreatesFrozenGenesis(t *testing.T) {
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := Open(s)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := block.Genesis()
	if c.Height() != 0 || c.TipHash() != g.Hash {
		t.Fatalf("height=%d tip=%s", c.Height(), c.TipHash())
	}
	if err := ValidateChain(c.ChainCopy()); err != nil {
		t.Fatal(err)
	}
}

func mineTestBlock(t *testing.T, parent *block.Block, receiver string, timestamp float64) *block.Block {
	t.Helper()
	tx := transaction.Transaction{Sender: "SYJ-COINBASE-0000000000000000000000000000", Receiver: receiver, AmountBaseUnits: 5_000_000_000, Timestamp: timestamp}
	b, err := block.New(parent.Index+1, parent.Hash, timestamp, 0, 4, []transaction.Transaction{tx})
	if err != nil {
		t.Fatal(err)
	}
	for i := uint64(0); i < 5_000_000; i++ {
		if b.MeetsDifficulty(4) {
			return b
		}
		b.Nonce++
		if err := b.Recompute(); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("unable to mine test block")
	return nil
}
func TestHigherWorkForkReorganizes(t *testing.T) {
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := Open(s)
	if err != nil {
		t.Fatal(err)
	}
	g := c.Tip()
	a := mineTestBlock(t, g, "SYJaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1735689631)
	if ok, reason, err := c.Accept(a); err != nil || !ok || reason != "best" {
		t.Fatalf("branch A accept: %v %s", err, reason)
	}
	b := mineTestBlock(t, g, "SYJbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1735689632)
	if ok, reason, err := c.Accept(b); err != nil || !ok || reason != "fork" {
		t.Fatalf("branch B accept: %v %s", err, reason)
	}
	b2 := mineTestBlock(t, b, "SYJbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1735689662)
	if ok, reason, err := c.Accept(b2); err != nil || !ok || reason != "best" {
		t.Fatalf("branch B reorg: %v %s", err, reason)
	}
	if c.TipHash() != b2.Hash || c.Height() != 2 {
		t.Fatalf("best chain not reorganized: height=%d tip=%s", c.Height(), c.TipHash())
	}
}

func TestRequiredNextDifficultyBeforeRetargetWindow(t *testing.T) {
	cfg := protocol.DefaultDifficultyConfig()
	gen, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	b1 := &block.Block{Header: block.Header{Index: 1, Difficulty: 4, Timestamp: 100.5, PreviousHash: gen.Hash}}
	b2 := &block.Block{Header: block.Header{Index: 2, Difficulty: 4, Timestamp: 200.75, PreviousHash: b1.Hash}}
	for _, b := range []*block.Block{gen, b1, b2} {
		if b.Hash == "" {
			if err := b.Recompute(); err != nil {
				t.Fatal(err)
			}
		}
	}
	d, err := requiredNextDifficulty([]*block.Block{gen, b1, b2}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if d != 4 {
		t.Fatalf("difficulty=%d, want 4", d)
	}
}
