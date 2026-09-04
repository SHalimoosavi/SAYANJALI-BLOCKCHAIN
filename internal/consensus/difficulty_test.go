package consensus

import (
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
	"testing"
)

func TestFractionalRetargetSpanIsExplicitlyRejected(t *testing.T) {
	chain := make([]*block.Block, 10)
	for i := range chain {
		chain[i] = &block.Block{Header: block.Header{Timestamp: 1000 + float64(i)}}
	}
	chain[9].Timestamp = 1001.25
	_, err := NextDifficulty(chain, protocol.DefaultDifficultyConfig(), 4)
	if err == nil {
		t.Fatal("fractional retarget span was silently accepted")
	}
}
