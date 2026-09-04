package consensus

import (
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
	"math/big"
)

func Work(difficulty int) *big.Int { return protocol.WorkForDifficulty(difficulty) }
func ChainWork(chain []*block.Block) *big.Int {
	total := new(big.Int)
	for _, b := range chain {
		total.Add(total, Work(b.Difficulty))
	}
	return total
}
func ChainWorkFromDifficulties(ds []int) *big.Int {
	total := new(big.Int)
	for _, d := range ds {
		total.Add(total, Work(d))
	}
	return total
}
