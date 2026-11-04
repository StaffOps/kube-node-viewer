# Bugfix: hook-tests-git-env

> Status: draft · Reported: 2026-11-04 · Reviewed: —

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

**R1** WHEN `scripts/checks/test-checks.sh` runs with any of git's
repository variables set (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`,
`GIT_COMMON_DIR`, `GIT_OBJECT_DIRECTORY`…) THEN the system SHALL ignore them
and leave the calling repository's config, refs, tags, index and HEAD
unchanged.

**R2** WHEN CI runs the hook tests THEN it SHALL run them with `GIT_DIR`
set, as the pre-push hook does, and fail if the checkout's `core.bare`,
local `user.name`, tags or HEAD changed.

## Unchanged behavior

- All 111 cases pass, with and without `GIT_DIR`.
- `make check` and the pre-push hook keep the same steps and messages.

## Root cause

`scripts/checks/test-checks.sh` changes directory into its throwaway
repository but keeps the environment it was started with; git gives
`GIT_DIR` precedence over the current directory, and git sets it for hooks.
