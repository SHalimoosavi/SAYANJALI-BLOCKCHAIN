package mempool

import (
	"errors"
	"sort"
	"sync"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)

// V2Pool enforces contiguous per-sender nonce sequencing without fees or replacement.
type V2Pool struct {
	mu      sync.RWMutex
	max     int
	byID    map[string]transaction.Transaction
	byNonce map[string]map[uint64]string
}

func NewV2(max int) *V2Pool {
	if max < 1 {
		max = 1000
	}
	return &V2Pool{max: max, byID: make(map[string]transaction.Transaction), byNonce: make(map[string]map[uint64]string)}
}

func (p *V2Pool) Add(tx transaction.Transaction, expectedNonce uint64, balance uint64, networkID string) error {
	if err := tx.ValidateV2(networkID); err != nil {
		return err
	}
	if tx.Nonce != expectedNonce {
		return errors.New("nonce gap or stale nonce")
	}
	if tx.Nonce == ^uint64(0) {
		return errors.New("sender nonce exhausted")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.byID[tx.TxID]; ok {
		return errors.New("duplicate transaction")
	}
	if len(p.byID) >= p.max {
		return errors.New("mempool full")
	}
	senderMap := p.byNonce[tx.Sender]
	if senderMap == nil {
		senderMap = make(map[uint64]string)
		p.byNonce[tx.Sender] = senderMap
	}
	if _, ok := senderMap[tx.Nonce]; ok {
		return errors.New("duplicate sender nonce")
	}
	// Existing pending transactions reserve balance. Since admission requires the
	// next contiguous nonce, summing this sender's pending amounts is sufficient.
	var pending uint64
	for _, v := range p.byID {
		if v.Sender == tx.Sender {
			if ^uint64(0)-pending < v.AmountBaseUnits {
				return errors.New("pending spend overflow")
			}
			pending += v.AmountBaseUnits
		}
	}
	if tx.AmountBaseUnits > balance || pending > balance-tx.AmountBaseUnits {
		return errors.New("insufficient confirmed balance after pending spend")
	}
	p.byID[tx.TxID] = tx
	senderMap[tx.Nonce] = tx.TxID
	return nil
}

func (p *V2Pool) List(limit int) []transaction.Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// The caller's chain nonce is not known here, so return deterministic ordering
	// by sender then nonce. The node filters from the confirmed nonce boundary.
	out := make([]transaction.Transaction, 0, len(p.byID))
	for _, tx := range p.byID {
		out = append(out, tx)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sender == out[j].Sender {
			if out[i].Nonce == out[j].Nonce {
				return out[i].TxID < out[j].TxID
			}
			return out[i].Nonce < out[j].Nonce
		}
		return out[i].Sender < out[j].Sender
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (p *V2Pool) RemoveIDs(ids []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range ids {
		tx, ok := p.byID[id]
		if !ok {
			continue
		}
		delete(p.byID, id)
		if m := p.byNonce[tx.Sender]; m != nil {
			delete(m, tx.Nonce)
			if len(m) == 0 {
				delete(p.byNonce, tx.Sender)
			}
		}
	}
}

func (p *V2Pool) ExpectedNonce(address string, confirmedNext uint64) (uint64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	next := confirmedNext
	for {
		ids := p.byNonce[address]
		if ids == nil {
			return next, nil
		}
		if _, ok := ids[next]; !ok {
			return next, nil
		}
		if next == ^uint64(0) {
			return 0, errors.New("sender nonce exhausted")
		}
		next++
	}
}

func (p *V2Pool) Get(id string) (transaction.Transaction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	tx, ok := p.byID[id]
	return tx, ok
}

func (p *V2Pool) Len() int { p.mu.RLock(); defer p.mu.RUnlock(); return len(p.byID) }
