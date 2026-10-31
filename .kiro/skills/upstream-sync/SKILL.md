---
name: upstream-sync
description: Use when bringing code into kube-node-viewer from awslabs/eks-node-viewer or from another fork (e.g. sule26) — checking what is new, cherry-picking with authorship preserved, adapting import paths, crediting the author, and keeping Apache 2.0 obligations.
---

# Upstream / fork sync (kube-node-viewer)

Histories are unrelated (ADR-001): bring changes by **cherry-pick**, never merge.

## 1. See what is new

```bash
git fetch upstream && git fetch <fork-remote>
git log --oneline --no-merges upstream/main -20           # upstream activity
git merge-base <fork>/main upstream/main                  # where the fork branched
git log --format='%h %an %cs %s' upstream/main..<fork>/main   # fork-only commits
```

The last imported upstream SHA is in the `chore: import from …` commit and in
later `(cherry picked from commit …)` trailers:
`git log --grep='cherry picked from' --format='%h %s%n%b' | grep -o 'commit [0-9a-f]*'`.

Skip: dependabot bumps already superseded, AWS-internal CI/governance changes.

## 2. Bring it in

```bash
git switch -c feat/<source>-<topic> main
git cherry-pick -x <sha>...        # -x records the origin; author stays the original
```

- Conflicts from the rebrand: import path `github.com/awslabs/eks-node-viewer`
  → `github.com/staffops/kube-node-viewer` (exact string; `awslabs/operatorpkg`
  is a real dependency), `cmd/eks-node-viewer/` → `cmd/kube-node-viewer/`
  (`git cherry-pick` usually follows the rename), and new label names use the
  `model.LabelPrefix` constant. Resolve, add the modification note to changed
  upstream files, then `scripts/verify.sh`.
- Never rewrite the original author. If you had to change their commit
  substantially, add a follow-up commit of your own instead of amending.
- New files from the source keep their license header; check it is Apache 2.0
  (a fork could relicense its own additions — if not Apache-2.0-compatible, stop
  and ask).

## 3. Credit and record

- `README.md` → "Credits" section: `<feature> by @<github-user> (from <fork URL>)`.
- `CHANGELOG.md` → `## [Unreleased]` entry naming the source.
- Behavior change → it still needs a spec (`specs/<feature>/`), written from
  the imported code (skill `spec-new`), reviewed before merging to `main`.

## 4. Verify and hand off

`scripts/verify.sh`, then the review step of skill `change-loop`. Push or open
a PR only when the owner asks.
