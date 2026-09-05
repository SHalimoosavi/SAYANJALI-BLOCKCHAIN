package p2p_test

import (
	"encoding/hex"
	"encoding/json"
	p2p "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2p"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type payloadVector struct {
	Name       string `json:"name"`
	FrameHex   string `json:"frame_hex"`
	ErrorClass string `json:"error_class"`
}

func TestFrozenInvalidPayloadVectors(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("fixtures", "invalid_payload.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vs []payloadVector
	if err = json.Unmarshal(b, &vs); err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		data, _ := hex.DecodeString(v.FrameHex)
		f, e := p2p.DecodeFrame(data)
		if e == nil {
			switch f.Type {
			case p2p.GET_PEERS:
				_, e = p2p.DecodeGetPeers(f)
			case p2p.PEERS:
				_, e = p2p.DecodePeers(f)
			case p2p.GET_HEADERS:
				_, e = p2p.DecodeGetHeaders(f)
			case p2p.NEW_TRANSACTION:
				_, e = p2p.DecodeNewTransaction(f)
			case p2p.REJECT:
				_, e = p2p.DecodeReject(f)
			}
		}
		if e == nil {
			t.Errorf("%s: expected rejection", v.Name)
			continue
		}
		if !strings.Contains(strings.ToLower(e.Error()), v.ErrorClass) {
			t.Errorf("%s: got %q want class %q", v.Name, e.Error(), v.ErrorClass)
		}
	}
}
