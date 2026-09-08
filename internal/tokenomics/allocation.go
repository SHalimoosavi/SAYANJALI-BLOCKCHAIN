package tokenomics

import (
	"errors"
	"fmt"
	"math"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

// Category is one of the five fixed Phase 7 non-mining genesis allocations.
type Category string

const (
	Presale   Category = "presale"
	Treasury  Category = "treasury"
	Ecosystem Category = "ecosystem"
	Liquidity Category = "liquidity"
	Team      Category = "team"
)

// Allocation is a deterministic genesis-state allocation entry. Recipient is
// an existing SYJ public address; validation is delegated to the repository's
// existing address validator by Validate.
type Allocation struct {
	Category        Category `json:"category"`
	Recipient       string   `json:"recipient"`
	AmountBaseUnits uint64   `json:"amount_base_units"`
}

// GenesisAllocation is the separately versioned Phase 7 tokenomics genesis
// allocation. It is not an ordinary transaction and must not alter the frozen
// Phase 4 genesis identity.
type GenesisAllocation struct {
	Version        uint64       `json:"version"`
	Sender         string       `json:"sender"`
	Allocations    []Allocation `json:"allocations"`
	TotalBaseUnits uint64       `json:"total_base_units"`
}

const Version uint64 = 1

var expectedAmounts = map[Category]uint64{
	Presale:   7_200_000_000_000_000,
	Treasury:  5_760_000_000_000_000,
	Ecosystem: 3_600_000_000_000_000,
	Liquidity: 4_320_000_000_000_000,
	Team:      7_920_000_000_000_000,
}

var expectedOrder = []Category{Presale, Treasury, Ecosystem, Liquidity, Team}

func ExpectedAmounts() map[Category]uint64 {
	out := make(map[Category]uint64, len(expectedAmounts))
	for k, v := range expectedAmounts {
		out[k] = v
	}
	return out
}

func ExpectedCategories() []Category {
	return append([]Category(nil), expectedOrder...)
}

func ExpectedGenesisTotal() uint64      { return 28_800_000_000_000_000 }
func MiningAllocationBaseUnits() uint64 { return 43_200_000_000_000_000 }

// AddressValidator is intentionally injected so the tokenomics package cannot
// accidentally create a second, divergent SYJ address format.
type AddressValidator func(string) bool

func (g GenesisAllocation) ValidateStructure() error {
	if g.Version != Version {
		return fmt.Errorf("unsupported genesis allocation version %d", g.Version)
	}
	if g.Sender != protocol.GenesisAllocationSender {
		return errors.New("invalid GenesisAllocationSender")
	}
	if len(g.Allocations) != len(expectedOrder) {
		return fmt.Errorf("exactly %d genesis allocation entries required", len(expectedOrder))
	}

	seen := make(map[Category]struct{}, len(g.Allocations))
	var total uint64
	for _, a := range g.Allocations {
		if _, ok := expectedAmounts[a.Category]; !ok {
			return fmt.Errorf("unknown allocation category %q", a.Category)
		}
		if _, ok := seen[a.Category]; ok {
			return fmt.Errorf("duplicate allocation category %q", a.Category)
		}
		seen[a.Category] = struct{}{}
		expected := expectedAmounts[a.Category]
		if a.AmountBaseUnits != expected {
			return fmt.Errorf("allocation %q amount %d != expected %d", a.Category, a.AmountBaseUnits, expected)
		}
		if a.Recipient == "" {
			return fmt.Errorf("allocation %q recipient is required", a.Category)
		}
		if math.MaxUint64-total < a.AmountBaseUnits {
			return errors.New("genesis allocation total overflow")
		}
		total += a.AmountBaseUnits
	}
	if len(seen) != len(expectedAmounts) {
		return errors.New("genesis allocation categories are incomplete")
	}
	if total != ExpectedGenesisTotal() {
		return fmt.Errorf("genesis allocation total %d != expected %d", total, ExpectedGenesisTotal())
	}
	if g.TotalBaseUnits != total {
		return fmt.Errorf("declared total %d != computed total %d", g.TotalBaseUnits, total)
	}
	if total+MiningAllocationBaseUnits() != protocol.MaxSupplyBaseUnits {
		return errors.New("genesis plus mining allocation does not equal maximum supply")
	}
	return nil
}

func (g GenesisAllocation) Validate(addressValidator AddressValidator) error {
	if addressValidator == nil {
		return errors.New("existing SYJ address validator is required")
	}
	if err := g.ValidateStructure(); err != nil {
		return err
	}
	for _, a := range g.Allocations {
		if !addressValidator(a.Recipient) {
			return fmt.Errorf("invalid SYJ address for %q", a.Category)
		}
	}
	return nil
}

// SupplyPlan describes the Phase 7 monetary boundary without changing the
// existing reward mechanism. Mining issuance may never exceed RemainingMining.
type SupplyPlan struct {
	InitialSupply   uint64
	RemainingMining uint64
	MaximumSupply   uint64
}

func NewSupplyPlan(initialSupply uint64) (SupplyPlan, error) {
	if initialSupply != ExpectedGenesisTotal() {
		return SupplyPlan{}, fmt.Errorf("initial supply %d != genesis allocation %d", initialSupply, ExpectedGenesisTotal())
	}
	remaining := protocol.MaxSupplyBaseUnits - initialSupply
	if remaining != MiningAllocationBaseUnits() {
		return SupplyPlan{}, errors.New("remaining mining allocation mismatch")
	}
	return SupplyPlan{InitialSupply: initialSupply, RemainingMining: remaining, MaximumSupply: protocol.MaxSupplyBaseUnits}, nil
}

func (p SupplyPlan) ValidateMiningIssued(mined uint64) error {
	if p.InitialSupply+p.RemainingMining != p.MaximumSupply {
		return errors.New("invalid supply plan invariant")
	}
	if mined > p.RemainingMining {
		return errors.New("mining issuance exceeds remaining mining allocation")
	}
	if p.InitialSupply+mined > p.MaximumSupply {
		return errors.New("total supply exceeds maximum supply")
	}
	return nil
}
