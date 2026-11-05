# Tasks: short-arn-context

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. `shortContext` in `pkg/model/header.go` and its use for the three headers; tests `TestShortContextName`, `TestHeadersShowTheShortContextName`, `TestShortNameOnlyInTheHeaders`, `TestARNRegionIsNotThePricingRegion` in `pkg/model/header_internal_test.go`; `TestClusterKeyKeepsTheFullContext` in `pkg/client/client_test.go`
  - _Covers: R1, R2, R3 · Depends on: — · Verified by: `go test ./pkg/model/`, the golden screens unchanged_
- [x] T2. Docs: README header paragraphs (full panel and compact box); CHANGELOG `[Unreleased]` `Changed`; dated notes in compact-header-no-url requirements.md (after the counter-example "SHALL NOT shorten the context name" and after the ARN out-of-scope bullet) and under interactive-tui R16 ("superseded for ARN contexts by short-arn-context"); `TODO.md`: remove the ARN item, add GKE context names and a shorter export file name
  - _Covers: R1, R3 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster whose context is renamed to a made-up ARN (account 111122223333): full panel, `ctrl+e`, about 12 rows, and `c`; note what was checked, never the cluster's name
  - _Covers: R1, R2, R3 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

  > **Verified 2026-11-05.** `go test ./pkg/model/ ./pkg/client/` green and
  > the golden screens unchanged; with `header.go:158` reading
  > `u.info.Context` again, `TestHeadersShowTheShortContextName` fails in
  > all three headers. `scripts/verify.sh` green apart from these open
  > tasks before ticking, `make lint` 0 issues. On a local one-node kind
  > cluster (own kubeconfig and state, deleted afterwards) whose context was
  > renamed to `arn:aws:eks:us-east-1:111122223333:cluster/demo-prod`,
  > driven through a pty with pricing off: at 40 rows the full panel shows
  > `Context: demo-prod` and `Region: -` (the ARN's region is not used);
  > `ctrl+e` gives `demo-prod · 1 of 1 shown · 9 pods`, and again restores
  > the panel; `c` lists the full ARN marked `(current)`; the account id
  > appears only in that popup; at 12 rows the one-line header starts with
  > `demo-prod │`.

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).
