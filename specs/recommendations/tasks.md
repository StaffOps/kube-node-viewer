# Tasks: recommendations

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. Catalog: `hack/catalog` generator, `zz_generated_instances.go`, refresh from Price List products; zone-aware spot lookup
  - _Covers: R7, R8, R9, R11, R15 · Depends on: — · Verified by: catalog and pricing tests_
- [x] T2. Owner kind kept by `SlimPod` and on `model.Pod`
  - _Covers: R6, R7 · Depends on: — · Verified by: transform tests_
- [x] T3. `pkg/recommend`: idle, consolidate, rightsize, generation, graviton, old node, spot; targets; total; exclusions
  - _Covers: R3, R5, R6, R7, R8, R9, R10, R11, R13, R14, R15 · Depends on: T1, T2 · Verified by: fixture and table tests; benchmark_
- [x] T4. Overview: `IDLE/H` column, idle sort, idle in group headings
  - _Covers: R4 · Depends on: — · Verified by: unit tests_
- [ ] T5. Screens and command line (`:`), `tab`, screen kept per cluster
  - _Covers: R1, R2 · Depends on: — · Verified by: key-driven tests_
- [ ] T6. Recommendations screen: rows, total, keys, math popup; analysis off the UI goroutine, cached
  - _Covers: R3 · Depends on: T3, T5 · Verified by: key-driven tests_
- [ ] T7. Config keys `recommend-headroom`, `recommend-node-age`
  - _Covers: R12 · Depends on: T3 · Verified by: config tests in cmd_
- [ ] T8. README, CHANGELOG, screenshots and GIF; interactive-tui note on command mode
  - _Covers: — · Depends on: T1–T7 · Verified by: review_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| GCP / Azure catalogs and family tables | stage 2 (multi-cloud) |
| Pod-level rightsizing (requests too high) | owner asks |
| Commitments (Savings Plans, RIs) | owner asks |
| A second total including Spot | owner asks |

## Benchmark

`go test ./pkg/recommend -run x -bench Analyze -benchtime 20x` on
darwin/arm64, 2026-11-02: 500 nodes / 15,000 pods, 10 groups → 9.1 ms per
analysis. Rightsize candidates are indexed by family, so the catalog size
(1,393 types) doesn't multiply the cost.
