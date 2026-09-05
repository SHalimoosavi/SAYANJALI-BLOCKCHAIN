package p2p

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"
)

func hash32(s string) [32]byte { b, _ := hex.DecodeString(s); var h [32]byte; copy(h[:], b); return h }

func TestFrameRoundTrip(t *testing.T) {
	f := Frame{ProtocolMajor, ProtocolMinor, NEW_TRANSACTION, 7, []byte(`{"amount":1}`)}
	enc, err := EncodeFrame(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeFrame(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != f.Type || got.RequestID != f.RequestID || !bytes.Equal(got.Payload, f.Payload) {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}
func TestReadFrameTruncated(t *testing.T) {
	_, err := ReadFrame(bytes.NewReader([]byte("SYJP\x01\x00\x00")))
	if err == nil {
		t.Fatal("expected error")
	}
}
func TestFrameRejectsHugeLength(t *testing.T) {
	b := make([]byte, HeaderSize)
	copy(b[:4], Magic)
	b[4] = 1
	b[5] = 0
	b[6] = 0
	b[7] = uint8(NEW_TRANSACTION)
	b[15] = 1
	b[16] = 0xff
	b[17] = 0xff
	b[18] = 0xff
	b[19] = 0xff
	if _, err := DecodeFrame(b); err != ErrFrameTooLarge {
		t.Fatalf("got %v", err)
	}
}
func TestAllMessagesRoundTrip(t *testing.T) {
	var h [32]byte
	for i := range h {
		h[i] = byte(i)
	}
	cases := []struct {
		name string
		typ  MessageType
		enc  func(uint64) ([]byte, error)
		dec  func(Frame) error
	}{
		{"hello", HELLO, func(r uint64) ([]byte, error) {
			return EncodeHello(Hello{"sayanjali-p2p", "main", "node", "127.0.0.1:1000", 1, 0, h[:], make([]byte, 64), make([]byte, 32), make([]byte, 64), CapBlocks | CapTransactions}, r)
		}, func(f Frame) error { _, e := DecodeHello(f); return e }},
		{"hello_ack", HELLO_ACK, func(r uint64) ([]byte, error) {
			return EncodeHelloAck(HelloAck{"sayanjali-p2p", "main", "node", "127.0.0.1:1000", 1, 0, h[:], make([]byte, 64), make([]byte, 32), make([]byte, 32), make([]byte, 64), CapBlocks | CapSync}, r)
		}, func(f Frame) error { _, e := DecodeHelloAck(f); return e }},
		{"get_peers", GET_PEERS, func(r uint64) ([]byte, error) { return EncodeGetPeers(GetPeers{"node-a", 16}, r) }, func(f Frame) error { _, e := DecodeGetPeers(f); return e }},
		{"peers", PEERS, func(r uint64) ([]byte, error) {
			return EncodePeers(Peers{[]Peer{{"node-a", "127.0.0.1", 3030, CapSync}}, "node-a"}, r)
		}, func(f Frame) error { _, e := DecodePeers(f); return e }},
		{"get_headers", GET_HEADERS, func(r uint64) ([]byte, error) {
			return EncodeGetHeaders(GetHeaders{[][32]byte{h}, hash32("0000000000000000000000000000000000000000000000000000000000000000"), 64}, r)
		}, func(f Frame) error { _, e := DecodeGetHeaders(f); return e }},
		{"headers", HEADERS, func(r uint64) ([]byte, error) { return EncodeHeaders(Headers{[][]byte{[]byte(`{"index":1}`)}}, r) }, func(f Frame) error { _, e := DecodeHeaders(f); return e }},
		{"get_blocks", GET_BLOCKS, func(r uint64) ([]byte, error) { return EncodeGetBlocks(GetBlocks{[][32]byte{h}}, r) }, func(f Frame) error { _, e := DecodeGetBlocks(f); return e }},
		{"blocks", BLOCKS, func(r uint64) ([]byte, error) { return EncodeBlocks(Blocks{[][]byte{[]byte(`{"hash":"x"}`)}}, r) }, func(f Frame) error { _, e := DecodeBlocks(f); return e }},
		{"new_block", NEW_BLOCK, func(r uint64) ([]byte, error) { return EncodeNewBlock(NewBlock{[]byte(`{"hash":"x"}`)}, r) }, func(f Frame) error { _, e := DecodeNewBlock(f); return e }},
		{"new_transaction", NEW_TRANSACTION, func(r uint64) ([]byte, error) {
			return EncodeNewTransaction(NewTransaction{[]byte(`{"tx_hash":"x"}`)}, r)
		}, func(f Frame) error { _, e := DecodeNewTransaction(f); return e }},
		{"reject", REJECT, func(r uint64) ([]byte, error) {
			return EncodeReject(Reject{RejectInvalidRequest, false, true, "invalid request"}, r)
		}, func(f Frame) error { _, e := DecodeReject(f); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, e := tc.enc(42)
			if e != nil {
				t.Fatal(e)
			}
			f, e := DecodeFrame(b)
			if e != nil {
				t.Fatal(e)
			}
			if f.Type != tc.typ {
				t.Fatalf("type %v", f.Type)
			}
			if e = tc.dec(f); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestReaderConsumesExactlyOneFrame(t *testing.T) {
	b, _ := EncodeNewTransaction(NewTransaction{[]byte("x")}, 1)
	r := bytes.NewBuffer(append(b, b...))
	_, e := ReadFrame(r)
	if e != nil {
		t.Fatal(e)
	}
	_, e = ReadFrame(r)
	if e != nil {
		t.Fatal(e)
	}
	_, e = ReadFrame(r)
	if e != io.EOF {
		t.Fatalf("%v", e)
	}
}
func FuzzDecodeFrameNeverPanics(f *testing.F) {
	f.Add([]byte("SYJP"))
	f.Add(make([]byte, 64))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = DecodeFrame(b) })
}
