# Design: aks-recommendations

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Screen gate | analysis runs when the shown cloud is `aws`; `shownCloud` returns `aws` as soon as one AWS node is shown, and also when no node is shown | `pkg/model/recommendations.go:63,196-207` |
| Analysis input | every selected node, whatever its cloud | `pkg/model/recommendations.go:84-86` |
| Cloud checks in rules | catalog rules skip sizes `Spec` doesn't know; `idle()` and `oldNodes()` have none, so Azure nodes in a mixed view get AWS-worded rows | `pkg/recommend/recommend.go:242,601` |
| Rightsize candidates | a candidate without a price is dropped before the fit check | `pkg/recommend/recommend.go:485-491` |
| Advisor | one, AWS only | `cmd/kube-node-viewer/session.go:240-282`, `pkg/model/recommendations.go:28-34` |
| `--disable-pricing` | the session's provider is `aws.NewStaticPricingProvider()` with no `ByCloud`; EKS rows show with money hidden | `cmd/kube-node-viewer/session.go:205-206`, `pkg/model/recommendations.go:161` |
| Rule input | `Spec(type)`, `OnDemand(type)`, `SpotIn(type, zone)`, `Types()`; no cloud, region, OS or `status.capacity` | `pkg/recommend/recommend.go:97-114` |
| Kind label | `Kind.String()`, no cloud; used by the row, the math title and the popup | `pkg/recommend/recommend.go:129`, `pkg/model/recommendations.go:164,249,253` |
| Families | `family.size` split on `.`; fixed AWS tables; architecture compared as `x86_64` | `pkg/recommend/recommend.go:227-230,551`, `tables.go` |
| Azure prices | per (region, size) for node sizes, with every meter of that size cached; batching per region | `pkg/azure/pricing.go:44-49,153-256`; the batch split lands with [fix-azure-price-batch](../fix-azure-price-batch/bugfix.md) |
| Azure catalog | none | see D1 |

## Target state

```
store ── nodes ──► recommendInput: each node tagged with cloud, region, OS, pool mode,
                    │               status.capacity, provider-ID pool kind (VMSS / VMs)
recommend.Analyze(in) ── advisor by node cloud ──► aws   (as today)
                                               └─► azure: catalog  pkg/azure/zz_generated_instance_types.go
                                                          prices   azure.Provider.ListPrice(region, size, windows, spot)
                                                                   miss → wanted → fetch loop (batch fix's split); generation++
                                                                   pricing disabled → "no price", never queued
UI: recKey includes the Azure price generation → analysis again when prices arrive
```

## Components touched

| Path | Change |
|------|--------|
| `Makefile` | `azure-catalog`: copy the cluster autoscaler's table verbatim |
| `pkg/azure/zz_generated_instance_types.go` (new, copied verbatim) | the table |
| `pkg/azure/catalog.go` (new) | lookups by name and family; family normalisation; `amd64` → `x86_64`; 0 vCPU/memory = missing; constrained flag; counterpart (family, vCPU, min memory) |
| `pkg/azure/pricing.go` | `ListPrice` queues a miss like `NodePrice`; generation counter; a disabled mode that never queues |
| `pkg/recommend/recommend.go`, `tables.go`, `tables_azure.go` (new) | `Node` gets `Cloud`, `Region`, `Windows`, `System`, `Capacity` (`status.capacity`), `VMPool`; price functions take the node; family per cloud; `Rec` gets `Cloud` and `Label()`; Azure tables with the temp-disk set; D4 estimate with the `≤ 0` guard; R15 limits; AKS wording; Azure-only rejected-candidate math lines |
| `pkg/recommend/fixture_test.go:50-51` | the AWS fixture's `OnDemand`/`SpotIn` follow the new signatures |
| `pkg/recommend/aks_fixture_test.go` (new) | the usage-example fixture, the made-up virtual node, the coverage and temp-disk table tests |
| `pkg/model/recommendations.go` | advisor per cloud; the gate; labels and `status.capacity` into `recommend.Node`; `rec.Label()` at :164, :249, :253; generation in `recKey`; the waiting and pricing-disabled notes |
| `pkg/model/screens_internal_test.go:41-42` | `fakeAdvisor` follows the new signatures |
| `pkg/model/golden_test.go`, `pkg/model/testdata/screens/` | a new AKS Recommendations golden file; the AWS files unchanged |
| `pkg/model/aks_internal_test.go:79-99` | `TestAKSRecommendationsSayEKSOnly` is rewritten for R1; GCP and empty-cluster tests |
| `cmd/kube-node-viewer/session.go:205-206,240-282` | `awsAdvisor` signatures (:270-276); `azureAdvisor` from the `ByCloud` Azure provider; with `--disable-pricing`, an `azureAdvisor` over the catalog with pricing disabled |
| `.github/workflows/refresh-data.yaml` | runs `make azure-catalog` with `make prices` (deferred 2026-11-10 to its own PR: tasks, Deferred) |
| `README.md`, `CHANGELOG.md` | Recommendations on AKS; credit to the cluster autoscaler table next to Karpenter's |
| `specs/aks-pricing/requirements.md` | R3 `Test: withdrawn (R3)`, with a dated note pointing here |
| `specs/recommendations/requirements.md` | dated notes: the Azure out-of-scope line and the services counter-example |

