package protocol

import "math/big"

type DifficultyConfig struct {
	TargetBlockTimeSeconds int64
	AdjustmentInterval     int
	MinDifficulty          int
	MaxDifficulty          int
	MaxAdjustmentFactor    int
}

func DefaultDifficultyConfig() DifficultyConfig {
	return DifficultyConfig{TargetBlockTimeSeconds: TargetBlockTimeSeconds, AdjustmentInterval: DifficultyInterval, MinDifficulty: MinDifficulty, MaxDifficulty: MaxDifficulty, MaxAdjustmentFactor: MaxAdjustmentFactor}
}

func WorkForDifficulty(difficulty int) *big.Int {
	if difficulty < 0 {
		difficulty = 0
	}
	return new(big.Int).Lsh(big.NewInt(1), uint(4*difficulty))
}
