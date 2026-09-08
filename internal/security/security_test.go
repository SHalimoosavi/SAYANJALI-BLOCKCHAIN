package security

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReplayCacheConsumeAndExpiry(t *testing.T) {
	now := time.Unix(1000, 0)

	c, err := NewReplayCache(2, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Consume([]byte("challenge-1"), now); err != nil {
		t.Fatal(err)
	}

	if err := c.Consume([]byte("challenge-1"), now.Add(time.Second)); err != ErrReplay {
		t.Fatalf("expected ErrReplay, got %v", err)
	}

	if err := c.Consume([]byte("challenge-2"), now); err != nil {
		t.Fatal(err)
	}

	if len(c.entries) != 2 {
		t.Fatalf("expected cache length 2, got %d", len(c.entries))
	}

	if err := c.Consume([]byte("challenge-1"), now.Add(11*time.Second)); err != nil {
		t.Fatalf("expired challenge should be reusable: %v", err)
	}
}

func TestReplayCacheCapacity(t *testing.T) {
	now := time.Unix(2000, 0)

	c, err := NewReplayCache(2, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Consume([]byte("a"), now); err != nil {
		t.Fatal(err)
	}
	if err := c.Consume([]byte("b"), now); err != nil {
		t.Fatal(err)
	}

	if err := c.Consume([]byte("c"), now); err != ErrAdmissionLimited {
		t.Fatalf("expected capacity rejection, got %v", err)
	}
}

func TestTokenBucket(t *testing.T) {
	now := time.Unix(3000, 0)

	b, err := NewTokenBucket(1, 2, now)
	if err != nil {
		t.Fatal(err)
	}

	if !b.Allow(now, 1) {
		t.Fatal("first token should be accepted")
	}

	if !b.Allow(now, 1) {
		t.Fatal("second token should be accepted")
	}

	if b.Allow(now, 1) {
		t.Fatal("bucket should be empty")
	}

	if !b.Allow(now.Add(time.Second), 1) {
		t.Fatal("one token should have regenerated")
	}
}

func TestReputationQuarantineAndBan(t *testing.T) {
	now := time.Unix(4000, 0)

	r, err := NewReputation(
		10,
		3,
		5,
		time.Minute,
		2*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	if state := r.Violate("peer-a", "127.0.0.1:1", ViolationMajor, now); state != PeerQuarantine {
		t.Fatalf("expected quarantine, got %v", state)
	}

	if r.Allow("peer-a", now) {
		t.Fatal("quarantined peer should not be allowed")
	}

	if state := r.Violate("peer-a", "127.0.0.1:1", ViolationMajor, now); state != PeerBan {
		t.Fatalf("expected ban, got %v", state)
	}

	if r.Allow("peer-a", now) {
		t.Fatal("banned peer should not be allowed")
	}

	if !r.Allow("peer-a", now.Add(3*time.Minute)) {
		t.Fatal("expired ban should recover")
	}
}

func TestAddressPolicyPrivateTestnet(t *testing.T) {
	p := DefaultAddressPolicy(PrivateTestnet)

	allowed := []string{
		"127.0.0.1:30301",
		"10.0.0.5:30301",
		"192.168.1.20:30301",
	}

	for _, address := range allowed {
		if err := p.Validate(address); err != nil {
			t.Fatalf("private-testnet address %s rejected: %v", address, err)
		}
	}

	rejected := []string{
		"0.0.0.0:30301",
		"224.0.0.1:30301",
		"169.254.169.254:80",
		"100.100.100.200:80",
	}

	for _, address := range rejected {
		if err := p.Validate(address); err == nil {
			t.Fatalf("dangerous address %s was accepted", address)
		}
	}
}

func TestAddressPolicyProduction(t *testing.T) {
	p := DefaultAddressPolicy(Production)

	rejected := []string{
		"127.0.0.1:30301",
		"10.0.0.5:30301",
		"192.168.1.20:30301",
		"169.254.1.1:30301",
		"169.254.169.254:80",
		"0.0.0.0:30301",
		"224.0.0.1:30301",
	}

	for _, address := range rejected {
		if err := p.Validate(address); err == nil {
			t.Fatalf("production policy accepted %s", address)
		}
	}

	if err := p.Validate("8.8.8.8:30301"); err != nil {
		t.Fatalf("public IPv4 rejected: %v", err)
	}

	if err := p.Validate("example.com:30301"); err == nil {
		t.Fatal("production policy should reject DNS by default")
	}
}

func TestAddressPolicyInvalidPort(t *testing.T) {
	p := DefaultAddressPolicy(PrivateTestnet)

	for _, address := range []string{
		"127.0.0.1:0",
		"127.0.0.1:65536",
		"127.0.0.1:notaport",
		"127.0.0.1",
	} {
		if err := p.Validate(address); err == nil {
			t.Fatalf("invalid address accepted: %s", address)
		}
	}
}

func TestResolveAndValidateLiteral(t *testing.T) {
	p := DefaultAddressPolicy(PrivateTestnet)

	got, err := p.ResolveAndValidate(
		context.Background(),
		"127.0.0.1:30301",
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0] != "127.0.0.1:30301" {
		t.Fatalf("unexpected resolved endpoint: %#v", got)
	}
}

func TestConstantTimeTokenEqual(t *testing.T) {
	if !ConstantTimeTokenEqual("secret-token", "secret-token") {
		t.Fatal("equal tokens rejected")
	}

	if ConstantTimeTokenEqual("secret-token", "wrong-token") {
		t.Fatal("different tokens accepted")
	}

	if ConstantTimeTokenEqual("secret-token", "secret-token-extra") {
		t.Fatal("different-length tokens accepted")
	}
}

func TestFileReadableSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")

	if err := os.WriteFile(path, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := FileReadableSecret(path); err != nil {
		t.Fatalf("0600 secret rejected: %v", err)
	}

	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}

	if err := FileReadableSecret(path); err == nil {
		t.Fatal("world/group-readable secret accepted")
	}
}

func TestMetadataIPDetection(t *testing.T) {
	for _, address := range []string{
		"169.254.169.254:80",
		"100.100.100.200:80",
	} {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}

		ip := net.ParseIP(host)
		if !isMetadataIP(ip) {
			t.Fatalf("metadata address not detected: %s", address)
		}
	}
}

func TestReplayCacheConcurrentConsume(t *testing.T) {
	now := time.Unix(5000, 0)

	c, err := NewReplayCache(100, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 32
	results := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			results <- c.Consume([]byte("same-challenge"), now)
		}()
	}

	successes := 0
	replays := 0

	for i := 0; i < workers; i++ {
		err := <-results
		switch err {
		case nil:
			successes++
		case ErrReplay:
			replays++
		default:
			t.Fatalf("unexpected replay-cache result: %v", err)
		}
	}

	if successes != 1 {
		t.Fatalf("expected exactly one successful consume, got %d", successes)
	}

	if replays != workers-1 {
		t.Fatalf("expected %d replay failures, got %d", workers-1, replays)
	}
}

func TestAddressPolicyRejectsIPv6LinkLocal(t *testing.T) {
	p := DefaultAddressPolicy(PrivateTestnet)

	if err := p.Validate("[fe80::1]:30301"); err == nil {
		t.Fatal("IPv6 link-local address accepted")
	}
}

func TestAddressPolicyAllowsPublicIPv6(t *testing.T) {
	p := DefaultAddressPolicy(Production)

	if err := p.Validate("[2001:4860:4860::8888]:30301"); err != nil {
		t.Fatalf("public IPv6 rejected: %v", err)
	}
}
