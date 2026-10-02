package c2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

const (
	depth    = 256
	leafTag  = "53594a2d53544154452d534d542d56332d4c45414600"
	nodeTag  = "53594a2d53544154452d534d542d56332d4e4f444500"
	emptyTag = "53594a2d53544154452d534d542d56332d454d50545900"
)

type account struct {
	Address string
	Balance uint64
	Nonce   uint64
}

type vectorDoc struct {
	Encoding map[string]any `json:"encoding"`
	Empty    []struct {
		Level int    `json:"level"`
		Hash  string `json:"hash"`
	} `json:"empty_subtree_hashes"`
	Vectors []map[string]any `json:"vectors"`
}

func sha(data []byte) []byte   { s := sha256.Sum256(data); return s[:] }
func u16(v int) []byte         { b := make([]byte, 2); binary.BigEndian.PutUint16(b, uint16(v)); return b }
func u32(v int) []byte         { b := make([]byte, 4); binary.BigEndian.PutUint32(b, uint32(v)); return b }
func u64(v uint64) []byte      { b := make([]byte, 8); binary.BigEndian.PutUint64(b, v); return b }
func tag(hexTag string) []byte { b, _ := hex.DecodeString(hexTag); return b }

var (
	leafDomain  = tag(leafTag)
	nodeDomain  = tag(nodeTag)
	emptyDomain = tag(emptyTag)
)

func emptyHashes() [depth + 1][]byte {
	var out [depth + 1][]byte
	out[depth] = sha(append(append([]byte{}, emptyDomain...), u16(depth)...))
	for level := depth - 1; level >= 0; level-- {
		buf := append(append([]byte{}, nodeDomain...), out[level+1]...)
		buf = append(buf, out[level+1]...)
		out[level] = sha(buf)
	}
	return out
}

func keyFor(address string) []byte { return sha([]byte(address)) }
func leafBytes(a account) []byte {
	addr := []byte(a.Address)
	b := []byte{1}
	b = append(b, keyFor(a.Address)...)
	b = append(b, u32(len(addr))...)
	b = append(b, addr...)
	b = append(b, u64(a.Balance)...)
	b = append(b, u64(a.Nonce)...)
	return b
}
func leafHash(a account) []byte { return sha(append(append([]byte{}, leafDomain...), leafBytes(a)...)) }
func bitAt(key []byte, level int) int {
	index := level - 1
	return int((key[index/8] >> uint(7-index%8)) & 1)
}

type tree struct {
	leaves map[string]account
}

func newTree() *tree { return &tree{leaves: map[string]account{}} }
func (t *tree) put(a account) error {
	k := hex.EncodeToString(keyFor(a.Address))
	if old, ok := t.leaves[k]; ok && old.Address != a.Address {
		return fmt.Errorf("duplicate key")
	}
	t.leaves[k] = a
	return nil
}
func (t *tree) del(address string) error {
	k := hex.EncodeToString(keyFor(address))
	old, ok := t.leaves[k]
	if !ok || old.Address != address {
		return fmt.Errorf("missing account")
	}
	delete(t.leaves, k)
	return nil
}

func (t *tree) root() []byte {
	e := emptyHashes()
	if len(t.leaves) == 0 {
		return e[0]
	}
	items := make([]account, 0, len(t.leaves))
	for _, a := range t.leaves {
		items = append(items, a)
	}
	return t.subroot(0, items, e)
}
func (t *tree) subroot(level int, items []account, e [depth + 1][]byte) []byte {
	if level == depth {
		return leafHash(items[0])
	}
	left, right := make([]account, 0), make([]account, 0)
	for _, a := range items {
		if bitAt(keyFor(a.Address), level+1) == 0 {
			left = append(left, a)
		} else {
			right = append(right, a)
		}
	}
	var lh, rh []byte
	if len(left) == 0 {
		lh = e[level+1]
	} else {
		lh = t.subroot(level+1, left, e)
	}
	if len(right) == 0 {
		rh = e[level+1]
	} else {
		rh = t.subroot(level+1, right, e)
	}
	b := append(append([]byte{}, nodeDomain...), lh...)
	b = append(b, rh...)
	return sha(b)
}

