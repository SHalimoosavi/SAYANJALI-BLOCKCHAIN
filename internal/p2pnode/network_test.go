package p2pnode

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/chain"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/codec"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/identity"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/mempool"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2p"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/security"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func testNetwork(t *testing.T, seed []string) (*Network, func()) {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYJ_IDENTITY_ENCRYPTION_KEY", hex.EncodeToString(b))
	dir := t.TempDir()
	id, _, err := identity.LoadOrCreate(dir + "/identity")
	if err != nil {
		t.Fatal(err)
	}
	st, err := storage.Open(dir + "/chain")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := chain.Open(st)
	if err != nil {
		t.Fatal(err)
	}
	p := mempool.New(100)
	ctx, cancel := context.WithCancel(context.Background())
	n := New(Config{NetworkName: "sayanjali-mainnet-mvp", GenesisHash: ch.TipHash(), NodeID: id.NodeID, PublicKeyHex: id.PublicKeyHex, Seeds: seed, MaxPeers: 8, Logger: slog.New(slog.NewTextHandler(os.Stdout, nil)), Identity: *id}, ch, p)
	if err := n.Start(ctx); err != nil {
		cancel()
		st.Close()
		t.Fatal(err)
	}
	return n, func() { cancel(); n.Stop(); st.Close() }
}

func TestSeedAddressPolicyRejectsUnsafeEndpoints(t *testing.T) {
	policy := security.DefaultAddressPolicy(security.PublicTestnet)

	unsafe := []string{
		"127.0.0.1:30303",
		"10.0.0.1:30303",
		"192.168.1.10:30303",
		"169.254.169.254:80",
		"100.100.100.200:80",
		"[::]:30303",
		"[::1]:30303",
	}

	for _, address := range unsafe {
		if _, err := policy.ResolveAndValidate(context.Background(), address); err == nil {
			t.Errorf("expected seed address to be rejected: %s", address)
		}
	}
}

func TestSeedAddressPolicyAllowsPrivateTestnetLoopback(t *testing.T) {
	policy := security.DefaultAddressPolicy(security.PrivateTestnet)

	endpoints, err := policy.ResolveAndValidate(
		context.Background(),
		"127.0.0.1:30303",
	)
	if err != nil {
		t.Fatalf("expected private-testnet loopback to be allowed: %v", err)
	}

	if len(endpoints) != 1 || endpoints[0] != "127.0.0.1:30303" {
		t.Fatalf("unexpected validated endpoints: %v", endpoints)
	}
}

func TestDefaultAdvertisedAddressUsesReachableLoopback(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()
	host, port, err := net.SplitHostPort(n.cfg.AdvertisedAddress)
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" || port == "" {
		t.Fatalf("unexpected advertised address: %q", n.cfg.AdvertisedAddress)
	}
}

func TestTwoNodeHandshake(t *testing.T) {
	a, closeA := testNetwork(t, nil)
	defer closeA()
	b, closeB := testNetwork(t, []string{a.cfg.AdvertisedAddress})
	defer closeB()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(a.Peers()) > 0 && len(b.Peers()) > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("handshake not established: A=%d B=%d", len(a.Peers()), len(b.Peers()))
}
func TestGenesisMatchesAcrossNodes(t *testing.T) {
	g, _ := block.Genesis()
	if g.Hash == "" {
		t.Fatal("empty genesis")
	}
}

func TestTwoNodeBlockPropagation(t *testing.T) {
	a, closeA := testNetwork(t, nil)
	defer closeA()
	b, closeB := testNetwork(t, []string{a.cfg.AdvertisedAddress})
	defer closeB()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (len(a.Peers()) == 0 || len(b.Peers()) == 0) {
		time.Sleep(25 * time.Millisecond)
	}
	if len(a.Peers()) == 0 || len(b.Peers()) == 0 {
		t.Fatal("nodes did not handshake")
	}
	receiver := b.cfg.Identity.Address
	g, _ := block.Genesis()
	blockTimestamp := g.Timestamp + float64(protocol.TargetBlockTimeSeconds)
	tx := transaction.Transaction{Sender: "SYJ-COINBASE-0000000000000000000000000000", Receiver: receiver, AmountBaseUnits: 5_000_000_000, Timestamp: blockTimestamp}
	bld, err := block.New(1, g.Hash, blockTimestamp, 0, 4, []transaction.Transaction{tx})
	if err != nil {
		t.Fatal(err)
	}
	for bld.Nonce < 2_000_000 {
		if bld.MeetsDifficulty(4) {
			break
		}
		bld.Nonce++
		if err := bld.Recompute(); err != nil {
			t.Fatal(err)
		}
	}
	if !bld.MeetsDifficulty(4) {
		t.Fatal("test block was not mined")
	}
	if ok, _, err := a.ch.Accept(bld); err != nil || !ok {
		t.Fatalf("accept block: ok=%v err=%v", ok, err)
	}
	a.BroadcastBlock(bld)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b.ch.HasBlock(bld.Hash) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("peer did not receive propagated block")
}

