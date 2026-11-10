# Bugfix: azure-price-batch

> Status: active · Reported: 2026-11-10 UTC (spec-refuter review of the AKS Recommendations draft; measured against the public API) · Reviewed: 2026-11-10 (spec-refuter; 2 HIGH, 4 MEDIUM, 4 LOW; applied); diff 2026-11-10 (spec-refuter; 2 MEDIUM, 4 LOW; applied)

Decisions: [design](design.md).

## Current behavior

The Azure provider (`pkg/azure`) asks for every wanted size of a region in
one Retail Prices API request: `fetch` (`pkg/azure/pricing.go:249-256`)
builds one `$filter` with three base conditions (`serviceName eq 'Virtual
Machines' and priceType eq 'Consumption' and armRegionName eq '<region>'`)
and, in parentheses, one `armSkuName eq '<size>'` comparison per size
joined by `or`.

1. **18 or more sizes in a region get no price at all.** The API refuses a
   filter with more than 20 comparisons. Measured 2026-11-10 in `eastus`
   only, with that filter shape: 17 sizes → `200` (one page, 102 items);
   18 sizes → `400 {"Error":{"Code":"BadRequest","Message":"Invalid OData
   parameters supplied"}}`. Also measured that day, by the review that
   found it: 1 base condition + 19 sizes → `200`, 1 + 20 → `400`, and 12
   sizes with long names (1179 characters of filter) → `200`. Every node of
   that region shows `-` for its price, on Overview and in the export.
2. **The refused request is sent again every minute, for ever.**
   `retryDelay` (`pkg/azure/retry.go:50-52`) gives a `400` the 1-minute
   floor, and the region's next pass sends the same filter, which the API
   refuses the same way. The header shows the failure line again each
   minute.
