package node

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestMutationRateLimiterStateIsBoundedAndEvictsLeastRecentlyUsed(t *testing.T) {
	a := &API{rates: make(map[string]*rateLimitEntry), maxRateEntries: 2}
	request := func(port int) {
		t.Helper()
		r := httptest.NewRequest("POST", "http://node.invalid/mine", nil)
		r.RemoteAddr = fmt.Sprintf("127.0.0.1:%d", port)
		if !a.allowMutationRequest(r) {
			t.Fatalf("request from port %d unexpectedly rate limited", port)
		}
	}
	request(1001)
	request(1002)
	request(1001)
	request(1003)
	if len(a.rates) != 2 {
		t.Fatalf("rate limiter retained %d entries, want 2", len(a.rates))
	}
	if _, ok := a.rates["127.0.0.1:1001"]; !ok {
		t.Fatal("recently used entry A was evicted")
	}
	if _, ok := a.rates["127.0.0.1:1002"]; ok {
		t.Fatal("least-recently-used entry B was not evicted")
	}
	if _, ok := a.rates["127.0.0.1:1003"]; !ok {
		t.Fatal("new entry C was not retained")
	}
}

func TestMutationRateLimiterNeverExceedsConfiguredCapacity(t *testing.T) {
	const capacity = 4
	a := &API{rates: make(map[string]*rateLimitEntry), maxRateEntries: capacity}
	for port := 2000; port < 2200; port++ {
		r := httptest.NewRequest("POST", "http://node.invalid/mine", nil)
		r.RemoteAddr = fmt.Sprintf("127.0.0.1:%d", port)
		if !a.allowMutationRequest(r) {
			t.Fatalf("request %d unexpectedly rate limited", port)
		}
		if got := len(a.rates); got > capacity {
			t.Fatalf("rate limiter state grew to %d entries, capacity %d", got, capacity)
		}
	}
	if got := len(a.rates); got != capacity {
		t.Fatalf("rate limiter retained %d entries, want %d", got, capacity)
	}
}
