package p2pnode

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
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
	_, client, wire := l02PipeSession(t, n, func(*block.Block) (bool, string, error) {
		calls.Add(1)
		return false, "storage", l02FatalP2PCause()
	})
	defer client.Close()
	l02SendAndReadReject(t, client, wire)
	if _, err := client.Write(wire); err == nil {
		_, _ = p2p.ReadFrame(client)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("accept callback calls = %d after fatal failure, want 1", got)
	}
	if !errors.Is(l02FatalP2PCause(), storage.ErrStoreFailed) {
		t.Fatal("test fatal cause does not wrap storage sentinel")
	}
}
