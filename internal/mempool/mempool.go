package mempool

import (
	"errors"
	"sort"
	"sync"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)

type Pool struct {
	mu  sync.RWMutex
	max int
	txs map[string]transaction.Transaction
}

func New(max int) *Pool {
	if max < 1 {
		max = 1000
	}
	return &Pool{max: max, txs: make(map[string]transaction.Transaction)}
}
func (p *Pool) Add(tx transaction.Transaction, balance func(string) uint64) error {
	if err := tx.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.txs[tx.TxHash]; ok {
		return errors.New("duplicate transaction")
	}
	if len(p.txs) >= p.max {
		return errors.New("mempool full")
	}
	var pending uint64
	for _, v := range p.txs {
		if v.Sender == tx.Sender {
			if ^uint64(0)-pending < v.AmountBaseUnits {
				return errors.New("pending spend overflow")
			}
			pending += v.AmountBaseUnits
		}
	}
	bal := balance(tx.Sender)
	if tx.AmountBaseUnits > bal || pending > bal-tx.AmountBaseUnits {
		return errors.New("insufficient confirmed balance after pending spend")
	}
	p.txs[tx.TxHash] = tx
	return nil
}
func (p *Pool) RemoveHashes(hashes []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, h := range hashes {
		delete(p.txs, h)
	}
}
func (p *Pool) Get(hash string) (transaction.Transaction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.txs[hash]
	return v, ok
}
func (p *Pool) List(limit int) []transaction.Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]transaction.Transaction, 0, len(p.txs))
	for _, v := range p.txs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp == out[j].Timestamp {
			return out[i].TxHash < out[j].TxHash
		}
		return out[i].Timestamp < out[j].Timestamp
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
func (p *Pool) Len() int { p.mu.RLock(); defer p.mu.RUnlock(); return len(p.txs) }
