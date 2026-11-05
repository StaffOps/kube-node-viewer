# Tasks: fix-azure-price-product

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [ ] T1. `variantOf` uses only items whose product name starts with `Virtual Machines `; other products are logged once per run; the fixture gets a Cloud Services item with a later date and the Windows price
  - _Covers: R1, R2 · Depends on: — · Verified by: `go test ./pkg/azure/...`_
- [ ] T2. Log when the 20-page cap cuts an answer short
  - _Covers: R3 · Depends on: — · Verified by: `go test ./pkg/azure/...`_
- [ ] T3. Docs: CHANGELOG `Fixed`, dated correction in aks-pricing D3, stale comments (`pkg/client/controller.go` override label, `pkg/model/aks_internal_test.go` "no price source")
  - _Covers: R1, R2 · Depends on: T1 · Verified by: review_
- [ ] T4. Verify
  - _Covers: R1, R2, R3 · Depends on: T1, T2, T3 · Verified by: `scripts/verify.sh`, `make lint`, the fixed provider against the live public API for the sizes in the bugfix table, a run on an AKS cluster (owner)_
