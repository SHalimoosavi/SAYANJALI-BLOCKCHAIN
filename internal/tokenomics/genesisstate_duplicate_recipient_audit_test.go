package tokenomics

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
)

// TestAuditCharacterizationDuplicateRecipients records the baseline behavior.
// GenesisState.Validate currently accepts the same valid recipient for all five
// allocation categories, although the production genesis-config validator
// rejects duplicate recipient addresses. This test does not select the desired
// protocol policy; see docs/decisions/genesis-package-integration.md.
func TestAuditCharacterizationDuplicateRecipients(t *testing.T) {
	const recipient = "SYJc7181103b1e2cc71112a1517e1fe6c11bf1420a9"
	state := GenesisState{
		Version: Version,
		Sender:  "SYJ-GENESIS-ALLOCATION-0000000000000000",
		Allocations: []Allocation{
			{Category: Presale, Recipient: recipient, AmountBaseUnits: 7_200_000_000_000_000},
			{Category: Treasury, Recipient: recipient, AmountBaseUnits: 5_760_000_000_000_000},
			{Category: Ecosystem, Recipient: recipient, AmountBaseUnits: 3_600_000_000_000_000},
			{Category: Liquidity, Recipient: recipient, AmountBaseUnits: 4_320_000_000_000_000},
			{Category: Team, Recipient: recipient, AmountBaseUnits: 7_920_000_000_000_000},
		},
		TotalBaseUnits: ExpectedGenesisTotal(),
	}
	if err := state.Validate(wallet.ValidAddress); err != nil {
		t.Fatalf("baseline behavior changed: duplicate-recipient state was previously accepted; got %v", err)
	}
}