## Decisions

### D1: The catalog is the cluster autoscaler's Azure table, copied verbatim

**Choice**: `make azure-catalog` downloads
`cluster-autoscaler/cloudprovider/azure/azure_instance_types.go` from
`kubernetes/autoscaler` into `pkg/azure/zz_generated_instance_types.go`
unchanged, as `make prices` copies Karpenter's tables (`Makefile:101-107`).
It is already `package azure` with its Apache-2.0 header, so it needs no
rewrite and no modification note. The file gives, per size:
- `SkuFamily`;
- `VCPU`;
- `MemoryMb`;
- `GPU`;
- `Architecture` (`amd64` or `arm64`, read as `x86_64` or `arm64`).

The node's own allocatable still wins for sizes in the cluster (R7).

**Evidence** (2026-11-10):

1. **Retail Prices API.** An answer for `Standard_D4ps_v5` had these fields:
   `armRegionName, armSkuName, currencyCode, effectiveStartDate,
   isPrimaryMeterRegion, location, meterId, meterName, productId,
   productName, retailPrice, serviceFamily, serviceId, serviceName, skuId,
   skuName, tierMinimumUnits, type, unitOfMeasure, unitPrice`. There is no
   vCPU, memory or architecture.
2. **The cluster autoscaler table.** The file is Apache-2.0
   ("Copyright The Kubernetes Authors"), generated with `go generate` from
   `az vm list-skus`. It was last changed 2026-03-17. It has 1,370 sizes in
   176 families, of which 97 sizes in 13 families are arm64. Sample
   entries:
   - `Standard_D64ps_v5`: `standardDPSv5Family`, 64 vCPU, 212992 MB, arm64;
   - `Standard_E8-4ds_v5`: VCPU 8, the full count;
   - `Standard_DS1_v2`: 3072 MB, though it is 3.5 GiB;
   - `Standard_B1ls`: 0 MB.

   Upstream's generator sets `MemoryMb = int64(memoryGB) * 1024`, which
   truncates fractions (spec-refuter, second pass).
3. **Resource SKUs API.** It answers `SubscriptionNotFound` for a made-up
   subscription. It needs a real subscription and identity.
4. **Other sources looked at:**
   - The docs pages (`MicrosoftDocs/azure-compute-docs`, CC BY 4.0) have
     table headers that differ from page to page.
   - The pricing-calculator endpoint answers 404.
   - The Azure Karpenter provider's `known_skus.yaml` has names and
     families only.

**Justification, strongest first**:
1. It needs no credentials and is Apache-2.0, the project's own license,
   copied the way an existing table already is. It needs no new process and
   no license question. The monthly workflow can refresh it.
2. The data comes from Azure's own SKU API through a maintained Kubernetes
   project that depends on it for scaling decisions.
3. It is compiled in, so no request is made at run time.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| It lags the newest sizes (last change 2026-03-17) | a size it lacks gets no catalog advice (R12); the monthly copy picks up upstream additions; T1 records which recent families are missing |
| Memory truncated to whole GB (DS1_v2 3 GiB; B1ls 0) | understating a candidate's memory is the safe side; a 0 entry counts as missing (R2), so no candidate has 0 or negative room |
| Upstream may change the struct | the copy fails to compile or a catalog test fails in the refresh PR; the old table stays |
| `SkuFamily` names are irregular (`standardDav6Family` holds Dasv6) | tables are keyed by the normalised family and checked against the table in T3 |

