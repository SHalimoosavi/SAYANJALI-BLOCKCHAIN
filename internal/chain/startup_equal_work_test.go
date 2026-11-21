package chain

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/networkid"
)

func TestStartupEqualWorkBranchesChooseFirstJournalEntryAcross50Reopens(t *testing.T) {
	for iteration := 0; iteration < 50; iteration++ {
		dir := t.TempDir()
		_, fs, genesisState, key, network := newAcceptFaultChain(t, dir, 0)
		genesis, err := fs.Store.GetBlock(mustGenesisHash(t))
		if err != nil {
			t.Fatal(err)
		}

		first := mineAcceptFaultBlock(t, genesis, key, network, 0, 31)
		second := mineAcceptFaultBlock(t, genesis, key, network, 0, 32)
		firstWork := work([]*block.Block{genesis, first})
		secondWork := work([]*block.Block{genesis, second})
		if firstWork.Cmp(secondWork) != 0 {
			t.Fatalf("iteration %d: branch work differs: %s != %s", iteration, firstWork, secondWork)
		}
		if err := fs.Store.SaveBlock(first); err != nil {
			t.Fatal(err)
		}
		if err := fs.Store.SaveBlock(second); err != nil {
			t.Fatal(err)
		}
		if fs.Store.Tip() != genesis.Hash {
			t.Fatalf("iteration %d: durable tip changed before restart", iteration)
		}
		if err := fs.Store.Close(); err != nil {
			t.Fatal(err)
		}

		reopened, err := storage.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		wireName, err := networkid.WireNetworkName(network)
		if err != nil {
			_ = reopened.Close()
			t.Fatal(err)
		}
		recovered, err := OpenV2WithGenesisState(reopened, genesisState, network, wireName)
		if err != nil {
			_ = reopened.Close()
			t.Fatalf("iteration %d: reopen chain: %v", iteration, err)
		}
		if recovered.TipHash() != first.Hash {
			_ = reopened.Close()
			t.Fatalf("iteration %d: selected %s, want first journal branch %s", iteration, recovered.TipHash(), first.Hash)
		}
		if err := reopened.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func mustGenesisHash(t *testing.T) string {
	t.Helper()
	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	return genesis.Hash
}