func TestVerifyPeerIdentityValid(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	pub, err := hex.DecodeString(n.cfg.Identity.PublicKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	if err := verifyPeerIdentity(n.cfg.Identity.NodeID, pub); err != nil {
		t.Fatalf("valid identity rejected: %v", err)
	}
}

func TestVerifyPeerIdentityRejectsMismatchedNodeID(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	pub, err := hex.DecodeString(n.cfg.Identity.PublicKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	err = verifyPeerIdentity("0000000000000000000000000000000000000000000000000000000000000000", pub)
	if err == nil {
		t.Fatal("expected mismatched node ID to be rejected")
	}
}

func TestVerifyPeerIdentityRejectsWrongPublicKey(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	pub, err := hex.DecodeString(n.cfg.Identity.PublicKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	pub[0] ^= 0xff

	err = verifyPeerIdentity(n.cfg.Identity.NodeID, pub)
	if err == nil {
		t.Fatal("expected wrong public key to be rejected")
	}
}

func TestVerifyHelloConsumesChallengeOnce(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	peerPriv := make([]byte, 32)
	for i := range peerPriv {
		peerPriv[i] = byte(i + 1)
	}

	peerPub, err := crypto.PublicKeyFromPrivate(peerPriv)
	if err != nil {
		t.Fatal(err)
	}
	peerPubHex := hex.EncodeToString(peerPub)

	pub := append([]byte(nil), peerPub...)

	peerIDBytes := crypto.SHA256Bytes([]byte(peerPubHex))
	peerNodeID := hex.EncodeToString(peerIDBytes[:])

	genesis, err := hex.DecodeString(n.cfg.GenesisHash)
	if err != nil {
		t.Fatal(err)
	}

	challenge := make([]byte, 32)
	challenge[0] = 1

	h := p2p.Hello{
		ProtocolName:      "sayanjali-p2p",
		NetworkName:       n.cfg.NetworkName,
		NodeID:            peerNodeID,
		AdvertisedAddress: "127.0.0.1:30399",
		VersionMajor:      p2p.ProtocolMajor,
		VersionMinor:      p2p.ProtocolMinor,
		GenesisHash:       genesis,
		PublicKey:         pub,
		Challenge:         challenge,
		Capabilities:      p2p.CapBlocks | p2p.CapTransactions | p2p.CapSync,
	}

	sb, err := p2p.HelloSigningBytes(h)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := crypto.SignECDSA(peerPriv, string(sb))
	if err != nil {
		t.Fatal(err)
	}
	h.Signature = sig

	if err := n.verifyHello(h); err != nil {
		t.Fatalf("first HELLO rejected: %v", err)
	}

	if err := n.verifyHello(h); !errors.Is(err, security.ErrReplay) {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestVerifyHelloAcceptsDifferentChallenge(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	peerPriv := make([]byte, 32)
	for i := range peerPriv {
		peerPriv[i] = byte(i + 1)
	}

	peerPub, err := crypto.PublicKeyFromPrivate(peerPriv)
	if err != nil {
		t.Fatal(err)
	}
	peerPubHex := hex.EncodeToString(peerPub)

	pub := append([]byte(nil), peerPub...)

	peerIDBytes := crypto.SHA256Bytes([]byte(peerPubHex))
	peerNodeID := hex.EncodeToString(peerIDBytes[:])

	genesis, err := hex.DecodeString(n.cfg.GenesisHash)
	if err != nil {
		t.Fatal(err)
	}

	makeHello := func(value byte) p2p.Hello {
		challenge := make([]byte, 32)
		challenge[0] = value

		h := p2p.Hello{
			ProtocolName:      "sayanjali-p2p",
			NetworkName:       n.cfg.NetworkName,
			NodeID:            peerNodeID,
			AdvertisedAddress: "127.0.0.1:30400",
			VersionMajor:      p2p.ProtocolMajor,
			VersionMinor:      p2p.ProtocolMinor,
			GenesisHash:       genesis,
			PublicKey:         pub,
			Challenge:         challenge,
			Capabilities:      p2p.CapBlocks | p2p.CapTransactions | p2p.CapSync,
		}

		sb, err := p2p.HelloSigningBytes(h)
		if err != nil {
			t.Fatal(err)
		}

		sig, err := crypto.SignECDSA(peerPriv, string(sb))
		if err != nil {
			t.Fatal(err)
		}
		h.Signature = sig
		return h
	}

	if err := n.verifyHello(makeHello(2)); err != nil {
		t.Fatalf("first challenge rejected: %v", err)
	}

	if err := n.verifyHello(makeHello(3)); err != nil {
		t.Fatalf("different challenge rejected: %v", err)
	}
}

func TestVerifyHelloConcurrentDuplicateChallenge(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	peerPriv := make([]byte, 32)
	for i := range peerPriv {
		peerPriv[i] = byte(i + 1)
	}

	peerPub, err := crypto.PublicKeyFromPrivate(peerPriv)
	if err != nil {
		t.Fatal(err)
	}
	peerPubHex := hex.EncodeToString(peerPub)
	peerNodeIDBytes := crypto.SHA256Bytes([]byte(peerPubHex))
	peerNodeID := hex.EncodeToString(peerNodeIDBytes[:])

	genesis, err := hex.DecodeString(n.cfg.GenesisHash)
	if err != nil {
		t.Fatal(err)
	}

	challenge := make([]byte, 32)
	challenge[0] = 0x7f

	h := p2p.Hello{
		ProtocolName:      "sayanjali-p2p",
		NetworkName:       n.cfg.NetworkName,
		NodeID:            peerNodeID,
		AdvertisedAddress: "127.0.0.1:30402",
		VersionMajor:      p2p.ProtocolMajor,
		VersionMinor:      p2p.ProtocolMinor,
		GenesisHash:       genesis,
		PublicKey:         peerPub,
		Challenge:         challenge,
		Capabilities:      p2p.CapBlocks | p2p.CapTransactions | p2p.CapSync,
	}

	sb, err := p2p.HelloSigningBytes(h)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := crypto.SignECDSA(peerPriv, string(sb))
	if err != nil {
		t.Fatal(err)
	}
	h.Signature = sig

	const workers = 32

	results := make(chan error, workers)
	start := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			results <- n.verifyHello(h)
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	accepted := 0
	replayed := 0

	for err := range results {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, security.ErrReplay):
			replayed++
		default:
			t.Fatalf("unexpected concurrent HELLO result: %v", err)
		}
	}

	if accepted != 1 {
		t.Fatalf("expected exactly one accepted HELLO, got %d", accepted)
	}

	if replayed != workers-1 {
		t.Fatalf("expected %d replay rejections, got %d", workers-1, replayed)
	}
}

func TestVerifyAckRejectsMismatchedNodeID(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	pub, err := hex.DecodeString(n.cfg.Identity.PublicKeyHex)
	if err != nil {
		t.Fatal(err)
	}

	genesis, err := hex.DecodeString(n.cfg.GenesisHash)
	if err != nil {
		t.Fatal(err)
	}

	echo := make([]byte, 32)
	echo[0] = 4

	challenge := make([]byte, 32)
	challenge[0] = 5

	a := p2p.HelloAck{
		ProtocolName:      "sayanjali-p2p",
		NetworkName:       n.cfg.NetworkName,
		NodeID:            "0000000000000000000000000000000000000000000000000000000000000000",
		AdvertisedAddress: "127.0.0.1:30401",
		VersionMajor:      p2p.ProtocolMajor,
		VersionMinor:      p2p.ProtocolMinor,
		GenesisHash:       genesis,
		PublicKey:         pub,
		EchoChallenge:     echo,
		Challenge:         challenge,
		Capabilities:      p2p.CapBlocks | p2p.CapTransactions | p2p.CapSync,
	}

	err = n.verifyAck(a, 1, echo)
	if err == nil {
		t.Fatal("expected mismatched ACK identity to be rejected")
	}
}

func TestHandshakeAdmissionPerIPLimit(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	n.cfg.MaxPendingHandshakes = 8
	n.cfg.MaxHandshakesPerIP = 2

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30301}

	if err := n.acquireHandshake(addr); err != nil {
		t.Fatalf("first admission rejected: %v", err)
	}
	if err := n.acquireHandshake(addr); err != nil {
		t.Fatalf("second admission rejected: %v", err)
	}
	defer n.releaseHandshake(addr)
	defer n.releaseHandshake(addr)

	if err := n.acquireHandshake(addr); err == nil {
		t.Fatal("expected per-IP handshake limit rejection")
	}
}

func TestHandshakeAdmissionGlobalLimit(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	n.cfg.MaxPendingHandshakes = 2
	n.cfg.MaxHandshakesPerIP = 8

	a := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30301}
	b := &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 30302}

	if err := n.acquireHandshake(a); err != nil {
		t.Fatalf("first admission rejected: %v", err)
	}
	if err := n.acquireHandshake(b); err != nil {
		t.Fatalf("second admission rejected: %v", err)
	}
	defer n.releaseHandshake(a)
	defer n.releaseHandshake(b)

	if err := n.acquireHandshake(&net.TCPAddr{IP: net.ParseIP("127.0.0.3"), Port: 30303}); err == nil {
		t.Fatal("expected global pending-handshake limit rejection")
	}
}

