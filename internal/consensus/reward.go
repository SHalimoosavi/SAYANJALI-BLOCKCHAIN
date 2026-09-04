package consensus

import "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"

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
