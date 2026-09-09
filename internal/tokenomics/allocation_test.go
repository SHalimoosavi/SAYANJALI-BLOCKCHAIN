package tokenomics

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func validAllocation() GenesisAllocation {
	return GenesisAllocation{
		Version: Version,
		Sender:  protocol.GenesisAllocationSender,
		Allocations: []Allocation{
			{Category: Presale, Recipient: "presale", AmountBaseUnits: 7_200_000_000_000_000},
			{Category: Treasury, Recipient: "treasury", AmountBaseUnits: 5_760_000_000_000_000},
			{Category: Ecosystem, Recipient: "ecosystem", AmountBaseUnits: 3_600_000_000_000_000},
			{Category: Liquidity, Recipient: "liquidity", AmountBaseUnits: 4_320_000_000_000_000},
			{Category: Team, Recipient: "team", AmountBaseUnits: 7_920_000_000_000_000},
		},
		TotalBaseUnits: ExpectedGenesisTotal(),
	}
}

func TestValidateExactAllocation(t *testing.T) {
	g := validAllocation()
	if err := g.ValidateStructure(); err != nil {
		t.Fatal(err)
	}
	if err := g.Validate(func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsDuplicateCategory(t *testing.T) {
	g := validAllocation()
	g.Allocations[1].Category = Presale
	if err := g.ValidateStructure(); err == nil {
		t.Fatal("accepted duplicate category")
	}
}

func TestValidateRejectsWrongAmount(t *testing.T) {
	g := validAllocation()
	g.Allocations[0].AmountBaseUnits++
	if err := g.ValidateStructure(); err == nil {
		t.Fatal("accepted incorrect amount")
	}
}

func TestValidateRejectsWrongTotal(t *testing.T) {
	g := validAllocation()
	g.TotalBaseUnits++
	if err := g.ValidateStructure(); err == nil {
		t.Fatal("accepted incorrect declared total")
	}
}

func TestValidateRejectsMissingAddressValidator(t *testing.T) {
	if err := validAllocation().Validate(nil); err == nil {
		t.Fatal("accepted nil address validator")
	}
}

func TestSupplyPlan(t *testing.T) {
	p, err := NewSupplyPlan(ExpectedGenesisTotal())
	if err != nil {
		t.Fatal(err)
	}
	if p.RemainingMining != MiningAllocationBaseUnits() {
		t.Fatalf("remaining mining = %d", p.RemainingMining)
	}
	if err := p.ValidateMiningIssued(MiningAllocationBaseUnits()); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateMiningIssued(MiningAllocationBaseUnits() + 1); err == nil {
		t.Fatal("accepted mining issuance beyond allocation")
	}
	if p.InitialSupply+p.RemainingMining != protocol.MaxSupplyBaseUnits {
		t.Fatal("maximum supply invariant failed")
	}
}

func TestGenesisStateCommitmentIsOrderIndependent(t *testing.T) {
	g := validAllocation()
	s := GenesisState(g)
	a, err := s.Commitment()
	if err != nil {
		t.Fatal(err)
	}
	s.Allocations[0], s.Allocations[4] = s.Allocations[4], s.Allocations[0]
	b, err := s.Commitment()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("commitment changed with allocation order: %s != %s", a, b)
	}
}

func TestGenesisStateCanonicalBytesAreDeterministic(t *testing.T) {
	b1, err := GenesisState(validAllocation()).CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	b2, err := GenesisState(validAllocation()).CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Fatal("canonical genesis-state bytes changed between identical constructions")
	}
}
