package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

func SHA256Bytes(data []byte) [32]byte { return sha256.Sum256(data) }
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
func SHA256String(s string) string { return SHA256Hex([]byte(s)) }