func TestHandshakeAdmissionCleanup(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	n.cfg.MaxPendingHandshakes = 1
	n.cfg.MaxHandshakesPerIP = 1

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30301}

	if err := n.acquireHandshake(addr); err != nil {
		t.Fatalf("admission rejected: %v", err)
	}

	if err := n.acquireHandshake(addr); err == nil {
		t.Fatal("expected admission to be exhausted")
	}

	n.releaseHandshake(addr)

	if err := n.acquireHandshake(addr); err != nil {
		t.Fatalf("admission was not released: %v", err)
	}
	n.releaseHandshake(addr)
}

func TestPeerMessageRateLimit(t *testing.T) {
	now := time.Now()

	bucket, err := security.NewTokenBucket(1, 2, now)
	if err != nil {
		t.Fatal(err)
	}

	if !bucket.Allow(now, 1) {
		t.Fatal("first message unexpectedly rejected")
	}
	if !bucket.Allow(now, 1) {
		t.Fatal("second message unexpectedly rejected")
	}
	if bucket.Allow(now, 1) {
		t.Fatal("expected third immediate message to be rate limited")
	}
}

func TestLegitimatePeersUnaffectedByAdmissionLimit(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	n.cfg.MaxPendingHandshakes = 4
	n.cfg.MaxHandshakesPerIP = 2

	a := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 30301}
	b := &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 30302}

	if err := n.acquireHandshake(a); err != nil {
		t.Fatalf("legitimate peer A rejected: %v", err)
	}
	n.releaseHandshake(a)

	if err := n.acquireHandshake(b); err != nil {
		t.Fatalf("legitimate peer B rejected after A cleanup: %v", err)
	}
	n.releaseHandshake(b)

	n.admissionMu.Lock()
	defer n.admissionMu.Unlock()

	if n.pendingHandshakes != 0 {
		t.Fatalf("pending handshake leak: %d", n.pendingHandshakes)
	}
	if len(n.handshakesByIP) != 0 {
		t.Fatalf("per-IP admission leak: %d entries", len(n.handshakesByIP))
	}
}

