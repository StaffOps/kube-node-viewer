# Bugfix: azure-price-product

> Status: active · Reported: 2026-11-04 (retroactive spec-refuter review of [aks-pricing](../aks-pricing/requirements.md)) · Reviewed: —

## Current behavior

`pkg/azure` asks the Retail Prices API for `serviceName eq 'Virtual
Machines'` items of a region and size, and `variantOf`
(`pkg/azure/pricing.go`) names each kept item regular, Spot, Windows or
Windows Spot from its meter (` Spot` suffix) and product name (contains
`Windows`). Every other item counts as **regular**, and the item with the
latest `effectiveStartDate` wins.

The API also returns, under the same service, the **Cloud Services** prices
of some series (product names such as `Easv5 Series CloudServices` or
`Dasv5 Series Cloud Services`, no `Virtual Machines` prefix, no `Windows`).
Their price is the Windows one, and their `effectiveStartDate` is often
later than the Linux item's, so a Linux node takes the Windows rate.

Live queries of the public API on 2026-11-04 (list prices, USD per hour):

| Region | Size | Linux item | Cloud Services item | Shown today |
|--------|------|-----------|---------------------|-------------|
| westeurope | Standard_E4as_v5 | 0.274 (2021-11-01) | 0.458 (2026-01-01) | 0.458 |
| eastus2 | Standard_D2as_v5 | 0.086 (2021-11-01) | 0.178 (2023-10-01) | 0.178 |
| westeurope | Standard_D4ads_v5 | 0.25 (2021-11-01) | 0.434 (2023-10-01) | 0.434 |

Sizes whose series has no Cloud Services item (Dsv5, BS, Av2, Dpdsv6 in the
same queries) are priced correctly. How many series and regions carry such
items is not measured.

The aks-pricing tests pass because their fixture has no Cloud Services
item.

## Expected behavior

Covers the aks-pricing R1 rule "any other node takes the one without
Windows" and its counter-example "MUST NOT show a Windows price for a Linux
node".

**R1** WHEN the API returns an item whose product name does not start with
`Virtual Machines ` THEN the system SHALL not use it for any node's price,
and SHALL log the product name once per run (`ignoring Azure price product
"<name>"`).

**R2** WHEN a Linux node's size has both a `Virtual Machines …` item and a
Cloud Services item with a later date THEN the system SHALL show the
`Virtual Machines …` price; Windows, Spot and Windows Spot nodes keep the
prices they get today.

**R3** WHEN a region's answer needs more than the 20 pages the provider
reads THEN the system SHALL log that the answer was cut short, so a size
shown without a price can be traced.

## Unchanged behavior

- Spot is a meter ending in ` Spot`, Windows a product name containing
  `Windows`, Low Priority meters and non-hourly or non-USD items are
  skipped, and the latest `effectiveStartDate` not in the future wins among
  the kept items.
- One request per region for all wanted sizes; a size the API does not
  list is not asked again; a failure is retried no sooner than a minute
  later.
- The `kube-node-viewer/instance-price` label still overrides the lookup.
- AWS nodes make no Azure request.

## Root cause

D3 of [aks-pricing design](../aks-pricing/design.md) assumed every item of
the `Virtual Machines` service is a VM price that differs only by meter
and OS; Cloud Services items share the service and the size name but are
another product.
