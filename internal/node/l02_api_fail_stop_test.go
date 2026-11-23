package node

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// TestConcurrentMineCannotCommitAfterFatalFlip deterministically holds an
// in-flight mining acceptance while another request marks the node failed.
func TestConcurrentMineCannotCommitAfterFatalFlip(t *testing.T) {
	for i := 0; i < 20; i++ {
		t.Run("iteration", func(t *testing.T) {
			testConcurrentMineCannotCommitAfterFatalFlip(t)
		})
	}
}

func testConcurrentMineCannotCommitAfterFatalFlip(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	entered := make(chan struct{})
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	var calls atomic.Int32
	n.acceptFn = func(*block.Block) (bool, string, error) {
		switch calls.Add(1) {
		case 1:
			close(entered)
			<-release
			return true, "best", nil
		default:
			return false, "storage", errors.Join(errors.New("injected storage failure"), storage.ErrStoreFailed)
		}
	}
	api := l02TestAPI(n)
	beforeTip := n.Chain().TipHash()
	mineDone := make(chan int, 1)
	go func() {
		r := httptest.NewRecorder()
		api.mine(r, httptest.NewRequest(http.MethodPost, "/mine", nil))
		mineDone <- r.Code
	}()
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatalf("in-flight /mine did not reach injected acceptFn after %s", 30*time.Second)
	}
	fatalDone := make(chan struct{})
	go func() {
		_, _, _ = n.AcceptBlock(n.Chain().Tip())
		close(fatalDone)
	}()
	select {
	case <-fatalDone:
	case <-time.After(10 * time.Second):
		t.Fatalf("fatal acceptance did not complete within %s", 10*time.Second)
	}
	if !errors.Is(n.Err(), storage.ErrStoreFailed) {
		t.Fatalf("Err() = %v, want storage sentinel before releasing in-flight mine", n.Err())
	}
	close(release)
	released = true
	select {
	case status := <-mineDone:
		if status != http.StatusServiceUnavailable {
			t.Fatalf("in-flight POST /mine status = %d after fatal flip, want 503", status)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("in-flight /mine did not finish after release within %s", 10*time.Second)
	}
	if got := n.Chain().TipHash(); got != beforeTip {
		t.Fatalf("chain tip changed after fatal flip: got %s want %s", got, beforeTip)
	}
	if got := n.Mempool().Len(); got != 0 {
		t.Fatalf("transaction pool contains %d transactions after fatal flip, want 0", got)
	}
	mine := httptest.NewRecorder()
	api.mine(mine, httptest.NewRequest(http.MethodPost, "/mine", nil))
	if mine.Code != http.StatusServiceUnavailable {
		t.Fatalf("later POST /mine status = %d, want 503", mine.Code)
	}
	txReq := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString("{}"))
	txReq.Header.Set("Authorization", "Bearer l02-test-token")
	tx := httptest.NewRecorder()
	api.transactions(tx, txReq)
	if tx.Code != http.StatusServiceUnavailable {
		t.Fatalf("later POST /transactions status = %d, want 503", tx.Code)
	}
	shutdown := httptest.NewRecorder()
	api.shutdown(shutdown, httptest.NewRequest(http.MethodPost, "/shutdown", nil))
	if shutdown.Code != http.StatusServiceUnavailable {
		t.Fatalf("later POST /shutdown status = %d, want 503", shutdown.Code)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("acceptFn calls = %d, want the in-flight and fatal trigger only", got)
	}
	if got := n.Mempool().Len(); got != 0 {
		t.Fatalf("transaction pool contains %d transactions after later mutating requests, want 0", got)
	}
}

const l02SafeStorageFailureText = "storage failure: the node stopped to protect data integrity"

func l02MarkerCause() error {
	return errors.Join(errors.New("SECRET-PATH-/tmp/secret-journal: permission denied"), storage.ErrStoreFailed)
}

type l02FatalOnReadBody struct {
	reader  *strings.Reader
	trigger func()
	fired   bool
}

func (r *l02FatalOnReadBody) Read(p []byte) (int, error) {
	if !r.fired {
		r.fired = true
		r.trigger()
	}
	return r.reader.Read(p)
}

func assertL02SafeHTTPFailure(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	body := response.Body.String()
	if strings.Contains(body, "SECRET-PATH") || strings.Contains(body, "/tmp/") {
		t.Fatalf("HTTP response leaked fatal cause: %s", body)
	}
	if !strings.Contains(body, l02SafeStorageFailureText) {
		t.Fatalf("HTTP response = %s, want safe failure text %q", body, l02SafeStorageFailureText)
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("HTTP status = %d, want 503; body=%s", response.Code, body)
	}
}

func TestFatalCauseNeverAppearsInUnhealthyHTTPResponses(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	n.markFatal(l02MarkerCause())
	api := l02TestAPI(n)

	mine := httptest.NewRecorder()
	api.mine(mine, httptest.NewRequest(http.MethodPost, "/mine", nil))
	assertL02SafeHTTPFailure(t, mine)

	txReq := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString("{}"))
	txReq.Header.Set("Authorization", "Bearer l02-test-token")
	tx := httptest.NewRecorder()
	api.transactions(tx, txReq)
	assertL02SafeHTTPFailure(t, tx)

	health := httptest.NewRecorder()
	api.health(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	assertL02SafeHTTPFailure(t, health)

	status := httptest.NewRecorder()
	api.status(status, httptest.NewRequest(http.MethodGet, "/status", nil))
	assertL02SafeHTTPFailure(t, status)

	shutdown := httptest.NewRecorder()
	api.shutdown(shutdown, httptest.NewRequest(http.MethodPost, "/shutdown", nil))
	assertL02SafeHTTPFailure(t, shutdown)
}

func TestMineFatalAcceptCauseNeverAppearsInHTTPResponse(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	n.acceptFn = func(*block.Block) (bool, string, error) {
		return false, "storage", l02MarkerCause()
	}
	api := l02TestAPI(n)
	response := httptest.NewRecorder()
	api.mine(response, httptest.NewRequest(http.MethodPost, "/mine", nil))
	assertL02SafeHTTPFailure(t, response)
}

func TestTransactionFatalCauseNeverAppearsAfterHealthPrecheck(t *testing.T) {
	n := newL02RunningNode(t, t.TempDir())
	api := l02TestAPI(n)
	body := &l02FatalOnReadBody{
		reader: strings.NewReader("{}"),
		trigger: func() {
			n.markFatal(l02MarkerCause())
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/transactions", body)
	req.Header.Set("Authorization", "Bearer l02-test-token")
	response := httptest.NewRecorder()
	api.transactions(response, req)
	assertL02SafeHTTPFailure(t, response)
}
