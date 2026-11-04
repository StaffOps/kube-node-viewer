# Design: fix-aws-pricing-region

> Implements [bugfix](bugfix.md).

## Current state

`sessions.pricing` (`cmd/kube-node-viewer/session.go`) builds one
`aws.NewPricingProvider(ctx, cfg)` per connection: `cfg.Region` is the EKS
URL's region when there is one, else the SDK's. The provider seeds
`onDemandPrices` from the static table of its region (`getStaticPrices`)
and refreshes on-demand, spot and Fargate prices (the region's) and the
Auto Mode fee (`us-west-2`'s) from the AWS APIs in a goroutine started by
the constructor, every 12 h, then calls its `OnUpdate` callbacks, which
make the controller reprice every node (`pkg/client/controller.go`,
`RefreshNodePrices`). `OnUpdate` appends to `onUpdateFuncs` without a
lock while that goroutine may read it.

## Target state

When the URL names no region (and the server is not AKS), the `"aws"`
provider and the fallback of `pricing.ByCloud` are an `aws.ByRegion`
(new, `pkg/aws/region.go`):

```go
type ByRegion struct {
	build    func(region string, onUpdate func()) *pricingProvider
	fallback string // the SDK region, us-west-2 if none

	mu        sync.Mutex
	providers map[string]*pricingProvider // built on first use
	nodes     map[string]string           // node name → region, for primary
	zones     map[string]string           // zone → region, from priced nodes
	onUpdate  []func()
}
```

- `NodePrice(n)`: `region(n)` = when `n.Cloud() == "aws"`: the region
  label if it is a known AWS region (`^[a-z]{2}(-gov|-iso[a-z]?)?-[a-z]+-\d+$`),
  else `zoneRegion(n.Zone())`; anything else → `fallback`. Under `mu`: get or
  build that region's provider, record the node and zone; then, outside
  `mu`, delegate.
- `build(region, fanout)` passes `fanout` to the provider before its
  goroutine starts (new `newPricingProvider(ctx, cfg, onUpdate)`), so no
  callback is added to a running provider. `fanout` copies `b.onUpdate`
  under `b.mu` and calls the copies without it: the callback is
  `RefreshNodePrices` → `NodePrice`, which takes `b.mu`.
- `OnUpdate(f)`: appends to `b.onUpdate` under `mu`. The controller calls
  it before `Start` (`controller.go`), so it is stored before any provider
  is built.
- `OnDemandPrice(t)`: the primary region (most nodes in `nodes`, ties
  alphabetical) if its provider exists, else the static table of
  `fallback` (no build). `SpotPriceInZone(t, zone)`: the provider of
  `zones[zone]`, else of `zoneRegion(zone)`, else false.
- `zoneRegion(zone)` = the match of `^[a-z]{2}(-[a-z]+)+-\d+` (`eu-west-1a` →
  `eu-west-1`, `us-gov-west-1a` → `us-gov-west-1`, `us-east-1-bos-1a` →
  `us-east-1`, `us-east-1-wl1-bos-wlz-1` → `us-east-1`), checked with the
  region shape; otherwise empty.

`NewPricingProvider` sets `cfg.Region` to its lookup region (`us-west-2`
when empty) before building the EC2 client (R4), and `OnUpdate` /
`updatePricing` copy `onUpdateFuncs` under `mu`.

`-disable-pricing` without a URL region: a `ByRegion` whose `build` returns
a static provider of the region (no client, no goroutine) (R6).

With an EKS URL, AKS server or `-disable-pricing` with a URL region,
`sessions.pricing` builds what it builds today.

## Components touched

| Path | Change |
|------|--------|
| `pkg/aws/region.go` (new, Apache header) | `ByRegion`, `NewRegionalPricingProvider(ctx, cfg)`, `NewRegionalStaticPricingProvider()`, `zoneRegion`, `knownRegion` |
| `pkg/aws/region_test.go` (new, Apache header) | table tests with a recording `build` |
| `pkg/aws/pricing.go` | `newPricingProvider` with an initial callback; EC2 client in the lookup region; `onUpdateFuncs` under `mu`; static provider for a region |
| `cmd/kube-node-viewer/session.go` | `pricing`: `ByRegion` when the URL names no region and the server is not AKS |
| `cmd/kube-node-viewer/session_test.go` (new) | `pricing` cases per server kind; `newAdvisor` non-nil over `ByRegion` |
| `README.md` ("Region"), `CHANGELOG.md`, `TODO.md` | text, `Fixed` entry, Auto Mode item |

## Decisions

### D1: one provider per AWS node region, built on first use

**Choice**: build a region's provider the first time an AWS node of that
region is priced, and keep it for the connection.

**Justification, strongest first**:
1. The node is the only region source when the URL has none, and nodes
   arrive after `pricing` returns.
2. A cluster spans one region in practice, so this builds one provider,
   the same API calls as today; a multi-region cluster gets each region's
   prices instead of one region's for all.
3. Each provider seeds static prices synchronously, so the first frame
   shows the right region's list prices before any API call returns.
4. Building does no network I/O (clients and config files only; the
   calls run on the provider's goroutine), so holding `mu` across it
   blocks informers only for file-read time.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| N regions → N refresh goroutines, N sets of API calls and N full repricings every 12 h | N is 1 for nearly every cluster |
| A node of another partition (`cn-*`, `us-gov-*`) under commercial credentials makes calls that fail, logged in the header | today the same node gets commercial prices silently; the error says why |
| Recommendations' on-demand prices come from the main region only (R5) | the advisor asks by instance type alone; a multi-region cluster is rare |

**When this would be wrong**: clusters routinely carry wrong region labels,
or a cluster spans many regions and the API calls cost more than they are
worth.

**Reversal action**: in `sessions.pricing`, build the single provider again
(one `if`); delete `pkg/aws/region.go`. Minutes.

**Alternatives discarded**: wait for the first node and build one provider
then (a two-region cluster still gets one region's prices); keep the SDK
region and warn when it differs (the prices stay wrong); route by label
only, without the zone (NodeClaim-only nodes and nodes without the label
would go to the SDK region while their provider ID names the zone).

> **Corrected 2026-11-04 (implementation and diff review).**
> - D1 point 3 is wrong for most regions: the static seed is the
>   `us-east-1` table unless the region is one of the four the tables hold,
>   so the first frame shows those prices until the lookup returns, as it
>   always has with an EKS URL.
> - `NewRegionalStaticPricingProvider` and R6 are dropped (see the bugfix).
> - The main region is counted per provider ID (a Karpenter node keeps it
>   from NodeClaim to Node), with a per-region count kept as nodes are
>   priced, not recounted on each Recommendations lookup; deleted nodes are
>   not forgotten during a connection. `OnDemandPrice` uses the fallback
>   provider when no AWS node has a region.
> - The R4 test lives in `region_test.go`.
> - "N is 1 for nearly every cluster" and "blocks informers only for
>   file-read time" are [ASSUMPTION]s, not measured.

## Invariants

- With an EKS URL, an AKS server or `-disable-pricing` with a URL region,
  the providers built and the AWS calls made are the same as before.
- No callback is added to a provider whose goroutine is running; no
  callback runs while `ByRegion.mu` is held.
- Every provider is built with the connection's `ctx`, so a context switch
  stops them all.

## Test plan

- `region_test.go` with a recording `build`: AWS nodes in `eu-west-1` (label),
  `ap-southeast-2` (zone label only) and `us-east-1` (provider ID only) →
  three regions; a kind node, a GKE-style `us-central1` label and an
  on-prem `dc1` label → fallback, no build; `OnUpdate` before builds reaches
  each provider's callback once, and a callback that calls `NodePrice` does
  not deadlock; `SpotPriceInZone` and `OnDemandPrice` routing, primary by
  count with ties; `zoneRegion` and `knownRegion` tables; `go test -race`.
- `pricing_test.go`: a provider built with no region looks up spot in
  `us-west-2` (EC2 client region).
- `session_test.go`: `pricing` per server kind returns the expected
  provider types; `newAdvisor(pricing(...))` is non-nil for the EKS URL and
  for `ByRegion`.
- A run through `kubectl proxy` on a cluster the owner allows, with
  `AWS_REGION` set to another region: on-demand and spot nodes show the
  same prices as the direct (EKS URL) run.
