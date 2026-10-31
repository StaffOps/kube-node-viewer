# Tasks: interactive-tui

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
>
> Every task that edits an upstream `.go` file keeps or adds the modification note (rebrand D5).

- [x] T0. Owner decisions recorded; spec-refuter review applied
  - _Covers: — · Depends on: — · Verified by: review date in the spec header_
- [ ] T1. Benchmark harness on a synthetic store; record baseline numbers below
  - _Covers: R11 · Depends on: T0 · Verified by: `go test -bench . ./pkg/model/...` output below_
- [ ] T2. Fix `SetPrice` and `GetPod` locking
  - _Covers: R11 · Depends on: T0 · Verified by: `go test -race`_
- [ ] T3. Indexed store with per-node totals, pending pods by node name, generation; fold `Cluster`
  - _Covers: R11 · Depends on: T1, T2 · Verified by: unit tests (pod before node, nodeclaim then node)_
- [ ] T4. Informers with `NewInformerWithOptions` + pod `Transform` (D3), events tagged with generation
  - _Covers: R11 · Depends on: T3 · Verified by: transform tests incl. Fargate and tombstones_
- [ ] T5. Program-first wiring, coalescer via `program.Send`, pricing `OnUpdate` notifies, 1 s tick replaces 100 ms
  - _Covers: R11 · Depends on: T4 · Verified by: unit tests; benchmark rerun_
- [ ] T6. Derived node facts: capacity type (R7 table), node group, zone, region, IP, status flags
  - _Covers: R3, R7 · Depends on: T3 · Verified by: unit tests_
- [ ] T7. Price kind in `pricing.Provider`; reserved as equivalent in `pkg/aws`; totals exclude equivalents
  - _Covers: R8 · Depends on: T6 · Verified by: unit tests_
- [ ] T8. `ViewState`: filters, group-by ≤3 with subtotals, sort, search, column priority
  - _Covers: R3, R4, R5, R6, R14 · Depends on: T6 · Verified by: unit tests_
- [ ] T9. Flag table (D8) with explicit-flag detection; per-cluster state file, debounced atomic writer
  - _Covers: R1, R2, R15 · Depends on: T8 · Verified by: one test per flag-table row; state tests_
- [ ] T10. Key routing, help (`?`), header line, pagination by lines; Filters panel (`f`)
  - _Covers: R3, R12, R14 · Depends on: T8 · Verified by: key-driven tests_
- [ ] T10b. Group by (`g`), Sort (`s`), Columns (`o`) panels; `/` search; `r`; `n`
  - _Covers: R4, R5, R6, R7, R9, R13 · Depends on: T10 · Verified by: key-driven tests_
- [ ] T11. Context picker (`c`): stop watches, bump generation, clear store, restart in a `tea.Cmd`, load state, re-scope pricing region
  - _Covers: R10 · Depends on: T5, T9 · Verified by: tests with fake clients; manual on two contexts_
- [ ] T12. Benchmark rerun recorded below; README (keys, panels, state file, `g`/`esc` changes) and CHANGELOG
  - _Covers: R11, R12 · Depends on: T1–T11 · Verified by: numbers below; review_

## Benchmark results

| Run | Nodes / pods | Event apply | Snapshot + render | Heap |
|-----|--------------|-------------|-------------------|------|
| Baseline (T1) | 500 / 15 000 | unmeasured | unmeasured | unmeasured |
| Baseline (T1) | 500 / 150 000 | unmeasured | unmeasured | unmeasured |
| After (T12) | 500 / 15 000 | unmeasured | unmeasured | unmeasured |
| After (T12) | 500 / 150 000 | unmeasured | unmeasured | unmeasured |

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Saved named views (`v`) | owner asks after using stage 1 |
| Typed filter syntax in `/` (`cap:spot pool:gpu`) | panels feel slow for frequent filters |
| Negation / OR across fields in filters | a real query the panels can't express |
| GKE/AKS node pool labels; nodes with empty `providerID` | stage 2 (multi-cloud) |
| GPU resource and columns | stage 3 roadmap |
