# Tasks: <feature>

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. <discrete change, < 1 day>
  - _Covers: R1 · Depends on: — · Verified by: <test name / command>_
- [ ] T2. <docs: README, CHANGELOG [Unreleased]>
  - _Covers: — · Depends on: T1 · Verified by: review_
- [ ] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a cluster: note what was checked, never the cluster's name
  - _Covers: all · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| <task> | <condition that makes it worth doing> |
