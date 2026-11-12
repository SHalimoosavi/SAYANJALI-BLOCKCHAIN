# SAYANJALI BLOCKCHAIN security policy

## Scope

This policy covers the SAYANJALI BLOCKCHAIN source repository, node software, protocol implementation, reference implementation, build workflows and project-operated infrastructure.

This document describes a reporting and triage process. It does not claim that the software has been independently audited or is safe for mainnet use.

## Reporting a vulnerability

Send vulnerability reports to:

**shalimoosavi@gmail.com**

Do not publish exploit details, proof-of-concept code, private keys, credentials or unpatched vulnerability information in public issues or pull requests.

A useful report includes, where available:

- affected repository commit, release or component;
- affected file/function or endpoint;
- supported operating system and toolchain;
- exact reproduction steps;
- expected and observed behavior;
- impact and prerequisites;
- a minimal proof of concept that does not expose real funds, user data or production systems;
- suggested mitigation, if known.

Do not include real private keys, seed phrases, access tokens or production secrets. Use disposable local test data.

The project will use the reporting email above for initial coordination. No response-time or remediation-time guarantee is made by this policy.

## Triage process

1. Acknowledge receipt when the report has been reviewed.
2. Reproduce the issue on an isolated test environment where possible.
3. Identify affected versions, attack prerequisites and potential impact.
4. Assign a severity using the scale below.
5. Develop and test a fix on a non-main branch.
6. Review whether the fix changes consensus, transaction validity, monetary policy, address encoding, genesis or chain identity. Such changes require an explicit protocol decision and owner approval.
7. Publish a security advisory or release note after a coordinated disclosure decision.
8. Record unresolved risk and mitigation if a complete fix is not immediately possible.

A finding is not considered confirmed until it has been reproduced or established by a deterministic regression test. Source-level suspicion must be labelled suspected.

## Severity scale

| Severity | Criteria | Examples |
|---|---|---|
| Critical | Remote or low-complexity compromise of consensus safety, arbitrary issuance, unauthorized asset transfer at scale, persistent network split, or exposure of signing material with broad impact | Supply invariant bypass; consensus acceptance of invalid state |
| High | Serious remote denial of service, reliable transaction theft, widespread node compromise, or a flaw that materially weakens consensus/network security | Remote peer crash across nodes; replay that enables unauthorized spending |
| Medium | Limited-scope security boundary failure, request validation issue, resource exhaustion with meaningful constraints, or privilege misuse requiring prerequisites | API parser ambiguity; bounded but ineffective admission control |
| Low | Narrow impact, high prerequisites, limited information disclosure or defense-in-depth weakness | Non-sensitive diagnostic leakage |
| Informational | Hardening recommendation, documentation gap or non-exploitable observation | Missing operational runbook |

Severity may be adjusted after reproduction and impact analysis. A scanner finding is not automatically a vulnerability.

## Handling and disclosure

- Keep the report private while reproduction and remediation are underway.
- Do not access, alter or attempt to recover real user funds.
- Do not test against infrastructure without authorization.
- Prefer a minimal regression test over a weaponized exploit.
- Coordinate disclosure timing with the reporter where reasonably possible.
- Publish affected versions, severity, impact, mitigation and fixed versions when disclosure is appropriate.
- If a report is not reproducible, document what was tested and what additional evidence is needed.

## Secret handling

- Never commit private keys, seed phrases, API tokens, TLS private keys, passwords or production credentials.
- Use disposable local credentials in tests.
- Keep node identity encryption keys outside the repository and configuration examples.
- Rotate any credential that may have been exposed.
- Do not place private key material in genesis files. Genesis allocation configuration must contain public addresses only.

## Protocol-sensitive changes

Changes to consensus, monetary policy, transaction format, address format, genesis data, network identity or chain IDs are protocol changes. They must not be introduced as ordinary security maintenance. Prepare a design note with options, trade-offs, compatibility impact and test requirements, then wait for explicit owner approval.

## Current security posture

The repository contains Go unit/integration tests, Python reference tests, security scanners and local multi-node harnesses. Their exact status is recorded in [TEST_MATRIX.md](TEST_MATRIX.md). These results do not replace an independent protocol and infrastructure audit.

The project is not represented as mainnet-ready by this policy.
