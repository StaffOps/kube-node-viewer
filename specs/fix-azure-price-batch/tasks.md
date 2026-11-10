# Tasks: fix-azure-price-batch

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. Tests first against a fake that refuses more than 20 comparisons and a quote: batching, a failed batch, the cap's and another `400`, other failures, resolution, invalid names, the error message
  - _Covers: R1, R2, R3, R4, R5, R6, R7 · Depends on: — · Verified by: the new tests fail on the current code_
- [x] T2. `maxFilterComparisons`, `baseConditions`, `maxSizesPerRequest`; `NodePrice` stores invalid names as no price; `fetchWanted` sends a region's sizes in sorted batches, stops the region on a retried failure, keeps a cap-refused batch as no price with one log line per region, retries another `400` after `maxRetryAfter`; `statusError` carries the API's message
  - _Covers: R1, R2, R3, R4, R5, R6, R7 · Depends on: T1 · Verified by: `go test -race ./pkg/azure/`_
- [x] T3. Docs: CHANGELOG `Fixed`; dated notes in (1) `specs/fix-azure-retry/bugfix.md` R1 ("Any other failure … keeps the retry delay (1 minute)") and (2) its unchanged list ("One request per region per pass"); (3) `specs/aks-pricing/requirements.md` usage note ("one request per region"), (4) its R2 ("One request per region asks for every unknown size", "retried no sooner than 1 minute later"); (5) `specs/fix-azure-price-product/bugfix.md` unchanged list ("One request per region for all wanted sizes … a minute later"); `README.md` AKS paragraph ("Prices show a second or two after start", never measured)
  - _Covers: R1, R3 · Depends on: T2 · Verified by: review_
- [x] T4. Verify
  - _Covers: R1, R2, R3, R4, R5, R6, R7 · Depends on: T1, T2, T3 · Verified by: `scripts/verify.sh`, `make lint`, `scripts/kind-it.sh` (kind has no Azure nodes: checks nothing else broke), and `TestLiveManySizesInOneRegion` with `KNV_AZURE_LIVE=1` (20 sizes in `eastus`), recorded as a dated note_

> **Verified 2026-11-10.** The nine tests of `pkg/azure/batch_test.go`
> were run on the old code with only the new names added: seven failed
> (batching, failed batch, both `400`s, resolution, invalid names, error
> message); `TestBatchSizeFitsTheAPICap` and `TestOtherFailuresAreRetried`
> guard the cap and the unchanged retries. All pass on the fix with
> `go test -race ./pkg/azure/`. `scripts/verify.sh` and `make lint` (0
> issues) green. `TestLiveManySizesInOneRegion` (`KNV_AZURE_LIVE=1`): 20
> public VM sizes in `eastus` through the provider, every one priced (the
> old code sends them in one request, which the API refuses). The cap
> itself re-measured that day: 17 sizes → `200`, 18 → `400 Invalid OData
> parameters supplied`. `scripts/kind-it.sh` passed on a throwaway kind
> cluster, deleted after; kind has no Azure nodes, so it only shows nothing
> else broke. Not run on an AKS cluster.
