# Tasks: interactive-tui

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
>
> Every task that edits an upstream `.go` file keeps or adds the modification note (rebrand D5).

- [x] T0. Owner decisions recorded; spec-refuter review applied
  - _Covers: — · Depends on: — · Verified by: review date in the spec header_
- [x] T1. Benchmark harness on a synthetic store; record baseline numbers below
  - _Covers: R11 · Depends on: T0 · Verified by: `go test -bench . ./pkg/model/...` output below_
- [x] T2. Fix `SetPrice` and `GetPod` locking
  - _Covers: R11 · Depends on: T0 · Verified by: `go test -race`_
- [x] T3. Indexed store with per-node totals, pending pods by node name, generation; fold `Cluster`
  - _Covers: R11 · Depends on: T1, T2 · Verified by: unit tests (pod before node, nodeclaim then node)_
- [x] T4. Informers with `NewInformerWithOptions` + pod `Transform` (D3), events tagged with generation
  - _Covers: R11 · Depends on: T3 · Verified by: transform tests incl. Fargate and tombstones_
- [x] T5. Program-first wiring, coalescer via `program.Send`, pricing `OnUpdate` notifies, 1 s tick replaces 100 ms
  - _Covers: R11 · Depends on: T4 · Verified by: unit tests; benchmark rerun_
- [x] T6. Derived node facts: capacity type (R7 table), node group, zone, region, IP, status flags
  - _Covers: R3, R7 · Depends on: T3 · Verified by: unit tests_
- [x] T7. Price kind in `pricing.Provider`; reserved as equivalent in `pkg/aws`; totals exclude equivalents
  - _Covers: R8 · Depends on: T6 · Verified by: unit tests_
- [x] T8. `ViewState`: filters, group-by ≤3 with subtotals, sort, search, column priority
  - _Covers: R3, R4, R5, R6, R14 · Depends on: T6 · Verified by: unit tests_
- [x] T9. Flag table (D8) with explicit-flag detection; per-cluster state file, debounced atomic writer
  - _Covers: R1, R2, R15 · Depends on: T8 · Verified by: one test per flag-table row; state tests_
- [x] T10. Key routing, help (`?`), header panel (R16) with one-line fallback, pagination by lines; Filters panel (`f`)
  - _Covers: R3, R12, R14, R16 · Depends on: T8 · Verified by: key-driven tests_
- [x] T10b. Group by (`g`), Sort (`s`), Columns (`o`) panels; `/` search; `r`; `n`
  - _Covers: R4, R5, R6, R7, R9, R13 · Depends on: T10 · Verified by: key-driven tests_
- [x] T11. Context picker (`c`): stop watches, bump generation, clear store, restart in a `tea.Cmd`, load state, re-scope pricing region
  - _Covers: R10 · Depends on: T5, T9 · Verified by: tests with fake clients; manual on two contexts_
- [x] T12. Benchmark rerun recorded below; README (keys, panels, state file, `g`/`esc` changes) and CHANGELOG
  - _Covers: R11, R12 · Depends on: T1–T11 · Verified by: numbers below; review_
- [x] T13. Row cursor and single-group folding (`↑`/`↓`, `enter`), saved with the view
  - _Covers: R17 · Depends on: T10b · Verified by: key-driven tests_

## Benchmark results

| Run | Nodes / pods | Event apply | Snapshot + render | Heap (model) |
|-----|--------------|-------------|-------------------|--------------|
| Baseline (T1, before T3) | 500 / 15 000 | 8.8 µs | 2.52 ms | 94 MiB |
| Baseline (T1, before T3) | 500 / 150 000 | 11.1 µs | 29.9 ms | 924 MiB |
| After T4 | 500 / 15 000 | 6.7 µs | 0.43 ms | 17 MiB |
| After T4 | 500 / 150 000 | 5.1 µs | 0.39 ms | 149 MiB |
| After (T12) | 500 / 15 000 | 6.3 µs | 1.70 ms | 17 MiB |
| After (T12) | 500 / 150 000 | 4.6 µs | 1.66 ms | 149 MiB |

Measured 2026-10-31 on darwin/arm64 with Go 1.27.3:
`go test -run x -bench 'PodEvent|Render' -benchtime 2s ./pkg/model/` and
`-bench Heap -benchtime 1x`. Pods request enough for nodes to sit at about 70%
allocation at both sizes, and the model gets pods through the informer
transform as the running tool does. The baseline rows ran the same benchmark
on commit 8cb94b6 with full pods. Heap is the model only; the informer cache
holds its own (now slimmed) copy.

> **Corrected 2026-10-31.** The first baseline (7.7 µs, 1.9/29.6 ms) used pods
> that overcommitted every node about 7.5×, so progress bars above 200% missed
> their cache and dominated render time at 150 000 pods. Re-measured as above.

R11 check after T4: render at 150 000 pods is 0.91× the 15 000-pod time
(limit 1.2×).

R11 check after T12 (measured 2026-11-02, same command): 0.97×. Render went
from 0.4 ms to 1.7 ms between T4 and T12 because the screen now has the
header panel, nested groups and a column layout whose widths come from every
selected node, so columns don't shift between pages; only the page on screen
is turned into lines. At most 10 frames per second, that is under 2% of one
core.

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Saved named views (`v`) | owner asks after using stage 1 |
| Typed filter syntax in `/` (`cap:spot pool:gpu`) | panels feel slow for frequent filters |
| Negation / OR across fields in filters | a real query the panels can't express |
| GKE/AKS node pool labels; nodes with empty `providerID` | stage 2 (multi-cloud) |
| GPU resource and columns | stage 3 roadmap |

> **Shipped 2026-11-02.** Released in v0.8.0 (Homebrew from v0.8.1). The
> owner ran it on `demo1-test-upgrade` and checked a context switch (`c`) to
> a local kind cluster and back: only the selected cluster's nodes showed,
> each with its saved view.
