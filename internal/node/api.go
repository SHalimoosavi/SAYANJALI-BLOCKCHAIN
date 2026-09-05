package node

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/codec"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)

type API struct{ n *Node }

func (n *Node) ServeAPI() *http.Server {
	mux := http.NewServeMux()
	a := &API{n: n}
	mux.HandleFunc("/health", a.health)
	mux.HandleFunc("/status", a.status)
	mux.HandleFunc("/peers", a.peers)
	mux.HandleFunc("/chain", a.chain)
	mux.HandleFunc("/blocks/", a.block)
	mux.HandleFunc("/transactions", a.transactions)
	mux.HandleFunc("/mine", a.mine)
	mux.HandleFunc("/shutdown", a.shutdown)
	return &http.Server{Addr: n.cfg.APIListenAddress, Handler: securityHeaders(mux)}
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
func (a *API) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	writeJSON(w, 200, a.n.Status())
}
func (a *API) peers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	ps := a.n.net.Peers()
	out := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, map[string]any{"node_id": p.ID, "address": p.Address, "inbound": p.Inbound, "capabilities": p.Capabilities})
	}
	writeJSON(w, 200, out)
}
func (a *API) chain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	c := a.n.chain
	writeJSON(w, 200, map[string]any{"height": c.Height(), "tip_hash": c.TipHash(), "chain_work": c.Work().String(), "supply_base_units": c.Supply()})
}
func (a *API) block(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	h := strings.TrimPrefix(r.URL.Path, "/blocks/")
	b, e := a.n.chain.GetBlock(h)
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	raw, e := codec.BlockBytes(b)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "serialization failure"})
		return
	}
	var obj any
	_ = json.Unmarshal(raw, &obj)
	writeJSON(w, 200, obj)
}
func (a *API) transactions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var tx transaction.Transaction
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		if e := json.NewDecoder(r.Body).Decode(&tx); e != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid transaction"})
			return
		}
		if e := a.n.SubmitTransaction(tx); e != nil {
			writeJSON(w, 400, map[string]string{"error": e.Error()})
			return
		}
		a.n.net.BroadcastTransaction(tx)
		writeJSON(w, 202, map[string]any{"accepted": true, "tx_hash": tx.TxHash})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"count": a.n.pool.Len()})
		return
	}
	w.WriteHeader(405)
}
func (a *API) mine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	receiver := a.n.id.Address
	if v := r.URL.Query().Get("receiver"); v != "" {
		receiver = v
	}
	txs := a.n.pool.List(500)
	b, e := MineNext(a.n.chain, receiver, txs)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	ok, reason, e := a.n.chain.Accept(b)
	if e != nil || !ok {
		writeJSON(w, 400, map[string]string{"error": errText(reason, e)})
		return
	}
	hashes := make([]string, 0)
	for _, active := range a.n.chain.ChainCopy() {
		for _, tx := range active.Transactions {
			hashes = append(hashes, tx.TxHash)
		}
	}
	a.n.pool.RemoveHashes(hashes)
	a.n.net.BroadcastBlock(b)
	writeJSON(w, 201, map[string]any{"accepted": true, "hash": b.Hash, "height": b.Index, "nonce": b.Nonce, "difficulty": b.Difficulty})
}
func (a *API) shutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	writeJSON(w, 202, map[string]any{"stopping": true})
	go a.n.Stop()
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func errText(reason string, e error) string {
	if e != nil {
		return e.Error()
	}
	if reason != "" {
		return reason
	}
	return errors.New("operation failed").Error()
}
