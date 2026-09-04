package wallet

import (
	"encoding/hex"

	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
)

// Sign returns a raw 64-byte R||S ECDSA signature. The protocol accepts
// existing valid ECDSA signatures without changing their representation. New
// signatures are produced by the vetted secp256k1 implementation using
// deterministic RFC6979 nonces and canonical low-S output.
func (k *KeyPair) Sign(message string) (string, error) {
	priv, err := hex.DecodeString(k.PrivateKeyHex)
	if err != nil {
		return "", err
	}
	sig, err := corecrypto.SignECDSA(priv, message)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sig), nil
}

func Verify(publicKeyHex, message, signatureHex string) bool {
	return corecrypto.VerifyECDSAHex(publicKeyHex, message, signatureHex)
}
