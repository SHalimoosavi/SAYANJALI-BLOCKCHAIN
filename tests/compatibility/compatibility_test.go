package compatibility_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/genesis"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/state"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func vector(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "protocol", "test-vectors", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err = dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}
func s(v any) string { return v.(string) }
func f(v any) float64 {
	n := v.(json.Number)
	x, e := n.Float64()
	if e != nil {
		panic(e)
	}
	return x
}
func ii(v any) int {
	n := v.(json.Number)
	x, e := n.Int64()
	if e != nil {
		panic(e)
	}
	return int(x)
}
func u64(v any) uint64 {
	n := v.(json.Number)
	x, e := strconv.ParseUint(string(n), 10, 64)
	if e != nil {
		panic(e)
	}
	return x
}

func concreteProtocolValues(v any) any {
	switch x := v.(type) {
	case json.Number:
		raw := string(x)
		if strings.ContainsAny(raw, ".eE") {
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				panic(err)
			}
			return f
		}
		if strings.HasPrefix(raw, "-") {
			i, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				panic(err)
			}
			return i
		}
		u, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			panic(err)
		}
		return u
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = concreteProtocolValues(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = concreteProtocolValues(item)
		}
		return out
	default:
		return v
	}
}

func TestCanonicalGenesisHeader(t *testing.T) {
	v := vector(t, "genesis")
	in := concreteProtocolValues(v["header_payload"]).(map[string]any)
	got, err := canonicaljson.String(in)
	if err != nil {
		t.Fatal(err)
	}
	if got != s(v["canonical_header_json"]) {
		t.Fatalf("VECTOR: genesis serialization\nEXPECTED: %s\nACTUAL: %s", s(v["canonical_header_json"]), got)
	}
	if hex.EncodeToString([]byte(got)) != s(v["canonical_header_utf8_hex"]) {
		t.Fatalf("VECTOR: genesis utf8 hex mismatch")
	}
}

func TestAddressVector(t *testing.T) {
	v := vector(t, "address")
	priv := s(v["private_key_hex"])
	k, err := wallet.FromPrivateKeyHex(priv)
	if err != nil {
		t.Fatal(err)
	}
	if k.PublicKeyHex != s(v["public_key_hex"]) {
		t.Fatalf("VECTOR: address public key\nEXPECTED: %s\nACTUAL: %s", s(v["public_key_hex"]), k.PublicKeyHex)
	}
	if k.Address != s(v["address"]) {
		t.Fatalf("VECTOR: address\nEXPECTED: %s\nACTUAL: %s", s(v["address"]), k.Address)
	}
	if corecrypto.SHA256String(k.PublicKeyHex) != s(v["public_key_sha256"]) {
		t.Fatal("VECTOR: public-key sha mismatch")
	}
}

func TestTransactionVector(t *testing.T) {
	v := vector(t, "transaction")
	p := v["signing_payload"].(map[string]any)
	tx := transaction.Transaction{Sender: s(p["sender"]), Receiver: s(p["receiver"]), AmountBaseUnits: u64(p["amount_base_units"]), Timestamp: f(p["timestamp"]), SenderPublicKey: s(v["sender_public_key"]), Signature: s(v["fixed_signature"]), TxHash: s(v["transaction_hash"])}
	msg, err := tx.SigningMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg != s(v["canonical_signing_message"]) {
		t.Fatalf("VECTOR: transaction signing serialization\nEXPECTED: %s\nACTUAL: %s", s(v["canonical_signing_message"]), msg)
	}
	h, err := tx.ComputeHash()
	if err != nil {
		t.Fatal(err)
	}
	if h != s(v["transaction_hash"]) {
		t.Fatalf("VECTOR: transaction hash\nEXPECTED: %s\nACTUAL: %s", s(v["transaction_hash"]), h)
	}
	if !wallet.Verify(tx.SenderPublicKey, msg, tx.Signature) {
		t.Fatal("VECTOR: fixed signature did not verify")
	}
	if !tx.Verify() {
		t.Fatal("VECTOR: full transaction verification failed")
	}
}

func TestMerkleVectors(t *testing.T) {
	v := vector(t, "merkle")
	for _, raw := range v["cases"].([]any) {
		c := raw.(map[string]any)
		arr := []string{}
		for _, x := range c["transaction_hashes"].([]any) {
			arr = append(arr, s(x))
		}
		got := block.MerkleRoot(arr)
		if got != s(c["expected_merkle_root"]) {
			t.Fatalf("VECTOR: merkle %s\nEXPECTED: %s\nACTUAL: %s", s(c["name"]), s(c["expected_merkle_root"]), got)
		}
	}
}