func TestLivePeerMessageRateLimit(t *testing.T) {
	cfg := Config{
		PeerMessageRate:  1,
		PeerMessageBurst: 1,
		Logger:           slog.Default(),
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverConnCh := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			serverConnCh <- conn
		}
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	server := <-serverConnCh
	defer server.Close()

	n := &Network{
		cfg:   cfg,
		peers: make(map[string]*Peer),
	}

	limit, err := security.NewTokenBucket(1, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	s := &Session{
		n:    n,
		conn: server,
		p: &Peer{
			ID:      "test-peer",
			Address: server.RemoteAddr().String(),
		},
		messageLimit: limit,
		closed:       make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		s.readLoop()
		close(done)
	}()

	frame := p2p.Frame{
		VersionMajor: p2p.ProtocolMajor,
		VersionMinor: p2p.ProtocolMinor,
		Type:         p2p.GET_PEERS,
		RequestID:    1,
	}

	data, err := p2p.EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Write(data); err != nil {
		t.Fatal(err)
	}

	frame.RequestID = 2
	data, err = p2p.EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Write(data); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("rate-limited peer session did not close")
	}
}

func TestReputationQuarantineBlocksConnection(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	now := time.Now()
	peerID := "quarantine-peer"
	address := "127.0.0.1:30303"

	state := n.reputation.Violate(
		peerID,
		address,
		security.ViolationMajor,
		now,
	)
	if state != security.PeerQuarantine {
		t.Fatalf("expected quarantine after major violation, got %v", state)
	}

	if n.reputation.Allow(peerID, now.Add(time.Second)) {
		t.Fatal("quarantined peer was incorrectly allowed")
	}
}

