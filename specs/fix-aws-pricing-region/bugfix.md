# Bugfix: aws-pricing-region

> Status: active · Reported: 2026-11-03 · Reviewed: spec-refuter 2026-11-04 (findings applied)

Listed in `TODO.md` (Follow-ups, "AWS pricing region when the API server
is not an EKS URL").

## Current behavior

When the API server URL does not name an EKS region (a private endpoint
behind a custom DNS name, `kubectl proxy`, an API gateway, a self-managed
cluster on EC2), `sessions.pricing` (`cmd/kube-node-viewer/session.go`)
builds the AWS provider from the AWS SDK's config: its region comes from
`AWS_REGION`, `AWS_DEFAULT_REGION` or the profile. In
`NewPricingProvider` (`pkg/aws/pricing.go`):

- on-demand and Fargate prices are looked up for that region, or for
  `us-west-2` when the SDK has none;
- spot prices use the EC2 client built from the SDK config itself, so with
  no SDK region `DescribeSpotPriceHistory` fails, `spotPrices` stays empty
  and spot nodes show no price;
- the Auto Mode management fee is always the `us-west-2` one (a separate
  limitation, below).

The header shows the region of the first node in display order
(`topology.kubernetes.io/region`, `pkg/model/header.go`).

Reproduction (made-up values): nodes labelled
`topology.kubernetes.io/region=eu-west-1`, reached through `kubectl proxy`
(`server: http://127.0.0.1:8001`). With `AWS_REGION` unset the header says
`Region: eu-west-1`, on-demand nodes show `us-west-2` prices and spot nodes
none; with `AWS_REGION=us-east-1` every price is a `us-east-1` one. No
warning either way.

## Expected behavior

**R1** WHEN the API server URL names no EKS region and an AWS node (provider
ID `aws://…`) is in a known AWS region THEN the system SHALL price it with
the on-demand, spot and Fargate prices of that region. The node's region is
its `topology.kubernetes.io/region` label, or else the region of its zone
(the zone label, or the zone in its provider ID); a region is known when it
has the shape of an AWS region (`us-east-1`, `us-gov-west-1`,
`cn-north-1`).

**R2** WHEN a node is not an AWS node, or its region is unknown or not
known to AWS (`dc1`, `us-central1`) THEN the system SHALL price it as
today, in the SDK's region (or `us-west-2`), and build no other provider
for it.

**R3** WHEN the nodes span several AWS regions THEN each node SHALL be
priced in its own region.

**R4** WHEN the provider is built without an SDK region THEN spot prices
SHALL be looked up in the same region as on-demand prices (`us-west-2`),
not fail.

**R5** WHEN the Recommendations screen asks for a spot price in a zone THEN
the system SHALL answer from that zone's region; WHEN it asks for an
on-demand list price THEN it SHALL answer from the region with the most
priced nodes (ties: the alphabetically first), or the SDK region before any
node is priced. In a cluster spanning several regions, savings for nodes
outside that region mix two regions' prices; this is a known limitation.

**R6** WHEN pricing is off (`-disable-pricing`) and the URL names no
region THEN an AWS node SHALL get the static list price of its region
(R1's rule), with no AWS call.

_Test: withdrawn (R6) — by the 2026-11-04 correction below._

> **Corrected 2026-11-04 (implementation and diff review).** R6 is
> withdrawn: the built-in price tables hold only `us-east-1`,
> `us-gov-east-1`, `us-gov-west-1` and `cn-north-1` (every other region falls
> back to `us-east-1`'s), so per-region static prices would change almost
> nothing; `-disable-pricing` stays as it was. For the same reason a
> region is "known" by its shape only (R1, R2): a label that only looks like
> a region (`us-east-9`) gets a provider whose lookups find nothing. The
> "changed on purpose" line holds only while no node at all is priced: a
> non-AWS node builds the fallback provider, which makes today's calls.

## Unchanged behavior

- An EKS API server URL sets the pricing region, whatever the node labels
  say (also after a context switch); one provider, the same calls.
- AKS servers (`*.azmk8s.io`) make no AWS lookup; Azure nodes go to the
  Azure provider; other nodes to the static provider (`AWS_REGION`, else
  `us-east-1`), as before.
- The `kube-node-viewer/instance-price` label still overrides the lookup,
  and such a node causes no provider to be built.
- The Auto Mode management fee stays the `us-west-2` one in every case
  (`updateAutoManagementPricing`); `TODO.md` lists it.

  > Correction 2026-11-04: the owner dropped this follow-up (no Auto Mode
  > cluster to test on); it is no longer in `TODO.md`.

- The header's region comes from the same places as today.
- A context switch stops every provider of the old connection.

Changed on purpose: a cluster with no AWS node priced yet builds no AWS
provider and makes no AWS call until one is.

## Root cause

`sessions.pricing` (`cmd/kube-node-viewer/session.go`) picks one region per
connection, before any node is seen, and `pricingProvider`
(`pkg/aws/pricing.go`) holds one region's tables; nothing reads the node's
region when pricing. `NewPricingProvider` defaults the lookup region to
`us-west-2` but builds its EC2 client from the config without it.
