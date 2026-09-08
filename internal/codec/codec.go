package codec

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)

// TransactionBytes is the consensus-compatible JSON dictionary representation
// used by the existing Python implementation for persistence/API exchange.
// Hashing remains governed exclusively by Transaction.ComputeHash().
func transactionMap(t transaction.Transaction) map[string]any {
	return map[string]any{
		"sender": t.Sender, "receiver": t.Receiver, "amount_base_units": t.AmountBaseUnits,
		"timestamp": t.Timestamp, "sender_public_key": nilOrString(t.SenderPublicKey),
		"signature": nilOrString(t.Signature), "tx_hash": t.TxHash,
	}
}

func TransactionBytes(t transaction.Transaction) ([]byte, error) {
	return canonicaljson.Marshal(transactionMap(t))
}

func DecodeTransaction(data []byte) (transaction.Transaction, error) {
	var raw struct {
		Sender          string  `json:"sender"`
		Receiver        string  `json:"receiver"`
		AmountBaseUnits uint64  `json:"amount_base_units"`
		Timestamp       float64 `json:"timestamp"`
		SenderPublicKey *string `json:"sender_public_key"`
		Signature       *string `json:"signature"`
		TxHash          string  `json:"tx_hash"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return transaction.Transaction{}, fmt.Errorf("decode transaction: %w", err)
	}
	if raw.Sender == "" || raw.Receiver == "" || raw.TxHash == "" {
		return transaction.Transaction{}, errors.New("transaction missing required field")
	}
	return transaction.Transaction{Sender: raw.Sender, Receiver: raw.Receiver, AmountBaseUnits: raw.AmountBaseUnits, Timestamp: raw.Timestamp, SenderPublicKey: deref(raw.SenderPublicKey), Signature: deref(raw.Signature), TxHash: raw.TxHash}, nil
}

func BlockBytes(b *block.Block) ([]byte, error) {
	txs := make([]any, 0, len(b.Transactions))
	for _, tx := range b.Transactions {
		txs = append(txs, transactionMap(tx))
	}
	m := map[string]any{"index": b.Index, "previous_hash": b.PreviousHash, "timestamp": b.Timestamp, "nonce": b.Nonce, "difficulty": b.Difficulty, "merkle_root": b.MerkleRoot, "hash": b.Hash, "transactions": txs}
	return canonicaljson.Marshal(m)
}

func DecodeBlock(data []byte) (*block.Block, error) {
	var raw struct {
		Index        int64             `json:"index"`
		PreviousHash string            `json:"previous_hash"`
		Timestamp    float64           `json:"timestamp"`
		Nonce        uint64            `json:"nonce"`
		Difficulty   int               `json:"difficulty"`
		MerkleRoot   string            `json:"merkle_root"`
		Hash         string            `json:"hash"`
		Transactions []json.RawMessage `json:"transactions"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode block: %w", err)
	}
	txs := make([]transaction.Transaction, 0, len(raw.Transactions))
	for _, tb := range raw.Transactions {
		tx, err := DecodeTransaction(tb)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	b := &block.Block{Header: block.Header{Index: raw.Index, PreviousHash: raw.PreviousHash, Timestamp: raw.Timestamp, Nonce: raw.Nonce, Difficulty: raw.Difficulty, MerkleRoot: raw.MerkleRoot}, Transactions: txs, Hash: raw.Hash}
	return b, nil
}

func nilOrString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func HeaderBytes(h block.Header) ([]byte, error) {
	return canonicaljson.Marshal(map[string]any{"index": h.Index, "previous_hash": h.PreviousHash, "timestamp": h.Timestamp, "nonce": h.Nonce, "difficulty": h.Difficulty, "merkle_root": h.MerkleRoot})
}
func DecodeHeader(data []byte) (block.Header, error) {
	var h block.Header
	if err := json.Unmarshal(data, &h); err != nil {
		return h, fmt.Errorf("decode header: %w", err)
	}
	if h.Index < 0 || h.PreviousHash == "" || h.MerkleRoot == "" {
		return h, errors.New("invalid header fields")
	}
	return h, nil
}