3. **One bad value refuses the whole request.** A size or region with a
   quote is escaped as `''` (`pkg/azure/pricing.go:252,255`), which the API
   does not accept: measured 2026-11-10, `["Standard_D2s_v5", "it's"]` →
   `400 … "Invalid parameters supplied"` (a different message from the
   cap's), while `["Standard_D2s_v5", "bogus.x_y-z"]` → `200`. Kubernetes
   label values cannot hold a quote (`pkg/model/node.go:224,499` read the
   labels), so no real node has hit this, but nothing in the provider
   enforces it.

Reproduction (fakes; no real API needed): an `httptest` server that
answers `400` to a filter with more than 20 `eq` comparisons and `200`
otherwise; 18 nodes of different sizes in one region.

## Expected behavior

**R1** WHEN a region has more wanted sizes than `maxSizesPerRequest` (17:
the API's 20 comparisons less the 3 base conditions) THEN the system SHALL
ask for them in batches of at most `maxSizesPerRequest` sizes, sorted by
name, one request after another in the same pass, and SHALL price every
size of every batch that is answered.

Tests: `// fix-azure-price-batch R1` on `TestManySizesAreBatched` and
`TestBatchSizeFitsTheAPICap`.

- [x] 18 sizes in one region → two requests (17 and 1 sizes), every size
      priced, against a fake that refuses more than 20 comparisons
- [x] 34 sizes → two requests; 35 → three
- [x] 17 sizes → one request (no change for small clusters)
- [x] `maxSizesPerRequest` plus the base conditions is at most 20 (the
      test counts the `eq` comparisons of the filter `fetch` builds)

**R2** WHEN one batch of a region fails with a status that is retried
(anything but the cap's `400`, R3) THEN the sizes of the batches already
answered in that pass SHALL keep their prices, the failed batch and the
region's remaining batches SHALL stay wanted, the region SHALL wait out its
retry time as today (fix-azure-retry R1, R2), and no later batch of that
region SHALL be sent in that pass.

Tests: `// fix-azure-price-batch R2` on `TestFailedBatchKeepsEarlierPrices`.

- [x] 35 sizes, the second request answered `429` with `Retry-After: 120`
      → the first 17 sizes priced, no third request, `retryAt = now + 2m`;
      after it, the next pass asks only for the 18 still wanted (two
      requests)
- [x] OnUpdate is called once in that pass (some prices arrived)

**R3** WHEN a batch is answered `400` with the message `Invalid OData
parameters supplied` (the cap's refusal) THEN the system SHALL NOT send that
batch again: its sizes SHALL be kept as having no price (like a size the
API does not list), the region SHALL get no retry time and its failure
state SHALL be neither set nor kept by it, the region's other batches SHALL
still be sent in that pass, and the system SHALL log once per region per
provider `Azure pricing for <region>: request refused (retail prices API:
400 Bad Request: Invalid OData parameters supplied), some sizes have no
price`. Any other `400` SHALL be retried, no sooner than `maxRetryAfter`
(10 minutes) later.

Tests: `// fix-azure-price-batch R3` on `TestBadRequestIsNotRetried` and
`TestOtherBadRequestIsRetriedSlowly`.

- [x] a fake answering the cap's `400` to one batch: 5 more passes send no
      new request for those sizes; they are stored with no price
- [x] the other batch of the same region is priced in the same pass
- [x] the log line appears once, even after a second refused batch in the
      region; it does not start with `updating Azure pricing for`, so a
      later success of the region does not clear it as resolved
      (fix-azure-retry R3)
- [x] a `400` with another message (`Invalid parameters supplied`) →
      `retryAt = now + 10m`, the sizes stay wanted

**R4** WHEN a region whose failure was logged has every batch of a pass
answered (`200`, or the cap's `400` of R3) THEN the system SHALL log the
resolution line once (fix-azure-retry R3); WHEN a batch of that pass fails
with a retried status, no resolution SHALL be logged.

Tests: `// fix-azure-price-batch R4` on `TestResolvedOnlyWhenEveryBatchSucceeds`.

- [x] failure, then a pass whose first batch succeeds and second answers
      `503` → no resolution line; the next pass, both answered → one
      resolution line
- [x] failure (`503`), then a pass with batch 1 `200` and batch 2 the cap's
      `400` → one resolution line and one refusal line

**R5** WHEN a node's region or size does not match
`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$` (the Kubernetes label value
charset, without the empty value) THEN the system SHALL store it as having
no price and SHALL NOT put it in any filter.

Tests: `// fix-azure-price-batch R5` on `TestInvalidNamesNeverReachTheAPI`.

- [x] `it's` among 17 good sizes → the 17 priced in one request; no `'`
      other than the filter's own quotes reaches the fake
- [x] a region `east us` → no request; its node has no price
- [x] `bogus.x_y-z` is valid and is asked for

**R6** WHEN a request is answered with a non-`200` status whose body is the
API's error JSON (`{"Error":{"Code":…,"Message":…}}`) THEN the failure log
line SHALL end the error with `: <Message>`; a body that is not that JSON
SHALL leave the line as today.

Tests: `// fix-azure-price-batch R6` on `TestErrorMessageInTheLogLine`.

- [x] `400` with the cap's JSON → `retail prices API: 400 Bad Request:
      Invalid OData parameters supplied`
- [x] `429` with the body `busy` → `retail prices API: 429 Too Many
      Requests` (as today)

> **Corrected 2026-11-10 (diff review).** The message is made safe for the
> one-line header: control characters become spaces and it is cut to 200
> characters (a newline could otherwise start a `resolved: …` line that
> clears an unrelated error, `cmd/kube-node-viewer/errlog.go`); a body over
> 4096 bytes is not read, so the line ends at the status.
> `TestErrorMessageInTheLogLine` checks both.

**R7** WHEN a request fails with any retried status or a network error THEN
the region SHALL get a retry time and its sizes SHALL stay wanted, as
today.

Tests: `// fix-azure-price-batch R7` on `TestOtherFailuresAreRetried`.

- [x] `403`, `404`, `500` and a closed server → `retryAt` set to the
      1-minute floor, every size still wanted; a branch that made every 4xx
      final fails this test

## Unchanged behavior

- A region with up to 17 wanted sizes: one request per pass, same filter
  as today (`TestPricesByMeter`, `TestFailureIsRetriedLater`).
- The 1-minute retry delay, the retry headers on `429` / `503`, one retry
  time per region, and other regions not delayed by a failing one
  (fix-azure-retry R1, R2; `retry_test.go`).
- Pagination: `NextPageLink` followed up to `maxPages` (20) per request, and
  the cut-short line once per region (fix-azure-price-product R4).
- Which items price a node (fix-azure-price-product R1-R3;
  `product_test.go`).
- No credentials; AWS nodes make no Azure request; `--disable-pricing`
  builds no provider (`TestAWSNodeMakesNoAzureRequest`).
- OnUpdate is called once per pass that priced something.

## Root cause

- `pkg/azure/pricing.go:177-180` (`fetchWanted`): one `fetch` per region
  with every wanted size of it.
- `pkg/azure/pricing.go:250-255` (`fetch`): one `armSkuName` comparison per
  size in a single filter, with no cap, and values put in it unchecked.
- `pkg/azure/retry.go:51-52` (`retryDelay`): a `400` gets the floor delay
  like a transient failure.

## Not done

- Pacing between batches. About 11 requests in about 11 seconds got a `429`
  in the 2026-11-10 review (unmeasured beyond that); a `429` is already
  read per region (fix-azure-retry R1). A cluster needs 18 sizes in one
  region before a second request is sent.
- Sequential batches with the 20-second timeout (`pkg/azure/pricing.go:39`)
  can delay later regions of the same pass: a region whose batches time out
  holds the next region for 20 seconds per batch sent (a failed batch stops
  the region, R2, so at most one timeout per region and pass). Each region
  already waits for the one before today.
- Splitting a refused batch to find a size the API rejects
  ([design](design.md) D2, alternatives).