func asAccount(m map[string]any) account {
	return account{Address: m["address"].(string), Balance: uint64(m["balance"].(float64)), Nonce: uint64(m["nonce"].(float64))}
}
func findVector(vs []map[string]any, id string) map[string]any {
	for _, v := range vs {
		if v["id"] == id {
			return v
		}
	}
	return nil
}
func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func verifyProof(t *testing.T, p map[string]any, expectedRoot []byte) {
	sibs := p["siblings_root_to_leaf"].([]any)
	dirs := p["directions_root_to_leaf"].([]any)
	if len(sibs) != depth || len(dirs) != depth {
		t.Fatalf("proof lengths: %d/%d", len(sibs), len(dirs))
	}
	cur := hexBytes(t, p["leaf_hash"].(string))
	for i := depth - 1; i >= 0; i-- {
		s := hexBytes(t, sibs[i].(string))
		d := int(dirs[i].(float64))
		var b []byte
		if d == 0 {
			b = append(append([]byte{}, nodeDomain...), cur...)
			b = append(b, s...)
		} else {
			b = append(append([]byte{}, nodeDomain...), s...)
			b = append(b, cur...)
		}
		cur = sha(b)
	}
	if !reflect.DeepEqual(cur, expectedRoot) || p["verification_result"] != true {
		t.Fatalf("proof verification mismatch")
	}
}

func TestFrozenStateSMTV3Vectors(t *testing.T) {
	data, err := os.ReadFile("../../protocol/test-vectors/state-smt-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc vectorDoc
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) != 10 {
		t.Fatalf("vector count=%d want 10", len(doc.Vectors))
	}
	e := emptyHashes()
	if len(doc.Empty) != depth+1 {
		t.Fatalf("empty hash count=%d want 257", len(doc.Empty))
	}
	for _, x := range doc.Empty {
		if x.Level < 0 || x.Level > depth || hex.EncodeToString(e[x.Level]) != x.Hash {
			t.Fatalf("empty hash mismatch level %d", x.Level)
		}
	}
	v1 := findVector(doc.Vectors, "V1")
	if v1["root"] != hex.EncodeToString(e[0]) {
		t.Fatal("V1 root mismatch")
	}
	v2 := findVector(doc.Vectors, "V2")
	a1 := asAccount(v2["accounts"].([]any)[0].(map[string]any))
	t2 := newTree()
	if err := t2.put(a1); err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(t2.root()); got != v2["root"] {
		t.Fatalf("V2 root %s != %s", got, v2["root"])
	}
	for _, id := range []string{"V3", "V4"} {
		v := findVector(doc.Vectors, id)
		tr := newTree()
		for _, raw := range v["accounts"].([]any) {
			if err := tr.put(asAccount(raw.(map[string]any))); err != nil {
				t.Fatal(err)
			}
		}
		if got := hex.EncodeToString(tr.root()); got != v["root"] {
			t.Fatalf("%s root mismatch", id)
		}
	}
	v5 := findVector(doc.Vectors, "V5")
	tr := newTree()
	for _, raw := range v5["operations"].([]any) {
		op := raw.(map[string]any)
		a := asAccount(op)
		if err := tr.put(a); err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(tr.root()); got != op["resulting_root"] {
			t.Fatalf("V5 operation root mismatch")
		}
	}
	v6 := findVector(doc.Vectors, "V6")
	tr = newTree()
	initial := asAccount(v6["initial"].(map[string]any))
	if err := tr.put(initial); err != nil {
		t.Fatal(err)
	}
	op := v6["operation"].(map[string]any)
	updated := asAccount(op)
	if err := tr.put(updated); err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(tr.root()); got != v6["final_root"] {
		t.Fatal("V6 root mismatch")
	}
	v7 := findVector(doc.Vectors, "V7")
	tr = newTree()
	for _, addr := range []string{"SYJ-delete-a", "SYJ-delete-b"} {
		bal := uint64(50)
		nonce := uint64(1)
		if addr == "SYJ-delete-b" {
			bal = 60
			nonce = 2
		}
		if err := tr.put(account{addr, bal, nonce}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.del("SYJ-delete-a"); err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(tr.root()); got != v7["final_root"] {
		t.Fatal("V7 root mismatch")
	}
	v8 := findVector(doc.Vectors, "V8")
	tr = newTree()
	for _, raw := range v8["accounts"].([]any) {
		if err := tr.put(asAccount(raw.(map[string]any))); err != nil {
			t.Fatal(err)
		}
	}
	if len(tr.leaves) != 1000 {
		t.Fatalf("V8 accounts=%d", len(tr.leaves))
	}
	if got := hex.EncodeToString(tr.root()); got != v8["root"] {
		t.Fatal("V8 root mismatch")
	}
	for _, id := range []string{"V9", "V10"} {
		v := findVector(doc.Vectors, id)
		for _, raw := range v["proofs"].([]any) {
			p := raw.(map[string]any)
			verifyProof(t, p, hexBytes(t, p["expected_root"].(string)))
		}
	}
}
