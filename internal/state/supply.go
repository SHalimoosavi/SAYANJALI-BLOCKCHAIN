package state

import "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"

type Supply struct{ Total uint64 }

func (s Supply) IssueReward() (uint64, bool) {
	if s.Total >= protocol.MaxSupplyBaseUnits {
		return 0, false
	}
	remaining := protocol.MaxSupplyBaseUnits - s.Total
	if remaining > protocol.DefaultBlockRewardUnits {
		remaining = protocol.DefaultBlockRewardUnits
	}
	return remaining, true
}

// Apply atomically advances supply only when the current state and resulting
// state both satisfy the protocol maximum. The invariant is checked before
// subtraction so invalid state can never trigger uint64 underflow.
func (s *Supply) Apply(amount uint64) bool {
	if s == nil || s.Total > protocol.MaxSupplyBaseUnits {
		return false
	}
	if amount > protocol.MaxSupplyBaseUnits-s.Total {
		return false
	}
	s.Total += amount
	return true
}
