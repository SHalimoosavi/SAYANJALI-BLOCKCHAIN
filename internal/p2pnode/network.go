package p2pnode

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/chain"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/codec"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/identity"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/mempool"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2p"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/security"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)

type Config struct {
	NetworkName          string
	GenesisHash          string
	NodeID               string
	PublicKeyHex         string
	AdvertisedAddress    string
	ListenAddress        string
	Seeds                []string
	MaxPeers             int
	MaxPendingHandshakes int
	MaxHandshakesPerIP   int
	PeerMessageRate      float64
	PeerMessageBurst     int
	Logger               *slog.Logger
	AddressPolicyMode    security.NetworkMode
	AllowLoopback        bool
	AllowPrivate         bool
	AllowLinkLocal       bool
	AllowUnspecified     bool
	AllowMulticast       bool
	AllowDNS             bool
	Identity             identity.Identity
}
type Network struct {
	cfg             Config
	ch              *chain.Chain
	pool            *mempool.Pool
	ln              net.Listener
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.RWMutex
	peers           map[string]*Peer
	nextReq         atomic.Uint64
	handshakeReplay *security.ReplayCache
	reputation      *security.Reputation

	admissionMu       sync.Mutex
	pendingHandshakes int
	handshakesByIP    map[string]int
	addressPolicy     security.AddressPolicy
}
type Peer struct {
	ID           string
	Address      string
	Capabilities uint32
	Inbound      bool
	Conn         net.Conn
	Session      *Session
}
type Session struct {
	n            *Network
	p            *Peer
	conn         net.Conn
	tracker      *p2p.RequestTracker
	pendingMu    sync.Mutex
	pending      map[uint64]chan p2p.Frame
	writeMu      sync.Mutex
	closed       chan struct{}
	messageLimit *security.TokenBucket
}

func New(cfg Config, ch *chain.Chain, pool *mempool.Pool) *Network {
	if cfg.MaxPeers < 1 {
		cfg.MaxPeers = 32
	}
	if cfg.MaxPendingHandshakes < 1 {
		cfg.MaxPendingHandshakes = cfg.MaxPeers * 2
	}
	if cfg.MaxHandshakesPerIP < 1 {
		cfg.MaxHandshakesPerIP = 4
	}
	if cfg.PeerMessageRate <= 0 {
		cfg.PeerMessageRate = 100
	}
	if cfg.PeerMessageBurst < 1 {
		cfg.PeerMessageBurst = 256
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.AddressPolicyMode == "" {
		cfg.AddressPolicyMode = security.PrivateTestnet
	}

	policy := security.DefaultAddressPolicy(cfg.AddressPolicyMode)
	if cfg.AllowLoopback {
		policy.AllowLoopback = true
	}
	if cfg.AllowPrivate {
		policy.AllowPrivate = true
	}
	if cfg.AllowLinkLocal {
		policy.AllowLinkLocal = true
	}
	if cfg.AllowUnspecified {
		policy.AllowUnspecified = true
	}
	if cfg.AllowMulticast {
		policy.AllowMulticast = true
	}
	if cfg.AllowDNS {
		policy.AllowDNS = true
	}

	replay, err := security.NewReplayCache(4096, 10*time.Minute)
	if err != nil {
		panic("invalid handshake replay cache configuration: " + err.Error())
	}

	reputation, err := security.NewReputation(
		4096,
		3,
		6,
		5*time.Minute,
		30*time.Minute,
	)
	if err != nil {
		panic("invalid reputation configuration: " + err.Error())
	}

	return &Network{
		cfg:             cfg,
		ch:              ch,
		pool:            pool,
		peers:           make(map[string]*Peer),
		handshakeReplay: replay,
		reputation:      reputation,
		handshakesByIP:  make(map[string]int),
		addressPolicy:   policy,
	}
}
func (n *Network) recordReputationViolation(
	peerID, address string,
	severity security.ViolationSeverity,
) security.PeerState {
	if n == nil || n.reputation == nil {
		return security.PeerGood
	}
	return n.reputation.Violate(peerID, address, severity, time.Now())
}

func remoteIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err == nil {
		return host
	}

	return addr.String()
}

