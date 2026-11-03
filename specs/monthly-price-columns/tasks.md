# Tasks: monthly-price-columns

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `PRICE/M` and `IDLE/M` fields, titles and cells; left out when pricing is disabled
  - _Covers: R1, R2, R5 · Depends on: — · Verified by: table-driven tests in `pkg/model`_
- [x] T2. Default columns and narrow-window priority; Columns panel offers both; no sort fields
  - _Covers: R3, R4, R6 · Depends on: T1 · Verified by: key-driven and layout tests_
- [x] T3. README (columns table, Columns panel sentence, troubleshooting), CHANGELOG, screenshots regenerated
  - _Covers: R3 · Depends on: T2 · Verified by: review_
- [x] T4. Verify: `make check` green; run on a cluster the owner names: `PRICE/M` and `IDLE/M` show next to `PRICE/H`, narrow window drops columns in the documented order, `--disable-pricing` leaves the price columns out
  - _Covers: R1–R6 · Depends on: T1–T3 · Verified by: command output and the cluster run, recorded here as a dated note_
  - _Note (2026-11-03): added when merging main; the release gate now needs the cluster run before merge._
  - _Note (2026-11-03): the author ran it on a cluster; confirmed by the owner. `scripts/verify.sh`, `make lint` and `scripts/build.sh` green on the merged branch._
