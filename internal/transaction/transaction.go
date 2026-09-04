package transaction

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type Transaction struct {
	Sender          string  `json:"sender"`
	Receiver        string  `json:"receiver"`
	AmountBaseUnits uint64  `json:"amount_base_units"`
	Timestamp       float64 `json:"timestamp"`
	SenderPublicKey string  `json:"sender_public_key,omitempty"`
	Signature       string  `json:"signature,omitempty"`
	TxHash          string  `json:"tx_hash,omitempty"`
}

func (t Transaction) SigningPayload() map[string]any {
	return map[string]any{"sender": t.Sender, "receiver": t.Receiver, "amount_base_units": t.AmountBaseUnits, "timestamp": t.Timestamp}
}
func (t Transaction) SigningMessage() (string, error) {
	return canonicaljson.String(t.SigningPayload())
}
func (t Transaction) HashPayload() map[string]any {
	return map[string]any{"sender": t.Sender, "receiver": t.Receiver, "amount_base_units": t.AmountBaseUnits, "timestamp": t.Timestamp, "sender_public_key": nilOrString(t.SenderPublicKey), "signature": nilOrString(t.Signature)}
}
func nilOrString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func (t Transaction) ComputeHash() (string, error) {
	s, err := canonicaljson.String(t.HashPayload())
	if err != nil {
		return "", err
	}
	return corecrypto.SHA256String(s), nil
}
func (t *Transaction) Sign(k *wallet.KeyPair) error {
	if t.Sender == protocol.CoinbaseSender {
		return errors.New("coinbase transactions cannot be signed")
	}
	if k.Address != t.Sender {
		return errors.New("wallet address does not match transaction sender")
	}
	t.SenderPublicKey = k.PublicKeyHex
	msg, err := t.SigningMessage()
	if err != nil {
		return err
	}
	t.Signature, err = k.Sign(msg)
	if err != nil {
		return err
	}
	t.TxHash, err = t.ComputeHash()
	return err
}
func (t Transaction) Verify() bool {
	if t.AmountBaseUnits == 0 || t.AmountBaseUnits > protocol.MaxSupplyBaseUnits || !wallet.ValidAddress(t.Receiver) {
		return false
	}
	h, err := t.ComputeHash()
	if err != nil || h != t.TxHash {
		return false
	}
	if t.Sender == protocol.CoinbaseSender {
		return t.SenderPublicKey == "" && t.Signature == ""
	}
	if !wallet.ValidAddress(t.Sender) || t.SenderPublicKey == "" || t.Signature == "" || wallet.AddressFromPublicKeyHex(t.SenderPublicKey) != t.Sender {
		return false
	}
	msg, err := t.SigningMessage()
	if err != nil {
		return false
	}
	if len(t.SenderPublicKey) != 128 || len(t.Signature) != 128 {
		return false
	}
	_, err = hex.DecodeString(t.SenderPublicKey)
	if err != nil {
		return false
	}
	_, err = hex.DecodeString(t.Signature)
	if err != nil {
		return false
	}
	return corecrypto.VerifyECDSAHex(t.SenderPublicKey, msg, t.Signature)
}
func (t Transaction) Validate() error {
	if t.AmountBaseUnits == 0 {
		return errors.New("SYJ amount must be positive")
	}
	if t.AmountBaseUnits > protocol.MaxSupplyBaseUnits {
		return errors.New("SYJ amount exceeds the maximum SYJ supply")
	}
	if t.Sender != protocol.CoinbaseSender && strings.TrimSpace(t.Sender) == "" {
		return errors.New("sender is required")
	}
	if t.Sender != protocol.CoinbaseSender && t.Sender == t.Receiver {
		return errors.New("sender and receiver must differ")
	}
	if !t.Verify() {
		return errors.New("transaction integrity or signature verification failed")
	}
	return nil
}
func (t Transaction) String() string {
	return fmt.Sprintf("%s:%s:%d", t.Sender, t.Receiver, t.AmountBaseUnits)
}
