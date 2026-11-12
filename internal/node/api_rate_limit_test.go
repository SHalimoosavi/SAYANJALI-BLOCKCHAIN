package node

import (
	"fmt"
	"net"
	"net/http/httptest"
	"testing"
)

func TestMutationRateLimiterStateIsBoundedAndEvictsLeastRecentlyUsed(t *testing.T) {
	a := &API{rates: make(map[string]*rateLimitEntry), maxRateEntries: 2}
	request := func(ip string) {
		t.Helper()
		r := httptest.NewRequest("POST", "http://node.invalid/mine", nil)
		r.RemoteAddr = net.JoinHostPort(ip, "3030")
		if !a.allowMutationRequest(r) {
			t.Fatalf("request from %s unexpectedly rate limited", ip)
		}
	}
	request("192.0.2.1")
	request("192.0.2.2")
	request("192.0.2.1")
	request("192.0.2.3")

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
		ip := fmt.Sprintf("198.51.100.%d", lastOctet)
		r := httptest.NewRequest("POST", "http://node.invalid/mine", nil)
		r.RemoteAddr = net.JoinHostPort(ip, "3030")
		if !a.allowMutationRequest(r) {
			t.Fatalf("request from %s unexpectedly rate limited", ip)
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
	r.RemoteAddr = net.JoinHostPort("192.0.2.250", "3030")
	if a.allowMutationRequest(r) {
		t.Fatal("rate limiter accepted a request without a configured capacity")
	}
	if len(a.rates) != 0 {
		t.Fatalf("rate limiter created %d entries without a configured capacity", len(a.rates))
	}
}
