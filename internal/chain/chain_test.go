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

func TestConfirmedTransactionIndexRebuildsAcrossReorg(t *testing.T) {
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

	// Branch A becomes active and contains tx A.
	a := mineTestBlock(t, g, "SYJaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1735689631)
	if len(a.Transactions) != 1 || a.Transactions[0].TxHash == "" {
		t.Fatal("branch A transaction hash missing")
	}
	txA := a.Transactions[0].TxHash

	if ok, reason, err := c.Accept(a); err != nil || !ok || reason != "best" {
		t.Fatalf("branch A accept: %v %s", err, reason)
	}

	if !c.HasConfirmedTransaction(txA) {
		t.Fatal("active-chain transaction A was not indexed")
	}

	// Branch B contains a different transaction and is initially a fork.
	b := mineTestBlock(t, g, "SYJbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1735689632)
	if len(b.Transactions) != 1 || b.Transactions[0].TxHash == "" {
		t.Fatal("branch B transaction hash missing")
	}
	txB := b.Transactions[0].TxHash

	if ok, reason, err := c.Accept(b); err != nil || !ok || reason != "fork" {
		t.Fatalf("branch B accept: %v %s", err, reason)
	}

	if c.HasConfirmedTransaction(txB) {
		t.Fatal("losing-fork transaction B incorrectly indexed")
	}

	if !c.HasConfirmedTransaction(txA) {
		t.Fatal("active-chain transaction A disappeared before reorg")
	}

	// Extend B so it wins the active chain.
	b2 := mineTestBlock(t, b, "SYJbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1735689662)
	if ok, reason, err := c.Accept(b2); err != nil || !ok || reason != "best" {
		t.Fatalf("branch B reorg: %v %s", err, reason)
	}

	if c.TipHash() != b2.Hash {
		t.Fatalf("unexpected tip after reorg: %s", c.TipHash())
	}

	if c.HasConfirmedTransaction(txA) {
		t.Fatal("losing-fork transaction A remained indexed after reorg")
	}

	if !c.HasConfirmedTransaction(txB) {
		t.Fatal("new active-chain transaction B was not indexed after reorg")
	}
}

func TestConfirmedTransactionIndexSurvivesRestart(t *testing.T) {
	dir := t.TempDir()

	s, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	c, err := Open(s)
	if err != nil {
		t.Fatal(err)
	}

	g := c.Tip()
	b := mineTestBlock(t, g, "SYJaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1735689631)
	if len(b.Transactions) != 1 || b.Transactions[0].TxHash == "" {
		t.Fatal("transaction hash missing")
	}
	txHash := b.Transactions[0].TxHash

	if ok, reason, err := c.Accept(b); err != nil || !ok || reason != "best" {
		t.Fatalf("block accept: %v %s", err, reason)
	}

	if !c.HasConfirmedTransaction(txHash) {
		t.Fatal("transaction was not indexed before restart")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	c2, err := Open(s2)
	if err != nil {
		t.Fatal(err)
	}

	if !c2.HasConfirmedTransaction(txHash) {
		t.Fatal("confirmed transaction index was not rebuilt after restart")
	}
}
