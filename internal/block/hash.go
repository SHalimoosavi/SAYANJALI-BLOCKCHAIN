package block

import (
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
)

func HashHeader(h Header) (string, string, error) {
	payload := map[string]any{"index": h.Index, "previous_hash": h.PreviousHash, "timestamp": h.Timestamp, "nonce": h.Nonce, "difficulty": h.Difficulty, "merkle_root": h.MerkleRoot}
	s, err := canonicaljson.String(payload)
	if err != nil {
		return "", "", err
	}
	return corecrypto.SHA256String(s), s, nil
}
