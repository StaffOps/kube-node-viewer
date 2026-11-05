# Tasks: fix-azure-retry

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `retryDelay` (both headers, floor, 10-minute cap) and `statusError` from `get`; per-region `retryAt` replaces `failedAt`; `run` waits for the earliest due region; the failure line says what retry header was read
  - _Covers: R1, R2 · Depends on: — · Verified by: `TestRetryDelay`, `TestRetryAfterDelaysTheRegion`, `TestRetryAfterOnALaterPage`, `TestRegionsRetryIndependently`; `go test -race ./pkg/azure/`_
- [x] T2. `model.ResolvedPrefix`; the provider logs one resolution after a region's failure is followed by a success; `errlog.Write` clears the kept line on a matching resolution (exact, or followed by `,`); `lastLine` strips the standard logger's date and time
  - _Covers: R3, R4 · Depends on: T1 · Verified by: `TestResolvedLineClearsTheMatchingIssue`, `TestResolvedThroughCaptureLogs`, `TestSuccessAfterFailureLogsResolved`, `TestLastLine`_
- [x] T3. Docs: CHANGELOG `Fixed`; dated note in aks-pricing D2; `TODO.md`: remove "Azure prices: on HTTP 429 the provider waits its fixed 1-minute retry…" and "Azure prices: one failure time is shared by every region…"
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2 · Verified by: review_
- [x] T4. Verify
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2, T3 · Verified by: `scripts/verify.sh`, `make lint`, `go test -race ./pkg/azure/ ./cmd/...`; not run on a cluster (a real 429 cannot be produced on demand; behaviour covered by fakes), stated in a dated note_

> **Verified 2026-11-05.** `go test -race -count=2 ./pkg/azure/`,
> `go test -race ./cmd/kube-node-viewer/` and `go test ./pkg/... ./cmd/...`
> pass; `scripts/verify.sh` and `make lint` green. Not run on a cluster: a
> real 429 cannot be produced on demand, so R1–R3 are covered by an
> `httptest` fake (429 with retry headers, two regions, a failure on page
> 2) and by `captureLogs` with `log.Printf`. Whether the global Retail
> Prices API sends a retry header stays unmeasured; the failure line now
> records it.
