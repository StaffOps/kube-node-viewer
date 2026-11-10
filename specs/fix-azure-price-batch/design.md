# Design: fix-azure-price-batch

> Decisions for [bugfix](bugfix.md).

## Current vs target

| | Now | After |
|--|-----|-------|
| Request per region and pass | one, all wanted sizes | one per batch of at most 17 sizes, in order |
| Cap's `400` answer | retried every minute | not retried; its sizes have no price; logged once per region |
| Other `400` | retried every minute | retried every 10 minutes |
| Failure in a batch | — | earlier batches keep their prices; later batches wait with the region |
| Region or size outside the label charset | escaped as `''`, refused with the whole request | no price, never in a filter |
| Error body | dropped | its `Message` ends the log line |

Components touched: `pkg/azure/pricing.go` (`NodePrice`, `fetchWanted`,
`fetch`, `get`, a `refused` set per region for the log line),
`pkg/azure/retry.go` (`statusError` gets the message), tests in
`pkg/azure/batch_test.go` and the live case in `pkg/azure/product_test.go`.

## D1. A fixed batch size from the measured cap

`maxSizesPerRequest = maxFilterComparisons - baseConditions` with
`maxFilterComparisons = 20` (measured 2026-11-10) and `baseConditions = 3`,
package-level constants in `pkg/azure/pricing.go`. Sizes are sorted, so the
same wanted set always gives the same batches.

Justification, strongest first:

1. The limit is on the number of terms, not on the length: 3 base + 17
   sizes (20 comparisons) and 1 base + 19 (20) pass, 3 + 18 (21) and 1 + 20
   (21) fail, whatever the split between base and size conditions; 12 sizes
   with 1179 characters of filter pass. In this filter shape the `and` /
   `or` operators are always one fewer than the comparisons, so a limit on
   operators (19) gives the same cap; the constant counts comparisons, the
   thing the code adds one of per size.
2. Deriving the batch size from the base conditions keeps it right if a
   condition is added; a test counts the comparisons of the filter `fetch`
   builds (R1), so the two cannot drift apart.

Trade-offs: a region with 18 sizes takes two requests where an unlimited API
would take one. Measured in `eastus` only; the API is global, and no
per-region limit is known.

**When this would be wrong**: the API lowers its cap, or counts something
else; batches of 17 would then be refused with the cap's `400` (R3) and
their sizes show no price for the rest of the session (until a restart or
`c` builds a new provider); the refusal line names the region but leaves the
header after `issueWindow` (2 minutes, `pkg/model/header.go:51`).

**Reversal action**: lower `maxFilterComparisons`; the R1 test follows.

Alternatives: probing the cap at run time (more requests, more `429`s);
batching by filter length (measured not to matter).

## D2. Only the cap's 400 is final

A `400` with the message `Invalid OData parameters supplied` is the cap's
refusal: the same filter gets the same answer (seen every minute before
this fix). Its sizes are stored as having no price and never asked again in
that provider; the region gets no retry time, so its other batches and new
sizes still go out. The log line has its own prefix, so a later success of
the region does not log it as resolved (fix-azure-retry R3) while those
sizes still have no price.

Any other `400` is retried, every `maxRetryAfter` (10 minutes): the other
`400` measured (`Invalid parameters supplied`) came from a value in the
filter, which R5 now keeps out, so another `400` is unexplained and may be
transient; 10 minutes keeps it from repeating every minute.

Justification: with R5, a value can no longer refuse a request; the cap's
message names a request shape that does not change between passes; the
rest of the region keeps working; the line tells the user why some nodes
show `-`.

Trade-offs: matching the message text ties the decision to the API's
wording. If the wording changes, a cap refusal becomes "another `400`" and
is retried every 10 minutes instead of never: never a one-minute loop.

**When this would be wrong**: the API answers the cap's message for a
transient fault; those sizes would show no price for the session (cost as
in D1). Or the cap changes and every batch is refused: the user sees `-`
for that region's sizes, logged once.

**Reversal action**: drop the final branch in `fetchWanted`; every `400`
then takes the 10-minute retry.

Alternatives: every `400` final (an unexplained `400` would cost prices for
the session); split a refused batch in halves until single sizes (finds one
size the API rejects, but turns one refusal into up to 33 requests and
invites a `429`); keep retrying every minute (the bug).

## D3. Check the values, not the escaping

A region or size outside the Kubernetes label value charset is stored as no
price in `NodePrice` and never queued. Label values cannot hold such a
value, so this only guards against a caller that is not a real node;
fixing the `''` escaping instead would need a measurement of what the API
accepts, for a case no node can produce.

**When this would be wrong**: Azure names a VM size or region with a
character outside the charset; it would show no price. Kubernetes labels
could not carry it either.

**Reversal action**: widen the pattern.

Alternatives: fix the escaping (unmeasured what works); reject at the
filter (a bad value would still be queued and retried).

## Invariants

- The filter of any request has at most `maxFilterComparisons` comparisons.
- No value outside the label value charset is put in a filter.
- A size is in exactly one of: wanted, priced (possibly with no price).
- A region's retry time is set only by a retried failure.
