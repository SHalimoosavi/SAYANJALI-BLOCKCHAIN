package node

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/chain"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
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

func nodePhase7TestState() tokenomics.GenesisState {
	addr := func(seed byte) string {
		var pub [64]byte
		for i := range pub {
			pub[i] = seed
		}
		return wallet.AddressFromPublicKeyHex(fmt.Sprintf("%x", pub[:]))
	}
	return tokenomics.GenesisState{
		Version: tokenomics.Version, Sender: protocol.GenesisAllocationSender,
		Allocations: []tokenomics.Allocation{
			{Category: tokenomics.Presale, Recipient: addr(1), AmountBaseUnits: 7_200_000_000_000_000},
			{Category: tokenomics.Treasury, Recipient: addr(2), AmountBaseUnits: 5_760_000_000_000_000},
			{Category: tokenomics.Ecosystem, Recipient: addr(3), AmountBaseUnits: 3_600_000_000_000_000},
			{Category: tokenomics.Liquidity, Recipient: addr(4), AmountBaseUnits: 4_320_000_000_000_000},
			{Category: tokenomics.Team, Recipient: addr(5), AmountBaseUnits: 7_920_000_000_000_000},
		}, TotalBaseUnits: tokenomics.ExpectedGenesisTotal(),
	}
}

func TestPhase7GenesisAllocationCannotEnterTransactionSubmission(t *testing.T) {
	dir := t.TempDir()
	n := New(DefaultConfig(dir))
	// The transaction path must reject the reserved genesis sender before it
	// can reach the mempool or any API/P2P propagation path.
	tx := transaction.Transaction{Sender: protocol.GenesisAllocationSender, Receiver: "SYJ0000000000000000000000000000000000000000", AmountBaseUnits: 1, Timestamp: 1, TxHash: "invalid"}
	if err := n.SubmitTransaction(tx); err == nil {
		t.Fatal("accepted genesis allocation as normal transaction")
	}
}

func TestPhase7NodeFailsClosedWithoutGenesisConfiguration(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	cfg.NetworkName = tokenomics.Phase7NetworkName
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AdvertisedAddress = ""
	cfg.APIListenAddress = "127.0.0.1:0"
	n := New(cfg)
	if err := n.Start(context.Background()); err == nil {
		n.Stop()
		t.Fatal("Phase 7 node started without genesis configuration")
	}
}

func TestPhase7NodeFailsClosedOnCommitmentMismatch(t *testing.T) {
	dir := t.TempDir()
	gs := nodePhase7TestState()
	data, err := json.Marshal(gs)
	if err != nil {
		t.Fatal(err)
	}
	path := dir + "/genesis.json"
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig(dir + "/node")
	cfg.NetworkName = tokenomics.Phase7NetworkName
	cfg.Phase7GenesisStatePath = path
	cfg.Phase7GenesisCommitment = "00"
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AdvertisedAddress = ""
	cfg.APIListenAddress = "127.0.0.1:0"
	n := New(cfg)
	if err := n.Start(context.Background()); err == nil {
		n.Stop()
		t.Fatal("Phase 7 node started with mismatched commitment")
	}
}

func TestPhase7NodeLoadsGenesisAndCommitment(t *testing.T) {
	dir := t.TempDir()
	gs := nodePhase7TestState()
	data, err := json.Marshal(gs)
	if err != nil {
		t.Fatal(err)
	}
	path := dir + "/genesis.json"
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	commitment, err := gs.Commitment()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig(dir + "/node")
	cfg.NetworkName = tokenomics.Phase7NetworkName
	cfg.Phase7GenesisStatePath = path
	cfg.Phase7GenesisCommitment = commitment
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AdvertisedAddress = ""
	cfg.APIListenAddress = "127.0.0.1:0"
	n := New(cfg)
	if err := n.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer n.Stop()
	if n.Chain().Supply() != tokenomics.ExpectedGenesisTotal() || n.Chain().MiningIssued() != 0 {
		t.Fatalf("unexpected Phase 7 supply: %d/%d", n.Chain().Supply(), n.Chain().MiningIssued())
	}
}
