# Bugfix: azure-retry

> Status: active · Reported: 2026-11-05 (owner's AKS run, [fix-azure-price-product](../fix-azure-price-product/tasks.md) T4; [aks-pricing](../aks-pricing/design.md) D2 retroactive review) · Reviewed: 2026-11-05 (spec-refuter; 2 BLOCKING, 3 HIGH, 4 MEDIUM, 5 LOW, applied)

Decisions: [design](design.md).

## Current behavior

Three problems in how the Azure provider (`pkg/azure`) handles a failed
Retail Prices API request, and one in how the header shows log lines:

1. **A retry header is ignored.** `get` turns any non-200 answer into the
   error `retail prices API: 429 Too Many Requests` and drops the headers,
   so the provider always waits its fixed 1-minute retry delay. Whether
   the API sends a retry header on 429 is unmeasured: the dropped headers
   were never seen.
2. **A resolved error stays red.** The failure is logged and the header
   shows the last logged line in red for 2 minutes (`issueWindow`). When
   the retry a minute later succeeds and prices appear, the red line
   stays for about another minute. Seen on the owner's AKS run on
   2026-11-05: prices filled in at the retry, the warning left 2 minutes
   after the 429.
3. **One region's failure delays every region.** The provider keeps one
   failure time; after a failure in region A, a size first seen in region
   B waits out A's retry delay, though B never failed.
4. **The header shows the standard logger's date and time.** `lastLine`
   strips klog's header but not the `2026/11/05 13:28:32 ` that
   `log.Printf` puts first, so the header reads
   `2026/11/05 13:28:32 updating Azure pricing for …`.

Reproduction (fakes; no real API needed): an `httptest` server answering
429 for one region and 200 for another; `log.Printf` through
`captureLogs`.

## Expected behavior

**R1** WHEN a request for a region is answered HTTP 429 or 503 THEN the
system SHALL not ask that region again before `max(retry delay,
min(retry header, 10 minutes))`, where the retry header is the larger
usable value of `Retry-After` and
`x-ms-ratelimit-microsoft.consumption-retry-after` (a non-negative
integer of seconds, or an HTTP date in the future), and SHALL put the
parsed value, or `no retry header`, at the end of the failure log line.
An unusable value (`soon`, `-5`, `+120`, `1.5`, a past date) counts as
none. Any other failure, or none usable, keeps the retry delay (1
minute).

Tests: `// fix-azure-retry R1` on `TestRetryDelay` (pure, table, fixed
time), `TestRetryAfterDelaysTheRegion` and `TestRetryAfterOnALaterPage`.

- [x] `Retry-After: 120` → `retryAt = now + 2m`; woken 5 times in that
      time, the region gets no new request
- [x] `Retry-After: 10` → 1 minute; a date 5 minutes ahead → 5 minutes;
      `3600` → 10 minutes; a 1-hour retry delay stays 1 hour
- [x] both headers → the larger; 503 is read like 429; 500 is not
- [x] `soon`, `-5`, `+120`, `1.5`, a past date, none → 1 minute and
      `(no retry header)` in the log line
- [x] a 429 on page 2 of an answer is read the same way

**R2** WHEN a request for a region fails THEN only that region SHALL wait
out its retry time: wanted sizes of other regions SHALL be asked for in
the same pass, without waiting for it, and each region SHALL keep its own
retry time. A wake while only waiting regions have wanted sizes SHALL
send no request.

Tests: `// fix-azure-retry R2` on `TestRegionsRetryIndependently`.

- [x] westeurope answers 429, a size of eastus2 is wanted afterwards:
      eastus2 is priced without waiting (one westeurope request may be in
      flight in the same pass)
- [x] westeurope's success after its time leaves eastus2's later retry
      time and request count unchanged

**R3** WHEN a region's request succeeds after a logged failure of that
region THEN the system SHALL log `resolved: updating Azure pricing for
<region>`, and the header SHALL stop showing the failure if the kept line
is exactly `updating Azure pricing for <region>` or starts with it
followed by `,`. A newer unrelated error, or another region's failure,
SHALL stay; a resolution line is never kept for the header; a region
that never failed logs no resolution.

Tests: `// fix-azure-retry R3` on `TestResolvedLineClearsTheMatchingIssue`,
`TestResolvedThroughCaptureLogs` (errlog) and
`TestSuccessAfterFailureLogsResolved` (azure).

- [x] failure eastus2 then its resolution → `Last()` is empty; the log
      file has both lines
- [x] failure eastus2, an unrelated error, the resolution → the unrelated
      error stays
- [x] failure eastus2, resolution westeurope → stays; resolution `eastus`
      → stays
- [x] two failures then one success → one resolution line; a region that
      never failed logs none
- [x] through `captureLogs` and `log.Printf` (with its date and time)

**R4** WHEN a log line starts with the standard logger's date and time
(`YYYY/MM/DD hh:mm:ss`, optionally with fractional seconds) THEN the
header SHALL show the line without it, as it already does for klog's
header.

Tests: `// fix-azure-retry R4` on `TestLastLine`.

- [x] `2026/11/02 12:00:01 Failed to create pricing client: no region` →
      `Failed to create pricing client: no region`
- [x] `2026/11/02 not a time` is kept as is

## Unchanged behavior

- The 1-minute retry delay after a failure without a usable retry header
  (`TestFailureIsRetriedLater`, adapted to read `retryAt` instead of
  `failedAt`, same behaviour; `TestProviderRetryAndTimeoutDefaults`).
- Nothing is asked for a waiting region, even when new sizes are wanted
  (`TestNoRequestBeforeTheRetryDelay`, retry delay 1 hour).
- One request per region per pass; sizes the API does not list are not
  asked again; the 20-page cap and its once-per-region log.
- The 20-second request timeout.
- Any other logged error stays in the header for `issueWindow` (2
  minutes); klog lines are shown as before (`TestLastLine`,
  `TestCaptureLogsKeepsTheTerminalClean`).
- AWS nodes make no Azure request; `--disable-pricing` builds no Azure
  provider (`TestAWSNodeMakesNoAzureRequest`, `TestSessionPricingPerServer`).
- Which items price a node (fix-azure-price-product): untouched
  (`product_test.go`).

> **Corrected 2026-11-05.** `TestLastLine`'s standard-logger case now
> expects the line without its date and time (R4); the klog case is
> unchanged.

## Root cause

- `pkg/azure/pricing.go:338-340` (`get`): a non-200 answer becomes a plain
  error; `resp.Header` is never read.
- `pkg/azure/pricing.go:63`, `:135`, `:164`: one `failedAt` for the
  provider; `run` waits `failedAt + retry` before fetching any region,
  and the failure path wakes itself to wait again.
- `pkg/azure/pricing.go:166`: a failure is logged, a later success logs
  nothing; `cmd/kube-node-viewer/errlog.go` (`Write`) only replaces the
  kept line with a newer one.
- `cmd/kube-node-viewer/errlog.go:60-67` (`lastLine`): strips klog's
  header only.

## Not done

- After `c` (context switch) the old session's failure line is not
  resolved by the new session; it expires with `issueWindow` (2 minutes).
- A retry header for the AWS pricing provider (different SDK, own
  retries).
- Showing the remaining wait in the header.
