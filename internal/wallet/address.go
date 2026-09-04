package wallet

import (
	"encoding/hex"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func AddressFromPublicKeyHex(publicKeyHex string) string {
	return protocol.AddressPrefix + corecrypto.SHA256String(publicKeyHex)[:protocol.AddressHashLength]
}

func ValidAddress(address string) bool {
	if len(address) != len(protocol.AddressPrefix)+protocol.AddressHashLength || address[:len(protocol.AddressPrefix)] != protocol.AddressPrefix {
		return false
	}
	_, err := hex.DecodeString(address[len(protocol.AddressPrefix):])
	return err == nil
}
