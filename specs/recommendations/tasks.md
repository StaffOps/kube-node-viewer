# Tasks: recommendations

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. Instance catalog: generator + embedded table; optional `DescribeInstanceTypes` refresh
  - _Covers: R7, R8, R9 · Depends on: — · Verified by: catalog tests_
- [ ] T2. DaemonSet flag kept by `SlimPod` and on `model.Pod`
  - _Covers: R6, R7 · Depends on: — · Verified by: transform tests_
- [ ] T3. `pkg/recommend`: idle, consolidate (FFD), rightsize, generation, graviton, old node, spot; ordering and savings
  - _Covers: R3, R5, R6, R7, R8, R9, R10, R11, R13 · Depends on: T1, T2 · Verified by: table tests; benchmark_
- [ ] T4. Overview: IDLE/H column, idle sort, idle in group headings
  - _Covers: R4 · Depends on: — · Verified by: unit tests_
- [ ] T5. Pages and command line (`:`), `tab`, page in the saved view
  - _Covers: R1, R2 · Depends on: — · Verified by: key-driven tests_
- [ ] T6. Recommendations page: rows, total, selection, math popup; analysis off the UI goroutine
  - _Covers: R3 · Depends on: T3, T5 · Verified by: key-driven tests_
- [ ] T7. Config keys `recommend-headroom`, `recommend-node-age`
  - _Covers: R12 · Depends on: T3 · Verified by: flag-table tests_
- [ ] T8. README, CHANGELOG, screenshots and GIF; interactive-tui note on command mode
  - _Covers: — · Depends on: T1–T7 · Verified by: review_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| GCP / Azure catalogs and family tables | stage 2 (multi-cloud) |
| Pod-level rightsizing (requests too high) | owner asks |
| Commitments (Savings Plans, RIs) | owner asks |
