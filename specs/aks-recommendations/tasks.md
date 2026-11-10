# Tasks: aks-recommendations

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

Before the first code commit:
- merge `origin/main` into this branch, which lacks `46cf2fe`;
- wait for [fix-azure-price-batch](../fix-azure-price-batch/bugfix.md) to
  merge, for T2.

The spec goes active in the PR that brings its tests. That PR updates the
status line and the `specs/README.md` index entry, and cites each R in the
tests, so spec-trace passes.

- [x] T1. Catalog:
  - `make azure-catalog` copies the cluster autoscaler's Azure table verbatim into `pkg/azure/zz_generated_instance_types.go`;
  - `pkg/azure/catalog.go`: lookups, family normalisation, `amd64` → `x86_64`, 0 vCPU or memory counts as missing, the constrained flag, the counterpart lookup;
  - record which recent families the copy lacks.
  - _Covers: R2, R16 · Depends on: — · Verified by: catalog tests (D64ps_v5 208 GiB, DS1_v2 3 GiB, B1ls missing)_

  > **Done 2026-11-10.** `pkg/azure/catalog.go` with `TestSizeCatalog` and
  > `TestNormalizeFamily`. The copy (upstream last changed 2026-03-17) holds
  > the AMD v7 families (`dasv7`, `dadsv7`, `dalsv7`, `daldsv7`, `easv7`,
  > `eadsv7`, `fasv7`…) and no Intel v7 family; none of them is a key or a
  > target of the D5 tables. The counterpart lookup lives in
  > `pkg/recommend` (it needs only the advisor's `Types` and `Spec`).
- [ ] T2. Prices:
  - `azure.Provider.ListPrice` through the fetch loop's split (the constant of [fix-azure-price-batch](../fix-azure-price-batch/bugfix.md), not redefined);
  - the generation counter;
  - a disabled mode that never queues.
  - _Covers: R3 · Depends on: fix-azure-price-batch merged · Verified by: pricing tests (candidate sizes beyond 17 → two requests; unknown size not asked again; disabled → no request)_
- [x] T3. `pkg/recommend`:
  - `Node` fields: `Cloud`, `Region`, `Windows`, `System`, `Capacity`, `VMPool`;
  - price functions take the node; `Rec.Cloud` and `Label()`; family per cloud;
  - `tables_azure.go`: the temp-disk set, the generation and Arm tables, the coverage rule;
  - the D4 estimate with the `≤ 0` guard; the R15 limits (no Arm or Spot on system pools); AKS wording, VMSS or VMs pool;
  - Azure-only rejected-candidate math lines;
  - the virtual node excluded from catalog advice and old node;
  - `fixture_test.go` signatures; `aks_fixture_test.go` with the full fixture and the made-up virtual node.
  - _Covers: R4, R5, R6, R7, R8, R9, R10, R11, R12, R13, R14, R15 · Depends on: T1 · Verified by: the AKS fixture ($322.66), the mixed fixture ($565.60), the AWS fixture unchanged ($242.94), per-rule tests, the table coverage and temp-disk tests_

  > **Done 2026-11-10.** `aks_fixture_test.go` (the usage example: rows,
  > order, `arm` label, $322.66; R4–R15 cases), `tables_azure_ext_test.go`
  > (tables against the copied catalog, Arm coverage, B1ls never a
  > candidate), and the AWS fixture unchanged ($242.94). The model and
  > session advisors take the new price signatures in the same commit, so
  > every commit builds; their Azure wiring is T4 and T5.
- [x] T4. `pkg/model`:
  - advisor per cloud; the gate (GCP message, empty cluster);
  - labels and `status.capacity` into `recommend.Node`;
  - `Label()` in the row, math title and popup;
  - generation in `recKey`; the waiting and pricing-disabled notes;
  - `screens_internal_test.go` fake advisor;
  - rewrite `aks_internal_test.go` `TestAKSRecommendationsSayEKSOnly` for R1;
  - golden: a new AKS Recommendations screen (`golden_test.go`); AWS golden files unchanged.
  - _Covers: R1, R3, R4 · Depends on: T2, T3 · Verified by: key-driven tests, golden screens_

  > **Done 2026-11-10**, ahead of T2: the model takes any advisor with
  > optional `Notes()` and `Generation()`, so T2's prices slot in.
  > `AdvisorsByCloud` routes by size name and node cloud. Tests:
  > `TestAKSRecommendationsAnalyseAzureNodes`,
  > `TestRecommendationsOtherCloudAndEmpty`,
  > `TestAzureNotesGenerationAndNoWait`, `TestRecommendNodeFromAKSLabels`,
  > and the `recommendations-aks` golden screen. The AWS golden files are
  > unchanged.
- [ ] T5. `cmd/kube-node-viewer/session.go`: `awsAdvisor` signatures; `azureAdvisor` from the `ByCloud` Azure provider, and over the catalog with pricing disabled when `--disable-pricing` (`session.go:205-206`)
  - _Covers: R1, R3 · Depends on: T2, T4 · Verified by: session tests with and without `--disable-pricing`_
- [ ] T6. `refresh-data.yaml` runs `make azure-catalog` with `make prices`
  - _Covers: R16 · Depends on: T1 · Verified by: a workflow_dispatch run's PR_
- [ ] T7. Docs:
  - README: Recommendations on AKS, and credit for the cluster autoscaler table;
  - CHANGELOG `### Added`;
  - aks-pricing R3 marked `_Test: withdrawn (R3) — <dated note>._` with a dated note pointing here;
  - recommendations dated notes (Azure out of scope, the services counter-example);
  - the Ideas item removed from TODO, and the deferred rows in aks-pricing and recommendations.
  - _Covers: — · Depends on: T1–T6 · Verified by: review, `scripts/spec-trace.py`_
- [ ] T8. Verify: `scripts/verify.sh` and `make lint` green; run on a cluster: note what was checked, never the cluster's name
  - _Covers: all · Depends on: T1–T7 · Verified by: command output and the cluster run (kind with made-up `azure://` nodes, priced through the public API), recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Arm v6 (Cobalt 100) targets (deferred by the owner, 2026-11-10) | AKS's Arm64 page names them, or the owner asks |
| Arm system pools | AKS documents Arm64 system pools |
| Non-temp-disk targets for nodes that don't use their temp disk | owner asks |
| Swaps between vendors (Intel `Dsv5` → AMD `Dasv5`), on AWS too (`m6i` → `m6a`) | owner asks |
| Generation and Arm targets for GPU, HPC, confidential, isolated, storage, M, `Av2`, `Fsv2`, v7 and B-series families | an owner cluster runs them |
| AKS's reservation formula instead of the D4 estimate | owners miss savings to the understatement |
| Catalog-only Azure advice with `--disable-pricing` (rank by size, not money) | owner asks |
| Fetch a family's prices by product name in one request (D3) | many families make many requests at screen open |
| Azure Reservations and Savings Plans | someone needs effective rather than list cost |
