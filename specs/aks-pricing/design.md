# Design: aks-pricing

> Implements [requirements](requirements.md).

## D1. A provider per cloud

`pkg/azure` holds a `pricing.Provider` for Azure nodes. The session builds a
`pricing.ByCloud` that sends each node to the provider of its provider-ID
scheme (`aws`, `azure`), with AWS as the fallback for any other scheme so
behaviour for EKS, kind and the override label is unchanged.

**Rationale:** the controller already re-prices every node on `OnUpdate`;
dispatching per node keeps it unaware of clouds and lets one cluster mix
both, which a per-session choice by server URL could not.

> **Corrected 2026-11-05.** The decision format asks for more than a
> rationale (retroactive review). Trade-offs: every node pays one map
> lookup on its provider-ID scheme; an unknown scheme falls back to AWS,
> which prices nothing without AWS labels. **When this would be wrong**: a
> cloud whose provider IDs do not name it, or nodes priced by a source
> other than their cloud. **Reversal action**: a per-session provider
> chosen by the API server URL, as before v0.9.0.

## D2. Cache and fetch

The Azure provider keeps `region|size → {regular, spot, windows,
windowsSpot}` prices. `NodePrice` reads the cache; on a miss it records the
size as wanted and wakes a single fetch goroutine, which batches all wanted
sizes of a region into one filtered query, stores the result, marks sizes
without items as unknown, and calls the `OnUpdate` functions.

**Rationale:** the region has thousands of VM items (dozens of pages); a
cluster has a handful of sizes, so a filtered query is one page.

> **Corrected 2026-11-05.** Trade-offs (retroactive review): one failure
> time is shared by every region, so a failure in one region delays new
> sizes in all of them by the retry delay; a fetch stops after 20 pages.
> **When this would be wrong**: clusters spanning many regions, or sizes
> whose answer is longer than 20 pages. **Reversal action**: keep the
> failure time per region, and raise or drop the page cap.

> **Corrected 2026-11-05.** The shared failure time is gone: each region
> keeps its own retry time, a 429 or 503 retry header is honoured, and a
> resolved failure leaves the header
> ([fix-azure-retry](../fix-azure-retry/bugfix.md)).

## D3. Matching

Items are kept when `type == Consumption`, `unitOfMeasure == 1 Hour`,
`currencyCode == USD` and the meter has no `Low Priority`. Spot is a meter
name ending in ` Spot`; Windows is a product name containing `Windows`. The
latest `effectiveStartDate` not in the future wins.

> **Corrected 2026-11-04.** The `Virtual Machines` service also lists
> Cloud Services products (`… Series CloudServices`, `… Series Cloud
> Services`) at the Windows rate, and some Windows products name it `Win`
> (`… Srs Win`); both counted as regular, so Linux nodes of those series
> showed the Windows price. Cloud Services items are now ignored, `Win` as
> a word means Windows, and on equal dates the lower price wins —
> [fix-azure-price-product](../fix-azure-price-product/bugfix.md).

> **Corrected 2026-11-05.** Trade-offs (retroactive review): the rules are
> read from product and meter names, which Azure does not document as
> stable. **When this would be wrong**: a product family that names its
> variant differently, which happened (see the correction above and
> [fix-azure-price-product](../fix-azure-price-product/bugfix.md)).
> **Reversal action**: match on the API's `skuName`/`productId` fields
> instead of names.
