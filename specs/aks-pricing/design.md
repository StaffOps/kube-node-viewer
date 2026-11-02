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

## D2. Cache and fetch

The Azure provider keeps `region|size → {regular, spot, windows,
windowsSpot}` prices. `NodePrice` reads the cache; on a miss it records the
size as wanted and wakes a single fetch goroutine, which batches all wanted
sizes of a region into one filtered query, stores the result, marks sizes
without items as unknown, and calls the `OnUpdate` functions.

**Rationale:** the region has thousands of VM items (dozens of pages); a
cluster has a handful of sizes, so a filtered query is one page.

## D3. Matching

Items are kept when `type == Consumption`, `unitOfMeasure == 1 Hour`,
`currencyCode == USD` and the meter has no `Low Priority`. Spot is a meter
name ending in ` Spot`; Windows is a product name containing `Windows`. The
latest `effectiveStartDate` not in the future wins.
