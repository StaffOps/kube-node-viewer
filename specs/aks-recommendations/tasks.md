# Tasks: aks-recommendations

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. `hack/azcatalog` generator and `pkg/azure/zz_generated_sizes.go`; `pkg/azure/catalog.go` lookups (by name, series, counterpart); `make azure-catalog`; NOTICE attribution
  - _Covers: R2, R15 · Depends on: owner's choice of the D1 source · Verified by: generator fixture tests, catalog tests_
- [ ] T2. `azure.Provider.ListPrice` (queued miss, ≤ 20 sizes per request, generation counter)
  - _Covers: R3 · Depends on: — · Verified by: pricing tests_
- [ ] T3. `pkg/recommend`: node cloud/region/OS/system fields, price functions take the node, family split per cloud, Azure tables (checked against the catalog), D4 allocatable, `arm` label, AKS wording, system-pool and Windows limits, exclusions
  - _Covers: R4, R5, R6, R7, R8, R9, R10, R11, R12, R13, R14 · Depends on: T1 · Verified by: the Azure fixture ($316.38), the mixed fixture, the AWS fixture unchanged ($242.94), per-rule tests_
- [ ] T4. `pkg/model`: advisor per cloud, the gate, labels into `recommend.Node`, generation in `recKey`, "waiting for N Azure prices" note; golden AKS Recommendations screen
  - _Covers: R1, R3, R4 · Depends on: T2, T3 · Verified by: key-driven tests, golden screens_
- [ ] T5. `cmd/kube-node-viewer/session.go`: `azureAdvisor` from the `ByCloud` Azure provider
  - _Covers: R1, R3 · Depends on: T2, T4 · Verified by: session tests_
- [ ] T6. `refresh-data.yaml` runs `make azure-catalog` with the price tables
  - _Covers: R15 · Depends on: T1 · Verified by: a workflow_dispatch run's PR_
- [ ] T7. Docs: README (Recommendations on AKS), CHANGELOG `### Added`, dated notes in aks-pricing R3 and recommendations (out of scope, counter-example on services), TODO item removed, the aks-pricing and recommendations deferred rows
  - _Covers: — · Depends on: T1–T6 · Verified by: review_
- [ ] T8. Verify: `scripts/verify.sh` and `make lint` green; run on a cluster: note what was checked, never the cluster's name
  - _Covers: all · Depends on: T1–T7 · Verified by: command output and the cluster run, recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Swaps between vendors (Intel `Dsv5` → AMD `Dasv5`), on AWS too (`m6i` → `m6a`) | owner asks |
| Generation and Arm targets for GPU, HPC, confidential, storage, M and `Fsv2` series | an owner cluster runs them |
| Fetch a series' prices by product name in one request (D3) | many series make many requests at screen open |
| Azure Reservations and Savings Plans | someone needs effective rather than list cost |
