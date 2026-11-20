package storage

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
)

func TestAllBlocksPreservesJournalOrderAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	b1, err := block.New(1, genesis.Hash, genesis.Timestamp+31, 0, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := block.New(2, b1.Hash, b1.Timestamp+31, 0, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []*block.Block{genesis, b1, b2} {
		if err := store.SaveBlock(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.AllBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("AllBlocks returned %d blocks, want 3", len(got))
	}
	for i, want := range []*block.Block{genesis, b1, b2} {
		if got[i].Hash != want.Hash {
			t.Fatalf("AllBlocks[%d].Hash=%s, want %s", i, got[i].Hash, want.Hash)
		}
	}
}
