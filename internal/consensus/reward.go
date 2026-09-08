package consensus

import (
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

// ExpectedReward preserves the frozen pre-Phase-7 monetary rule.
func ExpectedReward(currentSupply uint64) (uint64, bool) {
	if currentSupply >= protocol.MaxSupplyBaseUnits {
		return 0, false
	}
	remaining := protocol.MaxSupplyBaseUnits - currentSupply
	if remaining < protocol.DefaultBlockRewardUnits {
		return remaining, true
	}
	return protocol.DefaultBlockRewardUnits, true
}

// ExpectedMiningReward is the Phase 7 issuance rule. Mining can issue only
// from the dedicated 432M allocation; it never consumes the 288M genesis
// allocation and never permits issuance above the global 720M maximum.
func ExpectedMiningReward(miningIssued uint64) (uint64, bool) {
	budget := tokenomics.MiningAllocationBaseUnits()
	if miningIssued >= budget {
		return 0, false
	}
	remaining := budget - miningIssued
	if remaining < protocol.DefaultBlockRewardUnits {
		// Phase 7 has an exact 50-SYJ division, so this state is invalid rather
		// than an implicit fractional/final reward.
		return 0, false
	}
	return protocol.DefaultBlockRewardUnits, true
}
