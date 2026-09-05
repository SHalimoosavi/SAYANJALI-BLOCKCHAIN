package node

import (
	"context"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/chain"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
)

func TestNodeIdentityAndChainRecoverAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AdvertisedAddress = ""
	cfg.APIListenAddress = "127.0.0.1:0"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := New(cfg)
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	first := a.Identity()
	if a.Chain().Height() != 0 {
		t.Fatal("unexpected initial height")
	}
	a.Stop()
	b := New(cfg)
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	second := b.Identity()
	if first.NodeID != second.NodeID || first.PrivateKeyHex != second.PrivateKeyHex {
		t.Fatal("identity did not persist")
	}
	if b.Chain().Height() != 0 || b.Chain().TipHash() == "" {
		t.Fatal("chain state did not recover")
	}
}

func TestNextDifficultyFromGenesis(t *testing.T) {
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := chain.Open(s)
	if err != nil {
		t.Fatal(err)
	}
	d, err := nextDifficulty(c)
	if err != nil {
		t.Fatal(err)
	}
	if d != 4 {
		t.Fatalf("difficulty=%d, want 4", d)
	}
}
