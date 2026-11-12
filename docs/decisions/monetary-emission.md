# Decision note: fixed 50 SYJ reward and emission schedule

**Status:** OWNER DECISION REQUIRED. No monetary-policy change is authorized by this note.

## Current baseline

The current code defines a 50 SYJ block reward and a separate 432,000,000 SYJ mining allocation within a 720,000,000 SYJ maximum supply. The current reward path does not implement a halving schedule. `docs/PROTOCOL_GAPS.md` records a 210,000-block halving interval as reserved, not enforced.

## Options

### Option A — fixed 50 SYJ until the mining allocation is exhausted

- Advantages: matches current reward implementation and is straightforward to verify.
- Trade-offs: issuance per block remains constant; total time to exhaust the mining allocation depends on actual block production.

### Option B — scheduled halving

- Advantages: declining issuance per block and a defined long-term reward curve.
- Trade-offs: requires selecting interval, rounding behavior, reward-zero behavior and activation height; it changes consensus issuance.

### Option C — fixed reward with a terminal tail emission

- Advantages: can retain miner incentives after the initial allocation is exhausted.
- Trade-offs: conflicts with a strict 720M cap unless the cap or allocation model is redesigned.

## Recommendation

Treat the currently implemented fixed 50 SYJ reward until the mining allocation is exhausted as the observed code behavior only. Do not claim that it is the permanent owner-approved policy. Resolve the reserved 210,000-block halving reference before protocol freeze. Any change requires explicit owner approval and consensus review.
