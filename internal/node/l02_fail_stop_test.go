package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
)

func newL02RunningNode(t *testing.T, dir string) *Node {
	t.Helper()
	setNodeTestIdentityKey(t)
	cfg := DefaultConfig(dir)
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AdvertisedAddress = ""
	cfg.APIListenAddress = "127.0.0.1:0"
	n := New(cfg)
	if err := n.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(n.Stop)
	return n
}

func l02FatalCause() error {
	return fmt.Errorf("injected storage failure: %w", storage.ErrStoreFailed)
}

func TestNodeFatalStorageFailureMarksUnhealthy(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	n.acceptFn = func(*block.Block) (bool, string, error) { return false, "storage", l02FatalCause() }
	_, _, err := n.AcceptBlock(n.Chain().Tip())
	if !errors.Is(err, storage.ErrStoreFailed) {
		t.Fatalf("AcceptBlock error = %v, want storage sentinel", err)
	}
	if n.Healthy() {
		t.Fatal("Healthy() = true after fatal storage failure; want false")
	}
}

func TestNodeErrReturnsFirstCause(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	cause := l02FatalCause()
	n.acceptFn = func(*block.Block) (bool, string, error) { return false, "storage", cause }
	_, _, _ = n.AcceptBlock(n.Chain().Tip())
	if got := n.Err(); !errors.Is(got, storage.ErrStoreFailed) {
		t.Fatalf("Err() = %v, want first cause wrapping storage.ErrStoreFailed", got)
	}
}

func TestNodeFatalStorageFailureShutsDownOnce(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	var mu sync.Mutex
	calls := 0
	n.acceptFn = func(*block.Block) (bool, string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return false, "storage", l02FatalCause()
	}
	_, _, _ = n.AcceptBlock(n.Chain().Tip())
	select {
	case <-n.Done():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Done() remained open after fatal storage failure")
	}
	mu.Lock()
	got := calls
	mu.Unlock()
	if got != 1 {
		t.Fatalf("acceptFn calls = %d, want 1", got)
	}
}

func TestNodeConcurrentFatalFailuresShutDownOnce(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	var wg sync.WaitGroup
	var mu sync.Mutex
	calls := 0
	n.acceptFn = func(*block.Block) (bool, string, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return false, "storage", l02FatalCause()
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = n.AcceptBlock(n.Chain().Tip())
		}()
	}
	wg.Wait()
	select {
	case <-n.Done():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("concurrent fatal failures did not stop node")
	}
	mu.Lock()
	got := calls
	mu.Unlock()
	if got < 1 || got > 8 {
		t.Fatalf("acceptFn calls = %d, want 1..8", got)
	}
}

func TestNodeRestartAfterFatalStopOnSameDataDir(t *testing.T) {
	dir := t.TempDir()
	n := newL02RunningNode(t, dir)
	n.acceptFn = func(*block.Block) (bool, string, error) { return false, "storage", l02FatalCause() }
	_, _, _ = n.AcceptBlock(n.Chain().Tip())
	select {
	case <-n.Done():
	case <-time.After(250 * time.Millisecond):
		t.Fatal("fatal node did not stop; cannot verify directory lock release")
	}
	n2 := New(DefaultConfig(dir))
	if err := n2.Start(context.Background()); err != nil {
		t.Fatalf("restart with same data directory: %v", err)
	}
	n2.Stop()
}

func TestNodeOrdinaryErrorsDoNotShutDown(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	ordinary := errors.New("ordinary write failure with successful rollback")
	n.acceptFn = func(*block.Block) (bool, string, error) { return false, "storage", ordinary }
	_, _, err := n.AcceptBlock(n.Chain().Tip())
	if !errors.Is(err, ordinary) {
		t.Fatalf("AcceptBlock error = %v, want ordinary error", err)
	}
	if !n.Healthy() || n.Err() != nil {
		t.Fatalf("ordinary error changed health: healthy=%v err=%v", n.Healthy(), n.Err())
	}
	select {
	case <-n.Done():
		t.Fatal("ordinary error stopped node")
	default:
	}
}

func TestOrdinaryStorageErrorDoesNotShutDown(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	ordinary := errors.New("ordinary write failure with successful rollback")
	n.acceptFn = func(*block.Block) (bool, string, error) { return false, "storage", ordinary }
	_, _, err := n.AcceptBlock(n.Chain().Tip())
	if !errors.Is(err, ordinary) {
		t.Fatalf("AcceptBlock error = %v, want ordinary error", err)
	}
	if !n.Healthy() || n.Err() != nil {
		t.Fatal("ordinary storage error marked node unhealthy")
	}
	select {
	case <-n.Done():
		t.Fatal("ordinary storage error stopped node")
	default:
	}
}