func (n *Network) acquireHandshake(addr net.Addr) error {
	ip := remoteIP(addr)

	n.admissionMu.Lock()
	defer n.admissionMu.Unlock()

	if n.pendingHandshakes >= n.cfg.MaxPendingHandshakes {
		return security.ErrAdmissionLimited
	}

	if ip != "" && n.handshakesByIP[ip] >= n.cfg.MaxHandshakesPerIP {
		return security.ErrAdmissionLimited
	}

	n.pendingHandshakes++

	if ip != "" {
		n.handshakesByIP[ip]++
	}

	return nil
}

func (n *Network) releaseHandshake(addr net.Addr) {
	ip := remoteIP(addr)

	n.admissionMu.Lock()
	defer n.admissionMu.Unlock()

	if n.pendingHandshakes > 0 {
		n.pendingHandshakes--
	}

	if ip == "" {
		return
	}

	if count := n.handshakesByIP[ip]; count <= 1 {
		delete(n.handshakesByIP, ip)
	} else {
		n.handshakesByIP[ip] = count - 1
	}
}

func (n *Network) Start(ctx context.Context) error {
	n.ctx, n.cancel = context.WithCancel(ctx)
	ln, err := net.Listen("tcp", n.cfg.ListenAddress)
	if err != nil {
		return err
	}
	n.ln = ln
	if n.cfg.AdvertisedAddress == "" {
		// A wildcard listener address (for example [::]:3030) is a valid local
		// bind address but is not a valid peer-advertised endpoint. For local
		// operation, advertise the loopback endpoint while retaining the actual
		// listener address. Production deployments should configure an explicit
		// reachable advertised address.
		_, port, splitErr := net.SplitHostPort(ln.Addr().String())
		if splitErr != nil || port == "" {
			_ = ln.Close()
			return fmt.Errorf("cannot derive advertised address from listener: %w", splitErr)
		}
		n.cfg.AdvertisedAddress = net.JoinHostPort("127.0.0.1", port)
	}

	if err := n.addressPolicy.Validate(n.cfg.AdvertisedAddress); err != nil {
		_ = ln.Close()
		return fmt.Errorf("invalid advertised address: %w", err)
	}
	n.wg.Add(1)
	go n.acceptLoop()
	for _, seed := range n.cfg.Seeds {
		if strings.TrimSpace(seed) != "" {
			n.wg.Add(1)
			go func(a string) { defer n.wg.Done(); n.seedLoop(a) }(strings.TrimSpace(seed))
		}
	}
	n.cfg.Logger.Info("p2p listener started", "address", ln.Addr().String())
	return nil
}
func (n *Network) Stop() {
	if n.cancel != nil {
		n.cancel()
	}
	if n.ln != nil {
		_ = n.ln.Close()
	}
	n.mu.Lock()
	for _, p := range n.peers {
		_ = p.Conn.Close()
	}
	n.peers = make(map[string]*Peer)
	n.mu.Unlock()
	n.wg.Wait()
}
func (n *Network) acceptLoop() {
	defer n.wg.Done()
	for {
		c, err := n.ln.Accept()
		if err != nil {
			select {
			case <-n.ctx.Done():
				return
			default:
				continue
			}
		}
		n.mu.RLock()
		full := len(n.peers) >= n.cfg.MaxPeers
		n.mu.RUnlock()
		if full {
			_ = c.Close()
			continue
		}
		n.wg.Add(1)
		go func() { defer n.wg.Done(); n.handleConn(c, true) }()
	}
}
func (n *Network) seedLoop(addr string) {
	delay := time.Second
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}
		if n.hasAddress(addr) {
			t := time.NewTimer(delay)
			select {
			case <-n.ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			continue
		}
		resolveCtx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
		endpoints, err := n.addressPolicy.ResolveAndValidate(resolveCtx, addr)
		cancel()
		if err != nil {
			n.cfg.Logger.Warn("seed address rejected by policy",
				"address", addr,
				"error", err,
			)
			t := time.NewTimer(delay)
			select {
			case <-n.ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			if delay < 30*time.Second {
				delay *= 2
			}
			continue
		}

		var c net.Conn
		for _, endpoint := range endpoints {
			c, err = net.DialTimeout("tcp", endpoint, 5*time.Second)
			if err == nil {
				break
			}
			c = nil
		}
		if c == nil {
			t := time.NewTimer(delay)
			select {
			case <-n.ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
			if delay < 30*time.Second {
				delay *= 2
			}
			continue
		}

		delay = time.Second
		n.wg.Add(1)
		go func() { defer n.wg.Done(); n.handleConn(c, false) }()
		time.Sleep(3 * time.Second)
	}
}
func (n *Network) hasAddress(addr string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, p := range n.peers {
		if p.Address == addr {
			return true
		}
	}
	return false
}
func (n *Network) handleConn(conn net.Conn, inbound bool) {
	defer conn.Close()

	remoteAddress := ""
	if conn.RemoteAddr() != nil {
		remoteAddress = conn.RemoteAddr().String()
	}
	remoteIPValue := remoteIP(conn.RemoteAddr())

	if remoteIPValue != "" && !n.reputation.Allow(remoteIPValue, time.Now()) {
		n.cfg.Logger.Warn("connection rejected by reputation policy",
			"remote", remoteAddress,
			"inbound", inbound,
		)
		return
	}

	if err := n.acquireHandshake(conn.RemoteAddr()); err != nil {
		n.cfg.Logger.Warn("handshake admission rejected",
			"remote", remoteAddress,
			"inbound", inbound,
			"error", err,
		)
		if remoteIPValue != "" {
			n.recordReputationViolation(remoteIPValue, remoteAddress, security.ViolationMinor)
		}
		return
	}
	defer n.releaseHandshake(conn.RemoteAddr())

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	messageLimit, limitErr := security.NewTokenBucket(
		n.cfg.PeerMessageRate,
		n.cfg.PeerMessageBurst,
		time.Now(),
	)
	if limitErr != nil {
		n.cfg.Logger.Warn("invalid peer message rate configuration", "error", limitErr)
		return
	}

	s := &Session{
		n:            n,
		conn:         conn,
		tracker:      p2p.NewRequestTracker(),
		pending:      make(map[uint64]chan p2p.Frame),
		closed:       make(chan struct{}),
		messageLimit: messageLimit,
	}
	var remoteID, remoteAddr string
	var caps uint32
	var req uint64
	var err error
	if inbound {
		var f p2p.Frame
		f, err = p2p.ReadFrame(conn)
		if err == nil && f.Type == p2p.HELLO {
			req = f.RequestID
			var h p2p.Hello
			h, err = p2p.DecodeHello(f)
			if err == nil {
				err = n.verifyHello(h)
			}
			if err == nil {
				remoteID, remoteAddr, caps = h.NodeID, h.AdvertisedAddress, h.Capabilities
				err = n.sendAck(s, h, req)
			}
		} else if err == nil {
			err = errors.New("expected HELLO")
		}
	} else {
		var hello, challenge []byte
		var helloReq uint64
		hello, challenge, helloReq, err = n.makeHello()
		if err == nil {
			_, err = conn.Write(hello)
		}
		if err == nil {
			var f p2p.Frame
			f, err = p2p.ReadFrame(conn)
			if err == nil {
				var a p2p.HelloAck
				a, err = p2p.DecodeHelloAck(f)
				if err == nil {
					if f.RequestID != helloReq {
						err = errors.New("HELLO_ACK request id mismatch")
					} else {
						err = n.verifyAck(a, f.RequestID, challenge)
					}
				}
				if err == nil {
					remoteID, remoteAddr, caps = a.NodeID, a.AdvertisedAddress, a.Capabilities
				}
			}
		}
	}
	if err != nil {
		n.cfg.Logger.Debug("p2p handshake failed", "inbound", inbound, "error", err)
		if remoteIPValue != "" {
			state := n.recordReputationViolation(remoteIPValue, remoteAddress, security.ViolationMajor)
			n.cfg.Logger.Warn("peer reputation violation",
				"peer", remoteIPValue,
				"state", state,
				"error", err,
			)
		}
		n.sendRejectRaw(conn, rejectFor(err))
		return
	}
	_ = conn.SetDeadline(time.Time{})
	if remoteID == n.cfg.NodeID {
		return
	}
	p := &Peer{ID: remoteID, Address: remoteAddr, Capabilities: caps, Inbound: inbound, Conn: conn, Session: s}
	s.p = p
	n.mu.Lock()
	if len(n.peers) >= n.cfg.MaxPeers {
		n.mu.Unlock()
		return
	}
	if old, ok := n.peers[p.ID]; ok {
		_ = old.Conn.Close()
	}
	n.peers[p.ID] = p
	n.mu.Unlock()
	n.cfg.Logger.Info("p2p handshake established", "peer", p.ID, "address", p.Address)
	n.wg.Add(1)
	go func() { defer n.wg.Done(); s.readLoop() }()
	n.wg.Add(1)
	go func() { defer n.wg.Done(); n.syncPeer(s) }()
	select {
	case <-n.ctx.Done():
	case <-s.closed:
	}
}

func (n *Network) makeHello() ([]byte, []byte, uint64, error) {
	ch, err := identity.RandomChallenge()
	if err != nil {
		return nil, nil, 0, err
	}
	g, err := hex.DecodeString(n.cfg.GenesisHash)
	if err != nil {
		return nil, nil, 0, err
	}
	v := p2p.Hello{ProtocolName: "sayanjali-p2p", NetworkName: n.cfg.NetworkName, NodeID: n.cfg.NodeID, AdvertisedAddress: n.cfg.AdvertisedAddress, VersionMajor: p2p.ProtocolMajor, VersionMinor: p2p.ProtocolMinor, GenesisHash: g, PublicKey: mustHex(n.cfg.PublicKeyHex), Challenge: ch, Capabilities: p2p.CapBlocks | p2p.CapTransactions | p2p.CapSync}
	sb, err := p2p.HelloSigningBytes(v)
	if err != nil {
		return nil, nil, 0, err
	}
	sig, err := n.cfg.Identity.Sign(sb)
	if err != nil {
		return nil, nil, 0, err
	}
	v.Signature = sig
	req := n.nextRequest()
	b, e := p2p.EncodeHello(v, req)
	return b, ch, req, e
}
func (n *Network) sendAck(s *Session, h p2p.Hello, req uint64) error {
	ch, err := identity.RandomChallenge()
	if err != nil {
		return err
	}
	g, err := hex.DecodeString(n.cfg.GenesisHash)
	if err != nil {
		return err
	}
	v := p2p.HelloAck{ProtocolName: "sayanjali-p2p", NetworkName: n.cfg.NetworkName, NodeID: n.cfg.NodeID, AdvertisedAddress: n.cfg.AdvertisedAddress, VersionMajor: p2p.ProtocolMajor, VersionMinor: p2p.ProtocolMinor, GenesisHash: g, PublicKey: mustHex(n.cfg.PublicKeyHex), EchoChallenge: h.Challenge, Challenge: ch, Capabilities: p2p.CapBlocks | p2p.CapTransactions | p2p.CapSync}
	sb, err := p2p.HelloAckSigningBytes(v)
	if err != nil {
		return err
	}
	sig, err := n.cfg.Identity.Sign(sb)
	if err != nil {
		return err
	}
	v.Signature = sig
	b, err := p2p.EncodeHelloAck(v, req)
	if err != nil {
		return err
	}
	_, err = s.conn.Write(b)
	return err
}
func verifyPeerIdentity(nodeID string, publicKey []byte) error {
	if len(publicKey) != 64 {
		return errors.New("invalid public key length")
	}

	publicKeyHex := hex.EncodeToString(publicKey)
	expectedBytes := corecrypto.SHA256Bytes([]byte(publicKeyHex))
	expected := hex.EncodeToString(expectedBytes[:])

	if !strings.EqualFold(nodeID, expected) {
		return errors.New("node id does not match public key")
	}

	return nil
}

func (n *Network) verifyHello(h p2p.Hello) error {
	if h.NetworkName != n.cfg.NetworkName {
		return errors.New("wrong network")
	}
	if hex.EncodeToString(h.GenesisHash) != strings.ToLower(n.cfg.GenesisHash) {
		return errors.New("wrong genesis")
	}
	if err := n.addressPolicy.Validate(h.AdvertisedAddress); err != nil {
		return err
	}
	if h.NodeID == n.cfg.NodeID {
		return errors.New("self peer")
	}
	if err := verifyPeerIdentity(h.NodeID, h.PublicKey); err != nil {
		return err
	}
	sb, err := p2p.HelloSigningBytes(h)
	if err != nil {
		return err
	}
	if !corecrypto.VerifyECDSA(hex.EncodeToString(h.PublicKey), string(sb), h.Signature) {
		return errors.New("invalid hello signature")
	}
	if err := n.handshakeReplay.Consume(h.Challenge, time.Now()); err != nil {
		return err
	}
	return nil
}
func (n *Network) verifyAck(a p2p.HelloAck, req uint64, challenge []byte) error {
	if req == 0 || a.NetworkName != n.cfg.NetworkName || hex.EncodeToString(a.GenesisHash) != strings.ToLower(n.cfg.GenesisHash) || !p2p.SameChallenge(a.EchoChallenge, challenge) {
		return errors.New("invalid hello ack identity")
	}
	if err := verifyPeerIdentity(a.NodeID, a.PublicKey); err != nil {
		return err
	}
	sb, err := p2p.HelloAckSigningBytes(a)
	if err != nil {
		return err
	}
	if !corecrypto.VerifyECDSA(hex.EncodeToString(a.PublicKey), string(sb), a.Signature) {
		return errors.New("invalid ack signature")
	}
	return nil
}
func validateAdvertisedAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" || port == "" {
		return errors.New("invalid advertised address")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("invalid advertised port")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsUnspecified() || ip.IsMulticast()) {
		return errors.New("invalid advertised host")
	}
	return nil
}

