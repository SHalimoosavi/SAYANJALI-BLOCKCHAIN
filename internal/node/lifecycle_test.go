package node

import (
	"testing"
)

func TestStopClosesDone(t *testing.T) {
	n := New(DefaultConfig(t.TempDir()))
	select {
	case <-n.Done():
		t.Fatal("done closed before stop")
	default:
	}
	n.Stop()
	select {
	case <-n.Done():
	default:
		t.Fatal("done not closed after stop")
	}
}
