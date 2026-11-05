# Bugfix: azure-retry

> Status: draft · Reported: 2026-11-05 (owner's AKS run, [fix-azure-price-product](../fix-azure-price-product/tasks.md) T4; [aks-pricing](../aks-pricing/design.md) D2 retroactive review) · Reviewed: —

## Current behavior

Three problems in how the Azure provider (`pkg/azure`) handles a failed
Retail Prices API request:

1. **`Retry-After` is ignored.** On HTTP 429 the API answers with a
   `Retry-After` header. `get` turns any non-200 answer into the error
   `retail prices API: 429 Too Many Requests` and drops the headers, and
   the provider always waits its fixed 1-minute retry delay. If the API
   asks for longer, the next request comes too early and is refused
   again.
2. **A resolved error stays red.** The failure is logged
   (`updating Azure pricing for eastus2, retail prices API: 429 Too Many
   Requests`) and the header shows the last logged line in red for 2
   minutes (`issueWindow`). When the retry a minute later succeeds and the
   prices appear, the red line still shows for about another minute, so a
   transient rate limit looks like a live failure. Seen on the owner's
   AKS run on 2026-11-05: prices filled in at the retry, the warning left
   2 minutes after the 429.
3. **One region's failure delays every region.** The provider keeps one
   failure time for all regions. After a failure in region A, a size
   first seen in region B waits out A's retry delay before it is asked
   for, though B never failed.

Reproduction (fakes; no real API needed): an `httptest` server that
answers 429 with `Retry-After: 120` for region A and 200 for region B.

## Expected behavior

**R1** WHEN a request for a region is answered HTTP 429 or 503 with a
`Retry-After` header (delay in seconds, or an HTTP date) THEN the system
SHALL ask that region again no sooner than the later of the retry delay
(1 minute) and the `Retry-After` time, capped at 10 minutes. A missing,
unparsable, negative or past `Retry-After` SHALL leave the 1-minute
delay. Other failures keep the 1-minute delay.

Tests: `// fix-azure-retry R1` on `TestRetryAfterSecondsDelaysTheRegion`,
`TestRetryAfterDateAndBadValues` and `TestRetryAfterIsCapped`.

- [ ] `Retry-After: 120` → the region is not asked again for 2 minutes
- [ ] `Retry-After: 10` → still 1 minute
- [ ] an HTTP date 5 minutes ahead → 5 minutes; `Retry-After: 3600` → 10 minutes
- [ ] `Retry-After: soon`, `-5`, a past date, or none → 1 minute

**R2** WHEN a request for a region fails THEN only that region SHALL wait
out its retry time; wanted sizes of other regions SHALL be asked for at
once, and each region SHALL keep its own retry time.

Tests: `// fix-azure-retry R2` on `TestRegionsRetryIndependently`.

- [ ] region A answers 429, a size of region B is wanted afterwards: B is
      asked for without waiting, A not before its retry time
- [ ] A's later success does not reset B's retry time, and vice versa

**R3** WHEN a region's request succeeds after that region's failure was
logged THEN the system SHALL log a resolution line for it, and the header
SHALL stop showing the failure if it is still the last logged error. A
newer, unrelated error SHALL stay.

Mechanism (decision): a log line that starts with `model.ResolvedPrefix`
(`resolved: `) followed by the text a failure line starts with is a
resolution. The error log (`cmd/kube-node-viewer/errlog.go`) writes it to
the log file like any line, and clears the line kept for the header only
when that line starts with the given text; the resolution line itself is
never kept for the header. The provider logs failures as `updating Azure
pricing for <region>, <error>` and resolutions as `resolved: updating
Azure pricing for <region>`.

Tests: `// fix-azure-retry R3` on `TestResolvedLineClearsTheMatchingIssue`
(errlog) and `TestSuccessAfterFailureLogsResolved` (azure).

- [ ] failure for eastus2, then `resolved: updating Azure pricing for
      eastus2` → `Last()` is empty; the log file has both lines
- [ ] failure for eastus2, then an unrelated error, then the resolution →
      the unrelated error stays
- [ ] failure for eastus2, resolution for westeurope → the eastus2 failure
      stays
- [ ] a region that never failed logs no resolution line on success

## Unchanged behavior

- The 1-minute retry delay after a failure without `Retry-After`
  (`TestFailureIsRetriedLater`, `TestProviderRetryAndTimeoutDefaults`).
- Nothing is asked for a region before its retry time, even when new
  sizes are wanted (`TestNoRequestBeforeTheRetryDelay`).
- One request per region per wake; sizes the API does not list are not
  asked again; the 20-page cap and its once-per-region log.
- The 20-second request timeout.
- The header keeps any other logged error for `issueWindow` (2 minutes);
  `issueWindow` itself is unchanged (`pkg/model/header.go`).
- AWS nodes make no Azure request; `--disable-pricing` builds no Azure
  provider.
- Which items price a node (fix-azure-price-product): untouched.

## Root cause

- `pkg/azure/pricing.go:338-340` (`get`): a non-200 answer becomes a plain
  error; `resp.Header` is never read.
- `pkg/azure/pricing.go:63` and `:135`, `:164`: one `failedAt` for the
  provider; `run` waits `failedAt + retry` before fetching any region.
- `pkg/azure/pricing.go:166`: the failure is logged, but a later success
  logs nothing, and `cmd/kube-node-viewer/errlog.go` (`Write`, `Last`)
  only ever replaces the kept line with a newer one.

## Fix

- `get` returns a `retryError{after time.Duration}` wrapping the status
  error when the answer is 429 or 503 with a usable `Retry-After`.
- The provider keeps `retryAt map[region]time.Time` instead of
  `failedAt`; `fetchWanted` skips regions whose `retryAt` is in the
  future and returns the earliest one; `run` waits for a wake or that
  time, whichever comes first.
- `model.ResolvedPrefix` in `pkg/model` (the provider already imports
  it); `errlog.Write` handles a line with that prefix as above.

**Why this design.** It keeps Azure HTTP in `pkg/azure` and the header in
`pkg/model`/`cmd`, with one string convention between them, and no new
API between the provider and the UI. **Alternatives:** shorten
`issueWindow` (hides every other error sooner too); a structured issue
API keyed by source (more code for one producer); log nothing on 429
(hides real outages). **When this would be wrong:** if another producer
starts logging lines that begin with `resolved: `, it would clear issues
it did not cause; only an exact prefix match clears, which limits it.
**Reversal action:** drop the prefix handling in `errlog.Write` and the
resolution log; go back to one `failedAt`.

## Not done

- A Retry-After for the AWS pricing provider (different SDK, own retries).
- Showing the remaining wait in the header.
