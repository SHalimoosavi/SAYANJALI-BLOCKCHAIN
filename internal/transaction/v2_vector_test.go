package transaction

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type primaryVector struct {
	EffectiveNetworkID string `json:"effective_network_id"`
	SigningDigest      string `json:"signing_digest"`
	Signature          string `json:"signature"`
	TxID               string `json:"tx_id"`
	Transaction        struct {
		Version         uint8   `json:"version"`
		NetworkID       string  `json:"network_id"`
		Sender          string  `json:"sender"`
		Receiver        string  `json:"receiver"`
		AmountBaseUnits uint64  `json:"amount_base_units"`
		Nonce           uint64  `json:"nonce"`
		Timestamp       float64 `json:"timestamp"`
		SenderPublicKey string  `json:"sender_public_key"`
		Signature       string  `json:"signature"`
		TxID            string  `json:"tx_id"`
	} `json:"transaction"`
}

func TestV2PrimaryVectorFile(t *testing.T) {
	root := filepath.Join("..", "..", "protocol", "test-vectors", "v2", "primary.json")
	b, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	var v primaryVector
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	tx := Transaction{
		Version:         v.Transaction.Version,
		NetworkID:       v.Transaction.NetworkID,
		Sender:          v.Transaction.Sender,
		Receiver:        v.Transaction.Receiver,
		AmountBaseUnits: v.Transaction.AmountBaseUnits,
		Nonce:           v.Transaction.Nonce,
		Timestamp:       v.Transaction.Timestamp,
		SenderPublicKey: v.Transaction.SenderPublicKey,
		Signature:       v.Transaction.Signature,
		TxID:            v.Transaction.TxID,
	}
	if v.EffectiveNetworkID != tx.NetworkID {
		t.Fatal("vector network id mismatch")
	}
	digest, err := tx.V2SigningDigest()
	if err != nil {
		t.Fatal(err)
	}
	if digest != v.SigningDigest {
		t.Fatalf("digest = %s want %s", digest, v.SigningDigest)
	}
	id, err := tx.ComputeV2TxID()
	if err != nil {
		t.Fatal(err)
	}
	if id != v.TxID {
		t.Fatalf("tx id = %s want %s", id, v.TxID)
	}
	if tx.Signature != v.Signature {
		t.Fatalf("signature = %s want %s", tx.Signature, v.Signature)
	}
	if err := tx.ValidateV2(v.EffectiveNetworkID); err != nil {
		t.Fatal(err)
	}
}
