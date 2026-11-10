# Requirements: aks-pricing

> Status: shipped (v0.9.0, 2026-11-02) · Owner: Carlos Gomes · Reviewed: 2026-11-04 (spec-refuter, retroactive; 1 HIGH, 3 MEDIUM, 5 LOW; applied), earlier 2026-11-02 (owner)

> **Corrected 2026-11-05.** The 2026-11-02 review was by the owner, who
> is also the author, so it was not independent; the spec and the code
> landed in one commit, and T2's AKS run left no note of what it checked.
> A retroactive spec-refuter review on 2026-11-04 found one HIGH: Azure
> Cloud Services items at the Windows rate counted as Linux prices, so
> Linux nodes of the Easv5/Eadsv5/Dasv5/Dadsv5 and B-v2 series showed the
> Windows price, against R1 and the counter-example
> ([fix-azure-price-product](../fix-azure-price-product/bugfix.md)).

## Context

Since v0.9.0 the viewer reads AKS clusters right (agent pool, capacity,
region) but has no price for their nodes: `PRICE/H` is `-` and the header
says `prices unknown`. Azure publishes its list prices through the Retail
Prices API (`https://prices.azure.com/api/retail/prices`), public and without
authentication, so prices need no Azure login or permission.

The tool still only reads: the API is a read-only public endpoint; nothing is
sent to it but a region and VM size names.

## Usage example

```text
$ kube-node-viewer --context example1-demo1-aks
│ Nodes:    11 of 11 shown · 159 pods · $701.400/month (global cluster)
  NAME                               … INSTANCE           PRICE/H  CAPACITY  NODEGROUP
  aks-userpool-12345678-vmss000003   … Standard_D2pds_v6  $0.0918  OnDemand  userpool
```

On start the prices show `-` for a second or two, then fill in.

> **Corrected 2026-11-05.** "A second or two" was never measured: the
> prices show `-` until the first answer arrives (one request per region).

> **Corrected 2026-11-10 ([fix-azure-price-batch](../fix-azure-price-batch/bugfix.md) R1).**
> One request per batch of at most 17 sizes of a region, not one per region.

## Requirements

**R1** WHEN a node's provider ID starts with `azure://` THEN the system SHALL
price it from the Retail Prices API by its region label
(`topology.kubernetes.io/region`) and VM size
(`node.kubernetes.io/instance-type`).

- [x] Price used: `priceType eq 'Consumption'`, unit `1 Hour`, USD. A Spot
  node (`kubernetes.azure.com/scalesetpriority=spot`) takes the `… Spot`
  meter; any other node takes the regular meter. `Low Priority` meters are
  never used. A Windows node (`kubernetes.io/os=windows`) takes the meter
  whose product name has `Windows`; any other node the one without.
- [x] When several items match, the one with the latest
  `effectiveStartDate` not after now wins.
- [x] The instance-price override label still wins over the API.

**R2** WHEN an Azure node has no cached price THEN the system SHALL look it
up in the background and re-price the nodes when the answer arrives.

- [x] Lazy: nothing is fetched until an Azure node is seen, so EKS sessions
  make no call to Azure. One request per region asks for every unknown size
  seen so far (`armSkuName eq 'a' or …`), following `NextPageLink`.
- [x] Off the UI and informer goroutines; when prices arrive the nodes are
  re-priced and the screen redraws.
- [x] A failed request is logged (it shows in the header like other errors)
  and retried no sooner than 1 minute later; a size the API doesn't know is
  not asked again in the session.
- [x] Timeout 20 s per request.

> **Corrected 2026-11-10 ([fix-azure-price-batch](../fix-azure-price-batch/bugfix.md) R1, R3).**
> A region's unknown sizes go out in batches of at most 17, one request
> each (the API refuses a filter with more than 20 comparisons). A request
> the API refuses as over its cap (`400`, `Invalid OData parameters
> supplied`) is not retried, and its sizes have no price; any other `400`
> is retried no sooner than 10 minutes later.
- [x] With `--disable-pricing` nothing is fetched.

> **Corrected 2026-11-05.** These boxes were ticked "by unit tests" that did
> not check them all. Tests added on 2026-11-05: the retry delay is at least
> 1 minute and no request is made before it
> (`TestProviderRetryAndTimeoutDefaults`, `TestNoRequestBeforeTheRetryDelay`),
> the 20 s timeout (same test), no Azure provider with `--disable-pricing`
> (`TestSessionPricingPerServer`), an `aws://` node through `ByCloud`
> makes no Azure request (`TestAWSNodeMakesNoAzureRequest`), and the
> override label on an Azure node (`TestUpdatePriceOverrideOnAKSNode`).
> "A failed request is logged (it shows in the header)" is still checked
> only by reading the code.

**R3** WHEN only Azure nodes are shown THEN the Recommendations screen SHALL
keep saying it covers EKS nodes only (no Azure catalog yet).

## Counter-examples

- MUST NOT ask for Azure credentials or read `~/.azure`.
- MUST NOT show the Spot price for a regular node, or a Windows price for a
  Linux node.
- MUST NOT block the screen while prices load.

## Out of scope

Azure Reservations and Savings Plans, other currencies, disk and network
costs, AKS control-plane (Standard/Premium tier) fees, Azure recommendations.
