package consensus

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func TestExpectedMiningRewardExactly50SYJ(t *testing.T) {
	reward, ok := ExpectedMiningReward(0)
	if !ok || reward != protocol.DefaultBlockRewardUnits {
		t.Fatalf("reward=%d ok=%v", reward, ok)
	}
}

func TestExpectedMiningRewardAt8640000BlockBoundary(t *testing.T) {
	issued := tokenomics.MiningAllocationBaseUnits() - protocol.DefaultBlockRewardUnits
	reward, ok := ExpectedMiningReward(issued)
	if !ok || reward != protocol.DefaultBlockRewardUnits {
		t.Fatalf("final reward=%d ok=%v", reward, ok)
	}
	reward, ok = ExpectedMiningReward(tokenomics.MiningAllocationBaseUnits())
	if ok || reward != 0 {
		t.Fatalf("post-exhaustion reward=%d ok=%v", reward, ok)
	}
}

func TestExpectedMiningRewardRejectsPartialFinalReward(t *testing.T) {
	issued := tokenomics.MiningAllocationBaseUnits() - 1
	if reward, ok := ExpectedMiningReward(issued); ok || reward != 0 {
		t.Fatalf("accepted partial final reward: reward=%d ok=%v", reward, ok)
	}
}

func TestMiningAllocationPlusGenesisEqualsMaximum(t *testing.T) {
	if tokenomics.ExpectedGenesisTotal()+tokenomics.MiningAllocationBaseUnits() != protocol.MaxSupplyBaseUnits {
		t.Fatal("Phase 7 supply invariant failed")
	}
}
