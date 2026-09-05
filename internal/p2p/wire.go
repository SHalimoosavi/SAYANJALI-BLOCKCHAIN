package p2p

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const (
	Magic                        = "SYJP"
	HeaderSize                   = 20
	ProtocolMajor          uint8 = 1
	ProtocolMinor          uint8 = 0
	MaxFrameSize                 = 4 * 1024 * 1024
	MaxPeerEntries               = 256
	MaxLocatorHashes             = 32
	MaxHeaders                   = 2048
	MaxBlockIDs                  = 128
	MaxBlocks                    = 32
	MaxBlockPayload              = 512 * 1024
	MaxTransactionPayload        = 64 * 1024
	MaxStringSize                = 256
	MaxCapabilities              = 32
	MaxOutstandingRequests       = 64
)

type MessageType uint16

const (
	HELLO MessageType = iota + 1
	HELLO_ACK
	GET_PEERS
	PEERS
	GET_HEADERS
	HEADERS
	GET_BLOCKS
	BLOCKS
	NEW_BLOCK
	NEW_TRANSACTION
	REJECT
)

func (t MessageType) String() string {
	names := map[MessageType]string{HELLO: "HELLO", HELLO_ACK: "HELLO_ACK", GET_PEERS: "GET_PEERS", PEERS: "PEERS", GET_HEADERS: "GET_HEADERS", HEADERS: "HEADERS", GET_BLOCKS: "GET_BLOCKS", BLOCKS: "BLOCKS", NEW_BLOCK: "NEW_BLOCK", NEW_TRANSACTION: "NEW_TRANSACTION", REJECT: "REJECT"}
	if s, ok := names[t]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN(%d)", t)
}
func (t MessageType) Valid() bool { return t >= HELLO && t <= REJECT }

type Frame struct {
	VersionMajor uint8
	VersionMinor uint8
	Type         MessageType
	RequestID    uint64
	Payload      []byte
}

func EncodeFrame(f Frame) ([]byte, error) {
	if f.VersionMajor != ProtocolMajor || f.VersionMinor != ProtocolMinor {
		return nil, ErrUnsupportedVersion
	}
	if !f.Type.Valid() {
		return nil, ErrUnknownMessageType
	}
	if len(f.Payload) > MaxFrameSize-HeaderSize {
		return nil, ErrFrameTooLarge
	}
	if isRequestType(f.Type) && f.RequestID == 0 {
		return nil, ErrInvalidRequestID
	}
	if isResponseType(f.Type) && f.RequestID == 0 && f.Type != REJECT {
		return nil, ErrInvalidRequestID
	}
	var b bytes.Buffer
	b.Grow(HeaderSize + len(f.Payload))
	b.WriteString(Magic)
	b.WriteByte(f.VersionMajor)
	b.WriteByte(f.VersionMinor)
	var u16 [2]byte
	binary.BigEndian.PutUint16(u16[:], uint16(f.Type))
	b.Write(u16[:])
	var u64 [8]byte
	binary.BigEndian.PutUint64(u64[:], f.RequestID)
	b.Write(u64[:])
	var u32 [4]byte
	binary.BigEndian.PutUint32(u32[:], uint32(len(f.Payload)))
	b.Write(u32[:])
	b.Write(f.Payload)
	return b.Bytes(), nil
}

func DecodeFrame(data []byte) (Frame, error) {
	if len(data) < HeaderSize {
		return Frame{}, io.ErrUnexpectedEOF
	}
	if string(data[:4]) != Magic {
		return Frame{}, ErrBadMagic
	}
	if data[4] != ProtocolMajor || data[5] != ProtocolMinor {
		return Frame{}, ErrUnsupportedVersion
	}
	typ := MessageType(binary.BigEndian.Uint16(data[6:8]))
	if !typ.Valid() {
		return Frame{}, ErrUnknownMessageType
	}
	requestID := binary.BigEndian.Uint64(data[8:16])
	n := binary.BigEndian.Uint32(data[16:20])
	if n > uint32(MaxFrameSize-HeaderSize) {
		return Frame{}, ErrFrameTooLarge
	}
	if len(data)-HeaderSize < int(n) {
		return Frame{}, io.ErrUnexpectedEOF
	}
	if len(data)-HeaderSize > int(n) {
		return Frame{}, ErrMalformedPayload
	}
	if isRequestType(typ) && requestID == 0 {
		return Frame{}, ErrInvalidRequestID
	}
	if isResponseType(typ) && requestID == 0 && typ != REJECT {
		return Frame{}, ErrInvalidRequestID
	}
	payload := append([]byte(nil), data[HeaderSize:]...)
	return Frame{ProtocolMajor, ProtocolMinor, typ, requestID, payload}, nil
}

