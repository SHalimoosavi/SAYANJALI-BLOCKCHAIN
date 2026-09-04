package consensus

import (
	"errors"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"strings"
)

func MeetsDifficulty(hash string, difficulty int) bool {
	if difficulty < 0 || len(hash) < difficulty {
		return false
	}
	return strings.HasPrefix(hash, strings.Repeat("0", difficulty))
}
func ValidatePoW(b *block.Block, requiredDifficulty int) error {
	if b.Hash == "" {
		return errors.New("missing block hash")
	}
	h, _, err := block.HashHeader(b.Header)
	if err != nil {
		return err
	}
	if h != b.Hash {
		return errors.New("block hash mismatch")
	}
	if b.Difficulty < requiredDifficulty {
		return errors.New("block difficulty below required difficulty")
	}
	if !MeetsDifficulty(b.Hash, b.Difficulty) {
		return errors.New("proof-of-work target not satisfied")
	}
	return nil
}
