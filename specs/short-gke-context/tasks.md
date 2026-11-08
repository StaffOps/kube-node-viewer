# Tasks: short-gke-context

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. The GKE pattern in `shortContext`, `exportBase` using it; the `gke` rule of `content.sh` widened, with cases; tests of the design's test plan
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: `go test ./pkg/model/ ./pkg/client/`, the golden screens unchanged_
- [x] T2. Docs: README header paragraphs and export section; CHANGELOG `[Unreleased]` `Changed`, naming the changed file names and not implying an export free of ids (the JSON keeps the full context); `> **Corrected YYYY-MM-DD.**` notes in short-arn-context requirements.md (R2's GKE example, R3, Out of scope), views-and-export requirements.md (file name rule) and under interactive-tui R16; `TODO.md`: remove both items, add `connectgateway_` and multicloud names and ids elsewhere on screen
  - _Covers: R1, R3, R4 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster whose context is renamed to a made-up GKE name and to a made-up ARN (account 111122223333): the three headers, `c`, and `e` (file name and notice); note what was checked, and that no real GKE cluster was used
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

  > **Verified 2026-11-08.** No real GKE cluster was used: the names are made
  > up and the formats come from Google's documentation and the gcloud
  > source (design D2). `go test ./pkg/model/ ./pkg/client/` green (8 runs,
  > to catch ordering flakes), golden screens unchanged. Mutation check on a
  > copy, each alone, `TestShortGKEContextName` failing for all: no `^`, no
  > `$`, project 5 or 31 characters, a project ending in `-`, no domain part,
  > domain without a dot, cluster 41 characters, a cluster starting with a
  > digit or ending in `-`, no zone suffix, no digits in the location, an
  > empty first location group; without `shortContext` in `exportBase`,
  > `TestExportBaseName` and `TestExportFileNameIsShort` fail. On a local
  > one-node kind cluster (own kubeconfig, state and config, deleted
  > afterwards), driven through a pty with pricing off, with contexts renamed
  > to `gke_example-project_us-central1-a_demo-prod` and
  > `arn:aws:eks:us-east-1:111122223333:cluster/demo-arn`: the full panel shows
  > the short name and `Region: -`; `ctrl+e` the short name; at 12 rows the
  > one-line header starts with it; the project or account id does not appear
  > before `c`; `c` lists the full name; `e` writes
  > `kube-node-viewer-demo-prod-…` and `kube-node-viewer-demo-arn-…` with the
  > notice naming them; the JSON `context` is the full name.

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).
