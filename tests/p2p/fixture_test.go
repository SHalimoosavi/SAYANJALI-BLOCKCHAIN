package p2p_test

import (
	"encoding/hex"
	"encoding/json"
	p2p "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2p"
	"os"
	"path/filepath"
	"testing"
)

type validVector struct {
	FrameHex string `json:"frame_hex"`
}
type invalidVector struct {
	Name     string `json:"name"`
	FrameHex string `json:"frame_hex"`
	Error    string `json:"error"`
}

func TestFrozenValidWireVectors(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("fixtures", "valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vectors map[string]validVector
	if err = json.Unmarshal(b, &vectors); err != nil {
		t.Fatal(err)
	}
	var h [32]byte
	for i := range h {
		h[i] = byte(i)
	}
	enc := map[string][]byte{}
	pub := make([]byte, 64)
	for i := range pub {
		pub[i] = byte(i)
	}
	enc["hello"], _ = p2p.EncodeHello(p2p.Hello{ProtocolName: "sayanjali-p2p", NetworkName: "sayanjali-mainnet-mvp", NodeID: "node-a", AdvertisedAddress: "127.0.0.1:3030", VersionMajor: 1, VersionMinor: 0, GenesisHash: h[:], PublicKey: pub, Challenge: bytes32(7), Signature: bytes64fill(8), Capabilities: 7}, 1)
	enc["hello_ack"], _ = p2p.EncodeHelloAck(p2p.HelloAck{ProtocolName: "sayanjali-p2p", NetworkName: "sayanjali-mainnet-mvp", NodeID: "node-a", AdvertisedAddress: "127.0.0.1:3030", VersionMajor: 1, VersionMinor: 0, GenesisHash: h[:], PublicKey: pub, EchoChallenge: bytes32(7), Challenge: bytes32(9), Signature: bytes64fill(8), Capabilities: 7}, 1)
	enc["get_peers"], _ = p2p.EncodeGetPeers(p2p.GetPeers{StartAfter: "", Limit: 256}, 2)
	enc["peers"], _ = p2p.EncodePeers(p2p.Peers{Entries: []p2p.Peer{{NodeID: "node-a", Host: "127.0.0.1", Port: 3030, Capabilities: 7}}}, 2)
	enc["get_headers"], _ = p2p.EncodeGetHeaders(p2p.GetHeaders{Locator: [][32]byte{h}, MaxCount: 2048}, 3)
	enc["headers"], _ = p2p.EncodeHeaders(p2p.Headers{Items: [][]byte{[]byte(`{"index":1}`)}}, 3)
	enc["get_blocks"], _ = p2p.EncodeGetBlocks(p2p.GetBlocks{Hashes: [][32]byte{h}}, 4)
	enc["blocks"], _ = p2p.EncodeBlocks(p2p.Blocks{Items: [][]byte{[]byte(`{"hash":"00"}`)}}, 4)
	enc["new_block"], _ = p2p.EncodeNewBlock(p2p.NewBlock{Block: []byte(`{"hash":"00"}`)}, 0)
	enc["new_transaction"], _ = p2p.EncodeNewTransaction(p2p.NewTransaction{Transaction: []byte(`{"tx_hash":"00"}`)}, 0)
	enc["reject"], _ = p2p.EncodeReject(p2p.Reject{Code: p2p.RejectInvalidRequest, Close: true, Reason: "invalid request"}, 4)
	for name, v := range vectors {
		got, ok := enc[name]
		if !ok {
			t.Fatalf("missing encoder %s", name)
		}
		want, _ := hex.DecodeString(v.FrameHex)
		if string(got) != string(want) {
			t.Errorf("%s vector mismatch\n got %x\nwant %x", name, got, want)
		}
	}
}
func TestFrozenInvalidWireVectors(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("fixtures", "invalid.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vs []invalidVector
	if err = json.Unmarshal(b, &vs); err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		data, _ := hex.DecodeString(v.FrameHex)
		_, err := p2p.DecodeFrame(data)
		if err == nil {
			t.Errorf("%s: expected %s", v.Name, v.Error)
		}
	}
}
func bytes32(x byte) []byte     { return bytesN(32, x) }
func bytes64() []byte           { return bytesN(64, 0) }
func bytes64fill(x byte) []byte { return bytesN(64, x) }
func bytesN(n int, x byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = x
	}
	return b
}
