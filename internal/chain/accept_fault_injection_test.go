package chain

import (
	"errors"
	"sync"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/networkid"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
)

var errInjectedStoreFailure = errors.New("injected storage failure")

type acceptFaultStore struct {
	*storage.Store
	failSaveBlock int
	failSetTip    int
}

func (s *acceptFaultStore) SaveBlock(b *block.Block) error {
	if s.failSaveBlock > 0 {
		s.failSaveBlock--
		return errInjectedStoreFailure
	}
	return s.Store.SaveBlock(b)
}

func (s *acceptFaultStore) SetTip(hash string) error {
	if s.failSetTip > 0 {
		s.failSetTip--
		return errInjectedStoreFailure
	}
	return s.Store.SetTip(hash)
}

func newAcceptFaultChain(t *testing.T, dir string, failSetTip int) (*Chain, *acceptFaultStore, tokenomics.GenesisState, *wallet.KeyPair, string) {
	t.Helper()
	raw, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = raw.Close()
	})
	gs, key, network := v2TestState(t)
	wireName, err := networkid.WireNetworkName(network)
	if err != nil {
		t.Fatal(err)
	}
	fs := &acceptFaultStore{Store: raw}
	c, err := openWithProtocolAndClock(fs, &gs, 2, network, wireName, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Arm the one-shot failure only after genesis has been initialized.
	fs.failSetTip = failSetTip
	return c, fs, gs, key, network
}

func mineAcceptFaultBlock(t *testing.T, parent *block.Block, key *wallet.KeyPair, network string, nonce uint64, timestampStep float64) *block.Block {
	t.Helper()
	timestamp := parent.Timestamp + timestampStep
	reward, ok := consensus.ExpectedMiningReward(0)
	if !ok {
		t.Fatal("expected mining reward")
	}
	receiver := "SYJ1111111111111111111111111111111111111111"
	coinbase, err := transaction.NewV2Coinbase(receiver, network, reward, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	tx := transaction.Transaction{
		Version:         transaction.V2Version,
		NetworkID:       network,
		Sender:          key.Address,
		Receiver:        receiver,
		AmountBaseUnits: 1,
		Timestamp:       timestamp,
		Nonce:           nonce,
	}
	if err := tx.SignV2(key, network); err != nil {
		t.Fatal(err)
	}
	b, err := block.New(parent.Index+1, parent.Hash, timestamp, 0, 4, []transaction.Transaction{coinbase, tx})
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
	t.Fatal("unable to mine fault-injection test block")
	return nil
}

func requireBestAccept(t *testing.T, c *Chain, b *block.Block) {
	t.Helper()
	ok, reason, err := c.Accept(b)
	if err != nil || !ok || reason != "best" {
		t.Fatalf("Accept() = (%v, %q, %v), want (true, best, nil)", ok, reason, err)
	}
}

func TestAcceptRetriesPromotionAfterSetTipFailure(t *testing.T) {
	c, _, _, key, network := newAcceptFaultChain(t, t.TempDir(), 1)
	initialRoot := c.StateRoot()
	initialBalance := c.Balance(key.Address)
	b := mineAcceptFaultBlock(t, c.Tip(), key, network, 0, 31)

	if ok, reason, err := c.Accept(b); err == nil || ok || reason != "storage" {
		t.Fatalf("first Accept() = (%v, %q, %v), want storage failure", ok, reason, err)
	}
	if !c.HasBlock(b.Hash) {
		t.Fatal("block should remain durably stored after SetTip failure")
	}
	if c.TipHash() == b.Hash || c.StateRoot() != initialRoot || c.Balance(key.Address) != initialBalance || c.NextNonce(key.Address) != 0 {
		t.Fatal("active in-memory state changed before durable tip update")
	}

	requireBestAccept(t, c, b)
	if c.TipHash() != b.Hash || c.NextNonce(key.Address) != 1 || c.Balance(key.Address) != initialBalance-1 || c.StateRoot() == initialRoot {
		t.Fatal("retry did not promote the persisted block and publish its state")
	}
}

func TestAcceptRestartReconcilesPersistedBestWorkTipAndState(t *testing.T) {
	dir := t.TempDir()
	c, fs, gs, key, network := newAcceptFaultChain(t, dir, 1)
	initialRoot := c.StateRoot()
	initialBalance := c.Balance(key.Address)
	b := mineAcceptFaultBlock(t, c.Tip(), key, network, 0, 31)
	if ok, reason, err := c.Accept(b); err == nil || ok || reason != "storage" {
		t.Fatalf("Accept() = (%v, %q, %v), want storage failure", ok, reason, err)
	}
	if err := fs.Store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	wireName, err := networkid.WireNetworkName(network)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := OpenV2WithGenesisState(reopened, gs, network, wireName)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.TipHash() != b.Hash {
		t.Fatalf("recovered tip=%s, want persisted best-work block %s", recovered.TipHash(), b.Hash)
	}
	if recovered.StateRoot() == "" || recovered.StateRoot() == initialRoot {
		t.Fatal("recovered state root does not represent the recovered block")
	}
	if recovered.NextNonce(key.Address) != 1 || recovered.Balance(key.Address) != initialBalance-1 {
		t.Fatalf("recovered balance/nonce = %d/%d, want %d/1", recovered.Balance(key.Address), recovered.NextNonce(key.Address), initialBalance-1)
	}
	if recovered.TipHash() != reopened.Tip() {
		t.Fatal("recovered in-memory tip and durable tip record disagree")
	}
}

func TestAcceptSaveBlockFailureDoesNotInsertAndRetryWorks(t *testing.T) {
	c, fs, _, key, network := newAcceptFaultChain(t, t.TempDir(), 0)
	b := mineAcceptFaultBlock(t, c.Tip(), key, network, 0, 31)
	fs.failSaveBlock = 1
	if ok, reason, err := c.Accept(b); err == nil || ok || reason != "storage" {
		t.Fatalf("first Accept() = (%v, %q, %v), want SaveBlock failure", ok, reason, err)
	}
	if c.HasBlock(b.Hash) {
		t.Fatal("block was inserted into in-memory map after SaveBlock failed")
	}
	requireBestAccept(t, c, b)
	if c.TipHash() != b.Hash {
		t.Fatal("retry after SaveBlock failure did not promote block")
	}
}

func TestAcceptFullyActiveBlockStillReturnsDuplicate(t *testing.T) {
	c, _, _, key, network := newAcceptFaultChain(t, t.TempDir(), 0)
	b := mineAcceptFaultBlock(t, c.Tip(), key, network, 0, 31)
	requireBestAccept(t, c, b)
	if ok, reason, err := c.Accept(b); err != nil || ok || reason != "duplicate" {
		t.Fatalf("duplicate Accept() = (%v, %q, %v), want (false, duplicate, nil)", ok, reason, err)
	}
}

func TestAcceptConcurrentSameBlockPromotesExactlyOnce(t *testing.T) {
	c, _, _, key, network := newAcceptFaultChain(t, t.TempDir(), 0)
	b := mineAcceptFaultBlock(t, c.Tip(), key, network, 0, 31)
	const workers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	best, duplicates, failures := 0, 0, 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, reason, err := c.Accept(b)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures++
				return
			}
			if ok && reason == "best" {
				best++
			} else if !ok && reason == "duplicate" {
				duplicates++
			} else {
				failures++
			}
		}()
	}
	wg.Wait()
	if best != 1 || duplicates != workers-1 || failures != 0 {
		t.Fatalf("best=%d duplicates=%d failures=%d", best, duplicates, failures)
	}
	if c.TipHash() != b.Hash {
		t.Fatal("concurrent Accept did not leave the block active")
	}
}

