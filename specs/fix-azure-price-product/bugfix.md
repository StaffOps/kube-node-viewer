# Bugfix: azure-price-product

> Status: active · Reported: 2026-11-04 (retroactive spec-refuter review of [aks-pricing](../aks-pricing/requirements.md)) · Reviewed: 2026-11-04 (spec-refuter; 1 BLOCKING, 2 HIGH, 5 MEDIUM, 3 LOW, applied; code 2026-11-04, 1 MEDIUM, 3 LOW, applied)

## Current behavior

`pkg/azure` asks the Retail Prices API for the `serviceName eq 'Virtual
Machines'` items of a region and its wanted sizes, and `variantOf`
(`pkg/azure/pricing.go`) names each kept item regular, Spot, Windows or
Windows Spot: Spot is a meter ending in ` Spot`, Windows a product name
containing `Windows`, **anything else is regular**, and the latest
`effectiveStartDate` wins (a later date strictly replaces an earlier one,
so on equal dates the item the API lists first wins).

Two kinds of item break this:

- **Cloud Services prices.** The same service lists, for some series, a
  product named `<series> Series CloudServices` or `<series> Series Cloud
  Services`. It has no `Windows` in its name and carries the Windows price,
  so it counts as regular; when its date is later than the Linux item's, or
  equal and listed first, a Linux node shows the Windows rate.
- **`Srs Win` products.** `Virtual Machines NCCadsv5 Srs Win` is the
  Windows product of that series but has no `Windows` in its name, so it
  also counts as regular and ties on date with the Linux item.

Reproduce (westeurope, `Standard_E4as_v5`):

```bash
curl -sG https://prices.azure.com/api/retail/prices --data-urlencode \
  "\$filter=serviceName eq 'Virtual Machines' and priceType eq 'Consumption' and armRegionName eq 'westeurope' and armSkuName eq 'Standard_E4as_v5'"
```

lists, among others, `Virtual Machines Easv5 Series | E4as v5 | 0.274 |
2021-11-01` and `Easv5 Series CloudServices | E4as v5 | 0.458 |
2026-01-01`; a Linux node of that size shows 0.458 today.

Measured on 2026-11-04 from the whole westeurope and eastus2 answers (list
prices, USD per hour):

| Region | Size | Linux item | Cloud Services item | Shown today |
|--------|------|-----------|---------------------|-------------|
| westeurope | Standard_E4as_v5 | 0.274 (2021-11-01) | 0.458 (2026-01-01) | 0.458 |
| westeurope | Standard_D4ads_v5 | 0.25 (2021-11-01) | 0.434 (2023-10-01) | 0.434 |
| westeurope | Standard_B2s_v2 | 0.096 (2023-09-01) | 0.105 (2023-09-01) | either, by API order |
| eastus2 | Standard_D2as_v5 | 0.086 (2021-11-01) | 0.178 (2023-10-01) | 0.178 |

Sizes with a Cloud Services item, westeurope / eastus2: Easv5 20/20,
Eadsv5 20/20, Dasv5 8/8, Dadsv5 8/8, Bpsv2 6/5, Bsv2 5/3, Basv2 4/5, 71/69
in all. No Cloud Services item is a Spot meter, and none is cheaper than
the Linux item, so the Cloud Services error only ever raises a Linux price.
The `Srs Win` error also hits Spot and Windows: a Linux Spot
`Standard_NCC40ads_H100_v5` showed the Windows Spot price (eastus2
1.629936 instead of 1.289904; westeurope 1.984752 instead of 1.64472), and
Windows nodes of that series had no price. Other regions are not measured.

> **Corrected 2026-11-04.** The counts above are sizes whose price changed
> in the captured listing order, not sizes with a Cloud Services item. Each
> region has 87 sizes with one (Easv5 20, Eadsv5 20, Bsv2 11, Basv2 11,
> Bpsv2 9, Dasv5 8, Dadsv5 8). Replaying the old Go code on the captured
> answers changes 72 prices in westeurope and 74 in eastus2 (75 with
> NCC40ads_H100_v5): all 56 Easv5/Eadsv5/Dasv5/Dadsv5 sizes showed the
> Windows rate through a later date; the 31 B-series sizes depended on the
> API's order (same date). Measured by replaying `pkg/azure` on the
> captured 2026-11-04 answers.

The aks-pricing tests pass because their fixture has neither kind of item.

## Expected behavior

Covers the aks-pricing R1 rule "any other node takes the one without
Windows" and its counter-example "MUST NOT show a Windows price for a Linux
node".

**R1** WHEN the API returns an item whose product name, lower-cased with
spaces removed, contains `cloudservices` THEN the system SHALL not use it
for any node's price. It SHALL NOT log it: logs reach the header as a red
line for two minutes (`cmd/kube-node-viewer/errlog.go`), and this item is
expected on every session with an affected size.

**R2** WHEN a product name has `Windows` in it, or `Win` as a separate word
(split on spaces and `-`), THEN the system SHALL treat its items as Windows
prices.

**R3** WHEN two kept items of the same size and variant have the same
`effectiveStartDate` THEN the system SHALL keep the lower price, whatever
the order the API lists them in. _Why:_ on a tie the error to avoid is a
Windows or Cloud Services rate shown for Linux, which is always the higher
one.

**R4** WHEN a region's answer needs more than the 20 pages the provider
reads THEN the system SHALL log once per region per provider that the
answer was cut short. This is defensive: a whole region is about 10 pages
unfiltered, and a filtered query one.

## Unchanged behavior

- Products without the `Virtual Machines ` prefix that are not Cloud
  Services keep their prices (`Lasv3 Series Linux`, `ECasv7 Series VM`,
  `DCasv5-series Linux`, …).
- Low Priority meters and non-hourly or non-USD items are skipped; a later
  `effectiveStartDate` not in the future replaces an earlier one.
- A Linux node MUST NOT show a Spot price, and a regular node a Spot one.
- One request per region for all wanted sizes; a size the API does not
  list gets no price and is not asked again; a failure is retried no sooner
  than a minute later.
- `--disable-pricing` fetches nothing; the `kube-node-viewer/instance-price`
  label still overrides the lookup; AWS nodes make no Azure request.

## Not done

- The filter stays client-side. The API rejects `not startswith(…)` in
  `$filter`, and a server-side allow-list of product names would drop real
  products such as the ones above.
- One retry delay is shared by all regions (aks-pricing review, LOW).

## Root cause

D3 of [aks-pricing design](../aks-pricing/design.md) assumed every item of
the `Virtual Machines` service is a VM price that differs only by meter and
by `Windows` in the product name.