func TestReputationBanBlocksConnection(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	now := time.Now()
	peerID := "banned-peer"
	address := "127.0.0.1:30304"

	if state := n.reputation.Violate(
		peerID,
		address,
		security.ViolationMajor,
		now,
	); state != security.PeerQuarantine {
		t.Fatalf("expected quarantine, got %v", state)
	}

	if state := n.reputation.Violate(
		peerID,
		address,
		security.ViolationMajor,
		now.Add(time.Second),
	); state != security.PeerBan {
		t.Fatalf("expected ban after cumulative violations, got %v", state)
	}

	if n.reputation.Allow(peerID, now.Add(2*time.Second)) {
		t.Fatal("banned peer was incorrectly allowed")
	}
}

func TestReputationCooldownRestoresPeer(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	now := time.Now()
	peerID := "cooldown-peer"

	if state := n.reputation.Violate(
		peerID,
		"127.0.0.1:30305",
		security.ViolationMajor,
		now,
	); state != security.PeerQuarantine {
		t.Fatalf("expected quarantine, got %v", state)
	}

	if n.reputation.Allow(peerID, now.Add(6*time.Minute)) == false {
		t.Fatal("peer was not restored after quarantine cooldown")
	}
}

func TestReputationLegitimatePeerAllowed(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	if !n.reputation.Allow("legitimate-peer", time.Now()) {
		t.Fatal("new legitimate peer was incorrectly rejected")
	}
}

func TestLiveReputationBlocksQuarantinedIP(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	ip := "127.0.0.1"
	now := time.Now()

	state := n.reputation.Violate(
		ip,
		"127.0.0.1:30303",
		security.ViolationMajor,
		now,
	)
	if state != security.PeerQuarantine {
		t.Fatalf("expected quarantine, got %v", state)
	}

	conn, err := net.DialTimeout("tcp", n.ListenAddress(), time.Second)
	if err != nil {
		// Connection refusal is also acceptable because the listener may
		// reject/close the connection immediately.
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var b [1]byte
	if _, err := conn.Read(b[:]); err == nil {
		t.Fatal("quarantined IP unexpectedly received data")
	}
}

func TestLiveReputationBlocksBannedIP(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()

	ip := "127.0.0.1"
	now := time.Now()

	if state := n.reputation.Violate(
		ip,
		"127.0.0.1:30303",
		security.ViolationCritical,
		now,
	); state != security.PeerQuarantine {
		t.Fatalf("expected quarantine after critical violation, got %v", state)
	}

	if state := n.reputation.Violate(
		ip,
		"127.0.0.1:30303",
		security.ViolationCritical,
		now.Add(time.Second),
	); state != security.PeerBan {
		t.Fatalf("expected ban after cumulative violations, got %v", state)
	}

	conn, err := net.DialTimeout("tcp", n.ListenAddress(), time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var b [1]byte
	if _, err := conn.Read(b[:]); err == nil {
		t.Fatal("banned IP unexpectedly received data")
	}
}

func TestGenesisAllocationCannotEnterP2PTransactionPath(t *testing.T) {
	p := mempool.New(10)
	n := &Network{pool: p}
	s := &Session{n: n}
	tx := transaction.Transaction{Sender: protocol.GenesisAllocationSender, Receiver: "SYJ0000000000000000000000000000000000000000", AmountBaseUnits: 1, Timestamp: 1, TxHash: "not-valid"}
	raw, err := codec.TransactionBytes(tx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.acceptTx(raw); err == nil {
		t.Fatal("accepted genesis allocation through NEW_TRANSACTION path")
	}
	if p.Len() != 0 {
		t.Fatal("genesis allocation entered mempool through P2P path")
	}
}