func TestBlockAndPoWVectors(t *testing.T) {
	v := vector(t, "block")
	p := v["header_payload"].(map[string]any)
	h := block.Header{Index: int64(ii(p["index"])), PreviousHash: s(p["previous_hash"]), Timestamp: f(p["timestamp"]), Nonce: u64(p["nonce"]), Difficulty: ii(p["difficulty"]), MerkleRoot: s(p["merkle_root"])}
	got, canon, err := block.HashHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if canon != s(v["canonical_header_json"]) {
		t.Fatalf("VECTOR: block serialization\nEXPECTED: %s\nACTUAL: %s", s(v["canonical_header_json"]), canon)
	}
	if got != s(v["expected_block_hash"]) {
		t.Fatalf("VECTOR: block hash\nEXPECTED: %s\nACTUAL: %s", s(v["expected_block_hash"]), got)
	}
	pv := vector(t, "pow")
	for _, raw := range pv["cases"].([]any) {
		c := raw.(map[string]any)
		d := ii(c["difficulty"])
		meets := consensus.MeetsDifficulty(got, d)
		if meets != c["meets_difficulty"].(bool) {
			t.Fatalf("VECTOR: pow difficulty %d", d)
		}
	}
}

func TestDifficultyVectors(t *testing.T) {
	v := vector(t, "difficulty")
	cfg := protocol.DefaultDifficultyConfig()
	for _, raw := range v["cases"].([]any) {
		c := raw.(map[string]any)
		first := f(c["first_timestamp"])
		last := f(c["last_timestamp"])
		base := ii(c["previous_difficulty"])
		chain := make([]*block.Block, 10)
		for j := 0; j < 10; j++ {
			ts := first
			if j == 9 {
				ts = last
			} else {
				ts = first + float64(j)*(last-first)/9
			}
			chain[j] = &block.Block{Header: block.Header{Index: int64(j), Timestamp: ts, Difficulty: base}}
		}
		got, err := consensus.NextDifficulty(chain, cfg, 4)
		if err != nil {
			t.Fatalf("VECTOR: difficulty %s: %v", s(c["name"]), err)
		}
		exp := ii(c["expected_difficulty"])
		if got != exp {
			t.Fatalf("VECTOR: difficulty %s\nEXPECTED: %d\nACTUAL: %d", s(c["name"]), exp, got)
		}
	}
}

func TestChainWorkVectors(t *testing.T) {
	v := vector(t, "chain_work")
	for _, raw := range v["cases"].([]any) {
		c := raw.(map[string]any)
		var ds []int
		for _, x := range c["difficulties"].([]any) {
			ds = append(ds, ii(x))
		}
		got := consensus.ChainWorkFromDifficulties(ds)
		exp := new(big.Int)
		exp.SetString(stringifyNumber(c["accumulated_work"]), 10)
		if got.Cmp(exp) != 0 {
			t.Fatalf("VECTOR: chain work\nEXPECTED: %s\nACTUAL: %s", exp, got)
		}
	}
}
func stringifyNumber(v any) string { return new(big.Int).SetUint64(u64(v)).String() }

func TestMonetaryVectors(t *testing.T) {
	v := vector(t, "monetary")
	if protocol.BaseUnitsPerSYJ != u64(v["base_units_per_syj"]) {
		t.Fatal("VECTOR: base unit scale")
	}
	if protocol.MaxSupplyBaseUnits != u64(v["max_supply_base_units"]) {
		t.Fatal("VECTOR: max supply")
	}
	if protocol.DefaultBlockRewardUnits != u64(v["default_block_reward_base_units"]) {
		t.Fatal("VECTOR: reward")
	}
	for _, raw := range v["cases"].([]any) {
		c := raw.(map[string]any)
		if s(c["name"]) != "conversion" {
			continue
		}
		ins := c["inputs"].([]any)
		outs := c["base_units"].([]any)
		for j := range ins {
			got, err := state.ToBaseUnits(s(ins[j]))
			if err != nil {
				t.Fatal(err)
			}
			exp := u64(outs[j])
			if got != exp {
				t.Fatalf("VECTOR: monetary conversion %s", s(ins[j]))
			}
		}
	}
	if state.FormatAmount(123456789) != "1.23456789" {
		t.Fatal("VECTOR: format_amount")
	}
	if _, err := state.ToBaseUnits("720000000.00000001"); err == nil {
		t.Fatal("VECTOR: over-supply conversion accepted")
	}
}

func TestGenesisConstruction(t *testing.T) {
	g, err := genesis.Build()
	if err != nil {
		t.Fatal(err)
	}
	v := vector(t, "genesis")
	if g.Hash != s(v["block_hash"]) {
		t.Fatalf("VECTOR: genesis hash\nEXPECTED: %s\nACTUAL: %s", s(v["block_hash"]), g.Hash)
	}
	if g.Transactions[0].TxHash != s(v["genesis_transaction_hash"]) {
		t.Fatal("VECTOR: genesis tx hash")
	}
	if corecrypto.SHA256String(protocol.GenesisMessage) != s(v["genesis_transaction_hash"]) {
		t.Fatal("VECTOR: genesis transaction was not derived from message")
	}
}

func TestCanonicalPrimitiveEscaping(t *testing.T) {
	got, err := canonicaljson.String(map[string]any{"z": "é", "a": true, "n": 1735689600.0})
	if err != nil {
		t.Fatal(err)
	}
	exp := "{\"a\":true,\"n\":1735689600.0,\"z\":\"\\u00e9\"}"
	if got != exp {
		t.Fatalf("canonical mismatch: %s", got)
	}
	_ = sha256.Sum256([]byte(got))
}
