# Tasks: aks-recommendations

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. `make azure-catalog` copies the cluster autoscaler's Azure table into `pkg/azure/zz_generated_instance_types.go` (package line, modification note); `pkg/azure/catalog.go`: lookups, family normalisation, constrained flag, counterpart; note which recent families the copy lacks
  - _Covers: R2, R16 · Depends on: — · Verified by: catalog tests (R2 samples, D64ps_v5 208 GiB)_
- [ ] T2. `azure.Provider.ListPrice`; requests of at most 17 sizes, one after another; generation counter
  - _Covers: R3 · Depends on: — (the shipped `NodePrice` split may land first as its own bugfix) · Verified by: pricing tests (18 sizes → two requests)_
- [ ] T3. `pkg/recommend`: `Node` cloud/region/OS/system fields; price functions take the node; `Rec.Cloud` and `Label()`; family per cloud; `tables_azure.go` checked against the copied table; D4 estimate; R15 system limits; AKS wording; exclusions; `fixture_test.go` signatures; `aks_fixture_test.go` with the made-up virtual node
  - _Covers: R4, R5, R6, R7, R8, R9, R10, R11, R12, R13, R14, R15 · Depends on: T1 · Verified by: the AKS fixture ($445.30), the mixed fixture ($688.24), the AWS fixture unchanged ($242.94), per-rule tests_
- [ ] T4. `pkg/model`: advisor per cloud, the gate, labels into `recommend.Node`, `Label()` in the row, math title and popup, generation in `recKey`, the waiting note; `screens_internal_test.go` fake advisor; rewrite `aks_internal_test.go` `TestAKSRecommendationsSayEKSOnly` for R1 and add the GCP message test; golden AKS Recommendations screen, AWS golden files unchanged
  - _Covers: R1, R3, R4 · Depends on: T2, T3 · Verified by: key-driven tests, golden screens_
- [ ] T5. `cmd/kube-node-viewer/session.go`: `awsAdvisor` signatures, `azureAdvisor` from the `ByCloud` Azure provider
  - _Covers: R1, R3 · Depends on: T2, T4 · Verified by: session tests_
- [ ] T6. `refresh-data.yaml` runs `make azure-catalog` with `make prices`
  - _Covers: R16 · Depends on: T1 · Verified by: a workflow_dispatch run's PR_
- [ ] T7. Docs:
  - README: Recommendations on AKS, and credit for the cluster autoscaler table;
  - CHANGELOG `### Added`;
  - aks-pricing R3 marked `_Test: withdrawn (R3) — <dated note>._` with a dated note pointing here;
  - recommendations dated notes (Azure out of scope, the services counter-example);
  - the TODO item removed, and the deferred rows in aks-pricing and recommendations.
  - _Covers: — · Depends on: T1–T6 · Verified by: review, `scripts/spec-trace.py`_
- [ ] T8. Verify: `scripts/verify.sh` and `make lint` green; run on a cluster: note what was checked, never the cluster's name
  - _Covers: all · Depends on: T1–T7 · Verified by: command output and the cluster run (kind with made-up `azure://` nodes, priced through the public API), recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Arm v6 (Cobalt 100) targets | AKS's Arm64 page names them, or the owner accepts them first (D5) |
| Swaps between vendors (Intel `Dsv5` → AMD `Dasv5`), on AWS too (`m6i` → `m6a`) | owner asks |
| Generation and Arm targets for GPU, HPC, confidential, storage, M, `Av2` and `Fsv2` families | an owner cluster runs them |
| AKS's reservation formula instead of the D4 estimate | owners miss savings to the understatement |
| Fetch a family's prices by product name in one request (D3) | many families make many requests at screen open |
| Azure Reservations and Savings Plans | someone needs effective rather than list cost |