**When this would be wrong**: upstream stops generating the file, or it
lags the families owners run by more than a release. Then go to
alternative B.

**Reversal action**: delete the table and the Make target. Catalog rules
then show no rows on Azure.

**Alternatives discarded**:
- **B: `az vm list-skus` run by a maintainer.** The data is the same, with
  untruncated memory, but it needs an Azure login and can't run in CI
  without an Azure identity. It is the fallback.
- **C: the Azure docs tables (CC BY 4.0).** 48 of 162 pages differ in
  their header. The license is CC BY, and "not `p`, so x86_64" is false for
  the ND GB200/GB300 Grace sizes. They are still used once, by hand, for
  the temp-disk set of D5.
- **D: parsing the size name, with memory from a per-family ratio.** The
  memory comes out wrong: `Standard_D64ps_v5` is 208 GiB, not 256.
- **E: the nodes' own capacity only.** Rightsize and the swaps need sizes
  the cluster doesn't run.

### D2: An advisor per cloud; the rules see the node's cloud, region, OS, pool mode and capacity

**Choice**:
- `recommend.Node` gains these fields:
  - `Cloud`, `Region`, `Windows`;
  - `System` (mode `system`; a missing mode label is `user`);
  - `Capacity` (`status.capacity`);
  - `VMPool` (the provider ID names `/virtualMachines/` without a scale
    set).
- The price functions take the node and a size: `OnDemand(n, size)` and
  `SpotIn(n, size)`.
- `Spec` keeps one argument; Azure names start with `Standard_` and never
  collide with AWS types.
- `Rec` gains `Cloud` and a `Label()` that gives the kind's word for that
  cloud.
- `model` routes each node to its cloud's advisor. Groups, families and
  candidates are keyed by cloud.

**Justification, strongest first**:
1. Azure prices depend on the region and the OS (aks-pricing R1), which
   `OnDemand(type)` can't express. The R15 limits need the pool mode, D4
   needs `status.capacity`, and the R7 wording needs the pool kind.
2. Today a mixed view gives Azure nodes AWS-worded idle and old-node rows
   and nothing else (Current state). Keying by cloud fixes that (R14).

**Trade-offs accepted**: the AWS advisor and both test fakes change
signatures and ignore the new fields. That is mechanical, guarded by the
unchanged $242.94 fixture and the AWS golden screens.

**When this would be wrong**: AWS prices also needed the node's region per
call. The shape already allows it.

**Reversal action**: put the old signatures back and keep Azure behind a
session-level advisor, which drops R14.

**Alternatives discarded**:
- An Azure advisor fixed to one region per session. It can't tell Windows
  prices from Linux ones, and it keeps the "first cloud wins" gate.

### D3: Candidate prices on demand, through the batch fix's split, analysis again on arrival

**Choice**: `azure.Provider.ListPrice` reads the cache. On a miss it adds
the (region, size) to `wanted`, as `NodePrice` does.

The fetch loop's split into requests of at most 17 sizes is not defined
here. It comes from
[fix-azure-price-batch](../fix-azure-price-batch/bugfix.md), which fixes
the shipped `NodePrice` path, and this spec reuses that fix's constant
without redefining it. Seventeen is 20 comparisons minus the three fixed
clauses: `serviceName`, `priceType`, `armRegionName`. The requests run one
after another.

Every stored answer bumps a generation counter, which `recKey` includes,
so the next update runs the analysis again. A node's own size is cached
with all its meters (regular, Spot, Windows; `pricing.go:44-49`), so only
the sizes no node runs are awaited. The waiting note counts those.

This spec depends on that fix: T2 starts after it merges.

**With `--disable-pricing`**: the session builds an `azureAdvisor` over the
catalog whose price lookups answer "no price" and never queue. The rules
then give:
- consolidation, with saving `—`;
- old node.

Every rule that needs a candidate's or a node's price gives nothing:
idle, rightsize, generation, Arm and Spot. There is no waiting note; a note
says Azure rows that need a price are not shown. This is less than EKS
shows, because the AWS static tables still price EKS rows with the money
hidden. Azure has no static table (alternatives below).

