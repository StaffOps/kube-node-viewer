# Tasks: monthly-price-columns

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `PRICE/M` and `IDLE/M` fields, titles and cells; left out when pricing is disabled
  - _Covers: R1, R2, R5 · Depends on: — · Verified by: table-driven tests in `pkg/model`_
- [x] T2. Default columns and narrow-window priority; Columns panel offers both; no sort fields
  - _Covers: R3, R4, R6 · Depends on: T1 · Verified by: key-driven and layout tests_
- [x] T3. README (columns table, Columns panel sentence, troubleshooting), CHANGELOG, screenshots regenerated
  - _Covers: R3 · Depends on: T2 · Verified by: review_
