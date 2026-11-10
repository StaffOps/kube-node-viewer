# Tasks: azure-live-skus

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [ ] T1. `az` runner: `pkg/azure/azcli.go` — account list, token, cache (two minutes before expiry; `expires_on` or `expiresOn`), stdin detached, stderr captured, warnings with exit 0 accepted, 30 s timeout, Unix process group killed and `WaitDelay` set; R5 rows 1–5; R10 validation before any run
  - _Covers: R4, R5, R10 · Depends on: — · Verified by: `go test ./pkg/azure -run 'AzCLI|Token|Validate'` with a fake `az` on `PATH`_
- [ ] T2. SKU client and parse: `pkg/azure/skus.go` — `url.Values` query, host and `nextLink` checks, paging, 64 MiB cap, 60 s timeout, `armError` (status and code only), `virtualMachines` only, R2 fields, restrictions and zones, `MaxResourceVolumeMB`; `NormalizeFamily` moved here; the recorded answer `pkg/azure/testdata/skus-eastus.json`, each size's numbers checked against the public size pages (date recorded here)
  - _Covers: R1, R2, R3, R5, R9, R10 · Depends on: — · Verified by: `go test ./pkg/azure -run SKU` against an `httptest` TLS server_
- [ ] T3. Advisor and rules: pair states, `Begin(pairs)`, `Available`, notes (loading, R5 lines, agent pools when one subscription of two fails), SKU generation on every state change and due retry; `recommend.Node.Subscription`, `Input.Available`, availability per rule, user-pool minimum, temp-disk check, `sizes:` math line; `pkg/model` fills the subscription, gathers pairs, calls `Begin` before `Types()`; session wiring (none with pricing disabled) and the injection option for `TestKindCluster`
  - _Covers: R1, R3, R5, R6, R7, R8, R9, R11 · Depends on: T1, T2 · Verified by: advisor, recommend, model and session tests; AWS golden screens unchanged; AKS golden screens changed only by the `sizes:` line_
- [ ] T4. Removal, test data and docs: remove the copied table, `catalog.go`, `catalog_test.go` (R2 citations move to `skus_test.go`), `make azure-catalog`; add `pkg/recommend/testdata/azure-sizes.tsv` (frozen extract of `03e49d9cb822`, credit line) and move the coverage tests onto it; AKS fixtures on the recorded answer; aks-recommendations R16 marked `Test: withdrawn` with a dated note, its Deferred refresh row and the TODO item removed; AGENTS.md `pkg/azure/` row; README (source, Reader, R5 lines, Troubleshooting); the `[Unreleased]` AKS CHANGELOG entry edited
  - _Covers: R12 · Depends on: T3 · Verified by: `scripts/verify.sh` (spec-trace, drift-check, headers), fixture totals, coverage tests on the extract_
- [ ] T5. Verify: `scripts/verify.sh`, `make lint` and `make licenses` green (no new module); `TestKindCluster` with made-up `azure://` nodes and the injected fake `az` and Resource Manager (each R5 line, recovery after the fake sign-in, rows appearing); Windows `az.cmd` checked by hand if a Windows machine is available, else recorded as not run (_Test: manual_); a real subscription only if the owner names one (answer size, `MaxResourceVolumeMB` 0 meaning), else recorded as not run; note what was checked, never the cluster's or subscription's name
  - _Covers: all · Depends on: T1–T4 · Verified by: command output and the run, recorded here as a dated note_

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Managed identity, workload identity, service-principal env vars (`azcore` with a token function, or IMDS directly) | an admin needs it where `az` can't run |
| Live AWS catalog (`DescribeInstanceTypes`) | owner asks |
| Sovereign clouds (`az cloud set`) | a user on one |
| Spot filtered by `LowPriorityCapable` | a Spot row for a size that can't run Spot is reported |