**Evidence** (spec-refuter, 2026-11-10):
- Against the live API, 3 clauses + 17 sizes → 200, and 18 sizes → 400
  "Invalid OData parameters supplied".
- A 429 came after about 11 requests in about 11 s. The fix-azure-retry
  handling honours its retry header.

**Justification, strongest first**:
1. It reuses the provider's retry, back-off and unknown-size handling, so
   there is no second HTTP path.
2. The number of sizes is bounded and known from the catalog. A family
   holds 6 to 20 sizes: `dsv5` 8, `dpsv5` 7, `epsv5` 6, `edsv5` 20. The
   rightsize candidates are the smaller sizes of the node's family, plus
   one generation target and one Arm target per pool. A cluster with three
   families asks for roughly 20 to 40 sizes, that is 2 to 3 requests per
   region. This is computed, not measured.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Rows appear a moment after the screen opens | the note says how many sizes are awaited |
| Several requests in a row can hit the 429 | the retry header is honoured; the rows wait, nothing breaks |
| Few Azure rows with pricing off | `--disable-pricing` asks for no price; the note says why rows are missing |

**When this would be wrong**: the API changes its comparison limit. The
fix's constant moves, and its test pins the split; this spec only checks
that candidate sizes go through it. Also wrong if users run AKS with
pricing off and want catalog-only advice; then rank candidates by size,
not by money.

**Reversal action**: fetch a whole family by its product name in one
request (`productName eq 'Virtual Machines Dpsv5 Series'` returned all
seven `Dpsv5` sizes, 2026-11-10).

**Alternatives discarded**:
- Fetching the whole region at start: thousands of items in dozens of
  pages.
- Static Azure price tables. The prices are per region, per OS and per
  Spot, and they move: the Spot meters checked carry effective dates from
  2026-05-01 to 2026-11-01.

### D4: Allocatable of a size the cluster doesn't run: the same absolute reservation (understates)

**Choice**: for an Azure candidate with no node of that size in the
cluster, the estimate is per resource:

> allocatable = candidate's catalog capacity − reservation
>
> reservation = max(node's catalog capacity − node's allocatable,
> node's `status.capacity` − node's allocatable)

The node is the one being rightsized. The catalog figure also covers the
memory the kernel and firmware keep, which `status.capacity` already
hides. Taking the larger of the two keeps the estimate on the safe side
even when the catalog figure is truncated below `status.capacity`, as for
DS1_v2. An estimate of 0 or less in any resource means "doesn't fit". The
math shows the catalog capacity, `status.capacity` and allocatable it used.

**It understates** under the AKS defaults (Kubernetes 1.29+, Azure CNI
Overlay with max-pods 250). Memory kube-reserved is
min(20 MB × max-pods + 50 MB, 25% of memory), plus eviction:

| Node | Reserved |
|---|---|
| D8s_v5, 32 GiB | about 5.05 GB |
| D2s_v5, 8 GiB | at most 2 GB |

Carrying the D8's reservation onto a D2 understates the D2 by about 3 GB.
The fixture's verdict does not depend on it. vmss000003 needs 2.0 vCPU, and
a D2s_v5 has 2 vCPU in all, so at 80% it can't fit whatever the memory.

**Justification, strongest first**:
1. The figures of the published AKS schemes reviewed here never make a
   smaller node reserve more, in absolute terms, than a larger one. Those
   are the cpu scale, the per-max-pods memory figure capped at 25%, and the
   older regressive memory scale. So under those schemes the estimate is
   never above the truth. A scheme outside them could break this, and the
   math says "may understate" rather than claiming a bound.
2. It needs no formula that AKS can change from one release to the next.

**Trade-offs accepted**: rightsizes that would fit on a candidate the
cluster doesn't run can be missed.

**When this would be wrong**: owners miss real savings because of it.
Then code the published formula. Its inputs are on the node: kubelet
version (`status.nodeInfo.kubeletVersion`), max-pods (allocatable pods)
and the OS label.

**Reversal action**: replace the estimate with the formula, keyed by
kubelet version and OS.

**Alternatives discarded**:
- **Coding the formula now.** The inputs are available, but the formula is
  defined by AKS docs, not an API. It has changed before, and a coded copy
  goes stale silently.
