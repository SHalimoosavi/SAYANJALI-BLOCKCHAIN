package p2pnode

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"strings"
	"testing"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/codec"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2p"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/security"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
)

func l02FatalP2PCause() error {
	return fmt.Errorf("injected p2p storage failure: %w", storage.ErrStoreFailed)
}

func l02PipeSession(t *testing.T, n *Network, accept func(*block.Block) (bool, string, error)) (*Session, net.Conn, []byte) {
	t.Helper()
	server, client := net.Pipe()
	peer := &Peer{ID: "l02-fatal-peer", Address: "127.0.0.1:39001", Conn: server}
	n.mu.Lock()
	n.peers[peer.ID] = peer
	n.mu.Unlock()
	n.acceptBlock = accept
	s := &Session{n: n, p: peer, conn: server, closed: make(chan struct{}), pending: make(map[uint64]chan p2p.Frame)}
	peer.Session = s
	genesis := n.ch.Tip()
	raw, err := codec.BlockBytes(genesis)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := p2p.EncodeNewBlock(p2p.NewBlock{Block: raw}, 0)
	if err != nil {
		t.Fatal(err)
	}
	go s.readLoop()
	return s, client, wire
}

func l02SendAndReadReject(t *testing.T, client net.Conn, wire []byte) {
	t.Helper()
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(wire); err != nil {
		t.Fatalf("write NEW_BLOCK: %v", err)
	}
	_, _ = p2p.ReadFrame(client) // Either a reject or connection close completes the response path.
}

func TestFatalStorageFailureIsNotPeerViolation(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()
	s, client, wire := l02PipeSession(t, n, func(*block.Block) (bool, string, error) {
		return false, "storage", l02FatalP2PCause()
	})
	defer client.Close()
	// Two prior minor violations leave the peer below the configured threshold.
	// A fatal local storage failure must not add a third violation.
	n.reputation.Violate(s.p.ID, s.p.Address, security.ViolationMinor, time.Now())
	n.reputation.Violate(s.p.ID, s.p.Address, security.ViolationMinor, time.Now())
	if !n.reputation.Allow(s.p.ID, time.Now()) {
		t.Fatal("peer was unexpectedly quarantined before the fatal storage failure")
	}
	l02SendAndReadReject(t, client, wire)
	if !n.reputation.Allow(s.p.ID, time.Now()) {
		t.Fatal("fatal local storage failure added a peer violation and quarantined the peer")
	}
}

