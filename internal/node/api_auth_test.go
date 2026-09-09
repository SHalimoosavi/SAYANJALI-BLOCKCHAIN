package node

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)

func startTestAPI(t *testing.T, token string) (*Node, string, func()) {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SYJ_IDENTITY_ENCRYPTION_KEY", hex.EncodeToString(b))
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AdvertisedAddress = ""
	cfg.APIListenAddress = "127.0.0.1:0"
	cfg.APIAuthToken = token

	ctx, cancel := context.WithCancel(context.Background())
	n := New(cfg)
	if err := n.Start(ctx); err != nil {
		t.Fatal(err)
	}

	srv := n.ServeAPI()
	ln := httptest.NewServer(srv.Handler)

	cleanup := func() {
		ln.Close()
		cancel()
		n.Stop()
	}

	return n, ln.URL, cleanup
}

func doAuthed(t *testing.T, method, url, bearer string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}

	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	return resp
}

func TestMineRequiresAuth(t *testing.T) {
	_, base, cleanup := startTestAPI(t, "correct-token")
	defer cleanup()

	resp := doAuthed(t, http.MethodPost, base+"/mine", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-token: status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	resp.Body.Close()

	resp = doAuthed(t, http.MethodPost, base+"/mine", "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-token: status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	resp.Body.Close()

	resp = doAuthed(t, http.MethodPost, base+"/mine", "correct-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("correct-token: status=%d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

func TestMineDisabledWithoutConfiguredToken(t *testing.T) {
	_, base, cleanup := startTestAPI(t, "")
	defer cleanup()

	resp := doAuthed(t, http.MethodPost, base+"/mine", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	resp.Body.Close()

	resp = doAuthed(t, http.MethodPost, base+"/mine", "anything-at-all")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("supplied token with no configured token: status=%d, want %d",
			resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestShutdownRequiresAuth(t *testing.T) {
	_, base, cleanup := startTestAPI(t, "correct-token")
	defer cleanup()

	resp := doAuthed(t, http.MethodPost, base+"/shutdown", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-token: status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	resp.Body.Close()

	resp = doAuthed(t, http.MethodPost, base+"/shutdown", "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-token: status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	resp.Body.Close()

	resp = doAuthed(t, http.MethodPost, base+"/shutdown", "correct-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("correct-token: status=%d, want %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestTransactionsPostRequiresAuthGetDoesNot(t *testing.T) {
	_, base, cleanup := startTestAPI(t, "correct-token")
	defer cleanup()

	// GET remains public.
	resp := doAuthed(t, http.MethodGet, base+"/transactions", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /transactions: status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	resp.Body.Close()

	tx := transaction.Transaction{}
	body, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}

	// Missing token.
	req, err := http.NewRequest(
		http.MethodPost,
		base+"/transactions",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	r1, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if r1.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /transactions no-token: status=%d, want %d",
			r1.StatusCode, http.StatusUnauthorized)
	}
	r1.Body.Close()

	// Wrong token.
	req, err = http.NewRequest(
		http.MethodPost,
		base+"/transactions",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")

	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /transactions wrong-token: status=%d, want %d",
			r2.StatusCode, http.StatusUnauthorized)
	}
	r2.Body.Close()

	// Correct token passes authentication. The zero-value transaction may
	// subsequently fail transaction validation, but it must not fail with 401.
	req, err = http.NewRequest(
		http.MethodPost,
		base+"/transactions",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer correct-token")

	r3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r3.Body.Close()

	if r3.StatusCode == http.StatusUnauthorized {
		t.Fatalf("POST /transactions with correct token was rejected as unauthorized")
	}
}

func TestReadOnlyEndpointsRemainUnauthenticated(t *testing.T) {
	n, base, cleanup := startTestAPI(t, "correct-token")
	defer cleanup()

	// /blocks/ requires an actual existing block hash.
	blockHash := n.Chain().TipHash()

	paths := []string{
		"/health",
		"/status",
		"/peers",
		"/chain",
		"/blocks/" + blockHash,
		"/transactions",
	}

	for _, path := range paths {
		resp := doAuthed(t, http.MethodGet, base+path, "")
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s without auth: status=%d, want %d",
				path, resp.StatusCode, http.StatusOK)
		}
	}
}

func TestMineReceiverParameterRemoved(t *testing.T) {
	n, base, cleanup := startTestAPI(t, "correct-token")
	defer cleanup()

	resp := doAuthed(
		t,
		http.MethodPost,
		base+"/mine?receiver=SYJattackercontrolledaddress0000000000000",
		"correct-token",
	)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusCreated)
	}

	b, err := n.Chain().GetBlock(n.Chain().TipHash())
	if err != nil {
		t.Fatal(err)
	}

	for _, tx := range b.Transactions {
		if tx.Receiver == "SYJattackercontrolledaddress0000000000000" {
			t.Fatal("mining reward was redirected to caller-supplied receiver")
		}
	}
}