- **The AWS ratio rule.** It overstates small nodes under the per-max-pods
  formula.

### D5: Fixed Azure tables that keep the temp disk; same price is information; Arm only to documented v5 families

**Decided** (owner, 2026-11-10):
- A generation row at the same price is an `info` row, outside the total.
- Arm targets are the v5 families only (Dpsv5, Dplsv5, Epsv5 and their `d`
  variants). v6 Cobalt stays deferred.

**Decided** (reviewer, technical, 2026-11-10): a family with a temp disk
maps only to a family with one.

**Choice**: `pkg/recommend/tables_azure.go` holds three tables, keyed by
the normalised `SkuFamily`.

The temp-disk set (from each family's docs "Local Storage" line, read
2026-11-10):

| Has a temp disk | No temp disk |
|---|---|
| `dv2`, `dsv2`, `dv3`, `dsv3`, `dav4`, `dasv4`, `ddv4`, `ddsv4`, `ddv5`, `ddsv5`, `dadsv5`, `ddsv6`, `dadv6`, `dldsv5`, `dldsv6`, `daldv6`, `ev3`, `esv3`, `eav4`, `easv4`, `edv4`, `edsv4`, `edv5`, `edsv5`, `eadsv5`, `edsv6`, `eadv6`, `bs`, `dpdsv5`, `dpldsv5`, `epdsv5` | `dv4`, `dsv4`, `dv5`, `dsv5`, `dasv5`, `dsv6`, `dav6`, `dlsv5`, `dlsv6`, `dalv6`, `ev4`, `esv4`, `ev5`, `esv5`, `easv5`, `esv6`, `eav6`, `bsv2`, `dpsv5`, `dplsv5`, `epsv5` |

The generation and Arm tables:

| Generation (x86 → newer x86, same class and features, temp disk kept) | Arm counterpart (Linux, not system pools), temp disk kept |
|---|---|
| `dv2`, `dv3` → `ddv5`; `dsv2`, `dsv3` → `ddsv5` | no temp disk: `dv4`, `dv5`, `dsv4`, `dsv5`, `dsv6`, `dasv5`, `dav6` → `dpsv5` |
| `dv4` → `dv5`; `dsv4` → `dsv5` | temp disk: `dv2`, `dsv2`, `dv3`, `dsv3`, `dav4`, `dasv4`, `ddv4`, `ddv5`, `ddsv4`, `ddsv5`, `ddsv6`, `dadsv5`, `dadv6` → `dpdsv5` |
| `ddv4` → `ddv5`; `ddsv4` → `ddsv5` | `dlsv5`, `dlsv6`, `dalv6` → `dplsv5`; `dldsv5`, `dldsv6`, `daldv6` → `dpldsv5` |
| `dav4`, `dasv4` → `dadsv5` | no temp disk: `ev4`, `ev5`, `esv4`, `esv5`, `esv6`, `easv5`, `eav6` → `epsv5` |
| `ev3` → `edv5`; `esv3` → `edsv5`; `ev4` → `ev5`; `esv4` → `esv5` | temp disk: `ev3`, `esv3`, `eav4`, `easv4`, `edv4`, `edv5`, `edsv4`, `edsv5`, `edsv6`, `eadsv5`, `eadv6` → `epdsv5` |
| `edv4` → `edv5`; `edsv4` → `edsv5`; `eav4`, `easv4` → `eadsv5` | — |

All the families named exist in the copied table (checked 2026-11-10).
`bs` has no generation target: no B-series v2 family has a temp disk.

**Coverage rule, tested in T3**:
- Every x86 `d*` and `e*` family of v2 to v6 in the copied table is either
  a key of the Arm table, or in an exclusion list with its reason:
  - confidential `dc*` and `ec*`;
  - isolated `ei*`;
  - remote-storage `eb*`;
  - the Arm families themselves.
- v7 families are excluded as newer than the targets.
- Every mapping keeps temp-disk membership.
- Every key and target exists in the table.

**How a row is decided**:
- The target is the size of the target family with the same vCPU count and
  no less memory. For example, `Standard_DS2_v2` (2 vCPU, 7 GiB) maps to
  `Standard_D2ds_v5` (8 GiB).
- Same price means within $0.0001/h. A generation row at the same price is
  `info`, a cheaper one is priced, and a dearer one gives no row.
