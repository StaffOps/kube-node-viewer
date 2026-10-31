# Tasks: interactive-tui

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
>
> Every task that edits an upstream `.go` file keeps or adds the modification note (rebrand D5).

- [ ] T0. Owner review of this spec; spec-refuter review applied
  - _Covers: — · Depends on: — · Verified by: review date in the spec header_
- [ ] T1. Benchmark harness on a synthetic cluster; record baseline numbers here
  - _Covers: R10 · Depends on: T0 · Verified by: `go test -bench . ./pkg/model/...` output pasted below_
- [ ] T2. Indexed store with incremental per-node totals; fold `Cluster`; fix `SetPrice`/`GetPod` locking
  - _Covers: R10 · Depends on: T1 · Verified by: unit tests; `go test -race`_
- [ ] T3. Pod informer transform; controller writes to the store; coalesced `changed` message; remove the 100 ms tick
  - _Covers: R10 · Depends on: T2 · Verified by: unit tests; benchmark rerun_
- [ ] T4. Derived node facts: capacity type (incl. `reserved`), node group (D5), zone, region, internal IP, status; reserved price shown as equivalent
  - _Covers: R7 · Depends on: T2 · Verified by: unit tests_
- [ ] T5. `ViewState`: filters (D4), group-by ≤3 nested with subtotals, sort by column/label, search
  - _Covers: R3, R4, R5, R6 · Depends on: T4 · Verified by: unit tests_
- [ ] T6. Panels and keys: `f`, `g`, `s`, `o`, `/`, `r`, `n`, `?`; header line with active state
  - _Covers: R3, R4, R5, R6, R7, R8, R11, R12 · Depends on: T5 · Verified by: key-driven UI tests_
- [ ] T7. Per-context state file (D6), debounced atomic writer, tolerant reader; flag > state > config precedence (D8)
  - _Covers: R1, R2 · Depends on: T5 · Verified by: unit tests (round-trip, corrupt file, per-context)_
- [ ] T8. Context picker `c`: stop watches, clear store, restart on the new context, load its state
  - _Covers: R9 · Depends on: T3, T7 · Verified by: unit test with fake clients; manual on two contexts_
- [ ] T9. Benchmark rerun and comparison recorded here; README (keys, panels, state file, `g` change) and CHANGELOG
  - _Covers: R10, R12 · Depends on: T1–T8 · Verified by: numbers below; review_

## Benchmark results

| Run | Cluster size | Event apply | Snapshot + render | Heap |
|-----|--------------|-------------|-------------------|------|
| Baseline (T1) | — | unmeasured | unmeasured | unmeasured |
| After (T9) | — | unmeasured | unmeasured | unmeasured |

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Saved named views (`v`) | owner asks after using stage 1 |
| Typed filter syntax in `/` (`cap:spot pool:gpu`) | panels feel slow for frequent filters |
| Negation / OR across fields in filters | a real query the panels can't express |
| GKE/AKS node pool labels in the node group column | stage 2 (multi-cloud) |
| GPU resource and columns | stage 3 roadmap |
