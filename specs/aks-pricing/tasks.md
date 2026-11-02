# Tasks: aks-pricing

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `pkg/azure` provider: cache, batched fetch, matching, retry
  - _Covers: R1, R2 · Depends on: — · Verified by: unit tests against an httptest server_
- [x] T2. `pricing.ByCloud` and session wiring; advisor from the AWS provider
  - _Covers: R2, R3 · Depends on: T1 · Verified by: unit tests, a run against the owner's AKS cluster_
- [x] T3. README, CHANGELOG
  - _Covers: — · Depends on: T1–T2 · Verified by: review_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Azure catalog and recommendations | owner runs AKS clusters in production |
| Reservations / Savings Plans prices | someone needs effective rather than list cost |
