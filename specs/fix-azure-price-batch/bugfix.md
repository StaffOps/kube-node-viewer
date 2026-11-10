# Bugfix: azure-price-batch

> Status: draft · Reported: 2026-11-10 UTC (spec-refuter review of the AKS Recommendations draft; measured against the public API) · Reviewed: —

Decisions: [design](design.md).

## Current behavior

The Azure provider (`pkg/azure`) asks for every wanted size of a region in
one Retail Prices API request: `fetch` (`pkg/azure/pricing.go:249-256`)
builds one `$filter` with three base conditions (`serviceName`,
`priceType`, `armRegionName`) and one `armSkuName eq '<size>'` comparison
per size.

1. **18 or more sizes in a region get no price at all.** The API refuses a
   filter with more than 20 comparisons. Measured 2026-11-10 with the
   provider's filter shape in `eastus`: 17 sizes → `200` (one page, 102
   items); 18 sizes → `400 {"Error":{"Code":"BadRequest","Message":"Invalid
   OData parameters supplied"}}`. Also measured that day, by the review that
   found it: 1 base condition + 19 sizes → `200`, 1 + 20 → `400`, and 12
   sizes with long names (1179 characters of filter) → `200`, so the count
   of comparisons is the limit, not the length. Every node of that region
   shows `-` for its price, on Overview and in the export.
2. **The refused request is sent again every minute, for ever.**
   `retryDelay` (`pkg/azure/retry.go:50-52`) gives a `400` the 1-minute
   floor, and the region's next pass sends the same filter, which the API
   refuses the same way. The header shows the failure line again each
   minute.

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

- [ ] 18 sizes in one region → two requests (17 and 1 sizes), every size
      priced, against a fake that refuses more than 20 comparisons
- [ ] 34 sizes → two requests; 35 → three
- [ ] 17 sizes → one request (no change for small clusters)
- [ ] `maxSizesPerRequest` plus the base conditions is at most 20 (the
      test counts the `eq` comparisons of the filter `fetch` builds)

**R2** WHEN one batch of a region fails with a status that is retried
(anything but `400`) THEN the sizes of the batches already answered in that
pass SHALL keep their prices, the failed batch and the region's remaining
batches SHALL stay wanted, the region SHALL wait out its retry time as
today (fix-azure-retry R1, R2), and no later batch of that region SHALL be
sent in that pass.

Tests: `// fix-azure-price-batch R2` on `TestFailedBatchKeepsEarlierPrices`.

- [ ] 35 sizes, the second request answered `429` with `Retry-After: 120`
      → the first 17 sizes priced, no third request, `retryAt = now + 2m`;
      after it, the next pass asks only for the 18 still wanted (two
      requests)
- [ ] OnUpdate is called once in that pass (some prices arrived)

**R3** WHEN a batch is answered `400` THEN the system SHALL NOT send that
batch again: its sizes SHALL be kept as having no price (like a size the API
does not list), the region SHALL get no retry time, the region's other
batches SHALL still be sent in that pass, and the system SHALL log once per
region per provider `Azure pricing for <region>: request refused (400 Bad
Request), <n> sizes have no price`.

Tests: `// fix-azure-price-batch R3` on `TestBadRequestIsNotRetried`.

- [ ] a fake answering `400` to one batch: woken 5 more times, it gets no
      new request for those sizes; their nodes stay without a price
- [ ] the other batch of the same region is priced in the same pass
- [ ] the log line appears once, even after a second refused batch in the
      region; it does not start with `updating Azure pricing for`, so a
      later success of the region does not clear it as resolved
      (fix-azure-retry R3)

**R4** WHEN a region whose failure was logged has every batch of a pass
answered THEN the system SHALL log the resolution line once
(fix-azure-retry R3); WHEN a later batch of that pass fails, no resolution
SHALL be logged.

Tests: `// fix-azure-price-batch R4` on `TestResolvedOnlyWhenEveryBatchSucceeds`.

- [ ] failure, then a pass whose first batch succeeds and second answers
      `503` → no resolution line; the next pass, both answered → one
      resolution line

## Unchanged behavior

- A region with up to 17 wanted sizes: one request per pass, same filter
  as today (`TestPricesByMeter`, `TestFailureIsRetriedLater`).
- The 1-minute retry delay, the retry headers on `429` / `503`, one retry
  time per region, and other regions not delayed by a failing one
  (fix-azure-retry R1, R2; `retry_test.go`).
- Every status other than `400` is retried as today (`500`, `403`, `404`,
  network errors).
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
  size in a single filter, with no cap.
- `pkg/azure/retry.go:51-52` (`retryDelay`): a `400` gets the floor delay
  like a transient failure.

## Not done

- Pacing between batches. About 11 requests in about 11 seconds got a `429`
  in the 2026-11-10 review (unmeasured beyond that); a `429` is already
  read per region (fix-azure-retry R1). A cluster needs 18 sizes in one
  region before a second request is sent.
- Splitting a refused batch to find a size the API rejects
  ([design](design.md) D2, alternatives).
