# Git & GitHub Workflow — SAYANJALI BLOCKCHAIN

A solo-developer workflow designed to work entirely from Termux, matching
the branch/tag conventions already used across other SAYANJALI NEXUS repos.

## 1. Initialize Git (first time only)

```bash
cd ~/sayanjali-blockchain
git init
git config user.name "Syed Ali Hasan Moosavi"
git config user.email "your-email@example.com"
```

## 2. Confirm .gitignore is in place

The project ships with a `.gitignore` that excludes `database/*.db`,
`logs/*.log`, virtual environments, and secrets. Double-check before your
first commit:

```bash
git status
```

You should **not** see `database/sayanjali_chain.db` or `logs/sayanjali.log`
listed as untracked files ready to commit. If they appear, verify
`.gitignore` was created before `git add`.

## 3. First commit

```bash
git add .
git commit -m "Initial commit: SAYANJALI BLOCKCHAIN MVP"
```

## 4. Branch strategy (solo development)

A lightweight trunk-based flow works well for a solo builder:

- `main` — always deployable/stable.
- `dev` — active development integration branch.
- `feature/<short-name>` — short-lived branches for individual features
  (e.g. `feature/pos-consensus`, `feature/p2p-networking`).

```bash
git checkout -b dev
git checkout -b feature/wallet-cli-improvements
# ... work, commit ...
git checkout dev
git merge feature/wallet-cli-improvements
git branch -d feature/wallet-cli-improvements
```

Periodically fast-forward `main` from `dev` once a milestone is stable:

```bash
git checkout main
git merge dev
```

## 5. Tag releases

Follow semantic versioning, matching the `v0.1.0-mvp` style already used in
`blockchain/__init__.py`:

```bash
git tag -a v0.1.0-mvp -m "SAYANJALI BLOCKCHAIN MVP: core chain, wallets, PoW, REST API, CLI"
git push origin v0.1.0-mvp
```

## 6. Connect a remote repository

Create the repo on GitHub first (via the GitHub mobile app, github.com in a
Termux browser, or `gh repo create` if you install the GitHub CLI), then:

```bash
git remote add origin https://github.com/<your-username>/sayanjali-blockchain.git
git branch -M main
git push -u origin main
```

### Authenticating from Termux

GitHub no longer accepts password auth for pushes. Use a Personal Access
Token (PAT):

1. GitHub → Settings → Developer settings → Personal access tokens → Generate new token (fine-grained, scoped to this repo, `contents: read/write`).
2. When Termux prompts for a password on `git push`, paste the token instead.
3. To avoid re-entering it every time, cache it:

```bash
git config --global credential.helper store
```

(This stores the token in plaintext in `~/.git-credentials` — acceptable
for a personal device, but be mindful if the phone is shared.)

## 7. Push to GitHub

```bash
git push origin main
git push origin dev
```

## 8. Pull updates (e.g. syncing across devices)

```bash
git pull origin main
```

## 9. Handling merge conflicts

```bash
git pull origin main
# Git reports conflicting files
nano path/to/conflicted_file.py   # or your preferred Termux editor
# resolve conflict markers <<<<<<< ======= >>>>>>>
git add path/to/conflicted_file.py
git commit -m "Resolve merge conflict in <file>"
git push origin main
```

For CLI/API files especially, re-run `pytest` after resolving any conflict
before pushing — a clean merge doesn't guarantee the merged logic is still
correct.

## 10. Recommended commit message conventions

Short, imperative, scoped:

```
feat(wallet): add address validation helper
fix(consensus): correct difficulty retarget floor
docs(readme): update API endpoint table
test(mining): add coinbase reward assertion
chore(deps): bump fastapi to 0.110
```

This keeps `git log --oneline` scannable when reviewing months of solo
Termux commits later.
