# Tasks: fix-hook-tests-git-env

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. Unset git's repository variables at the top of `test-checks.sh`
  - _Covers: R1 · Depends on: — · Verified by: the scratch-clone run below_
- [x] T2. The `guardrails` job runs the hook tests with `GIT_DIR` set and checks the checkout afterwards
  - _Covers: R2 · Depends on: T1 · Verified by: actionlint, zizmor, the PR's `guardrails` job_
- [x] T3. Verify
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: dated note here_

> **Verified 2026-11-04.** In a fresh scratch clone, with `GIT_DIR` set:
> before the fix `104 passed, 7 failed` and the clone damaged as in the
> bugfix; after it `111 passed, 0 failed`, `core.bare` false, no local
> `user.name`, no `v0.0.1` tag, HEAD unchanged. The repository damaged by
> the push that found it was repaired by hand (config entries removed,
> branch reset to its own commit, the stray tag deleted).
