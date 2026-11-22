package node

import (
	"bytes"
	"go/format"
	"os"
	"testing"
)

func TestL02SourceFormattingProbe(t *testing.T) {
	paths := []string{
		"node.go",
		"api.go",
		"../p2pnode/network.go",
		"../../cmd/syjd/main.go",
		"../storage/storage.go",
		"l02_fail_stop_test.go",
		"../storage/l02_fail_stop_test.go",
		"../chain/l02_fail_stop_test.go",
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
			t.Errorf("not gofmt clean: %s", path)
		}
	}
}
