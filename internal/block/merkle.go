package block

import corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"

func MerkleRoot(txHashes []string) string {
	if len(txHashes) == 0 {
		return corecrypto.SHA256String("")
	}
	hashes := append([]string(nil), txHashes...)
	for len(hashes) > 1 {
		if len(hashes)%2 == 1 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}
		next := make([]string, 0, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			next = append(next, corecrypto.SHA256String(hashes[i]+hashes[i+1]))
		}
		hashes = next
	}
	return hashes[0]
}
