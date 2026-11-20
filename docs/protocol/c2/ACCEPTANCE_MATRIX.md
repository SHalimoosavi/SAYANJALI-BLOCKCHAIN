# SAYANJALI BLOCKCHAIN — C-2 Acceptance Matrix

**Status:** required Build 2B evidence; execution NOT PERFORMED in Build 2A.

| ID | Test | Input | Expected result | Invariant | Failure mode | Test location |
|---|---|---|---|---|---|---|
| A | Empty state | zero accounts | exact selected empty-tree root | empty-root definition | root mismatch | C-2 state/SMT acceptance suite |
| B | One account | one protocol-valid account | exact root | leaf encoding/root determinism | root mismatch | C-2 state/SMT acceptance suite |
| C | Multiple accounts | deterministic valid accounts | exact root | insertion independent of map order | root mismatch | C-2 state/SMT acceptance suite |
| D | 1,000 accounts | frozen V8 fixture | root equals frozen vector | vector checksum unchanged | vector mismatch | `scripts/c2/state_smt_v3_vectors_test.go` + production cross-check |
| E | 62,600 accounts | exact deterministic fixture | persists/reopens with same root | no single-record ceiling failure | persistence/recovery failure | Build 2B large-state suite |
| F | 200,000 accounts | exact deterministic fixture | persists/reopens with same root | bounded resource behavior | persistence/recovery/OOM | Build 2B large-state suite |
| G | Insert | absent valid account | selected post-state | deterministic leaf creation | wrong root/state | C-2 transition suite |
| H | Update | existing account | selected updated leaf/root | exact balance+nonce semantics | wrong root/state | C-2 transition suite |
| I | Delete | account meeting selected closure rule | selected deletion/tombstone state | D1 semantics | divergent root | C-2 lifecycle suite |
| J | Recreate | previously deleted/closed account | selected D1 result | nonce/replay rule | divergent acceptance/root | C-2 lifecycle suite |
| K | Nonce | valid sequential nonce | accepted and incremented once | nonce invariant | rejection/root mismatch | C-2 transition suite |
| L | Balance | debit/credit boundary values | exact balances | no overflow/underflow | invalid acceptance/state | C-2 transition suite |
| M | Minimum balance | below/at/above M | exact owner-selected behavior | D2 invariant | divergent acceptance | C-2 economics suite |
| N | Fee deduction | fee boundary values | exact payer balance/root | H-2 ordering | divergent root | C-2 economics suite |
| O | Empty block | permitted empty/required-tx block | exact state effect | empty-delta rule | wrong root | C-2 block transition suite |
| P | Multiple transactions | ordered block | sequential deterministic transition | block order preserved | root mismatch | C-2 transition suite |
| Q | Duplicate account mutation | ordered mutations | exact selected result | duplicate mutation rule | divergent result | C-2 transition suite |
| R | Invalid transaction | malformed/invalid tx | whole block rejected, no partial publication | failure atomicity | partial state | C-2 transition suite |
| S | Reorg | selected shallow fork | winning-chain state | canonical-chain invariant | wrong state/tip | C-2 reorg suite |
| T | Deep reorg | documented deep depth | recovered root equals clean replay | replay equivalence | divergence | C-2 reorg suite |
| U | Restart | valid committed state | same root/tip | durable-state invariant | recovery divergence | C-2 recovery suite |
| V | Clean replay | winning chain | same root as recovered state | deterministic replay | mismatch | C-2 recovery/replay suite |
| W | Corrupt journal | every byte offset of every new record type | exact frozen recovery action | no ambiguous corruption handling | wrong action | C-2 corruption suite |
| X | Torn journal | every legal truncation boundary | exact frozen recovery action | tail protocol | divergent recovery | C-2 corruption suite |
| Y | Corrupt checkpoint | each chunk byte offset | exact frozen recovery action | checkpoint integrity | unsafe recovery | C-2 checkpoint suite |
| Z | Incomplete checkpoint | missing chunks/manifest/commit marker | exact frozen action | no uncommitted publication | partial state | C-2 checkpoint suite |
| AA | Invalid manifest | malformed/hash-mismatched manifest | exact rejection | manifest authenticity | false acceptance | C-2 checkpoint suite |
| AB | Missing commit marker | complete payloads without marker | exact frozen action | commit atomicity | partial publication | C-2 recovery suite |
| AC | Truncated record | every truncation offset | exact frozen action | parser determinism | inconsistent recovery | C-2 corruption suite |
| AD | Invalid checksum | each checksum-bearing record | exact rejection/recovery | integrity invariant | corruption accepted | C-2 corruption suite |
| AE | Invalid state root | correct data + wrong claimed root | rejection | root verification | false acceptance | C-2 state suite |
| AF | Wrong proof | wrong key/leaf/sibling/root | rejection | proof binding | false acceptance | C-2 proof suite |
| AG | Non-inclusion | absent protocol-valid address | exact proof acceptance | empty-path semantics | false proof | C-2 proof suite |
| AH | V1 compatibility | historical V1 chain | byte-for-byte legacy result | V1 unchanged | regression | existing V1 regression + C-2 compatibility suite |
| AI | V2 compatibility | historical V2 chain | byte-for-byte legacy result | V2 unchanged | regression | existing V2 regression + C-2 compatibility suite |
| AJ | Deterministic repeated execution | same chain/input repeated | identical state/root | deterministic execution | nondeterminism | C-2 determinism suite |

## Required negative proof cases

The proof suite must reject:

1. wrong queried key with unchanged sibling path;
2. directions inconsistent with queried key;
3. wrong leaf hash;
4. one modified sibling;
5. wrong expected root;
6. truncated sibling list;
7. excess sibling list;
8. truncated direction list;
9. invalid direction value;
10. non-32-byte sibling;
11. malformed key length;
12. malformed hash encoding.

These are not claimed as executed in Build 2A.

## Large-state evidence

For E/F record elapsed time, peak memory, journal size, checkpoint size, materialized SMT nodes, SHA-256 count, recovery time and replay distance. Until execution, all performance fields are **NOT MEASURED**.

## Coverage qualification

V8 has 1,000 deterministic accounts. V4 tests first-byte `0x00` and `0xFF` key prefixes, not literal 256-bit all-zero/all-one keys. V10 uses an arbitrary string address, not a protocol-valid 43-byte address. Separate protocol-valid boundary coverage is therefore required unless the owner explicitly defines the proof API as accepting arbitrary byte strings.
