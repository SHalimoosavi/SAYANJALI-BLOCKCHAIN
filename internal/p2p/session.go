package p2p

type SessionState uint8

const (
	Disconnected SessionState = iota
	Connecting
	Handshaking
	Established
	Closing
)

func (s SessionState) String() string {
	switch s {
	case Disconnected:
		return "DISCONNECTED"
	case Connecting:
		return "CONNECTING"
	case Handshaking:
		return "HANDSHAKING"
	case Established:
		return "ESTABLISHED"
	case Closing:
		return "CLOSING"
	default:
		return "UNKNOWN"
	}
}
func AllowedInState(s SessionState, t MessageType) bool {
	switch s {
	case Handshaking:
		return t == HELLO || t == HELLO_ACK
	case Established:
		return t >= GET_PEERS && t <= REJECT
	default:
		return false
	}
}
func ValidTransition(from, to SessionState) bool {
	if from == Disconnected && to == Connecting {
		return true
	}
	if from == Connecting && to == Handshaking {
		return true
	}
	if from == Handshaking && to == Established {
		return true
	}
	if from == Established && to == Closing {
		return true
	}
	if from == Closing && to == Disconnected {
		return true
	}
	return false
}
