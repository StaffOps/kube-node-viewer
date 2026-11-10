# Tasks: azure-live-skus

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. Token: `pkg/azure/token.go`, `az account get-access-token` with a 30 s timeout, cache until two minutes before expiry, error classes from exit status and stderr; fake `az` tests for each R5 `az` row
  - _Covers: R4, R5 · Depends on: — · Verified by: `go test ./pkg/azure -run Token`_
- [ ] T2. SKU client and parse: `pkg/azure/skus.go` (request, `nextLink`, 64 MiB cap, `virtualMachines` only, R2 fields, restrictions), `NormalizeFamily` moved here; the recorded answer `pkg/azure/testdata/skus-eastus.json`, each size's numbers checked against the public size pages (date recorded here)
  - _Covers: R1, R2, R3, R5 · Depends on: — · Verified by: `go test ./pkg/azure -run SKU`_
- [ ] T3. Advisor: pairs from `Begin`, lazy fetch, union catalog, restrictions in `OnDemand`/`SpotIn`, notes (loading, R5 lines), `Generation` on arrival, retry once a minute while shown; `recommend.Node.Subscription` and its fill in `pkg/model`; math source line; session wiring, none with pricing disabled
  - _Covers: R1, R3, R5, R6, R7, R8 · Depends on: T1, T2 · Verified by: advisor, model and session tests; AWS golden screens unchanged_
- [ ] T4. Removal and docs: copied table, `catalog.go`, `make azure-catalog`, README credit; aks-recommendations fixtures and coverage tests on the recorded answer; README (source, permission, error lines), CHANGELOG `### Changed`, TODO item and the aks-recommendations Deferred row removed, dated notes there
  - _Covers: R9 · Depends on: T3 · Verified by: `scripts/verify.sh` (spec-trace, drift-check), fixture totals_
- [ ] T5. Verify: `scripts/verify.sh` and `make lint` green, `make licenses` (no new module); run on kind with made-up `azure://` nodes, a fake `az` and a fake Resource Manager behind a test build tag (each R5 line, recovery after the fake sign-in, rows appearing); a real subscription only if the owner names one, else recorded as not run; note what was checked, never the cluster's or subscription's name
  - _Covers: all · Depends on: T1–T4 · Verified by: command output and the run, recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Managed identity, workload identity, service-principal env vars (`azidentity` once its licences fit, or IMDS directly) | an admin needs it where `az` can't run |
| Live AWS catalog (`DescribeInstanceTypes`) | owner asks |
| Sovereign clouds | a user on one |
