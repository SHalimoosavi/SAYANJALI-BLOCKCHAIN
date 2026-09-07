package chain

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func phase7TestAddress(seed byte) string {
	var pub [64]byte
	for i := range pub {
		pub[i] = seed
	}
	return wallet.AddressFromPublicKeyHex(fmt.Sprintf("%x", pub[:]))
}

func phase7TestState(t *testing.T) tokenomics.GenesisState {
	t.Helper()
	return tokenomics.GenesisState{
		Version: tokenomics.Version,
		Sender:  protocol.GenesisAllocationSender,
		Allocations: []tokenomics.Allocation{
			{Category: tokenomics.Presale, Recipient: phase7TestAddress(1), AmountBaseUnits: 7_200_000_000_000_000},
			{Category: tokenomics.Treasury, Recipient: phase7TestAddress(2), AmountBaseUnits: 5_760_000_000_000_000},
			{Category: tokenomics.Ecosystem, Recipient: phase7TestAddress(3), AmountBaseUnits: 3_600_000_000_000_000},
			{Category: tokenomics.Liquidity, Recipient: phase7TestAddress(4), AmountBaseUnits: 4_320_000_000_000_000},
			{Category: tokenomics.Team, Recipient: phase7TestAddress(5), AmountBaseUnits: 7_920_000_000_000_000},
		},
		TotalBaseUnits: tokenomics.ExpectedGenesisTotal(),
	}
}

func TestPhase7GenesisStateInitializesExactSupplyAndBalances(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gs := phase7TestState(t)
	c, err := OpenWithGenesisState(st, gs)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsPhase7() || c.GenesisSupply() != tokenomics.ExpectedGenesisTotal() || c.MiningIssued() != 0 || c.Supply() != tokenomics.ExpectedGenesisTotal() {
		t.Fatalf("phase7 monetary state incorrect: phase7=%v genesis=%d mined=%d supply=%d", c.IsPhase7(), c.GenesisSupply(), c.MiningIssued(), c.Supply())
	}
	for _, a := range gs.Allocations {
		if got := c.Balance(a.Recipient); got != a.AmountBaseUnits {
			t.Fatalf("balance %s=%d want %d", a.Category, got, a.AmountBaseUnits)
		}
	}
}

func TestPhase7GenesisStateSurvivesRestartWithoutDuplication(t *testing.T) {
	dir := t.TempDir()
	gs := phase7TestState(t)
	st, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := OpenWithGenesisState(st, gs)
	if err != nil {
		t.Fatal(err)
	}
	b := mineTestBlock(t, c.Tip(), gs.Allocations[0].Recipient, 1735689631)
	if ok, reason, err := c.Accept(b); err != nil || !ok || reason != "best" {
		t.Fatalf("accept=%v reason=%s err=%v", ok, reason, err)
	}
	if c.Supply() != tokenomics.ExpectedGenesisTotal()+protocol.DefaultBlockRewardUnits {
		t.Fatalf("supply=%d", c.Supply())
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st2, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	c2, err := OpenWithGenesisState(st2, gs)
	if err != nil {
		t.Fatal(err)
	}
	if c2.GenesisSupply() != tokenomics.ExpectedGenesisTotal() || c2.MiningIssued() != protocol.DefaultBlockRewardUnits || c2.Supply() != tokenomics.ExpectedGenesisTotal()+protocol.DefaultBlockRewardUnits {
		t.Fatalf("restart monetary state incorrect: genesis=%d mined=%d supply=%d", c2.GenesisSupply(), c2.MiningIssued(), c2.Supply())
	}
}

func TestPhase7GenesisStateImmutableAcrossReorg(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gs := phase7TestState(t)
	c, err := OpenWithGenesisState(st, gs)
	if err != nil {
		t.Fatal(err)
	}
	g := c.Tip()
	a := mineTestBlock(t, g, gs.Allocations[0].Recipient, 1735689631)
	if ok, reason, err := c.Accept(a); err != nil || !ok || reason != "best" {
		t.Fatalf("A: %v %s", err, reason)
	}
	b := mineTestBlock(t, g, gs.Allocations[1].Recipient, 1735689632)
	if ok, reason, err := c.Accept(b); err != nil || !ok || reason != "fork" {
		t.Fatalf("B: %v %s", err, reason)
	}
	b2 := mineTestBlock(t, b, gs.Allocations[1].Recipient, 1735689662)
	if ok, reason, err := c.Accept(b2); err != nil || !ok || reason != "best" {
		t.Fatalf("B2: %v %s", err, reason)
	}
	if c.GenesisSupply() != tokenomics.ExpectedGenesisTotal() || c.MiningIssued() != 2*protocol.DefaultBlockRewardUnits || c.Supply() != tokenomics.ExpectedGenesisTotal()+2*protocol.DefaultBlockRewardUnits {
		t.Fatalf("reorg monetary state incorrect: genesis=%d mined=%d supply=%d", c.GenesisSupply(), c.MiningIssued(), c.Supply())
	}
	if c.Balance(gs.Allocations[0].Recipient) != gs.Allocations[0].AmountBaseUnits {
		t.Fatal("genesis allocation changed during reorg")
	}
}

func TestPhase7GenesisAllocationCannotAppearInBlock(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	gs := phase7TestState(t)
	c, err := OpenWithGenesisState(st, gs)
	if err != nil {
		t.Fatal(err)
	}
	bad := transaction.Transaction{Sender: protocol.GenesisAllocationSender, Receiver: gs.Allocations[0].Recipient, AmountBaseUnits: 1, Timestamp: 1735689631}
	b, err := block.New(1, c.TipHash(), bad.Timestamp, 0, 4, []transaction.Transaction{bad})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _, err := c.Accept(b); err == nil || ok {
		t.Fatal("accepted genesis allocation as block transaction")
	}
}

func TestNonPhase7ChainUnaffected(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c, err := Open(st)
	if err != nil {
		t.Fatal(err)
	}
	if c.IsPhase7() || c.GenesisSupply() != 0 || c.MiningIssued() != 0 || c.Supply() != 0 {
		t.Fatalf("legacy chain unexpectedly has Phase 7 state: phase7=%v genesis=%d mined=%d supply=%d", c.IsPhase7(), c.GenesisSupply(), c.MiningIssued(), c.Supply())
	}
}

func TestPhase7GenesisCommitmentIsOrderIndependent(t *testing.T) {
	gs := phase7TestState(t)
	a, err := gs.Commitment()
	if err != nil {
		t.Fatal(err)
	}
	gs.Allocations[0], gs.Allocations[4] = gs.Allocations[4], gs.Allocations[0]
	b, err := gs.Commitment()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("commitment changed with allocation ordering: %s != %s", a, b)
	}
}

func TestPhase7GenesisStateJSONRoundTrip(t *testing.T) {
	gs := phase7TestState(t)
	path := t.TempDir() + "/genesis.json"
	data, err := json.Marshal(gs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := tokenomics.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := gs.Commitment()
	b, _ := loaded.Commitment()
	if a != b {
		t.Fatalf("round-trip commitment mismatch: %s != %s", a, b)
	}
}
