package p2p

import (
	"bytes"
	"errors"
	"unicode/utf8"
)

const (
	CapBlocks uint32 = 1 << iota
	CapTransactions
	CapSync
)
const KnownCapabilities = CapBlocks | CapTransactions | CapSync

type Hello struct {
	ProtocolName, NetworkName, NodeID, AdvertisedAddress string
	VersionMajor, VersionMinor                           uint8
	GenesisHash, PublicKey, Challenge, Signature         []byte
	Capabilities                                         uint32
}
type HelloAck struct {
	ProtocolName, NetworkName, NodeID, AdvertisedAddress        string
	VersionMajor, VersionMinor                                  uint8
	GenesisHash, PublicKey, EchoChallenge, Challenge, Signature []byte
	Capabilities                                                uint32
}
type GetPeers struct {
	StartAfter string
	Limit      uint16
}
type Peer struct {
	NodeID, Host string
	Port         uint16
	Capabilities uint32
}
type Peers struct {
	Entries        []Peer
	NextStartAfter string
}
type GetHeaders struct {
	Locator  [][32]byte
	StopHash [32]byte
	MaxCount uint16
}
type Headers struct{ Items [][]byte }
type GetBlocks struct{ Hashes [][32]byte }
type Blocks struct{ Items [][]byte }
type NewBlock struct{ Block []byte }
type NewTransaction struct{ Transaction []byte }
type Reject struct {
	Code      uint16
	Retryable bool
	Close     bool
	Reason    string
}

func wrap(t MessageType, req uint64, payload []byte) ([]byte, error) {
	return EncodeFrame(Frame{ProtocolMajor, ProtocolMinor, t, req, payload})
}
func ensureHash32(b []byte) error {
	if len(b) != 32 {
		return ErrMalformedPayload
	}
	return nil
}
func ensurePub64(b []byte) error {
	if len(b) != 64 {
		return ErrMalformedPayload
	}
	return nil
}
func ensureSig64(b []byte) error {
	if len(b) != 64 {
		return ErrMalformedPayload
	}
	return nil
}
func checkCaps(c uint32) error {
	if c&^KnownCapabilities != 0 {
		return ErrMalformedPayload
	}
	return nil
}

