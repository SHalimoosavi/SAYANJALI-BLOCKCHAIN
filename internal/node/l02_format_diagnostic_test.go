package node

import (
	"bytes"
	"go/format"
	"os"
	"testing"
)

func TestL02FormattingDiagnostic(t *testing.T) {
	paths := []string{
		"node.go",
		"api.go",
		"l02_fail_stop_test.go",
		"l02_api_fail_stop_test.go",
		"../p2pnode/network.go",
		"../p2pnode/l02_fail_stop_test.go",
		"../storage/storage.go",
		"../chain/l02_fail_stop_test.go",
		"../../cmd/syjd/main.go",
		"../../cmd/syjd/l02_fail_stop_test.go",
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		formatted, err := format.Source(source)
		if err != nil {
			t.Errorf("format %s: %v", path, err)
			continue
		}
		if !bytes.Equal(source, formatted) {
			t.Errorf("L-02 diagnostic: not gofmt clean: %s", path)
		}
	}
}
