# Design: fix-azure-price-batch

> Decisions for [bugfix](bugfix.md).

## Current vs target

| | Now | After |
|--|-----|-------|
| Request per region and pass | one, all wanted sizes | one per batch of at most 17 sizes, in order |
| `400` answer | retried every minute | not retried; its sizes have no price; logged once per region |
| Failure in a batch | — | earlier batches keep their prices; later batches wait with the region |

Components touched: `pkg/azure/pricing.go` (`fetchWanted`, `fetch`, a
`refused` set per region for the log line), `pkg/azure/retry.go` unchanged,
tests in `pkg/azure/batch_test.go`.

## D1. A fixed batch size from the measured cap

`maxSizesPerRequest = maxFilterComparisons - baseConditions` with
`maxFilterComparisons = 20` (measured 2026-11-10) and `baseConditions = 3`,
package-level constants in `pkg/azure/pricing.go`. Sizes are sorted, so the
same wanted set always gives the same batches.

Justification: the limit was measured to count comparisons, not filter
length (bugfix, Current 1); deriving the size from the base conditions keeps
it right if a condition is added; a test counts the comparisons of the
filter `fetch` builds (R1), so the two cannot drift apart.

Trade-offs: a region with 18 sizes takes two requests where an unlimited API
would take one.

**When this would be wrong**: the API lowers its cap, or counts something
else; batches of 17 would then be refused (`400`, R3) and their sizes show no
price, with the log line naming the region.

**Reversal action**: lower `maxFilterComparisons`; the R1 test follows.

Alternatives: probing the cap at run time (more requests, more `429`s);
batching by filter length (the length was measured not to matter).

## D2. A 400 is final for its batch

A `400` means the API refuses the request itself: sending the same filter
again gets the same answer (seen every minute before this fix). Its sizes
are stored as having no price and never asked again in that provider; the
region gets no retry time, so its other batches and new sizes still go out.
The log line has its own prefix, so a later success of the region does not
log it as resolved (fix-azure-retry R3) while those sizes still have no
price.

Justification: stops a request that cannot succeed; the rest of the region
keeps working; the line tells the user why some nodes show `-`.

Trade-offs: a `400` caused by something transient (an API fault answered as
`400`) leaves those sizes without a price until the program restarts or `c`
builds a new session.

**When this would be wrong**: the API answers `400` for transient faults;
then retry a `400` with the normal delay, but a capped number of times.

**Reversal action**: drop the `400` branch in `fetchWanted`; the retry path
handles it as before.

Alternatives: split a refused batch in halves until single sizes (finds one
size the API rejects, but turns one refusal into up to 33 requests and
invites a `429`); keep retrying with a growing delay (still a request that
cannot succeed).

## Invariants

- The filter of any request has at most `maxFilterComparisons` comparisons.
- A size is in exactly one of: wanted, priced (possibly with no price).
- A region's retry time is set only by a retried failure.
