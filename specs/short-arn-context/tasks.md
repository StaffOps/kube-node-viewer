# Tasks: short-arn-context

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. `shortContext` in `pkg/model/header.go` and its use for the three headers; tests `TestShortContextName`, `TestHeadersShowTheShortContextName`, `TestShortNameOnlyInTheHeaders` in `pkg/model/header_internal_test.go`
  - _Covers: R1, R2, R3 · Depends on: — · Verified by: `go test ./pkg/model/`, the golden screens unchanged_
- [ ] T2. Docs: README header paragraph; CHANGELOG `[Unreleased]` `Changed`; dated notes in compact-header-no-url (counter-example and out-of-scope bullet) and under interactive-tui R16; `TODO.md`: remove the ARN item, add GKE context names
  - _Covers: R1, R3 · Depends on: T1 · Verified by: review_
- [ ] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster whose context is renamed to a made-up ARN (account 111122223333): full panel, `ctrl+e`, about 12 rows, and `c`; note what was checked, never the cluster's name
  - _Covers: R1, R2, R3 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).