func TestAcceptRetriesFailedHigherWorkReorganisation(t *testing.T) {
	c, fs, _, key, network := newAcceptFaultChain(t, t.TempDir(), 0)
	genesis := c.Tip()
	initialBalance := c.Balance(key.Address)
	a1 := mineAcceptFaultBlock(t, genesis, key, network, 0, 31)
	requireBestAccept(t, c, a1)

	b1 := mineAcceptFaultBlock(t, genesis, key, network, 0, 32)
	if ok, reason, err := c.Accept(b1); err != nil || !ok || reason != "fork" {
		t.Fatalf("fork Accept() = (%v, %q, %v), want fork", ok, reason, err)
	}
	b2 := mineAcceptFaultBlock(t, b1, key, network, 1, 31)
	fs.failSetTip = 1
	if ok, reason, err := c.Accept(b2); err == nil || ok || reason != "storage" {
		t.Fatalf("reorg Accept() = (%v, %q, %v), want storage failure", ok, reason, err)
	}
	if c.TipHash() != a1.Hash || c.NextNonce(key.Address) != 1 || c.Balance(key.Address) != initialBalance-1 {
		t.Fatal("failed reorganisation changed active state")
	}
	requireBestAccept(t, c, b2)
	if c.TipHash() != b2.Hash || c.Height() != 2 || c.NextNonce(key.Address) != 2 || c.Balance(key.Address) != initialBalance-2 {
		t.Fatal("retry did not complete reorganisation with consistent state")
	}
}

func TestOpenPreservesDurableTipWhenStoredForkHasEqualWork(t *testing.T) {
	dir := t.TempDir()
	c, fs, gs, key, network := newAcceptFaultChain(t, dir, 0)
	genesis := c.Tip()
	a1 := mineAcceptFaultBlock(t, genesis, key, network, 0, 31)
	requireBestAccept(t, c, a1)
	b1 := mineAcceptFaultBlock(t, genesis, key, network, 0, 32)
	if ok, reason, err := c.Accept(b1); err != nil || !ok || reason != "fork" {
		t.Fatalf("equal-work fork Accept() = (%v, %q, %v), want fork", ok, reason, err)
	}
	if err := fs.Store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	wireName, err := networkid.WireNetworkName(network)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := OpenV2WithGenesisState(reopened, gs, network, wireName)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.TipHash() != a1.Hash {
		t.Fatalf("equal-work startup reconciliation changed tip to %s, want durable tip %s", recovered.TipHash(), a1.Hash)
	}
}
