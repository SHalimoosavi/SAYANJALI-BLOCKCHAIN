package security

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrReplay           = errors.New("replay detected")
	ErrRateLimited      = errors.New("rate limited")
	ErrPeerQuarantined  = errors.New("peer quarantined")
	ErrPeerBanned       = errors.New("peer banned")
	ErrAdmissionLimited = errors.New("admission limited")
)

type ReplayCache struct {
	mu       sync.Mutex
	entries  map[string]time.Time
	capacity int
	ttl      time.Duration
}

func NewReplayCache(capacity int, ttl time.Duration) (*ReplayCache, error) {
	if capacity < 1 || ttl <= 0 {
		return nil, errors.New("invalid replay cache configuration")
	}

	return &ReplayCache{
		entries:  make(map[string]time.Time, capacity),
		capacity: capacity,
		ttl:      ttl,
	}, nil
}

func (c *ReplayCache) Consume(value []byte, now time.Time) error {
	key := string(value)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanupLocked(now)

	if _, exists := c.entries[key]; exists {
		return ErrReplay
	}

	if len(c.entries) >= c.capacity {
		return ErrAdmissionLimited
	}

	c.entries[key] = now.Add(c.ttl)
	return nil
}

func (c *ReplayCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanupLocked(time.Now())
	return len(c.entries)
}

func (c *ReplayCache) cleanupLocked(now time.Time) {
	for key, expiry := range c.entries {
		if !now.Before(expiry) {
			delete(c.entries, key)
		}
	}
}

type TokenBucket struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func NewTokenBucket(rate float64, burst int, now time.Time) (*TokenBucket, error) {
	if rate <= 0 || burst < 1 {
		return nil, errors.New("invalid token bucket configuration")
	}

	return &TokenBucket{
		rate:   rate,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   now,
	}, nil
}

func (b *TokenBucket) Allow(now time.Time, cost float64) bool {
	if cost <= 0 {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if now.After(b.last) {
		b.tokens += now.Sub(b.last).Seconds() * b.rate
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
		b.last = now
	}

	if b.tokens < cost {
		return false
	}

	b.tokens -= cost
	return true
}

type PeerState int

const (
	PeerGood PeerState = iota
	PeerQuarantine
	PeerBan
)

type ViolationSeverity int

const (
	ViolationMinor    ViolationSeverity = 1
	ViolationMajor    ViolationSeverity = 3
	ViolationCritical ViolationSeverity = 5
)

type ReputationEntry struct {
	PeerID       string
	Address      string
	Score        int
	Violations   uint64
	State        PeerState
	CooldownTill time.Time
	LastSeen     time.Time
}

type Reputation struct {
	mu            sync.Mutex
	entries       map[string]*ReputationEntry
	capacity      int
	quarantineAt  int
	banAt         int
	quarantineFor time.Duration
	banFor        time.Duration
}

func NewReputation(
	capacity, quarantineAt, banAt int,
	quarantineFor, banFor time.Duration,
) (*Reputation, error) {
	if capacity < 1 ||
		quarantineAt < 1 ||
		banAt <= quarantineAt ||
		quarantineFor <= 0 ||
		banFor <= 0 {
		return nil, errors.New("invalid reputation configuration")
	}

	return &Reputation{
		entries:       make(map[string]*ReputationEntry, capacity),
		capacity:      capacity,
		quarantineAt:  quarantineAt,
		banAt:         banAt,
		quarantineFor: quarantineFor,
		banFor:        banFor,
	}, nil
}

func (r *Reputation) Violate(
	peerID, address string,
	severity ViolationSeverity,
	now time.Time,
) PeerState {
	r.mu.Lock()
	defer r.mu.Unlock()

	e := r.entryLocked(peerID, address, now)
	e.Score += int(severity)
	e.Violations++
	e.LastSeen = now

	switch {
	case e.Score >= r.banAt:
		e.State = PeerBan
		e.CooldownTill = now.Add(r.banFor)

	case e.Score >= r.quarantineAt:
		e.State = PeerQuarantine
		e.CooldownTill = now.Add(r.quarantineFor)
	}

	return e.State
}

func (r *Reputation) Allow(peerID string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[peerID]
	if !ok {
		return true
	}

	if e.State == PeerGood {
		return true
	}

	if !now.Before(e.CooldownTill) {
		e.State = PeerGood
		e.Score = 0
		e.Violations = 0
		e.CooldownTill = time.Time{}
		return true
	}

	return false
}

func (r *Reputation) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.entries)
}

