# Decision note: V1 retirement

**Status:** OWNER DECISION REQUIRED. No version activation or retirement change is authorized by this note.

## Current baseline

V1 is retained as a historical compatibility path. V2 adds explicit network identity and account nonce semantics. V3 currently inherits the V2 economic/state model while adding C-1 time behavior.

## Options

### Option A — retain V1 indefinitely

- Advantages: maximal historical compatibility.
- Trade-offs: ongoing attack surface and ambiguity over which networks may accept legacy transactions.

### Option B — prohibit V1 on public networks immediately

- Advantages: reduces public exposure to legacy replay and signing semantics.
- Trade-offs: can strand older private networks or historical tooling unless compatibility is clearly isolated.

### Option C — height/epoch-based retirement

- Advantages: deterministic transition with an explicit grace period.
- Trade-offs: requires a consensus activation rule, coordination and a precise migration contract.

## Recommendation

Keep V1 available only for explicit historical/private compatibility contexts and require a separately approved public-network activation boundary. Do not silently change protocol acceptance in this audit phase.
