# Tasks: clusters-screen

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

> **Revised 2026-11-18.** T1, T4, T5, T6, T7 and T9 take the cloud groups (D9),
> nodes first (D10) and the cloud scope (D11); T12 measures node-only members.
> After the third review: T1 clears errors on sync; T4 keeps `tab` as Overview ↔
> Recommendations; T6 takes D12, Fargate pods, auth stop and coalesced redraws;
> T7 the pod-figure gate and header sizes; T9 pending ticks; T12 adds CPU and
> the Fargate label check. After the fourth review: T5 de-duplicates by
> server and passes the scope; T6 stops watches per informer; T12 checks the
> Fargate scheduler name; T7 gates filters on usage labels. After the
> re-check: an exclusion lasts only while the scope excludes the learned cloud (T6).

- [x] T1. `Controller.Status()`: wrapped list/watch recording and clearing the last error (no URL), cleared on sync or on the first event after it, `Synced` (nodes) and `PodsSynced` under a mutex, `Changed()` once each syncs; `StartNodes`/`StartPods` split out of `Start` (D10)
  - _Covers: R2, R3, R7, R15 · Depends on: — · Verified by: controller tests with a fake clientset (synced, empty cluster, error, recovery, `PodsSynced` false until `StartPods`, `StartPods` twice = one watch, watch-list and `List` paths); Overview's `Start` unchanged; Fargate-only pod watch by `spec.schedulerName`; per-informer stop on 403, a repeated 401 or an exec failure (prefix pinned)_
- [x] T2. Split `sessions.Switch` into `connect(context)` and the UI part (`explicit`, `starts` stay in `Switch`); `Session.Status`; route `gce` to a provider that prices nothing
  - _Covers: R3, R4 · Depends on: T1 · Verified by: existing session tests unchanged; a `gce://` node with an AWS-like instance type is not priced; the default view still applies_
- [x] T3. `client.GuessCloud`, contexts with their server, `model.IsGKEContext`
  - _Covers: R8 · Depends on: — · Verified by: table test_
- [x] T4. `ScreenClusters`, `:clusters`/`:cl` with an optional cloud word and its completion, unknown cloud notice, a scope change calls `fleetSync`, `tab` from Clusters to Overview (Overview ↔ Recommendations unchanged), unknown screen opens Overview, header in three sizes
  - _Covers: R1, R14 · Depends on: — · Verified by: key-driven tests_
- [x] T5. `model.Fleet`, `FleetStore`, slots by context and generation, messages, `fleetSync` from `Init`/`Connect`/`setScreen`/the command line, de-duplicating by normalized server (D12) and passing the scope, `switchedMsg.fromClusters`, `WatchPods` on first unfold, Overview's slot borrowed, panels take the union of nodes
  - _Covers: R2, R5, R9, R14, R15 · Depends on: T2, T4 · Verified by: fake-fleet tests_
- [x] T6. Fleet in `cmd`: epoch, members with own cancel and no `sizes`, node watches only until `WatchPods`, 4 starting at a time, place freed on node sync/error/cancel, a member's cloud final on node sync (closed when not the scope's, kept under `-` when it fails first), learned clouds kept for the visit (a context is left out only while the scope excludes its learned cloud; `:clusters aws` → `:clusters azure` reconnects it), Fargate-only pod watch kept until leave, `WatchPods` before connect applied after, per-informer stop (D3), one redraw per interval for the fleet, no AWS config leaves members unpriced, price set per visit (AWS per region, one Azure), `Close`; `SetFleet` and the state adapter in `main.go`
  - _Covers: R2, R7, R9, R14, R15, R16, R17 · Depends on: T1, T2, T5 · Verified by: fake-connector tests_
- [x] T7. Layout: spike first (per-block title in `paginate`, D6), then `clustersBody`, cloud groups with a total line each and none across clouds, cloud buckets `mixed`/`other`/`-` with no total, headings with idle `…` until pods, every pod figure of R15 gated by `podsReady` (render, sort and usage-label filters, with `pods not loaded` on the section and its cloud total), `>=`, filtered marks, order, sections folded by default,  prefixed cursor keys, header
  - _Covers: R3, R4, R5, R10, R11, R12, R15 · Depends on: T5 · Verified by: unit tests and golden screens; Overview goldens unchanged_
- [x] T8. `clustersKey` (R13) and `enter` opening the context in Overview: pending guard, same-context case, notice on error, screen forced
  - _Covers: R6, R13 · Depends on: T5, T7 · Verified by: key-driven tests_
- [x] T9. Cluster picker (`c` on the screen) grouped by cloud, `space` on a cloud line, ticks pending until `enter`, `esc` drops them, `missing` and `same cluster as` marks, first-use picker, `fleet` list and `fleetCloud` scope in the state file
  - _Covers: R2, R8, R14, R17 · Depends on: T3, T5, T6 · Verified by: key-driven and state tests_
