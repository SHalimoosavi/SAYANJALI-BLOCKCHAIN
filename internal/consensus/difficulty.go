package consensus

import (
	"errors"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
	"math/big"
)

func NextDifficulty(previousBlocks []*block.Block, cfg protocol.DifficultyConfig, configDifficulty int) (int, error) {
	if len(previousBlocks) < 2 {
		return configDifficulty, nil
	}
	interval := len(previousBlocks) - 1
	expected := int64(interval) * cfg.TargetBlockTimeSeconds
	if expected <= 0 || cfg.TargetBlockTimeSeconds <= 0 {
		return configDifficulty, nil
	}
	actual := previousBlocks[len(previousBlocks)-1].Timestamp - previousBlocks[0].Timestamp
	if actual <= 0 {
		actual = 1
	}
	// Frozen reference uses Fraction with integer timestamp windows. For Go,
	// preserve the same mathematical clamp while rejecting fractional spans
	// rather than silently changing the consensus rule.
	if actual != float64(int64(actual)) {
		return configDifficulty, errors.New("fractional retarget timespan is undefined by the frozen Python behavior")
	}
	act := int64(actual)
	factor := int64(cfg.MaxAdjustmentFactor)
	if factor < 1 {
		factor = 1
	}
	minTS := expected / factor
	if minTS < 1 {
		minTS = 1
	}
	maxTS := expected * factor
	if act < minTS {
		act = minTS
	}
	if act > maxTS {
		act = maxTS
	}
	base := previousBlocks[len(previousBlocks)-1].Difficulty
	if base < 0 {
		base = 0
	}
	current := Work(base)
	targetNum := new(big.Int).Mul(current, big.NewInt(expected))
	targetDen := big.NewInt(act)
	// Compare targetNum/targetDen with current exactly.
	left := targetNum
	right := new(big.Int).Mul(current, targetDen)
	newDiff := base
	if left.Cmp(right) > 0 {
		for newDiff < cfg.MaxDifficulty {
			level := Work(newDiff)
			threshold := new(big.Int).Mul(level, big.NewInt(4))
			if left.Cmp(new(big.Int).Mul(threshold, targetDen)) < 0 {
				break
			}
			newDiff++
		}
	} else if left.Cmp(right) < 0 {
		for newDiff > cfg.MinDifficulty {
			level := Work(newDiff)
			threshold := new(big.Int).Div(level, big.NewInt(4))
			if left.Cmp(new(big.Int).Mul(threshold, targetDen)) > 0 {
				break
			}
			newDiff--
		}
	}
	if newDiff < cfg.MinDifficulty {
		newDiff = cfg.MinDifficulty
	}
	if newDiff > cfg.MaxDifficulty {
		newDiff = cfg.MaxDifficulty
	}
	return newDiff, nil
}
