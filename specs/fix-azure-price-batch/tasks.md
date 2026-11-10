# Tasks: fix-azure-price-batch

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. Tests first against a fake that refuses more than 20 comparisons: batching, a failed batch, a refused batch, resolution
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: the new tests fail on the current code_
- [ ] T2. `maxFilterComparisons`, `baseConditions`, `maxSizesPerRequest`; `fetchWanted` sends a region's sizes in sorted batches, stops the region on a retried failure, keeps a `400` batch as no price with one log line per region
  - _Covers: R1, R2, R3, R4 · Depends on: T1 · Verified by: `go test -race ./pkg/azure/`_
- [ ] T3. Docs: CHANGELOG `Fixed`; dated notes in aks-pricing requirements (one request per region), fix-azure-price-product and fix-azure-retry unchanged lists
  - _Covers: R1 · Depends on: T2 · Verified by: review_
- [ ] T4. Verify
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2, T3 · Verified by: `scripts/verify.sh`, `make lint`, `scripts/kind-it.sh` (kind has no Azure nodes: checks nothing else broke), and the provider run against the public API with 18 or more sizes in one region (scratch program, not committed), recorded as a dated note_