- An Arm row must be cheaper.
- Generation targets are v5, because v6 Intel costs more.

`eastus` prices on 2026-11-10:

| Size | Price/h |
|---|---|
| D4s_v3 | $0.192 |
| D4s_v5 | $0.192 |
| D4ds_v5 | $0.226 |
| D4s_v6 | $0.202 |
| D4ps_v5 | $0.154 |
| D4pds_v5 | $0.181 |
| D4ps_v6 | $0.14 |

**Justification, strongest first**:
1. A temp disk can hold the ephemeral OS disk and pods' scratch data.
   Dsv5 has "Local Storage None" and no ephemeral OS disk, so D4s_v3 →
   D4s_v5 would remove what D4s_v3's 32 GiB temp disk gives.
2. Azure prices an older and a newer Intel generation the same (D4s_v3,
   D4s_v5). The `info` row tells the owner of free newer hardware where a
   like-for-like pair exists.
3. AKS's Arm64 node-pool page (`use-arm64-vms.md`, ms.date 2025-02-25)
   names only the Dpsv5, Dplsv5 and Epsv5 series (and their `d` variants on
   the same size pages).
4. Matching memory as well as vCPU avoids the `Standard_D64ps_v5` trap
   (208 GiB).

**Trade-offs accepted**:
- Keeping the temp disk often makes the target dearer: D4s_v3 → D4ds_v5 is
  $0.226, so the generation row disappears for v3.
- Arm saves less with a temp disk ($8.03 a month per D4 on D4pds_v5).
- `info` rows add lines with no money.

**When this would be wrong**:
- A node doesn't use its temp disk. Then a non-`d` target would save more,
  but the tool can't tell from the node.
- AKS documents the v6 Arm families.

**Reversal action**: edit the tables, or remove the `info` case (one
condition).

**Alternatives discarded**:
- Ignoring the temp disk: cheaper rows, but they remove a resource.
- v6 Arm targets now: deferred by the owner.
- Intel → AMD targets: out of scope, deferred.
- `Av2` and `Fsv2`: no like-for-like newer family; out of scope.

### D6: The kind reads `arm` on Azure rows

**Decided** (owner, 2026-11-10): `arm` on Azure rows; `graviton` stays on
AWS rows.

**Choice**: `Rec.Label()` gives `graviton` for AWS rows and `arm` for Azure
rows. The kind keeps its place in the order. Row, math title and popup use
`Label()` (`pkg/model/recommendations.go:164,249,253`).

**Justification**: Graviton is the AWS product. Azure's are Ampere Altra
(v5) and Cobalt 100 (v6).

**Trade-offs accepted**: a mixed screen shows two words for one kind.

**When this would be wrong**: the owner prefers one neutral word
everywhere. That changes the AWS screen and its golden files.

**Reversal action**: change `Label()`.

**Alternatives discarded**:
- Renaming the AWS kind: it changes released behaviour unasked.

### D7: AKS actions and limits

**Choice**:

| Rule | Action, or limit |
|---|---|
| Consolidation | `scale agent pool <name> to N nodes (or lower its autoscaler minimum)` |
| Rightsize and generation, VMSS pool | resize the pool in place (`az aks nodepool update --node-vm-size`, preview) only if every node fits and the change keeps the architecture, disk controller, hypervisor generation and confidential support (the resize also refuses `--max-surge 0`); else a pool of that size |
| Rightsize and generation, Virtual Machines pool | add the size to the pool, since a VMs pool can hold several sizes |
| Generation, system pool | `add a system-mode pool of <size>` |
| Arm | always a new pool; never for a system pool |
| Spot | a Spot agent pool alongside |
| Old node | `upgrade the node image` (`az aks nodepool upgrade --node-image-only`) |
| `mode=system` pools (missing label = `user`) | no Spot; no Arm; never below 2 nodes, with "AKS recommends three"; targets of at least 4 vCPU and 4 GiB, not B-series, pods at least 30 |
| Virtual nodes | `type=virtual-kubelet`: no catalog advice and no old-node row |

**Evidence** (MicrosoftDocs/azure-aks-docs, 2026-11-10):

