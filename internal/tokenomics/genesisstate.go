package tokenomics

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
)

const Phase7NetworkName = "sayanjali-syj-phase7-v1"

// GenesisState is the separately versioned Phase 7 economic initial state.
// Its JSON shape intentionally matches GenesisAllocation so the existing
// schema/template remain authoritative. It is not a block or transaction.
type GenesisState GenesisAllocation

func (s GenesisState) allocation() GenesisAllocation { return GenesisAllocation(s) }

func (s GenesisState) ValidateStructure() error { return s.allocation().ValidateStructure() }

// Validate checks the state using the repository's existing SYJ address
// validator supplied by the caller; this package never creates addresses.
func (s GenesisState) Validate(addressValidator AddressValidator) error {
	return s.allocation().Validate(addressValidator)
}

func (s GenesisState) CanonicalBytes() ([]byte, error) {
	g := s.allocation()
	if err := g.ValidateStructure(); err != nil {
		return nil, err
	}
	allocations := append([]Allocation(nil), g.Allocations...)
	sort.Slice(allocations, func(i, j int) bool { return allocations[i].Category < allocations[j].Category })
	entries := make([]any, 0, len(allocations))
	for _, a := range allocations {
		entries = append(entries, map[string]any{
			"amount_base_units": a.AmountBaseUnits,
			"category":          string(a.Category),
			"recipient":         a.Recipient,
		})
	}
	return canonicaljson.Marshal(map[string]any{
		"allocations":      entries,
		"sender":           g.Sender,
		"total_base_units": g.TotalBaseUnits,
		"version":          g.Version,
	})
}

func (s GenesisState) Commitment() (string, error) {
	b, err := s.CanonicalBytes()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:]), nil
}

// Load reads a Phase 7 genesis-state file. It never generates addresses or
// signing material; address validation is performed by the existing SYJ
// validator when the state is opened by the chain.
func Load(path string) (GenesisState, error) {
	if path == "" {
		return GenesisState{}, errors.New("phase 7 genesis state path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return GenesisState{}, err
	}
	var s GenesisState
	if err := json.Unmarshal(b, &s); err != nil {
		return GenesisState{}, fmt.Errorf("invalid phase 7 genesis state: %w", err)
	}
	if err := s.ValidateStructure(); err != nil {
		return GenesisState{}, err
	}
	return s, nil
}
