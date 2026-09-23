package mempool

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
)

const v2PoolNetwork = "237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3"

func poolTx(t *testing.T, k *wallet.KeyPair, nonce uint64) transaction.Transaction {
	t.Helper()
	tx := transaction.Transaction{Version: 2, NetworkID: v2PoolNetwork, Sender: k.Address, Receiver: "SYJ1111111111111111111111111111111111111111", AmountBaseUnits: 1, Timestamp: 1735689601.25 + float64(nonce), Nonce: nonce}
	if err := tx.SignV2(k, v2PoolNetwork); err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestV2PoolRejectsNonceGap(t *testing.T) {
	k, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	p := NewV2(10)
	tx1 := poolTx(t, k, 1)
	if err := p.Add(tx1, 0, 100, v2PoolNetwork); err == nil {
		t.Fatal("nonce gap accepted")
	}
	tx0 := poolTx(t, k, 0)
	if err := p.Add(tx0, 0, 100, v2PoolNetwork); err != nil {
		t.Fatal(err)
	}
	next, err := p.ExpectedNonce(k.Address, 0)
	if err != nil {
		t.Fatal(err)
	}
	if next != 1 {
		t.Fatalf("next nonce = %d", next)
	}
	if err := p.Add(tx1, next, 99, v2PoolNetwork); err != nil {
		t.Fatal(err)
	}
}

func TestV2PoolRejectsDuplicateSenderNonceAndID(t *testing.T) {
	k, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	p := NewV2(10)
	tx := poolTx(t, k, 0)
	if err := p.Add(tx, 0, 100, v2PoolNetwork); err != nil {
		t.Fatal(err)
	}
	if err := p.Add(tx, 1, 100, v2PoolNetwork); err == nil {
		t.Fatal("duplicate tx id accepted")
	}
	if err := p.Add(tx, 0, 100, v2PoolNetwork); err == nil {
		t.Fatal("duplicate sender nonce accepted")
	}
}
