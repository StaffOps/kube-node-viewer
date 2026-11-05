# Tasks: fix-azure-retry

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. `get` reads `Retry-After` on 429/503 (seconds or HTTP date, floor 1 minute, cap 10 minutes) into a `retryError`; per-region `retryAt` replaces `failedAt`; `run` waits for the earliest due region
  - _Covers: R1, R2 · Depends on: — · Verified by: `go test -race ./pkg/azure/` (fake API: 429 with `Retry-After`, two regions)_
- [ ] T2. `model.ResolvedPrefix`; the provider logs a resolution after a region's failure is followed by a success; `errlog.Write` clears the kept line on a matching resolution
  - _Covers: R3 · Depends on: T1 · Verified by: `TestResolvedLineClearsTheMatchingIssue`, `TestSuccessAfterFailureLogsResolved`_
- [ ] T3. Docs: CHANGELOG `Fixed`; dated note in aks-pricing D2; `TODO.md`: remove both Azure items
  - _Covers: R1, R2, R3 · Depends on: T1, T2 · Verified by: review_
- [ ] T4. Verify
  - _Covers: R1, R2, R3 · Depends on: T1, T2, T3 · Verified by: `scripts/verify.sh`, `make lint`, `go test -race ./pkg/azure/ ./cmd/...`; not run on a cluster (behaviour covered by fakes; a real 429 cannot be produced on demand), stated in a dated note_
