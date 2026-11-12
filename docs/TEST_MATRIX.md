# SAYANJALI BLOCKCHAIN test matrix

## Evidence baseline

Repository: `SHalimoosavi/SAYANJALI-BLOCKCHAIN`

Commit: `9502979638e48a43519360a3215eca2cb84e6ce9`

Recorded GitHub Actions runs:
- [Production Validation run 36946492832](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/36946492832)
- [Build 1 C-1 Verification run 36946492867](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/36946492867)

## Go production-track suite

Runner: Ubuntu 24.04, Linux amd64. Toolchain: Go 1.27.1.

| Check | Result | Evidence |
|---|---|---|
| Exact Go toolchain | PASS | Job prints `go version go1.27.1 linux/amd64` |
| `gofmt` | PASS | Build 1 C-1 workflow |
| `go vet ./...` | PASS | Production Validation |
| `go build ./...` | PASS | Production Validation |
| `go test ./...` | PASS | 18 test-bearing packages report `ok` |
| `go test -race ./...` | PASS | 18 test-bearing packages report `ok` |
| `govulncheck ./...` | PASS at baseline run date | Output: No vulnerabilities found |
| `staticcheck ./...` | PASS at baseline run date | Production Validation job completed |
| `gosec ./...` | PASS | 51 files, 9,560 lines, 22 `nosec` annotations, 0 issues |
| Go coverage | NOT PRODUCED | Baseline workflows did not generate a coverage profile |

The standard Go test output did not record an aggregate individual-test count. This matrix reports package outcomes and does not invent a test-case count.

The race detector ran on the GitHub-hosted Linux amd64 runner. The earlier Android/ARM64 limitation does not apply to this CI run.

## Python reference suite

Runner: Ubuntu 24.04, Python 3.12.

| Check | Result | Evidence |
|---|---|---|
| Dependency installation | PASS | `pip install -r requirements.lock.txt` |
| `pytest -q` | PASS | 262 passed, 2 warnings |
| `compileall` | PASS | `python -m compileall -q .` |
| `pip-audit` | PASS | No known vulnerabilities found |
| Bandit | PASS WITH WARNINGS | Two B104 `nosec` annotations in `config/settings.py`; no failed findings |

## Dependency clarification: ecdsa

The current GitHub baseline does not import the Python `ecdsa` package. `blockchain/wallet.py` uses `cryptography` for SECP256K1. Neither current requirements manifest declares `ecdsa`, and the Phase 9.3 remediation record documents its removal and migration to `cryptography`.

Adding `ecdsa` would reintroduce a dependency intentionally removed by the project. The missing-`ecdsa` failure from the earlier audit came from a separately uploaded older Python archive, not this GitHub baseline. No dependency change is appropriate.

## Historical counts and drift

Older README and TESTING.md values (195, 231, 244 and 262) refer to different repository snapshots or execution contexts. They must not be presented as interchangeable counts.

The current baseline Python result is 262 passed, 2 warnings. The Go result is reported by package because the CI output did not emit an aggregate test-case count.

## Coverage workflow

The audit coverage workflow runs Go 1.27.1 and executes:

- `go test -race -coverprofile=coverage.out ./...`
- `go tool cover -func=coverage.out`
- upload the coverage profile as a workflow artifact.

Coverage remains NOT PRODUCED for the baseline commit until this workflow completes successfully on a review PR.

## Subsequent Go vulnerability recheck (2026-11-12)

A later Go 1.27.1 PR validation run failed at `govulncheck ./...`:

- [Production Validation run 43134361620](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/43134361620)
- Go job: `130905980200`
- Result: 11 Go standard-library vulnerability findings, plus 2 package and 2 module findings for which the scanner did not identify a call path from this code.
- Fixed toolchain versions reported include Go 1.27.2, 1.27.3 and 1.27.4; GO-2026-6859 requires Go 1.27.4.

The earlier 2026-10-02 clean scan is historical evidence, not the current security status. PR #26 proposes moving the pinned toolchain and exact CI checks to Go 1.27.4. The upgrade is not considered verified until its CI run passes.

## Go coverage result

The first exact Go 1.27.1 race+coverage run completed successfully:

- [Audit Go Race and Coverage run 43134996150](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/43134996150)
- [Coverage artifact](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/43134996150/artifacts/13906812432)
- Exact runtime: `go version go1.27.1 linux/amd64`
- `go test -race -coverprofile=coverage.out ./...`: PASS
- Aggregate statement coverage: **52.6%**

Notable package coverage from that run: chain 64.3%, node 57.9%, P2P 75.4%, P2P node 62.3%, security 75.6%, state commitment 97.0%, storage 59.6%, tokenomics 60.2%, transaction 31.2%, consensus 15.8%, block 38.9%.

Because the subsequent vulnerability scan found Go 1.27.1 standard-library vulnerabilities, the workflow in this PR is updated to Go 1.27.4 for future runs. The Go 1.27.1 result above remains the exact requested baseline measurement.
