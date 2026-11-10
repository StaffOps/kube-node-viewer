# Tasks: short-edge-context

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. `edgeContainerContext` (with its offline suffix) in `shortContext`; `edgecontainer` in `content.sh`'s `gke` rule; tests of the design's test plan
  - _Covers: R1, R2, R3, R4, R5 · Depends on: — · Verified by: `go test ./pkg/model/ ./pkg/client/`, `scripts/checks/test-checks.sh`, the golden screens unchanged_
- [x] T2. Docs: README header paragraph and export section; CHANGELOG `[Unreleased]` `Changed`; a `> **Corrected YYYY-MM-DD.**` note in hide-cloud-ids requirements.md (Out of scope); `TODO.md`: remove the `edgecontainer_` item, add one for project numbers in gcloud context names
  - _Covers: R1, R2, R4 · Depends on: T1 · Verified by: review_

  > **Done 2026-11-10.** The README export section already names "an EKS ARN
  > or gcloud context" generically, so only the header paragraph changed.
- [x] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster with the context renamed to made-up `edgecontainer_` names, plain and offline: three headers, `c`, `e`; note what was checked, and that no real Distributed Cloud cluster was used
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

  > **Verified 2026-11-10.** No real Google Distributed Cloud cluster was used:
  > the names are made up and the format comes from the gcloud source (design
  > D1). `go test ./pkg/model/ ./pkg/client/` green, the golden screens
  > unchanged; `scripts/checks/test-checks.sh` 208 passed, 0 failed;
  > `scripts/verify.sh` green, `make lint` 0 issues. Mutation check of the
  > pattern, each alone: no `^`, no `$`, a capturing suffix, no suffix, `.+` or
  > `\S*` for the user id, `_offline` optional, a loose location, cluster or
  > project, case-insensitive, an optional prefix, the pattern left out of the
  > list: all fail the tests (the optional prefix after two rows added). The code
  > review's 43 mutants: the survivors were covered only through the shared
  > sub-patterns; rows for a 31-character project, a project ending in `-`, a
  > location without digits and a domain without a dot were added. On a local
  > one-node kind cluster (own kubeconfig, state and config, deleted
  > afterwards), driven through a pty with pricing off, with the context renamed
  > in turn to a plain `edgecontainer_` name, its offline form with `_` and `@`
  > in the user id, and a malformed one: the header shows the short name and no
  > project or user id before `c`; `c` lists the full name; `e` writes
  > `kube-node-viewer-<cluster>-…` with no project in the file names or the
  > notice, and the JSON `context` is the full name; the malformed name shows
  > whole.
