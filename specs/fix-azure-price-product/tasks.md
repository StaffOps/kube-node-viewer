# Tasks: fix-azure-price-product

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `variantOf` ignores Cloud Services products and reads `Win` as Windows; equal dates keep the lower price; the fixture is real items from the 2026-11-04 answers (Cloud Services in both spellings, Low Priority, `Srs Win`, `Lasv3 Series Linux`, `ECasv7 Series VM`, a Dedicated Host sku)
  - _Covers: R1, R2, R3 · Depends on: — · Verified by: `go test -race ./pkg/azure/...` (fails on the old code)_
- [x] T2. Log once per region per provider when the 20-page cap cuts an answer short
  - _Covers: R4 · Depends on: — · Verified by: a fake API with more than 20 pages_
- [x] T3. Docs: CHANGELOG `Fixed`, dated correction in aks-pricing D3
  - _Covers: R1, R2, R3 · Depends on: T1 · Verified by: review_
- [ ] T4. Verify
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2, T3 · Verified by: `scripts/verify.sh`, `make lint`; `KNV_AZURE_LIVE=1 go test -run Live ./pkg/azure/` against the public API (sizes and prices in the test); on an AKS cluster (owner): a Linux node of a Dasv5, Easv5 or Bsv2 size shows the Linux price and a Windows node keeps the Windows price, or the note says the cluster has no affected size_

> **Partly verified 2026-11-04.** `go test -race ./pkg/azure/` passes, and
> the new tests fail on the old `pricing.go` (Linux E4as_v5 0.458, D2as_v5
> 0.178, B2s_v2 0.105, `Srs Win` read as Linux, Linux Spot NCC40ads_H100_v5
> 1.629936); after the code review a Spot `Srs` case and the real
> Dedicated Host item (asserted never to price a node) were added. `make lint` clean;
> `scripts/verify.sh` green except this open task.
> `KNV_AZURE_LIVE=1 go test -run Live ./pkg/azure/` against the public
> API: westeurope E4as_v5 Linux 0.274 / Windows 0.458, B2s_v2 0.096 /
> 0.105, L8as_v3 0.744 / 1.112; eastus2 D2as_v5 0.086 / 0.178,
> NCC40ads_H100_v5 6.98 / 8.82 — all as expected.
> **Not run on a cluster: the owner runs it on AKS**; this task stays open
> until then.
