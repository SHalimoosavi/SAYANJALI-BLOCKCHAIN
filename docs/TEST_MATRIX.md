# SAYANJALI BLOCKCHAIN test matrix

## Evidence baseline

Repository: `SHalimoosavi/SAYANJALI-BLOCKCHAIN`

Commit: `9502979638e48a43519360a3215eca2cb84e6ce9`

The figures below are from completed GitHub Actions runs attached to the baseline commit. They are not estimates.

- [Production Validation run 36946492832](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/36946492832)
- [Build 1 C-1 Verification run 36946492867](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/36946492867)

## Go production-track suite

Runner: Ubuntu 24.04, Linux amd64.

Toolchain: Go 1.27.1 (the Build 1 C-1 job explicitly prints `go version go1.27.1 linux/amd64`).

| Check | Result | Evidence |
|---|---|---|
| Exact Go toolchain | PASS | Build 1 C-1 job: Go 1.27.1 |
| `gofmt` | PASS | Workflow step completed |
| `go vet ./...` | PASS | Production Validation job |
| `go build ./...` | PASS | Production Validation job |
| `go test ./...` | PASS | 18 test-bearing packages report `ok`; packages without tests are reported separately |
| `go test -race ./...` | PASS | 18 test-bearing packages report `ok` |
| `govulncheck ./...` | PASS | Output: No vulnerabilities found |
| `staticcheck ./...` | PASS | Production Validation job completed successfully |
| `gosec ./...` | PASS | 51 files, 9,560 lines, 22 `nosec` annotations, 0 issues |
| Go coverage | NOT PRODUCED | Baseline workflows do not generate a coverage profile |

The standard `go test ./...` output does not record an aggregate individual-test count. This document therefore reports package outcomes and does not invent a test-case count.

The race detector ran on the GitHub-hosted Linux amd64 runner. The earlier Android/ARM64 limitation does not apply to this CI run.

## Python reference suite

Runner: Ubuntu 24.04, Python 3.12.14.

| Check | Result | Evidence |
|---|---|---|
| Dependency installation | PASS | `pip install -r requirements.lock.txt` |
| `pytest -q` | PASS | 262 passed, 2 warnings |
| `compileall` | PASS | `python -m compileall -q .` |
| `pip-audit` | PASS | No known vulnerabilities found |
| Bandit | PASS WITH WARNINGS | Two B104 `nosec` annotations in `config/settings.py`; no failed Bandit findings |

## Dependency clarification: ecdsa

The current GitHub `main` source does not import the Python `ecdsa` package. `blockchain/wallet.py` uses `cryptography` for SECP256K1. The current `requirements.txt` and `requirements.lock.txt` do not declare `ecdsa`, and the Phase 9.3 remediation record explicitly documents its removal and migration to `cryptography`.

Therefore adding `ecdsa` would reintroduce a dependency the current source intentionally removed. The earlier missing-`ecdsa` failure came from the separately uploaded older Python archive, not the current GitHub baseline. No dependency change is appropriate for this item.

## Historical counts and drift

Older README and TESTING.md values (195, 231, 244, and 262) refer to different repository snapshots or execution contexts. They must not be presented as interchangeable counts.

The current baseline Python result is 262 passed, 2 warnings. The Go result is reported by package, because the CI output did not emit an aggregate test-case count.

## Coverage workflow

A separate Go 1.27.1 workflow is added in this audit phase to run:

- `go test -race -coverprofile=coverage.out ./...`
- `go tool cover -func=coverage.out`
- upload the coverage profile and summary as workflow artifacts.

Coverage remains NOT PRODUCED for the baseline commit until that new workflow runs successfully on an audit PR.
