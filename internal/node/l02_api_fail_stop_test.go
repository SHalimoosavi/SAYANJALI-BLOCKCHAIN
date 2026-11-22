package node

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/security"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
)

func l02TestAPI(n *Node) *API {
	n.cfg.APIAuthToken = "l02-test-token"
	return &API{n: n, rates: make(map[string]*security.TokenBucket), maxRateEntries: 4096}
}

func l02FatalAccept(n *Node, calls *atomic.Int32) {
	n.acceptFn = func(*block.Block) (bool, string, error) {
		if calls != nil {
			calls.Add(1)
		}
		return false, "storage", errors.Join(errors.New("injected storage failure"), storage.ErrStoreFailed)
	}
}

func TestMineReturns503AfterFatal(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	var calls atomic.Int32
	l02FatalAccept(n, &calls)
	api := l02TestAPI(n)
	for i := 0; i < 2; i++ {
		r := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mine", nil)
		api.mine(r, req)
		if r.Code != http.StatusServiceUnavailable {
			t.Fatalf("POST /mine attempt %d status = %d, want 503; body=%s", i+1, r.Code, r.Body.String())
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("acceptFn calls = %d after fatal failure, want exactly 1", got)
	}
}

func TestHealthReports503WhenUnhealthy(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	l02FatalAccept(n, nil)
	_, _, _ = n.AcceptBlock(n.Chain().Tip())
	r := httptest.NewRecorder()
	api := l02TestAPI(n)
	api.health(r, httptest.NewRequest(http.MethodGet, "/health", nil))
	if r.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /health status = %d, want 503", r.Code)
	}
}

func TestSubmitTransactionReturns503WhenUnhealthy(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	l02FatalAccept(n, nil)
	_, _, _ = n.AcceptBlock(n.Chain().Tip())
	api := l02TestAPI(n)
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString("{}"))
	req.Header.Set("Authorization", "Bearer l02-test-token")
	r := httptest.NewRecorder()
	api.transactions(r, req)
	if r.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST /transactions status = %d, want 503; body=%s", r.Code, r.Body.String())
	}
}

func TestMutatingEndpointsNeverSucceedAfterUnhealthy(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	var calls atomic.Int32
	l02FatalAccept(n, &calls)
	_, _, _ = n.AcceptBlock(n.Chain().Tip())
	api := l02TestAPI(n)

	mine := httptest.NewRecorder()
	api.mine(mine, httptest.NewRequest(http.MethodPost, "/mine", nil))
	if mine.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST /mine after unhealthy status = %d, want 503", mine.Code)
	}

	txReq := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString("{}"))
	txReq.Header.Set("Authorization", "Bearer l02-test-token")
	tx := httptest.NewRecorder()
	api.transactions(tx, txReq)
	if tx.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST /transactions after unhealthy status = %d, want 503", tx.Code)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("acceptFn calls after unhealthy flag = %d, want 1 initial call only", got)
	}
}
