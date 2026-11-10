# Tasks: hide-cloud-ids

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. The two context patterns in `shortContext`; `displayServer` for the `Cluster:` row; `displayProviderID`, used by the Provider ID row and `InstanceID`; tests of the design's test plan
  - _Covers: R1, R2, R3, R4, R5, R6, R7, R8, R9, R10 · Depends on: — · Verified by: `go test ./pkg/model/ ./pkg/client/`, the golden screens unchanged_
- [x] T2. Docs: README header paragraph, export section and node details; CHANGELOG `[Unreleased]` `Changed`; a `> **Corrected YYYY-MM-DD.**` note in short-gke-context requirements.md (R2, R3, R4 and Out of scope); `TODO.md`: remove the context item, reword the ids item to labels and node names, add `edgecontainer_`, a content rule for real `gce://` ids and resource groups, and the recursive read lock on `Node.Name`
  - _Covers: R1, R2, R4, R5, R6, R10 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh` and `make lint` green; run on a local kind cluster with contexts renamed to made-up forms and fake Nodes carrying made-up `gce://` and `azure://` ids: three headers, `c`, `e`, the Provider ID row; note what was checked, that no real cluster of those kinds was used, and that R8 and R9 are unit-tested only (the fake Nodes have names)
  - _Covers: R1, R2, R3, R4, R5, R6, R7, R10 · Depends on: T1, T2 · Verified by: command output and the cluster run, recorded here as a dated note_

  > **Verified 2026-11-09.** No real GKE multi-cloud, attached, Connect
  > gateway, GCE or Azure cluster was used: the names and ids are made up, the
  > context formats come from the gcloud source (design D1). `go test
  > ./pkg/model/ ./pkg/client/` green; the golden screens unchanged (no file in
  > `pkg/model/testdata/screens/` changed); `scripts/verify.sh` green, `make
  > lint` 0 issues. The code review ran 75 mutants on a copy; the 7 that
  > survived (R10's version and host rules, white space in the GCE project and
  > the Azure parts, the Azure anchor) now fail with the rows added after it.
  > On a local one-node kind cluster (own kubeconfig, state and config, deleted
  > afterwards), driven through a pty with pricing off, with the context renamed
  > in turn to a `connectgateway_` name, one with `_ns-team-a`, a `gke_aws_` and
  > a `gke_attached_` name, and a malformed `connectgateway_` name: the header
  > shows the short name and no project before `c`; `c` lists the full name; `e`
  > writes `kube-node-viewer-<short>-…` with no project in the file names or the
  > notice, and the JSON `context` is the full name; the malformed name shows
  > whole. Three fake Nodes with made-up ids: details show `gce://…/<zone>/<name>`
  > and `azure:///…/virtualMachineScaleSets/…/virtualMachines/3` without the
  > project, subscription or resource group, and a `custom://` id unchanged.
  > Not covered by the run: R10's `Cluster:` row (the kind server is a local
  > URL) and R8 and R9 (the fake Nodes have names); those are unit-tested only.
