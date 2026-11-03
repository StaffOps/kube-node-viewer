---
name: pre-push
description: Use before every git push or gh pr create in kube-node-viewer ("vou subir", "posso fazer push?", "abrir o PR", "validar antes"), and when CI failed on a PR. The local checks that mirror CI, the cluster run, and the traps that already broke PRs here. A trusted author's PR merges and releases itself once CI passes, so this is the last human-paced gate.
---

# Before a push (kube-node-viewer)

Rules: `AGENTS.md`. If this file disagrees with it, it wins — fix this file.

Why it matters: a PR from a writer of this repo is approved, merged and
merged automatically when CI is green and ships in the next release.
Whatever passes here ships.

The full map of checks (git hooks, agent hooks, CI) is the "Guardrails"
table in `AGENTS.md`. Git hooks must be on: `git config core.hooksPath`
prints `.githooks` (else `make hooks`). Then `pre-commit` and `commit-msg`
run on every commit and `pre-push` runs `make check` on every push.

## 1. Bring in main first

```bash
git fetch origin && git merge origin/main      # never rebase a pushed branch
```

Then look at the top of `CHANGELOG.md`: your lines must sit under
`## [Unreleased]`. A branch cut before a release merges them into the
released section with no conflict (happened on #14 and #15). Conflicts in
`specs/README.md`: keep both entries and renumber the list.

## 2. One command

```bash
make check          # pre-push runs it too; run it yourself to see it sooner
```

It refuses to run if `main` has moved, then runs what CI runs:

| Step | What fails it |
|------|---------------|
| `scripts/verify.sh` | gofmt, `go vet`, `go test -race`, spec lint, `changelog-check.py` (released sections changed), `spec-tasks-check.py` (open tasks in a touched spec) |
| `make lint` | golangci-lint at the pinned version (staticcheck, revive, gosec…) — same as CI |
| `scripts/build.sh` | the binary does not build |

It ends with the next release version, from `[Unreleased]`
(`Fixed` only → patch; `Added`/`Changed`/`Removed` → minor; none → no
release). Wrong number? Fix the CHANGELOG headings now.

Needs: Go from `go.mod`, `python3`, `git`; `jq` and `shellcheck` for the
hook scripts.

## 3. Run it on a cluster (UI or behaviour changes)

```bash
scripts/build.sh
mkdir -p /tmp/knv && cd /tmp/knv && <repo>/bin/kube-node-viewer --context <any cluster>
```

- Any cluster works (a local `kind` one too). Agents: only one the owner
  allows; never pick one yourself.
- From a scratch directory: `e` (export) writes node names and IPs to the
  working directory. Exports are git-ignored, and the commit guard blocks
  them, but don't leave them in the repo.
- Check what the spec's requirements describe, on a real screen: keys,
  narrow and wide terminals, `--disable-pricing`, an empty filter result.
- Record it in the verify task of `specs/<feature>/tasks.md` as a dated
  note of what was checked, never the cluster's name (this repo is public), and tick it `[x]`. Until every task is
  `[x]`, CI fails the PR and the `pr-guard` hook refuses `gh pr create`;
  use `gh pr create --draft` meanwhile.

## 4. Before committing

- No AI trailers; Conventional Commits (`feat(model): …`, `fix: …`).
- Nothing from a cluster staged (`git status`); no personal files (local
  agent state, PR drafts). Put personal ignores in `.git/info/exclude`,
  not in `.gitignore`.
- README keys/flags table and `-h` output updated when they changed.
- New files carry the Apache header (`hack/boilerplate.go` format).

## 5. Push and open the PR

```bash
git push -u origin <branch>
gh pr create --fill            # --draft while tasks are open
```

To review before it merges itself, add the `hold` label. After the merge:
skill `release` to confirm the version and `brew upgrade`.

## When CI fails

Run the failing step locally (`make check` runs them all), fix, push again.
`lint` failing locally too means your golangci-lint is not the pinned one:
use `make lint`, not a global binary.