| Page | What it says |
|---|---|
| `resize-node-pool.md` | "Resize a VMSS node pool in place (preview)"; the resize refuses disk-controller, CPU-architecture, confidential-computing and hypervisor-generation changes |
| `use-system-pools.md` (ms.date 2026-04-30) | at least 4 vCPUs and 4 GB, no B series, at least two nodes (three recommended), at least 30 pods |
| `spot-node-pool.md` | a Spot pool can't be the default node pool; system pods have anti-affinity to Spot nodes |
| `use-arm64-vms.md` | no Arm64 Windows pools; no statement on Arm64 system pools |

The `--max-surge 0` refusal and VMs pools holding several sizes come from
the second spec-refuter pass. The virtual node's label shape is not
verified: the fixture uses a made-up node object, and a run on a cluster
with virtual nodes would confirm it.

**Justification**: every action has to be one AKS accepts. Where AKS's docs
are silent (Arm system pools), no row.

**Trade-offs accepted**:
- The autoscaler's settings can't be read without the ARM API, so the
  consolidation action names both scaling and the minimum.
- Arm savings on system pools are not shown.

**When this would be wrong**: the in-place resize leaves preview, AKS
documents Arm64 system pools, or AKS allows Spot in system pools.

**Reversal action**: change the action strings and the R15 checks.

**Alternatives discarded**:
- The EKS wording ("node group", "fresh AMI"): it names things AKS doesn't
  have.

## Invariants

- No Azure credentials, no `~/.azure`, no ARM API, at run time or at
  refresh.
- At run time, only the Retail Prices API, which is already used.
- Requests vs allocatable only; same input, same advice.
- An EKS-only cluster gives the same rows, wording, math and total as
  before.
- `Update` never waits on analysis or I/O.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Retail Prices API down, 400 or 429 | the rows needing a missing price are absent; the provider retries (fix-azure-retry) | header shows the logged error; note "waiting for the prices of N Azure sizes" |
| `--disable-pricing` | consolidation (`—`) and old-node rows only | note "pricing disabled: Azure rows that need a price are not shown" |
| Size not in the catalog, or a 0 entry | no catalog advice for that node | its idle and old-node rows only |
| Upstream table changed shape | the copy fails to build or a catalog test fails | the monthly refresh PR is red; the old table stays |
| Size absent in the region | no row for that candidate | — |

## Test plan

- `pkg/azure`:
  - the copied table loads, with the R2 samples (D64ps_v5, DS1_v2 at
    3 GiB, B1ls missing);
  - family normalisation;
  - `amd64` → `x86_64`;
  - the constrained flag;
  - the counterpart lookup;
  - `ListPrice` queues a miss, goes through the batch split and bumps the
    generation;
  - a disabled provider never queues;
  - Windows and Spot meters.
- `pkg/recommend`:
  - the AKS fixture end to end: rows, order, `arm` label, $322.66;
  - R4 grouping and the missing mode label;
  - R5 idle values;
  - R6 the system floor of 2 with the note;
  - R7:
    - with and without a node of the candidate size;
    - the `≤ 0` guard (B2s → B1ls);
    - pod-capacity source;
    - VMSS and VMs pool wording;
    - Azure-only rejected-candidate lines;
  - R8: no row for D4ds_v5, the $0.0001 tolerance on synthetic prices;
  - R9: D8ps_v5, D4pds_v5, D64 memory, Windows, system;
  - R10: the age limit and the virtual node;
  - R11: regional Spot, system pools, Windows Spot;
  - R12: the virtual node, an unknown size, the instance-price label;
  - R14: the mixed fixture ($565.60);
  - R15: vCPU, B series, pods;
  - the AWS fixture unchanged at $242.94;
  - the table coverage and temp-disk tests (D5).
- `pkg/model`:
  - the gate (Azure analysed, the GCP message, an empty cluster);
  - `recKey` with the generation, and a blocking price source;
  - the waiting and pricing-disabled notes;
  - labels and `status.capacity` into `recommend.Node`;
  - the rewritten `aks_internal_test.go` test.
- `cmd/kube-node-viewer`: the session builds an `azureAdvisor` with and
  without `--disable-pricing`.
- Golden screens: the AWS screens unchanged, and a new AKS Recommendations
  screen.
- A run against a cluster: a kind cluster with made-up `azure://` nodes
  carrying region, size, agent pool, mode and OS labels. It is priced
  through the public API without credentials, and the verify task notes
  what was checked.
