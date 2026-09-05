package p2p_test

import (
	"encoding/hex"
	"encoding/json"
	p2p "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2p"
	"os"
	"path/filepath"
	"testing"
)

type maxV struct {
	FrameHex string `json:"frame_hex"`
}

func TestMaximumPayloadVectors(t *testing.T) {
	b, e := os.ReadFile(filepath.Join("fixtures", "valid_max_payload.json"))
	if e != nil {
		t.Fatal(e)
	}
	var v map[string]maxV
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	for n, x := range v {
		d, _ := hex.DecodeString(x.FrameHex)
		f, e := p2p.DecodeFrame(d)
		if e != nil {
			t.Fatalf("%s: %v", n, e)
		}
		if n == "new_transaction_max" {
			if _, e = p2p.DecodeNewTransaction(f); e != nil {
				t.Fatal(e)
			}
		} else {
			if _, e = p2p.DecodeNewBlock(f); e != nil {
				t.Fatal(e)
			}
		}
	}
}
