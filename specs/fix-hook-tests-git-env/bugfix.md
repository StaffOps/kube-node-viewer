# Bugfix: hook-tests-git-env

> Status: shipped (v0.14.0, 2026-11-05) · Reported: 2026-11-04 · Reviewed: spec-refuter 2026-11-04 (after the first fix; findings applied)

> **Corrected 2026-11-05.** Marked shipped by hand: this fix shipped in
> v0.14.0, when `scripts/release-prepare.py` did not ship bugfix specs
> ([fix-spec-tooling](../fix-spec-tooling/bugfix.md)).

## Current behavior

`git push` from a git worktree runs `.githooks/pre-push` → `make check` →
`scripts/checks/test-checks.sh` with `GIT_DIR` set to the worktree's git
directory. The script builds its throwaway repositories with `git init`,
`git config`, `git commit` and `git tag`, and every one of them acts on the
repository being pushed instead:

- the repository's `.git/config` gets `user.name`/`user.email`
  (`Example Dev`), `commit.gpgsign false` and, from the
  `git init --bare` of the push cases, `core.bare = true`, after which git
  refuses to run in the main checkout ("this operation must be run in a
  work tree");
- the pushed branch gets a `chore: base` commit with the test files, and a
  local tag `v0.0.1` appears;
- 7 cases fail (the deny-list and `--commits` ones), so the push is
  blocked.

Reproduced on 2026-11-04 in a scratch clone:
`GIT_DIR=$(git rev-parse --absolute-git-dir) dash scripts/checks/test-checks.sh`
→ `104 passed, 7 failed`, then `git config --local user.name` is
`Example Dev`, `git tag -l v0.0.1` lists the tag, and `HEAD` is
`chore: base`. Without `GIT_DIR` all 111 cases pass and nothing changes, so
CI (which runs the script from a plain shell) never saw it.

## Expected behavior

**R1** WHEN `scripts/checks/test-checks.sh` runs with any variable that
`git rev-parse --local-env-vars` lists (`GIT_DIR`, `GIT_WORK_TREE`,
`GIT_COMMON_DIR`, `GIT_INDEX_FILE`, `GIT_CONFIG`, `GIT_CONFIG_PARAMETERS`
from `git -c`…), or the author and committer identity variables, set
THEN the system SHALL ignore them, read no global or system git config, and
leave the calling repository's local config, refs, tags and HEAD
unchanged.

_Test: ci — the `guardrails` job of R2 is the check: it runs the
hook tests with these variables set and compares the checkout._

**R2** WHEN CI runs the hook tests THEN it SHALL run them once plainly and
once with `GIT_DIR` (absolute), `GIT_WORK_TREE` and `GIT_CONFIG_PARAMETERS`
set, as the pre-push hook of a worktree under `git -c` does, and fail if
the checkout's `git config --local --list`, `git for-each-ref` or HEAD
differ afterwards.

_Test: ci — the `guardrails` job in
`.github/workflows/test.yaml`, seen failing and passing on its PR._

## Unchanged behavior

- All 111 cases pass, with and without `GIT_DIR`.
- `make check` and the pre-push hook keep the same steps and messages.

## Root cause

`scripts/checks/test-checks.sh` (`git init` at the top, `git config` and
`git commit` right after, and the push cases' `git init --bare` and
`git tag`) changes directory into its throwaway repository but keeps the
environment it was started with; git gives `GIT_DIR` precedence over the
current directory, and git sets it for hooks. From a worktree `GIT_DIR`
is absolute (`.git/worktrees/<name>`), so it still names the pushed
repository after the `cd`. Pushes from the main checkout were not seen to
do this; the likely reason, not tested, is a relative `GIT_DIR=.git` that
resolves inside the throwaway repository after the `cd`.
