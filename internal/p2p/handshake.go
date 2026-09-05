package p2p

import "bytes"

var helloDomain = []byte("SYJ-P2P-HELLO-v1")
var helloAckDomain = []byte("SYJ-P2P-HELLO-ACK-v1")

// HelloSigningBytes returns the exact bytes covered by a HELLO signature.
// It intentionally excludes the signature field itself.
func HelloSigningBytes(v Hello) ([]byte, error) {
	var w writer
	if v.ProtocolName != "sayanjali-p2p" || v.VersionMajor != ProtocolMajor || v.VersionMinor != ProtocolMinor || len(v.NodeID) == 0 || len(v.AdvertisedAddress) == 0 || len(v.Challenge) != 32 || len(v.PublicKey) != 64 || checkCaps(v.Capabilities) != nil {
		return nil, ErrMalformedPayload
	}
	w.Write(helloDomain)
	w.str(v.ProtocolName)
	w.u8(v.VersionMajor)
	w.u8(v.VersionMinor)
	w.str(v.NetworkName)
	w.bytes32(v.GenesisHash)
	w.str(v.NodeID)
	w.raw(v.PublicKey, 64)
	w.str(v.AdvertisedAddress)
	w.u32(v.Capabilities)
	if len(v.Challenge) != 32 {
		w.err = ErrMalformedPayload
	} else {
		w.Write(v.Challenge)
	}
	if w.err != nil {
		return nil, w.err
	}
	return w.Bytes(), nil
}

// HelloAckSigningBytes returns the exact bytes covered by a HELLO_ACK signature.
func HelloAckSigningBytes(v HelloAck) ([]byte, error) {
	var w writer
	if v.ProtocolName != "sayanjali-p2p" || v.VersionMajor != ProtocolMajor || v.VersionMinor != ProtocolMinor || len(v.NodeID) == 0 || len(v.AdvertisedAddress) == 0 || len(v.EchoChallenge) != 32 || len(v.Challenge) != 32 || len(v.PublicKey) != 64 || checkCaps(v.Capabilities) != nil {
		return nil, ErrMalformedPayload
	}
	w.Write(helloAckDomain)
	w.str(v.ProtocolName)
	w.u8(v.VersionMajor)
	w.u8(v.VersionMinor)
	w.str(v.NetworkName)
	w.bytes32(v.GenesisHash)
	w.str(v.NodeID)
	w.raw(v.PublicKey, 64)
	w.str(v.AdvertisedAddress)
	w.u32(v.Capabilities)
	w.Write(v.EchoChallenge)
	w.Write(v.Challenge)
	if w.err != nil {
		return nil, w.err
	}
	return w.Bytes(), nil
}

func SameChallenge(a, b []byte) bool { return len(a) == 32 && len(b) == 32 && bytes.Equal(a, b) }