func (r *Reputation) entryLocked(
	peerID, address string,
	now time.Time,
) *ReputationEntry {
	if e, ok := r.entries[peerID]; ok {
		e.Address = address
		return e
	}

	if len(r.entries) >= r.capacity {
		var oldest string
		var oldestTime time.Time

		for id, e := range r.entries {
			if oldest == "" || e.LastSeen.Before(oldestTime) {
				oldest = id
				oldestTime = e.LastSeen
			}
		}

		if oldest != "" {
			delete(r.entries, oldest)
		}
	}

	e := &ReputationEntry{
		PeerID:   peerID,
		Address:  address,
		LastSeen: now,
	}

	r.entries[peerID] = e
	return e
}

type NetworkMode string

const (
	PrivateTestnet NetworkMode = "PRIVATE_TESTNET"
	PublicTestnet  NetworkMode = "PUBLIC_TESTNET"
	Production     NetworkMode = "PRODUCTION"
)

type AddressPolicy struct {
	Mode             NetworkMode
	AllowLoopback    bool
	AllowPrivate     bool
	AllowLinkLocal   bool
	AllowUnspecified bool
	AllowMulticast   bool
	AllowDNS         bool
	MaxPort          int
}

func DefaultAddressPolicy(mode NetworkMode) AddressPolicy {
	p := AddressPolicy{
		Mode:     mode,
		MaxPort:  65535,
		AllowDNS: mode != Production,
	}

	if mode == PrivateTestnet {
		p.AllowLoopback = true
		p.AllowPrivate = true
	}

	return p
}

func (p AddressPolicy) Validate(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return errors.New("invalid peer address")
	}

	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > p.MaxPort {
		return errors.New("invalid peer port")
	}

	if ip := net.ParseIP(host); ip != nil {
		return p.validateIP(ip)
	}

	if !p.AllowDNS {
		return errors.New("dns peer addresses disabled")
	}

	if strings.TrimSpace(host) == "" ||
		strings.ContainsAny(host, " /\\@") {
		return errors.New("invalid peer hostname")
	}

	return nil
}

func (p AddressPolicy) validateIP(ip net.IP) error {
	if ip.IsUnspecified() && !p.AllowUnspecified {
		return errors.New("unspecified address rejected")
	}

	if ip.IsMulticast() && !p.AllowMulticast {
		return errors.New("multicast address rejected")
	}

	if ip.IsLoopback() && !p.AllowLoopback {
		return errors.New("loopback address rejected")
	}

	if ip.IsLinkLocalUnicast() && !p.AllowLinkLocal {
		return errors.New("link-local address rejected")
	}

	if isMetadataIP(ip) {
		return errors.New("metadata/service-reserved address rejected")
	}

	if isPrivateIP(ip) && !p.AllowPrivate {
		return errors.New("private address rejected")
	}

	return nil
}

// ResolveAndValidate resolves a hostname once and validates every
// resulting address. Callers should dial only the returned endpoints.
func (p AddressPolicy) ResolveAndValidate(
	ctx context.Context,
	address string,
) ([]string, error) {
	if err := p.Validate(address); err != nil {
		return nil, err
	}

	host, port, _ := net.SplitHostPort(address)

	if ip := net.ParseIP(host); ip != nil {
		return []string{net.JoinHostPort(ip.String(), port)}, nil
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("peer hostname resolution failed: %w", err)
	}

	if len(ips) == 0 {
		return nil, errors.New("peer hostname has no addresses")
	}

	out := make([]string, 0, len(ips))

	for _, ip := range ips {
		if err := p.validateIP(ip); err != nil {
			return nil, err
		}

		out = append(out, net.JoinHostPort(ip.String(), port))
	}

	return out, nil
}

func isPrivateIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 10 ||
			(v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31) ||
			(v4[0] == 192 && v4[1] == 168)
	}

	// RFC4193 IPv6 Unique Local Addresses.
	return len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc
}

func isMetadataIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		return v4.String() == "169.254.169.254" ||
			v4.String() == "100.100.100.200"
	}

	return false
}

func ConstantTimeTokenEqual(expected, supplied string) bool {
	a := []byte(expected)
	b := []byte(supplied)

	if len(a) != len(b) {
		return false
	}

	return subtle.ConstantTimeCompare(a, b) == 1
}

// FileReadableSecret verifies that a secret file is owner-readable only.
func FileReadableSecret(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.Mode().Perm()&0077 != 0 {
		return errors.New("secret file permissions must be owner-only")
	}

	return nil
}
