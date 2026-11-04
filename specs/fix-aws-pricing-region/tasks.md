# Tasks: fix-aws-pricing-region

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `pricingProvider`: initial callback before the goroutine, callbacks under `mu`, EC2 client in the lookup region (the static provider of a region was dropped with R6, 2026-11-04)
  - _Covers: R4, R6 · Depends on: — · Verified by: `go test -race ./pkg/aws`_
- [x] T2. `aws.ByRegion` (`pkg/aws/region.go`) and its tests
  - _Covers: R1, R2, R3, R5, R6 · Depends on: T1 · Verified by: `go test -race ./pkg/aws -run 'Region|Zone'`_
- [x] T3. `sessions.pricing` uses it when the URL names no region and the server is not AKS; `session_test.go`
  - _Covers: R1, R2, R6 · Depends on: T2 · Verified by: `go test ./cmd/kube-node-viewer -run 'Pricing|Advisor'`_
- [x] T4. README "Region" paragraph, CHANGELOG `Fixed`, `TODO.md`: remove the item, add the Auto Mode fee region
  - _Covers: R1, R2, R3, R4, R5, R6 · Depends on: T1, T2, T3 · Verified by: review_
- [x] T5. Verify: `scripts/verify.sh` and `make lint` green; run through `kubectl proxy` with `AWS_REGION` set to another region, on-demand and spot prices equal to the direct run
  - _Covers: R1, R2, R3, R4, R5, R6 · Depends on: T1, T2, T3, T4 · Verified by: dated note here (what was checked, not which cluster)_

> **Verified 2026-11-04.** `scripts/verify.sh`, `make lint` and
> `go test -race ./pkg/aws ./cmd/kube-node-viewer` green. On an EKS cluster
> (16 on-demand nodes in one region, no spot nodes), from a scratch
> directory, driving the keys through a pseudo terminal and reading the
> export (`e`): one run through the EKS URL, one through `kubectl proxy`
> (`http://127.0.0.1:8001`) with `AWS_REGION=us-gov-west-1`, a region whose
> built-in table differs from the cluster's. With the fix every node's price
> and price kind were the same in both runs (16/16); the binary without it,
> same steps, matched 0/16. A first try with `AWS_REGION=eu-west-1` matched
> 16/16 for both binaries, because the built-in tables fall back to the
> `us-east-1` one for that region and the lookups had not returned within
> the run, so it proved nothing. Spot nodes were not checked (none in the
> cluster); spot routing is covered by `TestByRegionAdvisorPrices` and R4 by
> `TestPricingProviderWithoutRegionUsesOneRegion`.
