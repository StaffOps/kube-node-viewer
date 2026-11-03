# Tasks: node-pods

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. Keep the controller name in `slimOwners`; `Pod.ownerName`, `Owner()`
  - _Covers: R6 · Depends on: — · Verified by: `go test ./pkg/client ./pkg/model -run 'Slim|Owner'`_
- [x] T2. `nodepods.go`: rows, percents, share, cost, no-requests, DaemonSet group and rows
  - _Covers: R2, R3, R4, R5, R6 · Depends on: T1 · Verified by: `go test ./pkg/model -run NodePods`_
- [x] T3. Sort: cost, CPU, memory, name; `s` cycles, `S` flips; ties, no-requests last
  - _Covers: R7 · Depends on: T2 · Verified by: `go test ./pkg/model -run NodePodsSort`_
- [x] T4. Popup: `fixed` lines in `popup.go`; `tab` between Details and Pods, tab line, column header, cursor with fallback, `enter`/`space` on the group, reset on open, `helpText` per tab, help entries, narrow widths, node gone
  - _Covers: R1, R2, R4, R8 · Depends on: T2 · Verified by: `go test ./pkg/model -run 'NodeDetails|NodePodsKeys|Popup'`_
- [x] T5. Docs: README (Node details section, keys), CHANGELOG `[Unreleased]`
  - _Covers: — · Depends on: T3, T4 · Verified by: review_
- [x] T6. Verify: `scripts/verify.sh`, `make lint`, coverage ≥ 90% on new code
  - _Covers: R1, R2 · Depends on: T5 · Verified by: command output_
- [x] T7. Measure heap with owner references in the bench pods and the slim pods kept alive, before and after T1, record both in D3; manual run on a cluster: note what was checked, never the cluster's name
  - _Covers: D3 (and the manual run of R1–R8) · Depends on: T6 · Verified by: benchmark output and owner check_

## Notes

- 2026-11-02: T1 and T3 pass their commands. T2 and T4 stay `[-]`: every
  test passes but `TestNodePodsNarrowWindows/width=40` (part of `-run
  NodePods`). Below a 52-column window SHARE and COST/H are cut, because
  the 12-character name floor plus the kept columns need 46 of the
  `width-6` box. R2 does not say what goes below 52 columns; it needs a
  requirement change or a decision, then the fix.
- 2026-11-02: T5, README and CHANGELOG written; independent review pending.
- 2026-11-02: T6, `scripts/verify.sh` fails on that test alone; lint
  reports 0 issues; coverage of new code is above 90% (`nodepods.go`
  98.8%, `nodedetails.go` 98.4%, `popup.go` 98.1%, `pod.go` 91.8%).
- 2026-11-02: T7, the heap measurement is done and recorded in D3
  (+2.6 MiB at 150,000 pods). The manual run on a cluster
  is pending.
- 2026-11-02 (review loop 1): R2 resolved with option (a), recorded as a
  dated note in R2 and the design: below 52 columns CPU and MEMORY are
  dropped too (`podsTable.values`), so POD, SHARE and COST/H stay. The
  40-column case now passes unchanged; T2, T4 and T6 are done
  (`scripts/verify.sh` ok, lint 0 issues, `newPodsTable` 92.9%). Also from
  the review: CPU and memory sorts put rows without that request last in
  both directions (R7 note); `pgup`/`pgdown` move ten rows (R8 note); the
  README memory figure separates the model (~155 MiB) from the heap with
  slim pods (~680 MiB); help label `pgup/pgdown`. T5 waits for re-review.

- [x] T8. 2026-11-03 owner change: all pods listed with `ds` marks (R6), `d` toggles a DaemonSets view with a Total line (R9), keys without enter/space (R8), DaemonSets share line on the node list (R10); docs
  - _Covers: R6, R8, R9, R10 · Depends on: T4 · Verified by: `go test ./pkg/model -run 'NodePods|DaemonSetLine'`, `scripts/verify.sh`, lint_

- 2026-11-03: T8, spec-check findings applied as dated notes in requirements
  and design (D5 added). Implemented: all-pods view with `ds` marks, `d`
  DaemonSets view with the Total line, no group, `enter`/`space` do nothing,
  the R10 footer line in a reserved slot (an open panel takes it back).
  `scripts/verify.sh` ok, lint 0 issues; `nodepods.go` functions 88.9-100%.
  No new tests yet (test-author stage); existing tests adapted. Pending:
  independent tests, review, manual run.
- 2026-11-03: T8 docs, README (Pods tab, DaemonSets view, node list line,
  keys) and the CHANGELOG `[Unreleased]` Pods tab entry updated. T8 stays
  `[-]`: `scripts/verify.sh` fails on `TestAllPodsViewDsMarkOnANarrowWindow`
  (R6, ` ds` cut with the name) and `TestDaemonSetLineFrameHeights/8` (R10,
  9-line frame on an 8-line window); lint 0 issues. The README already
  describes the fixed behavior (name cut first, ` ds` kept).

- [x] T9. 2026-11-03 owner change: DaemonSets section in the Details tab; CPU in millicores in Details
  - _Covers: R11 · Depends on: T8 · Verified by: `go test ./pkg/model -run DetailsTab` (fail without the change), `scripts/verify.sh`, lint_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Cluster-wide DaemonSet cost in Recommendations | The owner wants the total DaemonSet overhead of the cluster |
| Group pods by Deployment, StatefulSet, Job | The owner asks for other owners than DaemonSets |
| GPUs and extended resources in the share | GPU nodes in the owner's clusters |
| Search inside the Pods tab | Nodes with more pods than fit in the popup |

> **Note 2026-11-03.** T8 verified: rebased on `origin/main` (`aaf0bab`), `scripts/verify.sh` green (content, changelog and spec-tasks checks included), golangci-lint v2.14.0 0 issues, `pkg/model` coverage 94.0%. `TestMonthlyColumnsOnScreen` (from main) read the new DaemonSets line as a node row; it now skips it. The DaemonSets line also shows absolute values (owner, 2026-11-03). T7's cluster run is still open.

> **Verified 2026-11-03.** Ran the binary (`--resources cpu,memory`) on a
> local kind cluster (3 nodes, Kubernetes v1.37) with a Deployment of three
> 500m / 1Gi pods, a pod with no requests, a DaemonSet requesting 200m /
> 256Mi next to the kind ones, and a node priced with the override label.
> Checked: the node-list line (`DaemonSets: 2% of the node (cpu 300m 2% · mem
> 306Mi 1%)`); Details with the monthly price, CPU in millicores, capacity
> only where it differs, the Pods section (`7 (6%) / 110`) and the DaemonSets
> section (3 pods, 3 sets, 300m, 306Mi); the Pods tab with `ds` marks and
> `no requests` last; `d` with the Total line; `s` to CPU and `S` reversing
> it. Node requests (1800m, 15%) and every pod's requests match `kubectl
> describe node`; memory shows 11% where kubectl truncates to 10%.
