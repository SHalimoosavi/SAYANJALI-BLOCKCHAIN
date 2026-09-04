package state

import "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"

type Supply struct{ Total uint64 }

func (s Supply) IssueReward() (uint64, bool) {
	if s.Total >= protocol.MaxSupplyBaseUnits {
		return 0, false
	}
	r := protocol.MaxSupplyBaseUnits - s.Total
	if r > protocol.DefaultBlockRewardUnits {
		r = protocol.DefaultBlockRewardUnits
	}
	return r, true
}
func (s *Supply) Apply(amount uint64) bool {
	if amount > protocol.MaxSupplyBaseUnits-s.Total {
		return false
	}
	s.Total += amount
	return true
}
