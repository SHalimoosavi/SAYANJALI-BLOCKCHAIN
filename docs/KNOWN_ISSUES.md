# Known issues on baseline main

Baseline: `9502979638e48a43519360a3215eca2cb84e6ce9`

This file records confirmed implementation issues observed on the baseline branch. A proposed fix on an unmerged PR does not change the status of `main`.

## API JSON request decoding accepts unknown fields and trailing JSON values

- **ID:** API-JSON-001
- **Severity:** Medium
- **Status on baseline main:** Confirmed, open until PR #19 is merged
- **Location:** `internal/node/api.go`, POST `/transactions` request decoding
- **Reproduction:** the baseline handler calls `json.Decoder.Decode` once without `DisallowUnknownFields` and without a second decode to require EOF. A standalone Go reproduction using the same standard-library decoder accepted both `{"name":"syj","unknown":true}` and `{"name":"syj"} {"name":"other"}` with a nil error.
- **Impact:** unknown request properties are silently ignored and a second JSON value after the first is not rejected at the decoding boundary. This weakens request-contract strictness and can cause client/server interpretation differences.
- **Proposed remediation:** PR #19 adds strict decoding, exactly-one-value enforcement and regression tests.
- **Verification status:** standard-library behavior reproduced locally; PR CI must pass before closure.

No consensus, monetary, transaction-format, address-format, genesis or chain-ID changes are included in the proposed remediation.

## Go 1.27.1 standard-library vulnerabilities reported by govulncheck

- **ID:** TOOLCHAIN-GOVULN-001
- **Severity:** High
- **Status on baseline main:** Confirmed scanner finding; open pending Go toolchain upgrade
- **Location:** `go.mod` toolchain directive and Go standard-library dependencies used by HTTP/TLS node and CLI paths
- **Reproduction:** Production Validation PR run [43134361620](https://github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/actions/runs/43134361620), Go job `130905980200`, failed at `govulncheck ./...` on 2026-11-12.
- **Standard-library vulnerability IDs:** GO-2026-6859, GO-2026-6809, GO-2026-6617, GO-2026-6613, GO-2026-6612, GO-2026-6611, GO-2026-6610, GO-2026-6608, GO-2026-6607, GO-2026-6605 and GO-2026-6603.
- **Fixed versions reported by the scanner:** Go 1.27.2, 1.27.3 or 1.27.4 depending on the package; GO-2026-6859 is fixed in Go 1.27.4.
- **Additional scanner note:** two vulnerabilities were found in imported packages and two in required modules, but the scanner reported that the code does not appear to call those vulnerable paths.
- **Proposed remediation:** PR #26 upgrades the pinned Go toolchain and exact CI checks to Go 1.27.4.
- **Verification status:** the scanner finding is confirmed; the proposed toolchain upgrade remains unverified until PR #26 CI completes.

Do not merge a Go 1.27.1-based release after this finding without either upgrading the toolchain or documenting an approved mitigation.

## Production genesis artifact cannot be loaded directly by the node

- **ID:** GENESIS-ARTIFACT-001
- **Severity:** High
- **Status on baseline main:** Confirmed integration gap
- **Location:** `cmd/syj-genesis/main.go:44-70`; `internal/genesisconfig/config.go:190-232`; `internal/node/node.go:228-281`; `internal/tokenomics/genesisstate.go:66-82`
- **Reproduction:** the `syj-genesis generate` command writes a `genesisconfig.Artifact`. The V2/V3 node startup path passes the configured file to `tokenomics.Load`, which expects a `tokenomics.GenesisState`. The artifact JSON uses `genesis_state_version` and allocation `address`; the node loader expects `version` and allocation `recipient`. Characterization test in PR #21: `internal/genesisconfig/production_artifact_node_load_audit_test.go`, passed in Build 1 C-1 Verification run 43136118051.
- **Impact:** the documented production genesis generation flow does not, by itself, produce the exact JSON file the node startup path loads. An operator needs an explicit, deterministic conversion step that is not currently exposed by the CLI.
- **Proposed remediation:** owner decision note in PR #21 recommends an explicit verified `export-state` command with golden vectors. No implementation is included pending approval.

## Node GenesisState validation accepts duplicate recipients

- **ID:** GENESIS-RECIPIENT-001
- **Severity:** Medium
- **Status on baseline main:** Confirmed validation inconsistency; owner decision required
- **Location:** `internal/tokenomics/allocation.go:72-135`; `internal/tokenomics/genesisstate.go:23-29`; `internal/genesisconfig/config.go:72-107`; `internal/node/node.go:234-246`
- **Reproduction:** `genesisconfig.Validate` rejects duplicate recipient addresses, but the node loads `tokenomics.GenesisState` directly and its `ValidateStructure` checks categories/amounts without checking recipient uniqueness. Characterization test in PR #21: `internal/tokenomics/genesisstate_duplicate_recipient_audit_test.go`, passed in Build 1 C-1 Verification run 43136118051.
- **Impact:** a GenesisState can aggregate multiple allocation categories to one address through the node loading path, despite the production config validator's unique-recipient rule.
- **Proposed remediation:** owner decision note in PR #21 documents the options. Do not change genesis acceptance rules or existing genesis data without explicit approval.
