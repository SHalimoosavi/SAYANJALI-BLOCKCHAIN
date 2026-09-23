# Phase 9 Manual Commit Instructions

**Baseline:** `5e02557363be6319cca06cd7e20a8be906265060`
**Intended branch:** `phase-9-protocol-formalization`
**Intended PR:** #11 (to be created manually)
**PR title:** `docs(protocol): formalize post-v2 consensus architecture`

## 1. Start from the required baseline

```bash
git checkout main
git pull origin main
git rev-parse HEAD
# Expected main HEAD at preparation time: 5e02557363be6319cca06cd7e20a8be906265060

git checkout 5e02557363be6319cca06cd7e20a8be906265060
git switch -c phase-9-protocol-formalization
```

If `git rev-parse HEAD` is not `5e02557363be6319cca06cd7e20a8be906265060`, stop and reconcile the repository state before copying Phase 9.

## 2. Copy the deliverable

From the extracted ZIP:

```bash
# Copy the Phase 9 docs into the repository
cp -r phase-9-protocol-formalization/docs/. docs/

# Copy the protocol schemas/version metadata
mkdir -p protocol
cp -r phase-9-protocol-formalization/protocol/. protocol/

# Review and then replace the repository README
cp project-root-updates/PROJECT_STATUS.md PROJECT_STATUS.md
cp project-root-updates/README.md README.md
```

Do not copy `MANIFEST.md`, `COMMIT_INSTRUCTIONS.md`, or `REVIEW_CHECKLIST.md` into the repository unless the project maintainer separately decides to do so.

## 3. Review

```bash
git status --short
git diff -- README.md PROJECT_STATUS.md
git diff -- docs protocol
```

Confirm that the changes contain specifications/schemas only and no implementation source.

## 4. Validate JSON

With Python available:

```bash
python - <<'PY'
import json
from pathlib import Path
for p in Path("protocol").rglob("*.json"):
    json.loads(p.read_text())
    print("OK", p)
PY
```

## 5. Validate documentation

Check invariant IDs:

```bash
grep -Rho 'I-[0-9][0-9][0-9]' docs phase-9-protocol-formalization 2>/dev/null | sort -u
```

Search for maturity language:

```bash
grep -RniE 'production-ready|mainnet-ready|fully decentralized|fully secure|finalized' README.md PROJECT_STATUS.md docs protocol
```

Review every occurrence for qualification.

## 6. Commit

```bash
git add docs protocol PROJECT_STATUS.md README.md
git diff --cached --check
git commit -m "docs(protocol): formalize post-v2 consensus architecture"
```

## 7. Push

```bash
git push origin phase-9-protocol-formalization
```

## 8. Open PR #11

- **Title:** `docs(protocol): formalize post-v2 consensus architecture`
- **Base:** `main`
- **Compare:** `phase-9-protocol-formalization`

### PR description

```markdown
## Phase 9 — Protocol Formalization

This PR formalizes the post-V2 protocol architecture and establishes a reviewable specification contract before any production consensus implementation.

### Scope

- Protocol overview and maturity model
- V2 transaction compatibility specification
- Proposed production block/state-transition contracts
- Proposed PoS/BFT consensus model
- Validator lifecycle and key separation
- Staking, fee, and governance specifications
- ADR-009 through ADR-012
- Threat model and launch gates
- Draft 2020-12 JSON schemas
- Protocol version metadata
- Project status / README maturity update

### Invariants

I-001 through I-017 are defined and traceable to specification sections and future test classes.

### Production architecture

Cosmos SDK + CometBFT is documented as the proposed production architecture. It is **NOT SHIPPED** and **NOT IMPLEMENTED**.

### Important maturity statement

The current PoW implementation remains a legacy/transitional/protocol-research track. SYJ is **NOT PRODUCTION-READY** and **NOT MAINNET-READY**.

### What is deliberately not included

- No PoS implementation
- No staking implementation
- No validator implementation
- No slashing implementation
- No governance execution
- No smart-contract runtime
- No mainnet genesis
- No token distribution
- No production deployment

### Testing / validation

The ZIP build validated file-tree completeness, JSON parsing/schema structure, invariant traceability, TBD-parameter coverage, prohibited-content scans, and maturity-language checks. Repository integration tests are not claimed by this documentation-only deliverable.

### Review request

Please review and freeze the protocol specifications, unresolved economic/governance parameters, invariants, threat model, and architecture decisions before Phase 9.1 test mapping or Phase 10 runtime prototyping.
```

## 9. Reviewer checklist

- [ ] Baseline is `5e02557363be6319cca06cd7e20a8be906265060`.
- [ ] All Phase 9 files are present.
- [ ] JSON schemas parse.
- [ ] I-001 through I-017 are traceable.
- [ ] No implementation code is introduced.
- [ ] Economic/governance parameters remain TBD.
- [ ] PoW is legacy/transitional/research.
- [ ] Cosmos SDK + CometBFT is proposed only.
- [ ] SYJ is not represented as production/mainnet-ready.
- [ ] Threat model and launch gates are reviewed.
