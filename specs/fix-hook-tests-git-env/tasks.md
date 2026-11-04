# Tasks: fix-hook-tests-git-env

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. Unset git's repository-local and identity variables, no global or system config, at the top of `test-checks.sh` (line 29
57)
  - _Covers: R1 · Depends on: — · Verified by: the scratch-clone run below_
- [ ] T2. The `guardrails` job runs the hook tests plainly and with `GIT_DIR`, `GIT_WORK_TREE` and `GIT_CONFIG_PARAMETERS` set, then diffs the checkout's config and refs
  - _Covers: R2 · Depends on: T1 · Verified by: actionlint, zizmor, the PR's `guardrails` job_
- [x] T3. Verify
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: dated note here_

> **Verified 2026-11-04 (after the review).** shellcheck, actionlint and
> zizmor clean. In a fresh scratch clone: plain run `111 passed`; run with
> `GIT_DIR` (absolute), `GIT_WORK_TREE`, `GIT_CONFIG_PARAMETERS
> 'commit.gpgsign=true'` and `GIT_AUTHOR_NAME` set, `111 passed` and
> the clone's local config, refs and HEAD identical before and after. T2
> waits for the PR's `guardrails` job.

> **Verified 2026-11-04 (first fix).** In a fresh scratch clone, with `GIT_DIR` set:
> before the fix `104 passed, 7 failed` and the clone damaged as in the
> bugfix; after it `111 passed, 0 failed`, `core.bare` false, no local
> `user.name`, no `v0.0.1` tag, HEAD unchanged. The repository damaged by
> the push that found it was repaired by hand (config entries removed,
> branch reset to its own commit, the stray tag deleted).
