#!/bin/sh
set -eu
echo '=== PHASE 6 VALIDATION ==='
go mod download
gofmt_out="$(gofmt -l $(find . -name '*.go' -not -path './.git/*' -print))"
if [ -n "$gofmt_out" ]; then
    echo "$gofmt_out"
    echo 'gofmt check failed'
    exit 1
fi
go test ./...
go vet ./...
go build ./...
if go env GOOS | grep -q '^android$' && [ "$(go env GOARCH)" = "arm64" ]; then echo 'race: unsupported by android/arm64 environment'; else go test -race ./...; fi
python -m pytest -q tests/p2p/test_reference_codec.py
./scripts/phase6_frozen_verify.sh
if grep -RIn --exclude-dir=.git '[[:blank:]]$' .; then echo 'trailing whitespace found'; exit 1; fi
if grep -RIn --exclude-dir=.git -E '^(<<<<<<<|=======|>>>>>>>)' .; then echo 'conflict marker found'; exit 1; fi
echo '=== PHASE 6 VALIDATION PASS ==='