func EncodeHello(v Hello, req uint64) ([]byte, error) {
	var w writer
	w.str(v.ProtocolName)
	w.u8(v.VersionMajor)
	w.u8(v.VersionMinor)
	w.str(v.NetworkName)
	w.bytes32(v.GenesisHash)
	w.str(v.NodeID)
	if ensurePub64(v.PublicKey) != nil {
		w.err = ErrMalformedPayload
	} else {
		w.raw(v.PublicKey, 64)
	}
	w.str(v.AdvertisedAddress)
	if checkCaps(v.Capabilities) != nil {
		w.err = ErrMalformedPayload
	}
	w.u32(v.Capabilities)
	if len(v.Challenge) != 32 {
		w.err = ErrMalformedPayload
	} else {
		w.Write(v.Challenge)
	}
	if ensureSig64(v.Signature) != nil {
		w.err = ErrMalformedPayload
	} else {
		w.Write(v.Signature)
	}
	return wrap(HELLO, req, w.Bytes())
}
func DecodeHello(f Frame) (Hello, error) {
	if f.Type != HELLO {
		return Hello{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	v := Hello{ProtocolName: r.str(), VersionMajor: r.u8(), VersionMinor: r.u8(), NetworkName: r.str()}
	v.GenesisHash = r.fixed(32)
	v.NodeID = r.str()
	v.PublicKey = r.raw(64)
	v.AdvertisedAddress = r.str()
	v.Capabilities = r.u32()
	v.Challenge = r.fixed(32)
	v.Signature = r.fixed(64)
	if err := r.done(); err != nil {
		return Hello{}, err
	}
	if v.ProtocolName != "sayanjali-p2p" || v.VersionMajor != ProtocolMajor || v.VersionMinor != ProtocolMinor || len(v.NodeID) == 0 || len(v.AdvertisedAddress) == 0 || checkCaps(v.Capabilities) != nil {
		return Hello{}, ErrMalformedPayload
	}
	return v, nil
}

func EncodeHelloAck(v HelloAck, req uint64) ([]byte, error) {
	var w writer
	w.str(v.ProtocolName)
	w.u8(v.VersionMajor)
	w.u8(v.VersionMinor)
	w.str(v.NetworkName)
	w.bytes32(v.GenesisHash)
	w.str(v.NodeID)
	if ensurePub64(v.PublicKey) != nil {
		w.err = ErrMalformedPayload
	} else {
		w.raw(v.PublicKey, 64)
	}
	w.str(v.AdvertisedAddress)
	if checkCaps(v.Capabilities) != nil {
		w.err = ErrMalformedPayload
	}
	w.u32(v.Capabilities)
	if len(v.EchoChallenge) != 32 || len(v.Challenge) != 32 {
		w.err = ErrMalformedPayload
	} else {
		w.Write(v.EchoChallenge)
		w.Write(v.Challenge)
	}
	if ensureSig64(v.Signature) != nil {
		w.err = ErrMalformedPayload
	} else {
		w.Write(v.Signature)
	}
	return wrap(HELLO_ACK, req, w.Bytes())
}
func DecodeHelloAck(f Frame) (HelloAck, error) {
	if f.Type != HELLO_ACK {
		return HelloAck{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	v := HelloAck{ProtocolName: r.str(), VersionMajor: r.u8(), VersionMinor: r.u8(), NetworkName: r.str()}
	v.GenesisHash = r.fixed(32)
	v.NodeID = r.str()
	v.PublicKey = r.raw(64)
	v.AdvertisedAddress = r.str()
	v.Capabilities = r.u32()
	v.EchoChallenge = r.fixed(32)
	v.Challenge = r.fixed(32)
	v.Signature = r.fixed(64)
	if err := r.done(); err != nil {
		return HelloAck{}, err
	}
	if v.ProtocolName != "sayanjali-p2p" || v.VersionMajor != ProtocolMajor || v.VersionMinor != ProtocolMinor || len(v.NodeID) == 0 || len(v.AdvertisedAddress) == 0 || checkCaps(v.Capabilities) != nil {
		return HelloAck{}, ErrMalformedPayload
	}
	return v, nil
}

func EncodeGetPeers(v GetPeers, req uint64) ([]byte, error) {
	var w writer
	w.str(v.StartAfter)
	if v.Limit == 0 || v.Limit > MaxPeerEntries {
		w.err = ErrCountTooLarge
	}
	w.u16(v.Limit)
	return wrap(GET_PEERS, req, w.Bytes())
}
func DecodeGetPeers(f Frame) (GetPeers, error) {
	if f.Type != GET_PEERS {
		return GetPeers{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	v := GetPeers{StartAfter: r.str(), Limit: r.u16()}
	if err := r.done(); err != nil {
		return GetPeers{}, err
	}
	if v.Limit == 0 || v.Limit > MaxPeerEntries {
		return GetPeers{}, ErrCountTooLarge
	}
	return v, nil
}
func EncodePeers(v Peers, req uint64) ([]byte, error) {
	var w writer
	if len(v.Entries) > MaxPeerEntries {
		w.err = ErrCountTooLarge
	}
	w.u16(uint16(len(v.Entries)))
	for _, p := range v.Entries {
		if p.Port == 0 {
			w.err = ErrMalformedPayload
		}
		w.str(p.NodeID)
		w.str(p.Host)
		w.u16(p.Port)
		if checkCaps(p.Capabilities) != nil {
			w.err = ErrMalformedPayload
		}
		w.u32(p.Capabilities)
	}
	w.str(v.NextStartAfter)
	return wrap(PEERS, req, w.Bytes())
}
func DecodePeers(f Frame) (Peers, error) {
	if f.Type != PEERS {
		return Peers{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	n := r.u16()
	if n > MaxPeerEntries {
		return Peers{}, ErrCountTooLarge
	}
	v := Peers{Entries: make([]Peer, 0, n)}
	for i := 0; i < int(n); i++ {
		p := Peer{NodeID: r.str(), Host: r.str(), Port: r.u16(), Capabilities: r.u32()}
		if checkCaps(p.Capabilities) != nil {
			r.err = ErrMalformedPayload
		}
		if p.NodeID == "" || p.Host == "" || p.Port == 0 {
			r.err = ErrMalformedPayload
		}
		v.Entries = append(v.Entries, p)
	}
	v.NextStartAfter = r.str()
	if err := r.done(); err != nil {
		return Peers{}, err
	}
	return v, nil
}

func EncodeGetHeaders(v GetHeaders, req uint64) ([]byte, error) {
	var w writer
	if len(v.Locator) == 0 || len(v.Locator) > MaxLocatorHashes {
		w.err = ErrCountTooLarge
	}
	w.u8(uint8(len(v.Locator)))
	for _, h := range v.Locator {
		w.Write(h[:])
	}
	w.Write(v.StopHash[:])
	if v.MaxCount == 0 || v.MaxCount > MaxHeaders {
		w.err = ErrCountTooLarge
	}
	w.u16(v.MaxCount)
	return wrap(GET_HEADERS, req, w.Bytes())
}
func DecodeGetHeaders(f Frame) (GetHeaders, error) {
	if f.Type != GET_HEADERS {
		return GetHeaders{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	n := r.u8()
	if n == 0 || int(n) > MaxLocatorHashes {
		r.err = ErrCountTooLarge
	}
	v := GetHeaders{Locator: make([][32]byte, 0, n)}
	for i := 0; i < int(n); i++ {
		var h [32]byte
		r.read(h[:])
		v.Locator = append(v.Locator, h)
	}
	copy(v.StopHash[:], r.fixed(32))
	v.MaxCount = r.u16()
	if err := r.done(); err != nil {
		return GetHeaders{}, err
	}
	if v.MaxCount == 0 || v.MaxCount > MaxHeaders {
		return GetHeaders{}, ErrCountTooLarge
	}
	return v, nil
}
func EncodeHeaders(v Headers, req uint64) ([]byte, error) {
	var w writer
	if len(v.Items) > MaxHeaders {
		w.err = ErrCountTooLarge
	}
	w.u16(uint16(len(v.Items)))
	for _, h := range v.Items {
		w.raw(h, 4096)
	}
	return wrap(HEADERS, req, w.Bytes())
}
func DecodeHeaders(f Frame) (Headers, error) {
	if f.Type != HEADERS {
		return Headers{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	n := r.u16()
	if n > MaxHeaders {
		return Headers{}, ErrCountTooLarge
	}
	v := Headers{Items: make([][]byte, 0, n)}
	for i := 0; i < int(n); i++ {
		v.Items = append(v.Items, r.raw(4096))
	}
	if err := r.done(); err != nil {
		return Headers{}, err
	}
	return v, nil
}

func EncodeGetBlocks(v GetBlocks, req uint64) ([]byte, error) {
	var w writer
	if len(v.Hashes) == 0 || len(v.Hashes) > MaxBlockIDs {
		w.err = ErrCountTooLarge
	}
	w.u16(uint16(len(v.Hashes)))
	for _, h := range v.Hashes {
		w.Write(h[:])
	}
	return wrap(GET_BLOCKS, req, w.Bytes())
}
func DecodeGetBlocks(f Frame) (GetBlocks, error) {
	if f.Type != GET_BLOCKS {
		return GetBlocks{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	n := r.u16()
	if n == 0 || n > MaxBlockIDs {
		return GetBlocks{}, ErrCountTooLarge
	}
	v := GetBlocks{Hashes: make([][32]byte, 0, n)}
	for i := 0; i < int(n); i++ {
		var h [32]byte
		r.read(h[:])
		v.Hashes = append(v.Hashes, h)
	}
	if err := r.done(); err != nil {
		return GetBlocks{}, err
	}
	return v, nil
}
func EncodeBlocks(v Blocks, req uint64) ([]byte, error) {
	var w writer
	if len(v.Items) > MaxBlocks {
		w.err = ErrCountTooLarge
	}
	w.u16(uint16(len(v.Items)))
	for _, b := range v.Items {
		w.raw(b, MaxBlockPayload)
	}
	return wrap(BLOCKS, req, w.Bytes())
}
func DecodeBlocks(f Frame) (Blocks, error) {
	if f.Type != BLOCKS {
		return Blocks{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	n := r.u16()
	if n > MaxBlocks {
		return Blocks{}, ErrCountTooLarge
	}
	v := Blocks{Items: make([][]byte, 0, n)}
	for i := 0; i < int(n); i++ {
		v.Items = append(v.Items, r.raw(MaxBlockPayload))
	}
	if err := r.done(); err != nil {
		return Blocks{}, err
	}
	return v, nil
}
func EncodeNewBlock(v NewBlock, req uint64) ([]byte, error) {
	var w writer
	w.raw(v.Block, MaxBlockPayload)
	return wrap(NEW_BLOCK, req, w.Bytes())
}
func DecodeNewBlock(f Frame) (NewBlock, error) {
	if f.Type != NEW_BLOCK {
		return NewBlock{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	v := NewBlock{Block: r.raw(MaxBlockPayload)}
	if err := r.done(); err != nil {
		return NewBlock{}, err
	}
	return v, nil
}
func EncodeNewTransaction(v NewTransaction, req uint64) ([]byte, error) {
	var w writer
	w.raw(v.Transaction, MaxTransactionPayload)
	return wrap(NEW_TRANSACTION, req, w.Bytes())
}
func DecodeNewTransaction(f Frame) (NewTransaction, error) {
	if f.Type != NEW_TRANSACTION {
		return NewTransaction{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	v := NewTransaction{Transaction: r.raw(MaxTransactionPayload)}
	if err := r.done(); err != nil {
		return NewTransaction{}, err
	}
	return v, nil
}

const (
	RejectMalformedMessage uint16 = iota + 1
	RejectUnsupportedVersion
	RejectWrongNetwork
	RejectWrongGenesis
	RejectInvalidRequest
	RejectInvalidBlock
	RejectInvalidTransaction
	RejectNotFound
	RejectTooLarge
	RejectRateLimited
	RejectInvalidState
	RejectUnauthorized
)

func EncodeReject(v Reject, req uint64) ([]byte, error) {
	var w writer
	if !v.Valid() {
		w.err = ErrMalformedPayload
	}
	w.u16(v.Code)
	if v.Retryable {
		w.u8(1)
	} else {
		w.u8(0)
	}
	if v.Close {
		w.u8(1)
	} else {
		w.u8(0)
	}
	w.str(v.Reason)
	return wrap(REJECT, req, w.Bytes())
}
func (rj Reject) Valid() bool { return rj.ValidCode() && rj.Reason != "" }
func (rj Reject) ValidCode() bool {
	return rj.Code >= RejectMalformedMessage && rj.Code <= RejectUnauthorized
}
func DecodeReject(f Frame) (Reject, error) {
	if f.Type != REJECT {
		return Reject{}, ErrMalformedPayload
	}
	r := reader{r: bytes.NewReader(f.Payload)}
	v := Reject{Code: r.u16()}
	retry := r.u8()
	closeFlag := r.u8()
	if retry > 1 || closeFlag > 1 {
		r.err = ErrMalformedPayload
	}
	v.Retryable = retry == 1
	v.Close = closeFlag == 1
	v.Reason = r.str()
	if err := r.done(); err != nil {
		return Reject{}, err
	}
	if !v.Valid() {
		return Reject{}, errors.New("invalid reject")
	}
	return v, nil
}

// Canonical consensus objects carried by the wire are opaque UTF-8 bytes to this package.
// Their interpretation belongs to the consensus/core layer; this codec never re-validates them.
func IsUTF8(b []byte) bool { return utf8.Valid(b) }
