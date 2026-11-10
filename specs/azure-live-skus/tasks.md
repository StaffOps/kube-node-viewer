# Tasks: azure-live-skus

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. `az` runner: `pkg/azure/azcli.go`, `azcli_unix.go` (`Setpgid`, group kill), `azcli_windows.go` — account list (other clouds ignored; `Disabled`/`Deleted` only for row 4), token, cache (two minutes before expiry; `expires_on` or `expiresOn`), stdin detached, stderr captured, warnings with exit 0 accepted, 30 s timeout, Unix process group killed and `WaitDelay` set; R5 rows 1–5 (row 2 from the az 2.88.0 wording); token dropped on 401/403 and on an account change; one in-flight request per subscription; R10 validation before any run
  - _Covers: R4, R5, R10 · Depends on: — · Verified by: `go test -race ./pkg/azure -run 'AzCLI|Token|Validate'` with a fake `az` on `PATH`; `GOOS=windows go vet ./pkg/azure`; `GOOS=windows go build ./cmd/...`_
- [x] T2. SKU client and parse: `pkg/azure/skus.go` — `url.Values` query, host and `nextLink` checks, paging, 64 MiB cap, 60 s timeout, `armError` (status and code only), redirects refused, default base `https://management.azure.com`, `virtualMachines` only, R2 fields, restrictions and zones, `MaxResourceVolumeMB`; `NormalizeFamily` moved here; the recorded answer `pkg/azure/testdata/skus-eastus.json`, each size's numbers checked against the public size pages (date recorded here)
  - _Covers: R1, R2, R3, R5, R9, R10 · Depends on: — · Verified by: `go test ./pkg/azure -run SKU` against an `httptest` TLS server_
- [x] T3. Advisor and rules: pair states, `Begin(pairs)` with a per-analysis snapshot, pairs only from comparable Azure nodes (row 14), `Available`, notes (loading, R5 lines, agent pools when one subscription of two fails), SKU generation on every state change and due retry; `recommend.Node.Subscription`, `Input.Available` (nil = available; AWS gives none; `AdvisorsByCloud.Available` via `forNode`), availability per rule, Spot without sizes while the pair is loading or failed, user-pool minimum, temp-disk check, `sizes:` math line; `pkg/model` fills the subscription, gathers pairs, calls `Begin` before `Types()`; session wiring (none with pricing disabled) and the injection option (D6)
  - _Covers: R1, R3, R5, R6, R7, R8, R9, R11 · Depends on: T1, T2 · Verified by: advisor, recommend, model and session tests under `-race`; AWS golden screens unchanged; AKS golden screens changed only by the `sizes:` line_
- [x] T4. Removal, test data and docs: remove the copied table, `catalog.go`, `catalog_test.go` (R2 citations move to `skus_test.go`), `make azure-catalog`; add `pkg/recommend/testdata/azure-sizes.tsv` (frozen extract of `03e49d9cb822`, credit line) and move the coverage tests onto it; AKS fixtures on the recorded answer; aks-recommendations R16 marked `Test: withdrawn` with a dated note, its Deferred refresh row and the TODO item removed; AGENTS.md `pkg/azure/` row; README (source, Reader, R5 lines, Troubleshooting); the `[Unreleased]` AKS CHANGELOG entry edited (or a `### Changed` entry if v0.16.0 released first)
  - _Covers: R12 · Depends on: T3 · Verified by: `scripts/verify.sh` (spec-trace, drift-check, headers), fixture totals, coverage tests on the extract, and a check that the `.tsv` header has the copyright line, the Apache-2.0 notice and the statement of change (headers.py skips `.tsv`)_
- [x] T6. Kind fixture: in `kind_integration_test.go`, Nodes created through the API with `azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/MC_example-rg_example_eastus/…` provider IDs and AKS labels (agent pool, mode, region, zone, instance type), the fake `az` and fake Resource Manager injected through D6's option; the session's Recommendations analysis shows the rows that need sizes after the fake answer, the R5 line with a failing fake, and recovery
  - _Covers: R1, R5, R6 · Depends on: T3 · Verified by: `go test -tags integration ./cmd/kube-node-viewer -run TestKindCluster` on a throwaway kind cluster (`scripts/kind-it.sh`)_
- [x] T5. Verify (after T6): `scripts/verify.sh`, `make lint` and `make licenses` green (no new module); `TestKindCluster` with made-up `azure://` nodes and the injected fake `az` and Resource Manager (each R5 line, recovery after the fake sign-in, rows appearing); Windows `az.cmd` checked by hand if a Windows machine is available, else recorded as not run (_Test: manual_); a real subscription only if the owner names one (answer size, `MaxResourceVolumeMB` 0 meaning), else recorded as not run; note what was checked, never the cluster's or subscription's name
  - _Covers: all · Depends on: T1–T4, T6 · Verified by: command output and the run, recorded here as a dated note_

> **Done 2026-11-10.**
> - T2: the recorded answer's sizes were checked against the public Azure
>   size pages (learn.microsoft.com, read 2026-11-10): vCPU, memory and temp
>   disk of each. `MaxResourceVolumeMB` 0 for sizes without a temp disk is
>   assumed, not seen in a real answer.
> - T5: `scripts/verify.sh` and `make lint` green; `make licenses` passes
>   with no new module; `go test -race ./pkg/... ./cmd/...` green;
>   `GOOS=windows go vet ./pkg/azure` and `GOOS=windows go build ./cmd/...`
>   pass. `scripts/kind-it.sh` passed on a throwaway kind cluster (deleted
>   after): `TestKindCluster` unchanged, and `TestKindClusterAzureSizes`
>   (two made-up AKS Nodes created through the API, zero-GUID subscription,
>   the fake `az` and Resource Manager through the Go option) showed the
>   permission line for a `403`, the request carried the node's subscription,
>   `location eq 'eastus'` and the fake token, no screen showed the
>   subscription, and a new session read the sizes with one request, after
>   which the note gave way to "waiting for the prices of 2 Azure sizes" and
>   a consolidation row for the made-up pool.
> - Not run: a real Azure subscription (the owner named none), so the
>   answer size and the `MaxResourceVolumeMB` 0 reading are unmeasured;
>   Windows `az.cmd` (no Windows machine here).

> **Corrected 2026-11-10 (diff review: 3 HIGH, 6 MEDIUM, 11 LOW, applied).**
> - Two regions of one subscription shared the token request's error value
>   and wrote it from two goroutines (a data race); each read now copies it,
>   with a `-race` test.
> - A test drives a UI model with the real Azure advisor on a static
>   cluster: no `az` run before Recommendations, the loading note, then a
>   rightsize row; none while another screen shows.
> - `Sizes` takes the session's context; a switch or `Close` kills a running
>   `az`. A test proves the timeout kills the process group (the fake's
>   `sleep` child is gone within a second) and that the command has no stdin.
> - Tests added for: the 429 `Retry-After`, a 401 or 403 dropping a token
>   still valid by the test clock, a changed account after a 404, logs and
>   stderr leaking nothing, plain `az` arguments, GUID anchors, the `az login`
>   match, fault-domain zones, a lower-cased `EastUS` label, the user-pool
>   minimum on swaps, `-` sizes without `vCPUsAvailable`.
> - T6 now recovers in the same session through the one-minute retry and
>   checks the arm row: `scripts/kind-it.sh` passed again (cluster deleted),
>   `TestKindClusterAzureSizes` showing the permission line, then idle,
>   consolidation and `add a Standard_D8ps_v5 agent pool` rows after the
>   retry, with one good answer and no subscription on screen.

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
