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
