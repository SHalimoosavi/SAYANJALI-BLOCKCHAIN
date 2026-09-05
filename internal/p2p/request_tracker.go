package p2p

import (
	"errors"
	"sync"
)

var ErrOutstandingLimit = errors.New("outstanding request limit reached")
var ErrRequestAlreadyOutstanding = errors.New("request id already outstanding")
var ErrRequestNotOutstanding = errors.New("request id not outstanding")

type RequestTracker struct {
	mu  sync.Mutex
	ids map[uint64]struct{}
}

func NewRequestTracker() *RequestTracker {
	return &RequestTracker{ids: make(map[uint64]struct{}, MaxOutstandingRequests)}
}
func (t *RequestTracker) Reserve(id uint64) error {
	if id == 0 {
		return ErrInvalidRequestID
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.ids[id]; ok {
		return ErrRequestAlreadyOutstanding
	}
	if len(t.ids) >= MaxOutstandingRequests {
		return ErrOutstandingLimit
	}
	t.ids[id] = struct{}{}
	return nil
}
func (t *RequestTracker) Complete(id uint64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.ids[id]; !ok {
		return ErrRequestNotOutstanding
	}
	delete(t.ids, id)
	return nil
}
func (t *RequestTracker) Has(id uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.ids[id]
	return ok
}
func (t *RequestTracker) Len() int { t.mu.Lock(); defer t.mu.Unlock(); return len(t.ids) }
