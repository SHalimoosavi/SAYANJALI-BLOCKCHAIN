package mempool

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
)

func TestPoolCapacityBoundary(t *testing.T) {
	const max = 1000
	const amount uint64 = 1

	p := New(max)

	// Each transaction uses a different real keypair, so every transaction
	// has a valid address, signature, and unique hash.
	for i := 0; i < max; i++ {
		key, err := wallet.New()
		if err != nil {
			t.Fatalf("generate key %d: %v", i, err)
		}

		receiverKey, err := wallet.New()
		if err != nil {
			t.Fatalf("generate receiver key %d: %v", i, err)
		}

		tx := transaction.Transaction{
			Sender:          key.Address,
			Receiver:        receiverKey.Address,
			AmountBaseUnits: amount,
			Timestamp:       float64(i + 1),
		}

		if err := tx.Sign(key); err != nil {
			t.Fatalf("sign transaction %d: %v", i, err)
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("transaction %d failed validation: %v", i, err)
		}

		if err := p.Add(tx, func(string) uint64 {
			return 1_000_000
		}); err != nil {
			t.Fatalf("transaction %d rejected before capacity: %v", i, err)
		}
	}

	if got := p.Len(); got != max {
		t.Fatalf("mempool size = %d, want %d", got, max)
	}

	// A valid 1001st transaction must be rejected by the capacity boundary.
	key, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	receiverKey, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	tx := transaction.Transaction{
		Sender:          key.Address,
		Receiver:        receiverKey.Address,
		AmountBaseUnits: amount,
		Timestamp:       float64(max + 1),
	}

	if err := tx.Sign(key); err != nil {
		t.Fatal(err)
	}

	if err := tx.Validate(); err != nil {
		t.Fatalf("1001st transaction is invalid: %v", err)
	}

	if err := p.Add(tx, func(string) uint64 {
		return 1_000_000
	}); err == nil || err.Error() != "mempool full" {
		t.Fatalf("1001st transaction error = %v, want mempool full", err)
	}

	if got := p.Len(); got != max {
		t.Fatalf("mempool size after rejected transaction = %d, want %d", got, max)
	}
}
