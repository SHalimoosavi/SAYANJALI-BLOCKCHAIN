package wallet

import (
	"encoding/hex"
	"errors"

	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
)

type KeyPair struct {
	PrivateKeyHex string
	PublicKeyHex  string
	Address       string
}

func New() (*KeyPair, error) {
	priv, err := corecrypto.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return FromPrivateKeyHex(hex.EncodeToString(priv))
}

func FromPrivateKeyHex(s string) (*KeyPair, error) {
	priv, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.New("invalid private key hex")
	}
	pub, err := corecrypto.PublicKeyFromPrivate(priv)
	if err != nil {
		return nil, err
	}
	pubHex := hex.EncodeToString(pub)
	return &KeyPair{PrivateKeyHex: s, PublicKeyHex: pubHex, Address: AddressFromPublicKeyHex(pubHex)}, nil
}
