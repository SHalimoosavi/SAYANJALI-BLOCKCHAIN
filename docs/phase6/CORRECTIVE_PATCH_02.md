# Phase 6 Corrective Patch 02

## Findings from clean-room 3-node validation

The first corrected Phase 6 artifact passed the complete Go build/test/vet suite and frozen protocol checks, but the private 3-node testnet exposed three lifecycle/network integration defects.

### 1. Stale duplicate peer sessions removed the live session
Multiple nodes intentionally dial the same seed while the remote node may simultaneously dial back. When an older duplicate session closed, its cleanup unconditionally deleted the peer-map entry belonging to the newer session. This caused repeated successful handshakes followed by `peer_count=0`.

**Fix:** peer removal is now session-aware: cleanup removes the map entry only when the session being closed is still the active session for that node ID.

### 2. Difficulty retarget was invoked before a complete frozen window existed
Mining block 2 could pass a short timestamp window into the frozen Fraction-compatible retarget implementation, producing the expected compatibility error for fractional timestamps even though no retarget should occur yet.

**Fix:** node difficulty selection now retains the previous block difficulty until a complete 10-block window exists, and only invokes the frozen retarget routine at the corresponding epoch boundary. The frozen consensus routine and vectors are unchanged.

### 3. API shutdown stopped the node but did not terminate `syjd start`
`POST /shutdown` called `Node.Stop`, but the CLI process was waiting on the signal context rather than the node lifecycle. This left the API process alive with `running=false` and allowed stale processes to accumulate across clean-testnet replacement.

**Fix:** `Node.Done()` is now closed exactly once by `Node.Stop`, and `syjd start` exits when either its signal context or node lifecycle completes.

## Protocol impact

No Phase 4 consensus rule, compatibility vector, Phase 5.1 cryptographic behavior, or Phase 5.2 wire format was changed.
