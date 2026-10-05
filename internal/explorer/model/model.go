package model

import (
	"fmt"
	"math"
)

const (
	V1Version        uint8  = 0
	V2Version        uint8  = 2
	DirectionIn             = "in"
	DirectionOut            = "out"
	MaxSQLiteInteger uint64 = math.MaxInt64
)

type Block struct {
	Hash         string
	Height       int64
	PreviousHash string
	Timestamp    float64
	Nonce        uint64
	Difficulty   int
	MerkleRoot   string
	ChainWork    string
	IsCanonical  bool
	Transactions []Transaction
}

type Transaction struct {
	Version         uint8
	NetworkID       string
	Nonce           uint64
	Sender          string
	Receiver        string
	AmountBaseUnits uint64
	Timestamp       float64
	SenderPublicKey string
	Signature       string
	TxHash          string
	TxID            string
}

func (t Transaction) IdentityHash() (string, error) {
	switch t.Version {
	case V1Version:
		if t.TxHash == "" || t.TxID != "" {
			return "", fmt.Errorf("V1 transaction requires tx_hash and forbids tx_id")
		}
		return t.TxHash, nil
	case V2Version:
		if t.TxID == "" || t.TxHash != "" {
			return "", fmt.Errorf("V2 transaction requires tx_id and forbids tx_hash")
		}
		return t.TxID, nil
	default:
		return "", fmt.Errorf("unsupported transaction version %d", t.Version)
	}
}

func CheckedAdd(a, b uint64) (uint64, error) {
	if b > math.MaxUint64-a {
		return 0, fmt.Errorf("uint64 overflow adding %d and %d", a, b)
	}
	return a + b, nil
}

func CheckedBalance(initial, received, sent uint64) (uint64, error) {
	total, err := CheckedAdd(initial, received)
	if err != nil {
		return 0, err
	}
	if total < sent {
		return 0, fmt.Errorf("balance underflow: initial=%d received=%d sent=%d", initial, received, sent)
	}
	return total - sent, nil
}