func ReadFrame(r io.Reader) (Frame, error) {
	var h [HeaderSize]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Frame{}, err
	}
	if string(h[:4]) != Magic {
		return Frame{}, ErrBadMagic
	}
	if h[4] != ProtocolMajor || h[5] != ProtocolMinor {
		return Frame{}, ErrUnsupportedVersion
	}
	typ := MessageType(binary.BigEndian.Uint16(h[6:8]))
	if !typ.Valid() {
		return Frame{}, ErrUnknownMessageType
	}
	req := binary.BigEndian.Uint64(h[8:16])
	n := binary.BigEndian.Uint32(h[16:20])
	if n > uint32(MaxFrameSize-HeaderSize) {
		return Frame{}, ErrFrameTooLarge
	}
	if isRequestType(typ) && req == 0 {
		return Frame{}, ErrInvalidRequestID
	}
	if isResponseType(typ) && req == 0 && typ != REJECT {
		return Frame{}, ErrInvalidRequestID
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Frame{}, err
	}
	return Frame{ProtocolMajor, ProtocolMinor, typ, req, payload}, nil
}

var (
	ErrBadMagic           = errors.New("bad frame magic")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrUnknownMessageType = errors.New("unknown message type")
	ErrFrameTooLarge      = errors.New("frame too large")
	ErrInvalidRequestID   = errors.New("invalid request id")
	ErrMalformedPayload   = errors.New("malformed payload")
	ErrFieldTooLarge      = errors.New("field too large")
	ErrCountTooLarge      = errors.New("count exceeds protocol limit")
)

func isRequestType(t MessageType) bool {
	return t == HELLO || t == GET_PEERS || t == GET_HEADERS || t == GET_BLOCKS
}
func isResponseType(t MessageType) bool {
	return t == HELLO_ACK || t == PEERS || t == HEADERS || t == BLOCKS || t == REJECT
}

// Writer helpers used by all message codecs. All integers are unsigned big-endian.
type writer struct {
	bytes.Buffer
	err error
}

func (w *writer) u8(v uint8) {
	if w.err == nil {
		w.WriteByte(v)
	}
}
func (w *writer) u16(v uint16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	if w.err == nil {
		_, w.err = w.Write(b[:])
	}
}
func (w *writer) u32(v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	if w.err == nil {
		_, w.err = w.Write(b[:])
	}
}
func (w *writer) u64(v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	if w.err == nil {
		_, w.err = w.Write(b[:])
	}
}
func (w *writer) bytes32(v []byte) {
	if len(v) != 32 {
		w.err = ErrMalformedPayload
		return
	}
	if w.err == nil {
		_, w.err = w.Write(v)
	}
}
func (w *writer) raw(v []byte, max int) {
	if len(v) > max {
		w.err = ErrFieldTooLarge
		return
	}
	w.u32(uint32(len(v)))
	if w.err == nil {
		_, w.err = w.Write(v)
	}
}
func (w *writer) str(v string) {
	if len(v) > MaxStringSize {
		w.err = ErrFieldTooLarge
		return
	}
	w.u16(uint16(len(v)))
	if w.err == nil {
		_, w.err = w.WriteString(v)
	}
}

type reader struct {
	r   *bytes.Reader
	err error
}

func (r *reader) u8() uint8 {
	if r.err != nil {
		return 0
	}
	v, e := r.r.ReadByte()
	r.err = e
	return v
}
func (r *reader) u16() uint16 { var b [2]byte; r.read(b[:]); return binary.BigEndian.Uint16(b[:]) }
func (r *reader) u32() uint32 { var b [4]byte; r.read(b[:]); return binary.BigEndian.Uint32(b[:]) }
func (r *reader) u64() uint64 { var b [8]byte; r.read(b[:]); return binary.BigEndian.Uint64(b[:]) }
func (r *reader) read(b []byte) {
	if r.err == nil {
		_, r.err = io.ReadFull(r.r, b)
	}
}
func (r *reader) fixed(n int) []byte { b := make([]byte, n); r.read(b); return b }
func (r *reader) raw(max int) []byte {
	n := r.u32()
	if r.err != nil {
		return nil
	}
	if n > uint32(max) {
		r.err = ErrFieldTooLarge
		return nil
	}
	return r.fixed(int(n))
}
func (r *reader) str() string {
	n := r.u16()
	if r.err != nil {
		return ""
	}
	if n > MaxStringSize {
		r.err = ErrFieldTooLarge
		return ""
	}
	b := r.fixed(int(n))
	if r.err != nil {
		return ""
	}
	if !utf8.Valid(b) {
		r.err = ErrMalformedPayload
		return ""
	}
	return string(b)
}
func (r *reader) done() error {
	if r.err != nil {
		return r.err
	}
	if r.r.Len() != 0 {
		return ErrMalformedPayload
	}
	return nil
}
