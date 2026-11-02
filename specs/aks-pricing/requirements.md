# Requirements: aks-pricing

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-02 (owner)

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
- [x] With `--disable-pricing` nothing is fetched.

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