func TestNewBlockAfterFatalFailureNotAcceptedAgain(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()
	var calls atomic.Int32
	firstProcessed := make(chan struct{}, 2)
	_, client, wire := l02PipeSession(t, n, func(*block.Block) (bool, string, error) {
		calls.Add(1)
		firstProcessed <- struct{}{}
		return false, "storage", l02FatalP2PCause()
	})
	defer client.Close()

	_ = client.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := client.Write(wire); err != nil {
		t.Fatalf("write first NEW_BLOCK: %v", err)
	}
	select {
	case <-firstProcessed:
	case <-time.After(5 * time.Second):
		t.Fatal("first NEW_BLOCK was not passed to the accept callback within 5 seconds")
	}
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := p2p.ReadFrame(client); err != nil {
		t.Fatalf("read first NEW_BLOCK rejection: %v", err)
	}

	_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write(wire); err == nil {
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, readErr := p2p.ReadFrame(client); readErr == nil {
			t.Fatal("node answered the second NEW_BLOCK after fatal failure")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("accept callback calls = %d after fatal failure, want exactly 1", got)
	}
	if !errors.Is(l02FatalP2PCause(), storage.ErrStoreFailed) {
		t.Fatal("test fatal cause does not wrap storage sentinel")
	}
}

// TestSyncStopsAfterFatalFailure exercises the syncPeer block-acceptance path.
// NO RED EVIDENCE: syncPeer already returns on any Accept error.
func TestSyncStopsAfterFatalFailure(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	_ = server.SetDeadline(time.Now().Add(3 * time.Second))
	s := &Session{n: n, p: &Peer{ID: "l02-sync-peer", Address: "127.0.0.1:39002", Conn: client}, conn: client, tracker: p2p.NewRequestTracker(), pending: make(map[uint64]chan p2p.Frame), closed: make(chan struct{})}
	var calls atomic.Int32
	n.acceptBlock = func(*block.Block) (bool, string, error) {
		calls.Add(1)
		return false, "storage", l02FatalP2PCause()
	}
	serverDone := make(chan error, 1)
	go func() {
		genesis := n.ch.Tip()
		header, err := codec.HeaderBytes(genesis.Header)
		if err != nil {
			serverDone <- err
			return
		}
		f, err := p2p.ReadFrame(server)
		if err != nil {
			serverDone <- err
			return
		}
		response, err := p2p.EncodeHeaders(p2p.Headers{Items: [][]byte{header}}, f.RequestID)
		if err != nil {
			serverDone <- err
			return
		}
		if _, err = server.Write(response); err != nil {
			serverDone <- err
			return
		}
		f, err = p2p.ReadFrame(server)
		if err != nil {
			serverDone <- err
			return
		}
		rawBlock, err := codec.BlockBytes(genesis)
		if err != nil {
			serverDone <- err
			return
		}
		response, err = p2p.EncodeBlocks(p2p.Blocks{Items: [][]byte{rawBlock}}, f.RequestID)
		if err != nil {
			serverDone <- err
			return
		}
		_, err = server.Write(response)
		serverDone <- err
	}()
	readLoopDone := make(chan struct{})
	go func() {
		s.readLoop()
		close(readLoopDone)
	}()
	syncDone := make(chan struct{})
	go func() {
		n.syncPeer(s)
		close(syncDone)
	}()
	select {
	case <-syncDone:
	case <-time.After(2 * time.Second):
		t.Fatal("syncPeer did not stop after fatal storage failure")
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("sync test peer: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("sync test peer did not finish its two request/response exchanges")
	}
	_ = client.Close()
	select {
	case <-readLoopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("sync session read loop did not exit after client close")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("sync acceptance calls = %d, want 1 before fail-stop", got)
	}
}

// TestUnknownAncestorSyncGoroutineStopsOnShutdown has NO RED EVIDENCE.
func TestUnknownAncestorSyncGoroutineStopsOnShutdown(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()
	started := make(chan struct{})
	finished := make(chan struct{})
	n.syncPeerFn = func(*Session) {
		close(started)
		<-n.ctx.Done()
		close(finished)
	}
	n.acceptBlock = func(*block.Block) (bool, string, error) {
		return false, "unknown_ancestor", errors.New("injected unknown ancestor")
	}
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	s := &Session{n: n, p: &Peer{ID: "l02-unknown-ancestor", Address: "127.0.0.1:39003", Conn: client}, conn: client, tracker: p2p.NewRequestTracker(), pending: make(map[uint64]chan p2p.Frame), closed: make(chan struct{})}
	raw, err := codec.BlockBytes(n.ch.Tip())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.acceptBlock(raw); err == nil {
		t.Fatal("unknown ancestor acceptance unexpectedly succeeded")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("unknown-ancestor sync worker did not start")
	}
	stopDone := make(chan struct{})
	go func() {
		n.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Network.Stop did not finish after canceling unknown-ancestor sync worker")
	}
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("unknown-ancestor sync worker did not exit on shutdown")
	}
	n.startSyncPeer(s)
	if got := n.stopping; !got {
		t.Fatal("Network.Stop did not set stopping flag")
	}
}


func TestFatalAcceptRejectFrameDoesNotExposeCause(t *testing.T) {
	n, closeN := testNetwork(t, nil)
	defer closeN()
	_, client, wire := l02PipeSession(t, n, func(*block.Block) (bool, string, error) {
		return false, "storage", errors.Join(errors.New("SECRET-PATH-/tmp/secret-journal: permission denied"), storage.ErrStoreFailed)
	})
	defer client.Close()
	_ = client.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := client.Write(wire); err != nil {
		t.Fatalf("write NEW_BLOCK: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	frame, err := p2p.ReadFrame(client)
	if err != nil {
		t.Fatalf("read fatal NEW_BLOCK rejection: %v", err)
	}
	reject, err := p2p.DecodeReject(frame)
	if err != nil {
		t.Fatalf("decode fatal NEW_BLOCK rejection: %v", err)
	}
	if strings.Contains(reject.Reason, "SECRET-PATH") || strings.Contains(reject.Reason, "/tmp/") {
		t.Fatalf("P2P rejection leaked fatal cause: %q", reject.Reason)
	}
	if reject.Reason != "node unavailable" {
		t.Fatalf("P2P rejection reason = %q, want %q", reject.Reason, "node unavailable")
	}
}