func (n *Network) sendRejectRaw(c net.Conn, r p2p.Reject) {
	b, err := p2p.EncodeReject(r, 0)
	if err == nil {
		_, _ = c.Write(b)
	}
}
func rejectFor(err error) p2p.Reject {
	code := p2p.RejectMalformedMessage
	reason := "malformed message"
	closeFlag := true
	s := err.Error()
	switch {
	case strings.Contains(s, "wrong network"):
		code = p2p.RejectWrongNetwork
		reason = "wrong network"
	case strings.Contains(s, "wrong genesis"):
		code = p2p.RejectWrongGenesis
		reason = "wrong genesis"
	case strings.Contains(s, "signature"):
		code = p2p.RejectUnauthorized
		reason = "unauthorized peer"
	}
	return p2p.Reject{Code: code, Close: closeFlag, Reason: reason}
}
func (s *Session) readLoop() {
	defer close(s.closed)
	defer s.n.removePeer(s.p.ID, s)
	for {
		f, err := p2p.ReadFrame(s.conn)
		if err != nil {
			return
		}

		if s.messageLimit != nil && !s.messageLimit.Allow(time.Now(), 1) {
			s.n.cfg.Logger.Warn(
				"peer message rate exceeded",
				"peer", s.p.ID,
				"address", s.p.Address,
			)
			state := s.n.recordReputationViolation(
				s.p.ID,
				s.p.Address,
				security.ViolationMajor,
			)
			_ = s.writeReject(
				f.RequestID,
				p2p.RejectInvalidRequest,
				true,
				"message rate exceeded",
			)
			s.n.cfg.Logger.Warn("peer quarantined or banned",
				"peer", s.p.ID,
				"state", state,
			)
			return
		}

		if !p2p.AllowedInState(p2p.Established, f.Type) {
			return
		}
		if f.RequestID != 0 {
			s.pendingMu.Lock()
			ch := s.pending[f.RequestID]
			s.pendingMu.Unlock()
			if ch != nil {
				select {
				case ch <- f:
				default:
				}
				continue
			}
		}
		if err := s.handle(f); err != nil {
			s.n.cfg.Logger.Debug("peer message rejected", "peer", s.p.ID, "type", f.Type.String(), "error", err)
			state := s.n.recordReputationViolation(
				s.p.ID,
				s.p.Address,
				security.ViolationMinor,
			)
			if state != security.PeerGood {
				s.n.cfg.Logger.Warn("peer reputation state changed",
					"peer", s.p.ID,
					"state", state,
				)
			}
			code := p2p.RejectInvalidRequest
			closeFlag := false
			if f.Type == p2p.NEW_BLOCK {
				code = p2p.RejectInvalidBlock
			}
			if f.Type == p2p.NEW_TRANSACTION {
				code = p2p.RejectInvalidTransaction
			}
			if f.Type == p2p.HELLO || f.Type == p2p.HELLO_ACK {
				code = p2p.RejectMalformedMessage
				closeFlag = true
			}
			_ = s.writeReject(f.RequestID, code, closeFlag, err.Error())
		}
	}
}
func (s *Session) handle(f p2p.Frame) error {
	switch f.Type {
	case p2p.GET_PEERS:
		v, e := p2p.DecodeGetPeers(f)
		if e != nil {
			return e
		}
		return s.replyPeers(f.RequestID, v)
	case p2p.GET_HEADERS:
		v, e := p2p.DecodeGetHeaders(f)
		if e != nil {
			return e
		}
		return s.replyHeaders(f.RequestID, v)
	case p2p.GET_BLOCKS:
		v, e := p2p.DecodeGetBlocks(f)
		if e != nil {
			return e
		}
		return s.replyBlocks(f.RequestID, v)
	case p2p.NEW_BLOCK:
		v, e := p2p.DecodeNewBlock(f)
		if e != nil {
			return e
		}
		return s.acceptBlock(v.Block)
	case p2p.NEW_TRANSACTION:
		v, e := p2p.DecodeNewTransaction(f)
		if e != nil {
			return e
		}
		return s.acceptTx(v.Transaction)
	case p2p.REJECT:
		return nil
	case p2p.PEERS, p2p.HEADERS, p2p.BLOCKS:
		return nil
	}
	return nil
}
func (s *Session) request(payload func(uint64) ([]byte, error), timeout time.Duration) (p2p.Frame, error) {
	id := s.n.nextRequest()
	if err := s.tracker.Reserve(id); err != nil {
		return p2p.Frame{}, err
	}
	defer s.tracker.Complete(id)
	b, err := payload(id)
	if err != nil {
		return p2p.Frame{}, err
	}
	ch := make(chan p2p.Frame, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()
	defer func() { s.pendingMu.Lock(); delete(s.pending, id); s.pendingMu.Unlock() }()
	if err = s.write(b); err != nil {
		return p2p.Frame{}, err
	}
	select {
	case f := <-ch:
		return f, nil
	case <-time.After(timeout):
		return p2p.Frame{}, errors.New("request timeout")
	case <-s.n.ctx.Done():
		return p2p.Frame{}, context.Canceled
	}
}
func (s *Session) replyPeers(id uint64, v p2p.GetPeers) error {
	entries := make([]p2p.Peer, 0)
	s.n.mu.RLock()
	for _, p := range s.n.peers {
		if p.ID > v.StartAfter && p.ID != s.p.ID {
			host, port, e := net.SplitHostPort(p.Address)
			if e == nil {
				var pu int
				_, _ = fmt.Sscan(port, &pu)
				if pu > 0 && pu <= 65535 {
					entries = append(entries, p2p.Peer{NodeID: p.ID, Host: host, Port: uint16(pu), Capabilities: p.Capabilities})
				}
			}
		}
	}
	s.n.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].NodeID < entries[j].NodeID })
	if len(entries) > int(v.Limit) {
		entries = entries[:v.Limit]
	}
	next := ""
	if len(entries) == int(v.Limit) && len(entries) > 0 {
		next = entries[len(entries)-1].NodeID
	}
	b, e := p2p.EncodePeers(p2p.Peers{Entries: entries, NextStartAfter: next}, id)
	if e != nil {
		return e
	}
	return s.write(b)
}
func (s *Session) replyHeaders(id uint64, v p2p.GetHeaders) error {
	bs := s.n.ch.HeadersFromLocator(v.Locator, v.StopHash, int(v.MaxCount))
	items := make([][]byte, 0, len(bs))
	for _, b := range bs {
		x, e := codec.HeaderBytes(b.Header)
		if e != nil {
			return e
		}
		if len(x) > 4096 {
			return errors.New("header too large")
		}
		items = append(items, x)
	}
	out, e := p2p.EncodeHeaders(p2p.Headers{Items: items}, id)
	if e != nil {
		return e
	}
	return s.write(out)
}
func (s *Session) replyBlocks(id uint64, v p2p.GetBlocks) error {
	bs := s.n.ch.BlocksByHashes(v.Hashes)
	items := make([][]byte, 0, len(bs))
	for _, b := range bs {
		x, e := codec.BlockBytes(b)
		if e != nil {
			return e
		}
		if len(x) > p2p.MaxBlockPayload {
			return errors.New("block too large")
		}
		items = append(items, x)
	}
	out, e := p2p.EncodeBlocks(p2p.Blocks{Items: items}, id)
	if e != nil {
		return e
	}
	return s.write(out)
}
func (s *Session) acceptBlock(data []byte) error {
	b, e := codec.DecodeBlock(data)
	if e != nil {
		return e
	}
	ok, reason, e := s.n.ch.Accept(b)
	if e != nil || !ok {
		return errOr(reason, e)
	}
	if reason == "best" {
		hashes := make([]string, 0)
		for _, active := range s.n.ch.ChainCopy() {
			for _, tx := range active.Transactions {
				hashes = append(hashes, tx.TxHash)
			}
		}
		s.n.pool.RemoveHashes(hashes)
	}
	s.n.propagateBlock(b, s.p.ID)
	return nil
}
func (s *Session) acceptTx(data []byte) error {
	tx, e := codec.DecodeTransaction(data)
	if e != nil {
		return e
	}
	if e = s.n.pool.Add(tx, s.n.ch.Balance); e != nil {
		return e
	}
	s.n.propagateTx(tx, s.p.ID)
	return nil
}
func (s *Session) write(b []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, e := s.conn.Write(b)
	return e
}
func (s *Session) writeReject(id uint64, code uint16, closeFlag bool, reason string) error {
	if len(reason) > p2p.MaxStringSize {
		reason = reason[:p2p.MaxStringSize]
	}
	if reason == "" {
		reason = "request rejected"
	}
	b, e := p2p.EncodeReject(p2p.Reject{Code: code, Close: closeFlag, Reason: reason}, id)
	if e != nil {
		return e
	}
	return s.write(b)
}
func (n *Network) syncPeer(s *Session) {
	locator := n.locator()
	for rounds := 0; rounds < 1024; rounds++ {
		f, e := s.request(func(id uint64) ([]byte, error) {
			var stop [32]byte
			return p2p.EncodeGetHeaders(p2p.GetHeaders{Locator: locator, StopHash: stop, MaxCount: 2048}, id)
		}, 30*time.Second)
		if e != nil {
			return
		}
		hs, e := p2p.DecodeHeaders(f)
		if e != nil || len(hs.Items) == 0 {
			return
		}
		hashes := make([][32]byte, 0, len(hs.Items))
		for _, raw := range hs.Items {
			h, e := codec.DecodeHeader(raw)
			if e != nil {
				return
			}
			hb, _, e := block.HashHeader(h)
			if e != nil {
				return
			}
			x, e := hex.DecodeString(hb)
			if e != nil || len(x) != 32 {
				return
			}
			var a [32]byte
			copy(a[:], x)
			hashes = append(hashes, a)
		}
		for i := 0; i < len(hashes); i += p2p.MaxBlocks {
			end := i + p2p.MaxBlocks
			if end > len(hashes) {
				end = len(hashes)
			}
			f, e := s.request(func(id uint64) ([]byte, error) { return p2p.EncodeGetBlocks(p2p.GetBlocks{Hashes: hashes[i:end]}, id) }, 30*time.Second)
			if e != nil {
				return
			}
			bs, e := p2p.DecodeBlocks(f)
			if e != nil {
				return
			}
			for _, raw := range bs.Items {
				b, e := codec.DecodeBlock(raw)
				if e != nil {
					return
				}
				if _, _, e = n.ch.Accept(b); e != nil {
					return
				}
			}
		}
		locator = n.locator()
		if len(hs.Items) < 2048 {
			return
		}
	}
}
func (n *Network) locator() [][32]byte {
	ch := n.ch.ChainCopy()
	out := make([][32]byte, 0, 32)
	for i := len(ch) - 1; i >= 0 && len(out) < 32; i-- {
		x, e := hex.DecodeString(ch[i].Hash)
		if e == nil && len(x) == 32 {
			var a [32]byte
			copy(a[:], x)
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		var z [32]byte
		out = append(out, z)
	}
	return out
}
func (n *Network) propagateBlock(b *block.Block, exclude string) {
	raw, e := codec.BlockBytes(b)
	if e != nil {
		return
	}
	frame, e := p2p.EncodeNewBlock(p2p.NewBlock{Block: raw}, 0)
	if e != nil {
		return
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	for id, p := range n.peers {
		if id == exclude || p.Session == nil {
			continue
		}
		_ = p.Session.write(frame)
	}
}
func (n *Network) propagateTx(tx transaction.Transaction, exclude string) {
	raw, e := codec.TransactionBytes(tx)
	if e != nil {
		return
	}
	frame, e := p2p.EncodeNewTransaction(p2p.NewTransaction{Transaction: raw}, 0)
	if e != nil {
		return
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	for id, p := range n.peers {
		if id == exclude {
			continue
		}
		_ = p.Session.write(frame)
	}
}
func (n *Network) removePeer(id string, session *Session) {
	n.mu.Lock()
	if p, ok := n.peers[id]; ok && p.Session == session {
		delete(n.peers, id)
		_ = p.Conn.Close()
	}
	n.mu.Unlock()
}
func (n *Network) ListenAddress() string {
	if n.ln == nil {
		return ""
	}
	return n.ln.Addr().String()
}

func (n *Network) Peers() []Peer {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]Peer, 0, len(n.peers))
	for _, p := range n.peers {
		out = append(out, *p)
	}
	return out
}
func (n *Network) nextRequest() uint64 {
	id := n.nextReq.Add(1)
	if id == 0 {
		return n.nextReq.Add(1)
	}
	return id
}
func mustHex(s string) []byte { b, _ := hex.DecodeString(s); return b }
func errOr(reason string, e error) error {
	if e != nil {
		return e
	}
	return errors.New(reason)
}

func (n *Network) BroadcastBlock(b *block.Block)                   { n.propagateBlock(b, "") }
func (n *Network) BroadcastTransaction(tx transaction.Transaction) { n.propagateTx(tx, "") }
