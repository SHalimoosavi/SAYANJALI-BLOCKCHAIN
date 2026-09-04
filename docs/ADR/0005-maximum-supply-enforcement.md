# ADR 0005 --- Maximum Supply Enforcement

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

The protocol maximum is exactly:

`720,000,000 SYJ`

No accepted state transition may make total supply exceed the
corresponding base-unit maximum.

## Rules preserved

-   coinbase reward is capped to remaining supply;
-   no issuance is allowed after exhaustion;
-   chain validation replays monetary state;
-   peer-submitted blocks cannot bypass the supply ceiling;
-   unfunded transfers are rejected.

Final token distribution and genesis allocation remain undefined.
