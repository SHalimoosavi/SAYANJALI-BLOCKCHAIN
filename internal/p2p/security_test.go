package p2p

import "testing"

func TestHandshakeSigningBytesDeterministic(t *testing.T) {
	var h [32]byte
	var pub [64]byte
	v := Hello{ProtocolName: "sayanjali-p2p", VersionMajor: 1, VersionMinor: 0, NetworkName: "sayanjali-mainnet-mvp", NodeID: "n", AdvertisedAddress: "127.0.0.1:3030", GenesisHash: h[:], PublicKey: pub[:], Challenge: h[:], Capabilities: 7}
	a, e := HelloSigningBytes(v)
	if e != nil {
		t.Fatal(e)
	}
	b, e := HelloSigningBytes(v)
	if e != nil {
		t.Fatal(e)
	}
	if string(a) != string(b) {
		t.Fatal("non-deterministic transcript")
	}
	if len(a) == 0 {
		t.Fatal("empty transcript")
	}
}
func TestRequestTrackerBounded(t *testing.T) {
	tr := NewRequestTracker()
	for i := uint64(1); i <= MaxOutstandingRequests; i++ {
		if e := tr.Reserve(i); e != nil {
			t.Fatal(e)
		}
	}
	if e := tr.Reserve(MaxOutstandingRequests + 1); e != ErrOutstandingLimit {
		t.Fatalf("got %v", e)
	}
	if e := tr.Reserve(1); e != ErrRequestAlreadyOutstanding {
		t.Fatalf("got %v", e)
	}
	if e := tr.Complete(1); e != nil {
		t.Fatal(e)
	}
	if e := tr.Reserve(MaxOutstandingRequests + 1); e != nil {
		t.Fatal(e)
	}
	if tr.Len() != MaxOutstandingRequests {
		t.Fatalf("len %d", tr.Len())
	}
}
func TestSessionStateMachine(t *testing.T) {
	steps := [][2]SessionState{{Disconnected, Connecting}, {Connecting, Handshaking}, {Handshaking, Established}, {Established, Closing}, {Closing, Disconnected}}
	for _, p := range steps {
		if !ValidTransition(p[0], p[1]) {
			t.Fatalf("invalid transition %v -> %v", p[0], p[1])
		}
	}
	if AllowedInState(Handshaking, GET_BLOCKS) {
		t.Fatal("GET_BLOCKS allowed before handshake")
	}
	if !AllowedInState(Established, NEW_TRANSACTION) {
		t.Fatal("NEW_TRANSACTION not allowed after handshake")
	}
}

func FuzzMessageDecodersNeverPanic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Fuzz(func(t *testing.T, payload []byte) {
		frames := []Frame{{ProtocolMajor, ProtocolMinor, HELLO, 1, payload}, {ProtocolMajor, ProtocolMinor, PEERS, 1, payload}, {ProtocolMajor, ProtocolMinor, GET_HEADERS, 1, payload}, {ProtocolMajor, ProtocolMinor, BLOCKS, 1, payload}, {ProtocolMajor, ProtocolMinor, REJECT, 1, payload}}
		for _, frame := range frames {
			switch frame.Type {
			case HELLO:
				_, _ = DecodeHello(frame)
			case PEERS:
				_, _ = DecodePeers(frame)
			case GET_HEADERS:
				_, _ = DecodeGetHeaders(frame)
			case BLOCKS:
				_, _ = DecodeBlocks(frame)
			case REJECT:
				_, _ = DecodeReject(frame)
			}
		}
	})
}