- [x] T10. README, help keys, CHANGELOG `[Unreleased]`
  - _Covers: — · Depends on: T4–T9 · Verified by: `drift-check.py`, review_
- [x] T11. Tests carry their `// clusters-screen R<n>` comments (spec-trace)
  - _Covers: all · Depends on: T1–T9 · Verified by: `scripts/spec-trace.py`_
- [x] T13. `clusters.pods` in `config.yaml` (R18): parse and check it, pass it to the fleet; `all` starts each member's pod watch as it connects (D10 corrected)
  - _Covers: R15, R18 · Depends on: T6 · Verified by: settings tests (values, errors), fleet tests (pods started on connect in `all`, not in `unfolded`)_
- [x] T14. Summary block per section (D13): requested/allocatable, percentage, bar per resource, cost on the first line, `…` before the pods sync, aligned across sections; README, CHANGELOG
  - _Covers: R3, R15 · Depends on: T7 · Verified by: model tests, golden screens_
- [-] T12. Verify: `scripts/verify.sh` and `make lint` green; run on two or more EKS clusters from a scratch directory: note what was checked and the memory and time of the chosen clusters' node-only watches, of unfolded sections' pod watches and of the price fetch, the CPU and redraw time with every member connected, and whether Fargate pods have `spec.schedulerName=fargate-scheduler`, never the clusters' names
  - _Covers: all · Depends on: T1–T11 · Verified by: command output and the cluster run, recorded here as a dated note_

> **2026-11-18, T12 in progress.** Checked: `scripts/verify.sh` green but
> for this task, `make lint` 0 issues, `go test -race` on every package, the
> golden screens (Overview's unchanged but help; four new Clusters scenes),
> `scripts/build.sh`. Three spec-refuter passes on the diff; their findings
> are fixed in the branch (row keys of ARN contexts, an error per informer,
> a leak-free stream wrapper, the status read with its snapshot, hosts out of
> errors). `scripts/kind-it.sh` fails on this machine with a TLS handshake
> timeout to the kind API server, on `main` too, so it says nothing of this
> branch. Not done: the run on two or more EKS clusters (memory, CPU, sync
> time, the Fargate scheduler name), which needs clusters the owner allows.
>
> **2026-11-18, cluster run on throwaway kind clusters.** The kind failure
> above was this machine: a VPN route covers the range of Docker's default networks, so the
> host could not reach the kind API servers. On a Docker network outside it
> (`KIND_EXPERIMENTAL_DOCKER_NETWORK`), `scripts/kind-it.sh` passes. Then the
> binary ran, from a scratch directory with its own HOME, on two kind
> clusters with the integration workload and a price label on every node.
> Checked: `:cl` opens the first-use picker with the context on screen ticked
> (R2, R8); after `enter` both sections show their nodes and cost, the member
> `idle …` (R3, R15); the choice is saved in the state file and comes back
> with no question (R8); unfolding the member adds exactly one cluster-scoped
> pod watch on its API server, 9 → 10 (R11, R15, D10); `enter` opens it in
> Overview and the other cluster's watches end (R6, R9); `tab` leaves the
> screen and closes the member's node and pod watches (R9); after `q`, no
> watch of the tool is left on either cluster (R9); a context whose API
> server does not answer shows `Get: dial tcp an address: …` in its heading,
> no address, the others `ok`, and the header counts it not connected (R7,
> R3, R4). Memory with both clusters connected and one unfolded: 42.8 MB RSS
> (kind clusters of 3 nodes and 19 pods each). kind nodes are `kind://`, so
> this run shows the `other` bucket, not a cloud total. Still open: the same
> run on EKS clusters (cloud totals with real prices, the `fargate-scheduler`
> field selector, memory and sync time on a large fleet).

> A code change with open tasks here cannot merge, and so cannot release:
> `scripts/spec-tasks-check.py` fails CI and the `pr-guard` hook blocks
> `gh pr create` (use `--draft` until the tasks are done).

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| GKE prices (GCE provider; [TODO.md](../../TODO.md)) | owner needs GKE cost in dollars |
| Recommendations across clusters | owner asks |
| Export of the Clusters screen | owner asks |
| Keep members and prices across visits (unregister in `OnUpdate`); promote a member's store on `enter` | owner opens the screen often on a large fleet |
| Folding and focus of groups inside a section | owner asks |
| A total across clouds, marked `mixed sources` (D9) | owner asks for one company-wide figure |
| A key that cycles the cloud scope (D11) | typing `:clusters <cloud>` proves slow in use |
| Pod watches for every member, or closed on fold (D10) | T12 shows memory is no issue, or that kept watches are |
