# Phase 2 audit: documentation and implementation drift

Baseline: `9502979638e48a43519360a3215eca2cb84e6ce9`

This file records documentation statements that do not match the current source or current CI evidence. A stale status statement is not proof that the underlying behavior is production-ready.

| Document claim | Current source/evidence | Audit conclusion |
|---|---|---|
| `docs/PROTOCOL_GAPS.md` G-015 says incremental synchronization is not implemented | `internal/p2pnode/network.go` contains `syncLoop`, `syncPeer`, locator generation, header exchange and block retrieval | Implementation exists; long-duration convergence and partition recovery remain unverified |
| G-018 says hostname validation does not pin the resolved IP to the connection | `AddressPolicy.ResolveAndValidate` returns validated IP endpoints; `seedLoop` dials those returned endpoints. Production address policy disables DNS by default | The stated gap is stale for the seed-dial path; full inbound/outbound DNS rebinding coverage remains unverified |
| G-020 says security scanner coverage is absent from CI | `.github/workflows/production-validation.yml` runs `govulncheck`, `staticcheck`, `gosec`, `pip-audit` and Bandit | Scanner integration exists; current Go 1.27.1 govulncheck fails on newly published standard-library advisories |
| G-021 says no Python lockfile exists | `requirements.lock.txt` exists and is installed by the current Python CI job | Documentation is stale; verify lockfile refresh and reproducibility as part of release engineering |
| Phase 9.5 matrix says race tests and scanners were not executed locally | Baseline GitHub Actions run executes `go test -race ./...`; Go 1.27.4 PR #26 passes govulncheck, staticcheck and gosec | Historical local limitation must not be presented as current CI status |
| Build 1.1 evidence says exact Go 1.27.1 verification was not executed | [Build 1 C-1 Verification run 36946492867](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/36946492867) completed successfully and printed Go 1.27.1 | Historical implementation note is superseded by the CI evidence |
| README shows a static 195-test badge and older ecdsa dependency wording | Current Python CI reports 262 passed, 2 warnings; source wallet uses `cryptography`; Phase 9.3 records ecdsa removal | Corrected in separate PR #23 |
| TESTING.md contains older 231/244 counts | Current baseline Python CI reports 262 passed, 2 warnings | Corrected in separate PR #23 |

## Important distinctions

- The existence of synchronization code does not prove reliable convergence under a long partition.
- Pinning DNS resolution for seed dials does not prove all address-admission and operator workflows are free of DNS rebinding risks.
- Having security scanners in CI does not mean the current toolchain passes them. Go 1.27.1 now fails govulncheck; PR #26 upgrades to Go 1.27.4 and has passing CI.
- A Python lockfile improves dependency reproducibility but does not establish reproducible Go builds or signed release artifacts.
- The Go 1.27.4 coverage run reports 51.6% statement coverage, with 0% in several important packages. Passing tests are not equivalent to complete coverage.
