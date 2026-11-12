package node

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestMutationRateLimiterStateIsBoundedAndEvictsLeastRecentlyUsed(t *testing.T) {
	a := &API{rates: make(map[string]*rateLimitEntry), maxRateEntries: 2}
	request := func(lastOctet int) {
		t.Helper()
		r := httptest.NewRequest("POST", "http://node.invalid/mine", nil)
		r.RemoteAddr = fmt.Sprintf("192.0.2.%d:3030", lastOctet)
		if !a.allowMutationRequest(r) {
			t.Fatalf("request from address suffix %d unexpectedly rate limited", lastOctet)
		}
	}
	request(1)
	request(2)
	request(1)
	request(3)
	if len(a.rates) != 2 {
		t.Fatalf("rate limiter retained %d entries, want 2", len(a.rates))
	}
	if _, ok := a.rates["192.0.2.1"]; !ok {
		t.Fatal("recently used entry A was evicted")
	}
	if _, ok := a.rates["192.0.2.2"]; ok {
		t.Fatal("least-recently-used entry B was not evicted")
	}
	if _, ok := a.rates["192.0.2.3"]; !ok {
		t.Fatal("new entry C was not retained")
	}
}

func TestMutationRateLimiterNeverExceedsConfiguredCapacity(t *testing.T) {
	const capacity = 4
	a := &API{rates: make(map[string]*rateLimitEntry), maxRateEntries: capacity}
	for lastOctet := 1; lastOctet <= 200; lastOctet++ {
		r := httptest.NewRequest("POST", "http://node.invalid/mine", nil)
		r.RemoteAddr = fmt.Sprintf("198.51.100.%d:3030", lastOctet)
		if !a.allowMutationRequest(r) {
			t.Fatalf("request %d unexpectedly rate limited", lastOctet)
		}
		if got := len(a.rates); got > capacity {
			t.Fatalf("rate limiter state grew to %d entries, capacity %d", got, capacity)
		}
	}
	if got := len(a.rates); got != capacity {
		t.Fatalf("rate limiter retained %d entries, want %d", got, capacity)
	}
}

func TestMutationRateLimiterFailsClosedWithoutCapacity(t *testing.T) {
	a := &API{rates: make(map[string]*rateLimitEntry)}
	r := httptest.NewRequest("POST", "http://node.invalid/mine", nil)
	r.RemoteAddr = "192.0.2.250:3030"
	if a.allowMutationRequest(r) {
		t.Fatal("rate limiter accepted a request without a configured capacity")
	}
	if len(a.rates) != 0 {
		t.Fatalf("rate limiter created %d entries without a configured capacity", len(a.rates))
	}
}
