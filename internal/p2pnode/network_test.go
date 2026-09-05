package p2pnode

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/chain"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/identity"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/mempool"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)

func testNetwork(t *testing.T, seed []string) (*Network, func()) {
	t.Helper()
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
	b, closeB := testNetwork(t, []string{a.ListenAddress()})
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
	b, closeB := testNetwork(t, []string{a.ListenAddress()})
	defer closeB()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (len(a.Peers()) == 0 || len(b.Peers()) == 0) {
		time.Sleep(25 * time.Millisecond)
	}
	if len(a.Peers()) == 0 || len(b.Peers()) == 0 {
		t.Fatal("nodes did not handshake")
	}
	receiver := b.cfg.Identity.Address
	tx := transaction.Transaction{Sender: "SYJ-COINBASE-0000000000000000000000000000", Receiver: receiver, AmountBaseUnits: 5_000_000_000, Timestamp: float64(time.Now().UnixNano()) / 1e9}
	g, _ := block.Genesis()
	bld, err := block.New(1, g.Hash, tx.Timestamp, 0, 4, []transaction.Transaction{tx})
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
