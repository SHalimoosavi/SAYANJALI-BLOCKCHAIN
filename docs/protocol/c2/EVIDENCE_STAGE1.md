# Stage 1 Evidence

Base commit read: `be76d48a5258686d081238c84d4e576b8be49da6` (GitHub exact ref).
Local repository execution was not available to the builder; evidence below is from the generated overlay's offline standard-library tooling.

## Executed
- Generator run twice + recursive byte comparison: PASS — `PASS generator byte comparison: two runs identical`
- Verifier A: PASS — `PASS verifier A: anchors, owner odd-parity golden, signature, tx_id, address length`
- EC negatives A: PASS — `PASS EC negative vectors: zero/high-S/tampered digest/pubkey/r`
- Verifier B: PASS — `PASS verifier B: independent EC arithmetic, RFC6979, low-S, tx_id`
- Transaction/EC negatives B: PASS — `PASS transaction/EC negative vectors: high-S, zero-r, tampered digest/pubkey`
- Transaction negative/boundary checks: PASS — `PASS transaction negative/boundary vectors: version, tx-id mutation, amount/fee, sender/address invariants`
- Domain registry + 43-character canonical-address scan: PASS — `PASS domain-registry and 43-character canonical-address scans`
- Key-2 anchor reproduced: x=`c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5`, y=`1ae168fea63dc339a3c58419466ceaeef7f632653266d0e1236431a950cfe52a`, compressed=`02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5`.
- Odd-parity golden reproduced exactly: digest=`49abd933182355725c2ecb7e641f37b3ae0b0f64e14ddda3e9f2aeefd6ed3c81`, r=`45922ce791663e9dd46ae275d08f46647de3c631d9c74c8d2a2b5a05da5207bb`, s=`21cf1e654515402614444e1af3940f61d20e38439d10a616e7b25dd94e450584`, tx_id=`dcdb66c98113fc8068d8f67ba277663a3b36e681406839e12e5af37452809cf5`.
- Key-1 transaction: digest=`6fe029c35e309f380230f5fd3142f4c5e72f7dcce6dfd08632459ba1832d0858`, tx_id=`ed6bb57d14743674b82ac4edeae10ee3713b02df5030dbb3b97cfe9723e77768`.

## Not executed
- Frozen `protocol/test-vectors/state-smt-v3.json` exact byte-length/SHA-256 verification: NOT EXECUTED because the complete 970155-byte file could not be consumed from the read result without truncation.
- SMT root reproductions/proof bitmap checks: NOT EXECUTED (Stage 3; frozen file also unavailable in complete bytes).
- Stage 2 vectors: NOT EXECUTED by owner instruction.
- Stage 3 vectors: NOT EXECUTED by owner instruction.
- Go cross-check, gofmt, go vet, Go tests/build: NOT EXECUTED (Go 1.27.1 toolchain unavailable in Termux and Stage 4 skipped).
- GitHub CI: NOT EXECUTED; no run was created.
- Real CI run URL: NOT AVAILABLE / NOT EXECUTED.

## STOP conditions
No Stage-1 STOP condition was hit. The frozen-SMT read limitation was recorded as NOT EXECUTED, not treated as a hash mismatch.
