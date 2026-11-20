#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

required_go='go1.27.4'
actual_go="$(GOTOOLCHAIN=local go version | awk '{print $3}')"
printf 'go version: %s\n' "$(GOTOOLCHAIN=local go version)"
if [[ "$actual_go" != "$required_go" ]]; then
  echo "ERROR: required Go $required_go; found $actual_go" >&2
  exit 1
fi

if [[ -n "$(gofmt -l .)" ]]; then
  echo "ERROR: gofmt reported unformatted files" >&2
  gofmt -l .
  exit 1
fi

go vet ./...
go build ./...
go test ./...
go test -race ./...
go test ./internal/chain ./internal/node -run 'TestC1' -count=1 -v
sha256sum -c build/SHA256SUMS
