# Tasks: compact-header-no-url

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. Compact box without the server URL in `pkg/model/header.go`; tests `TestCompactHeaderHasNoServerURL`, `TestFullHeaderKeepsServerURL`; golden scene `header-compact`
  - _Covers: R1, R2 · Depends on: — · Verified by: `go test ./pkg/model/`, the nine existing golden screens unchanged_
- [ ] T2. Docs: README sentence above the compact image, `.static/header-compact.png` regenerated, CHANGELOG `[Unreleased]` `Changed`, dated note under interactive-tui R16
  - _Covers: R1 · Depends on: T1 · Verified by: review; only `header-compact.png` changes in `.static/`_
- [ ] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster: `ctrl+e` both ways, about 20 rows (automatic compact), 12 rows (one line); note what was checked, never the cluster's name
  - _Covers: R1, R2 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).
