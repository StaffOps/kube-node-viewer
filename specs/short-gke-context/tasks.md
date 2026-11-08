# Tasks: short-gke-context

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. The GKE pattern in `shortContext`, `exportBase` using it; tests of the design's test plan
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: `go test ./pkg/model/ ./pkg/client/`, the golden screens unchanged_
- [ ] T2. Docs: README header paragraphs and export section; CHANGELOG `[Unreleased]` `Changed`; `> **Corrected YYYY-MM-DD.**` notes in short-arn-context requirements.md (R2's GKE example, R3, Out of scope), views-and-export requirements.md (file name rule) and under interactive-tui R16; `TODO.md`: remove both items
  - _Covers: R1, R3, R4 · Depends on: T1 · Verified by: review_
- [ ] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster whose context is renamed to a made-up GKE name and to a made-up ARN (account 111122223333): the three headers, `c`, and `e` (file name and notice); note what was checked, and that no real GKE cluster was used
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).
