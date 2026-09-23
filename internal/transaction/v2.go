package transaction

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/networkid"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

const (
	V2Version       uint8 = 2
	V2SigningDomain       = "SYJ-TX-SIGN-V2\x00"
	V2TxIDDomain          = "SYJ-TX-ID-V2\x00"
)

func (t Transaction) IsV2() bool { return t.Version == V2Version }

func (t Transaction) V2SigningPayload() map[string]any {
	return map[string]any{
		"amount_base_units": t.AmountBaseUnits,
		"network_id":        t.NetworkID,
		"nonce":             t.Nonce,
		"receiver":          t.Receiver,
		"sender":            t.Sender,
		"sender_public_key": t.SenderPublicKey,
		"timestamp":         t.Timestamp,
		"version":           uint64(V2Version),
	}
}

func (t Transaction) V2CanonicalSigningPayload() ([]byte, error) {
	return canonicaljson.Marshal(t.V2SigningPayload())
}

func (t Transaction) V2SigningBytes() ([]byte, error) {
	payload, err := t.V2CanonicalSigningPayload()
	if err != nil {
		return nil, err
	}
	return append([]byte(V2SigningDomain), payload...), nil
}

func (t Transaction) V2SigningDigest() (string, error) {
	b, err := t.V2SigningBytes()
	if err != nil {
		return "", err
	}
	return corecrypto.SHA256String(string(b)), nil
}

func (t Transaction) ComputeV2TxID() (string, error) {
	payload, err := t.V2CanonicalSigningPayload()
	if err != nil {
		return "", err
	}
	sum := corecrypto.SHA256Bytes(append([]byte(V2TxIDDomain), payload...))
	return hex.EncodeToString(sum[:]), nil
}

func (t *Transaction) SignV2(k *wallet.KeyPair, expectedNetworkID string) error {
	if t.Sender == protocol.CoinbaseSender {
		return errors.New("coinbase transactions cannot be signed")
	}
	if k == nil || k.Address != t.Sender {
		return errors.New("wallet address does not match transaction sender")
	}
	if expectedNetworkID != "" && t.NetworkID != expectedNetworkID {
		return errors.New("transaction network id mismatch")
	}
	t.Version = V2Version
	t.SenderPublicKey = k.PublicKeyHex
	b, err := t.V2SigningBytes()
	if err != nil {
		return err
	}
	sig, err := k.Sign(string(b))
	if err != nil {
		return err
	}
	t.Signature = sig
	t.TxID, err = t.ComputeV2TxID()
	return err
}

func (t Transaction) VerifyV2(expectedNetworkID string) bool {
	if err := t.ValidateV2(expectedNetworkID); err != nil {
		return false
	}
	return true
}

func (t Transaction) ValidateV2(expectedNetworkID string) error {
	if t.Version != V2Version {
		return errors.New("transaction version must be 2")
	}
	if err := networkid.ValidateHex64(t.NetworkID); err != nil {
		return err
	}
	if expectedNetworkID == "" || t.NetworkID != expectedNetworkID {
		return errors.New("transaction network id mismatch")
	}
	if t.AmountBaseUnits == 0 {
		return errors.New("SYJ amount must be positive")
	}
	if t.AmountBaseUnits > protocol.MaxSupplyBaseUnits {
		return errors.New("SYJ amount exceeds the maximum SYJ supply")
	}
	if math.IsNaN(t.Timestamp) || math.IsInf(t.Timestamp, 0) || t.Timestamp < 0 || t.Timestamp > float64(^uint64(0)) {
		return errors.New("transaction timestamp must be finite")
	}
	if t.Sender == protocol.CoinbaseSender {
		return errors.New("normal V2 validator cannot validate coinbase")
	}
	if !wallet.ValidAddress(t.Sender) || !wallet.ValidAddress(t.Receiver) {
		return errors.New("invalid sender or receiver address")
	}
	if t.Sender == t.Receiver {
		return errors.New("sender and receiver must differ")
	}
	if t.SenderPublicKey == "" || len(t.SenderPublicKey) != 128 {
		return errors.New("invalid sender public key")
	}
	if wallet.AddressFromPublicKeyHex(t.SenderPublicKey) != t.Sender {
		return errors.New("sender does not match sender public key")
	}
	if t.Signature == "" || len(t.Signature) != 128 {
		return errors.New("invalid signature")
	}
	if _, err := hex.DecodeString(t.SenderPublicKey); err != nil {
		return errors.New("invalid sender public key encoding")
	}
	if _, err := hex.DecodeString(t.Signature); err != nil {
		return errors.New("invalid signature encoding")
	}
	signingBytes, err := t.V2SigningBytes()
	if err != nil {
		return err
	}
	if !corecrypto.VerifyECDSAHex(t.SenderPublicKey, string(signingBytes), t.Signature) {
		return errors.New("signature verification failed")
	}
	txid, err := t.ComputeV2TxID()
	if err != nil {
		return err
	}
	if t.TxID != txid {
		return fmt.Errorf("tx_id mismatch: got %s want %s", t.TxID, txid)
	}
	return nil
}

func NewV2Coinbase(receiver, network string, amount uint64, timestamp float64) (Transaction, error) {
	if err := networkid.ValidateHex64(network); err != nil {
		return Transaction{}, err
	}
	tx := Transaction{
		Version:         V2Version,
		NetworkID:       network,
		Nonce:           0,
		Sender:          protocol.CoinbaseSender,
		Receiver:        receiver,
		AmountBaseUnits: amount,
		Timestamp:       timestamp,
	}
	if !wallet.ValidAddress(receiver) {
		return Transaction{}, errors.New("invalid coinbase receiver")
	}
	if amount == 0 || amount > protocol.MaxSupplyBaseUnits {
		return Transaction{}, errors.New("invalid coinbase amount")
	}
	id, err := tx.ComputeV2TxID()
	if err != nil {
		return Transaction{}, err
	}
	tx.TxID = id
	return tx, nil
}
