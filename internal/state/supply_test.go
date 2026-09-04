package state

import (
	"math"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func TestSupplyApplyInvariantHardening(t *testing.T) {
	max := protocol.MaxSupplyBaseUnits
	for _, tc := range []struct {
		name      string
		initial   uint64
		amount    uint64
		wantOK    bool
		wantTotal uint64
	}{
		{"zero_state_zero", 0, 0, true, 0},
		{"zero_state_one", 0, 1, true, 1},
		{"zero_state_max", 0, max, true, max},
		{"exact_max_zero", max, 0, true, max},
		{"exact_max_one", max, 1, false, max},
		{"near_max_exact_remaining", max - 1, 1, true, max},
		{"near_max_over_remaining", max - 1, 2, false, max - 1},
		{"invalid_max_plus_one", max + 1, 0, false, max + 1},
		{"invalid_max_plus_one_apply_one", max + 1, 1, false, max + 1},
		{"invalid_uint64_max", math.MaxUint64, 0, false, math.MaxUint64},
		{"invalid_uint64_max_apply_one", math.MaxUint64, 1, false, math.MaxUint64},
		{"full_supply_reward", max, 0, false, max},
		{"invalid_supply_reward", max + 1, 0, false, max + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Supply{Total: tc.initial}
			if tc.name == "full_supply_reward" || tc.name == "invalid_supply_reward" {
				if reward, ok := s.IssueReward(); ok || reward != 0 {
					t.Fatalf("IssueReward returned (%d, %v)", reward, ok)
				}
				return
			}
			ok := s.Apply(tc.amount)
			if ok != tc.wantOK || s.Total != tc.wantTotal {
				t.Fatalf("Apply(%d) = (%v, total=%d), want (%v, total=%d)", tc.amount, ok, s.Total, tc.wantOK, tc.wantTotal)
			}
		})
	}
}

func TestIssueRewardNormalAndExhaustion(t *testing.T) {
	s := Supply{}
	reward, ok := s.IssueReward()
	if !ok || reward != protocol.DefaultBlockRewardUnits {
		t.Fatalf("normal reward = %d, %v", reward, ok)
	}
	s.Total = protocol.MaxSupplyBaseUnits - 1
	reward, ok = s.IssueReward()
	if !ok || reward != 1 {
		t.Fatalf("final partial reward = %d, %v", reward, ok)
	}
	s.Total = protocol.MaxSupplyBaseUnits
	reward, ok = s.IssueReward()
	if ok || reward != 0 {
		t.Fatalf("exhausted reward = %d, %v", reward, ok)
	}
}

func TestIssueRewardDoesNotOverflowOnInvalidSupply(t *testing.T) {
	for _, total := range []uint64{protocol.MaxSupplyBaseUnits + 1, math.MaxUint64} {
		s := Supply{Total: total}
		reward, ok := s.IssueReward()
		if ok || reward != 0 {
			t.Fatalf("invalid total %d produced reward=%d ok=%v", total, reward, ok)
		}
	}
}
