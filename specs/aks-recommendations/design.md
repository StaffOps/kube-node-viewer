# Design: aks-recommendations

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Screen gate | analysis runs when the shown cloud is `aws`; `shownCloud` returns `aws` as soon as one AWS node is shown | `pkg/model/recommendations.go:63,196-207` |
| Analysis input | every selected node, whatever its cloud | `pkg/model/recommendations.go:84-86` |
| Cloud checks in rules | catalog rules skip sizes `Spec` doesn't know; `idle()` and `oldNodes()` have none, so Azure nodes in a mixed view get AWS-worded rows | `pkg/recommend/recommend.go:242,601` |
| Advisor | one, AWS only | `cmd/kube-node-viewer/session.go:240-282`, `pkg/model/recommendations.go:28-34` |
| Rule input | `Spec(type)`, `OnDemand(type)`, `SpotIn(type, zone)`, `Types()`; no cloud, region or OS | `pkg/recommend/recommend.go:97-114` |
| Kind label | `Kind.String()`, no cloud; used by the row, the math title and the popup | `pkg/recommend/recommend.go:129`, `pkg/model/recommendations.go:164,249,253` |
| Families | `family.size` split on `.`; fixed AWS tables | `pkg/recommend/recommend.go:227-230`, `tables.go` |
| Azure prices | per (region, size) for node sizes only; one request per region with every wanted size | `pkg/azure/pricing.go:153-230,249-256` |
| Azure catalog | none | see D1 |

## Target state

```
store ── nodes ──► recommendInput: each node tagged with cloud, region, OS, pool mode
                    │
recommend.Analyze(in) ── advisor by node cloud ──► aws   (as today)
                                               └─► azure: catalog  pkg/azure/zz_generated_instance_types.go
                                                          prices   azure.Provider.ListPrice(region, size, windows, spot)
                                                                   miss → wanted; ≤ 17 sizes per request; generation++
UI: recKey includes the Azure price generation → analysis again when prices arrive
```

## Components touched

| Path | Change |
|------|--------|
| `Makefile` | `azure-catalog`: copy the cluster autoscaler's table, as `prices` copies Karpenter's |
| `pkg/azure/zz_generated_instance_types.go` (new, copied) | the table; package line rewritten, modification note added |
| `pkg/azure/catalog.go` (new) | lookups by name and family, family normalisation, constrained flag, counterpart (family, vCPU, min memory) |
| `pkg/azure/pricing.go` | `ListPrice` queues a miss like `NodePrice`; batches of ≤ 17 sizes; a generation counter |
| `pkg/recommend/recommend.go`, `tables.go` (+ `tables_azure.go`) | `Node` gets `Cloud`, `Region`, `Windows`, `System`; price functions take the node; family per cloud; `Rec` gets `Cloud` and a `Label()`; Azure tables; D4; R15 limits; AKS wording |
| `pkg/recommend/fixture_test.go:50-51` | the AWS fixture's `OnDemand`/`SpotIn` follow the new signatures |
| `pkg/recommend/aks_fixture_test.go` (new) | the usage-example fixture and the made-up virtual node |
| `pkg/model/recommendations.go` | advisor per cloud; the gate; labels into `recommend.Node`; `rec.Label()` at :164, :249, :253; generation in `recKey`; the "waiting for the prices of N Azure sizes" note |
| `pkg/model/screens_internal_test.go:41-42` | `fakeAdvisor` follows the new signatures |
| `pkg/model/aks_internal_test.go:79-99` | `TestAKSRecommendationsSayEKSOnly` is rewritten for R1 (rows on Azure nodes); a GCP test keeps the message |
| `cmd/kube-node-viewer/session.go:240-282` | `awsAdvisor` follows the new signatures (:270-276); `azureAdvisor` from the `ByCloud` Azure provider |
| `.github/workflows/refresh-data.yaml` | runs `make azure-catalog` with `make prices` |
| `README.md`, `CHANGELOG.md` | Recommendations on AKS; credit to the cluster autoscaler table next to Karpenter's |
| `specs/aks-pricing/requirements.md` | R3 `Test: withdrawn (R3)`, with a dated note pointing here |
| `specs/recommendations/requirements.md` | dated notes: the Azure out-of-scope line and the services counter-example |

## Decisions

### D1: The catalog is the cluster autoscaler's Azure table, copied like Karpenter's prices

**Choice**: `make azure-catalog` downloads
`cluster-autoscaler/cloudprovider/azure/azure_instance_types.go` from
`kubernetes/autoscaler` into `pkg/azure/zz_generated_instance_types.go`.
It rewrites the package line and adds the D5 modification note, as
`make prices` does for Karpenter's tables (`Makefile:101-107`). The file
gives, per size:
- `SkuFamily`;
- `VCPU`;
- `MemoryMb`;
- `GPU`;
- `Architecture` (`amd64`/`arm64`).

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
   176 families, 97 of them arm64. Sample entries:
   - `Standard_D64ps_v5`: `standardDPSv5Family`, 64 vCPU, 212992 MB,
     arm64;
   - `Standard_E8-4ds_v5`: VCPU 8, the full count.
3. **Resource SKUs API.** It answers `SubscriptionNotFound` for a made-up
   subscription. It needs a real subscription and identity.
4. **Other sources looked at:**
   - The docs pages (`MicrosoftDocs/azure-compute-docs`, CC BY 4.0) have
     table headers that differ from page to page (review H3).
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
| It lags the newest sizes (last change 2026-03-17; check during T1 whether any v7 families are missing) | a size it lacks gets no catalog advice (R12), as AWS does for a type newer than its table; the monthly copy picks up upstream additions |
| Upstream may change the struct | the copy fails to compile or a catalog test fails in the refresh PR; the old table stays |
| `SkuFamily` names are irregular (`standardDav6Family` holds Dasv6) | tables are keyed by the normalised family and checked against the table in T3 |

**When this would be wrong**: upstream stops generating the file, or it
lags the families owners run by more than a release. Then go to
alternative B.

**Reversal action**: delete the table and the Make target. Catalog rules
then show no rows on Azure.

**Alternatives discarded**:
- **B: `az vm list-skus` run by a maintainer.** The data is the same, but
  it needs an Azure login and can't run in CI without an Azure identity.
  It is the fallback.
- **C: the Azure docs tables (CC BY 4.0).** 48 of 162 pages differ in
  their header (`Memory (GB)`, `vCPUs`, `Cores (Qty.)`, `<sup>`
  footnotes). Pages hold several tables, and the storage tables repeat
  size names. The license is CC BY, and "not `p`, so x86_64" is false for
  the ND GB200/GB300 Grace sizes.
- **D: parsing the size name, with memory from a per-family ratio.** The
  memory comes out wrong: `Standard_D64ps_v5` is 208 GiB, not 256.
- **E: the nodes' own capacity only.** Rightsize and the swaps need sizes
  the cluster doesn't run.

### D2: An advisor per cloud; the rules see the node's cloud, region, OS and pool mode

**Choice**:
- `recommend.Node` gains `Cloud`, `Region`, `Windows` and `System`.
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
   `OnDemand(type)` can't express. The R15 limits need the pool mode.
2. Today a mixed view gives Azure nodes AWS-worded idle and old-node rows
   and nothing else (Current state). Keying by cloud fixes that (R14).

**Trade-offs accepted**: the AWS advisor and both test fakes change
signatures and ignore the new fields. That is mechanical, guarded by the
unchanged $242.94 fixture and the golden screens.

**When this would be wrong**: AWS prices also needed the node's region per
call. The shape already allows it.

**Reversal action**: put the old signatures back and keep Azure behind a
session-level advisor, which drops R14.

**Alternatives discarded**:
- An Azure advisor fixed to one region per session. It can't tell Windows
  prices from Linux ones, and it keeps the "first cloud wins" gate.

### D3: Candidate prices on demand, at most 17 sizes per request, analysis again on arrival

**Choice**: `azure.Provider.ListPrice` reads the cache. On a miss it adds
the (region, size) to `wanted`, as `NodePrice` does. The fetch loop splits
a region's wanted sizes into requests of at most `maxSizesPerRequest = 17`.
That is 20 comparisons minus the three fixed clauses: `serviceName`,
`priceType`, `armRegionName`. The requests run one after another. Every
stored answer bumps a generation counter, which `recKey` includes, so the
next update runs the analysis again.

**Evidence** (spec-refuter, 2026-11-10):
- Against the live API, 3 clauses + 17 sizes → 200, and 18 sizes → 400
  "Invalid OData parameters supplied".
- A 429 came after about 11 requests in about 11 s. The fix-azure-retry
  handling honours its retry header.

**Justification, strongest first**:
1. It reuses the provider's retry, back-off and unknown-size handling, so
   there is no second HTTP path.
2. The number of sizes is bounded and known from the catalog. A family
   holds 7 to 20 sizes: `dsv5` 8, `dpsv5` 7, `edsv5` 20. The rightsize
   candidates are the smaller sizes of the node's family, plus one
   generation target and one Arm target per pool. A cluster with three
   families asks for roughly 20 to 40 sizes, that is 2 to 3 requests per
   region. This is computed, not measured.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Rows appear a moment after the screen opens | the note says how many sizes are awaited |
| Several requests in a row can hit the 429 | the retry header is honoured; the rows wait, nothing breaks |

**The same limit in today's code**: `NodePrice` shares `wanted`, so today
18 or more distinct node sizes in one region make every request 400. Since
`retry.go:51-52` retries a 400 forever, Overview prices never arrive. This
spec's split fixes it for both callers. The shipped behaviour gets its own
bugfix spec (TODO item), so it can ship before this feature.

**When this would be wrong**: the API changes its comparison limit. The
named constant moves, and a test pins the split.

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

> allocatable = candidate's catalog capacity − (node's catalog capacity −
> node's allocatable)

The node is the one being rightsized. The reservation is taken from the
catalog capacity, not `status.capacity`, so it also covers the memory the
kernel and firmware keep, which a smaller VM loses too. The math shows the
catalog capacity, `status.capacity` and allocatable it used.

**It understates** under the AKS defaults the review cites (Kubernetes
1.29+, Azure CNI Overlay with max-pods 250). Memory kube-reserved is
min(20 MB × max-pods + 50 MB, 25% of memory), plus eviction:

| Node | Reserved |
|---|---|
| D8s_v5, 32 GiB | about 5.05 GB |
| D2s_v5, 8 GiB | at most 2 GB |

Carrying the D8's reservation onto a D2 understates the D2 by about 3 GB.
The verdict in the fixture does not depend on it. vmss000003 needs 2.0
vCPU, and a D2s_v5 has 2 vCPU in all, so at 80% it can't fit whatever the
memory.

**Justification, strongest first**:
1. Under each AKS reservation scheme, a smaller node reserves no more in
   absolute terms than a larger one, so the error only goes one way. That
   is the safe side for an "it fits" verdict.
2. It needs no formula that AKS can change from one release to the next.

**Trade-offs accepted**: rightsizes that would fit on a candidate the
cluster doesn't run can be missed. The math says "may understate".

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

### D5: Fixed Azure tables; same price is information; Arm only to the families AKS documents

**Choice**: two tables in `pkg/recommend/tables_azure.go`, keyed by the
normalised `SkuFamily`. Every key and target is checked against the copied
table in T3; an entry missing from it is a test failure. This is reference
data.

| Generation (x86 → newer x86, same class and features) | Arm counterpart (Linux), AKS-documented families only |
|---|---|
| `dv2`, `dv3`, `dv4` → `dv5` | `dv3`, `dv4`, `dv5`, `dsv3`, `dsv4`, `dsv5`, `dsv6`, `dasv4`, `dasv5`, `dav6` → `dpsv5` |
| `dsv2`, `dsv3`, `dsv4` → `dsv5` | `ddv4`, `ddv5`, `ddsv4`, `ddsv5`, `ddsv6`, `dadsv5`, `dadv6` → `dpdsv5` |
| `ddv4` → `ddv5`; `ddsv4` → `ddsv5` | `dlsv5`, `dlsv6` → `dplsv5` |
| `dav4`, `dasv4` → `dasv5` | `dldsv5`, `dldsv6` → `dpldsv5` |
| `ev3`, `ev4` → `ev5`; `esv3`, `esv4` → `esv5` | `ev3`, `ev4`, `ev5`, `esv3`, `esv4`, `esv5`, `easv4`, `easv5`, `eav6` → `epsv5` |
| `edv4` → `edv5`; `edsv4` → `edsv5`; `easv4` → `easv5` | `edsv4`, `edsv5`, `eadsv5` → `epdsv5` |
| `bs` → `bsv2` | — (no B-series Arm in AKS's Arm64 list) |

How a row is decided:
- The target is the size of the target family with the same vCPU count and
  no less memory. For example, `Standard_DS2_v2` (2 vCPU, 7 GiB) maps to
  `Standard_D2s_v5` (8 GiB).
- Same price means within $0.0001/h. A generation row at the same price is
  `info`; a cheaper one is priced; a dearer one gives no row.
- An Arm row must be cheaper.
- Generation targets are v5, because v6 Intel costs more.

`eastus` prices on 2026-11-10:

| Size | Price/h |
|---|---|
| D4s_v3 | $0.192 |
| D4s_v5 | $0.192 |
| D4s_v6 | $0.202 |
| E4s_v3 | $0.252 |
| E4s_v5 | $0.252 |
| D4ps_v5 | $0.154 |
| D4ps_v6 | $0.14 |

**Justification, strongest first**:
1. Azure prices an older and a newer Intel generation the same. "Only when
   cheaper" would show no generation row on Azure, though the upgrade is
   free newer hardware.
2. AKS's Arm64 node-pool page (`use-arm64-vms.md`, ms.date 2025-02-25)
   names only the Dpsv5, Dplsv5 and Epsv5 series (and their `d` variants
   on the same size pages). Recommending a family AKS doesn't document
   could fail at pool creation. v6 Cobalt is cheaper, so it is an owner
   decision.
3. Matching memory as well as vCPU avoids the `Standard_D64ps_v5` trap
   (208 GiB).

**Trade-offs accepted**:
- `info` generation rows add lines with no money; they stay out of the
  total, like idle rows.
- v5 Arm saves less than v6: $0.038 an hour on a D4 against $0.052.

**When this would be wrong**:
- The owner wants only rows that save money; then drop the `info` case.
- AKS documents the v6 Arm families; then swap the targets.
- Azure prices v6 Intel below v5.

**Reversal action**: edit the tables, or remove the `info` case (one
condition).

**Alternatives discarded**:
- v6 Arm targets now: cheaper, but not on AKS's Arm64 page.
- Intel → AMD targets: out of scope, deferred.
- `DSv2` mapped to `Dv5`: it loses premium storage (`s`), so it maps to
  `dsv5`.
- `Av2`: a basic series with no like-for-like newer family; out of scope.

### D6: The kind reads `arm` on Azure rows

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
| Consolidation | "scale agent pool to N (or lower its autoscaler minimum)" |
| Rightsize and generation | resize the pool in place (`az aks nodepool update --node-vm-size`, preview, same architecture only) when every node fits, else a pool of that size |
| Arm | always a new pool: the in-place resize refuses an architecture change |
| Spot | a Spot agent pool alongside |
| Old node | "upgrade the node image" (`az aks nodepool upgrade --node-image-only`) |
| `mode=system` pools | no Spot; never below 2 nodes; targets of at least 4 vCPU and 4 GiB, not B-series, pods at least 30 (R15) |
| Exclusions | `type=virtual-kubelet` nodes (R12) |

**Evidence** (MicrosoftDocs/azure-aks-docs, 2026-11-10):

| Page | What it says |
|---|---|
| `resize-node-pool.md` | "Resize a VMSS node pool in place (preview)"; changing the CPU architecture is not supported |
| `use-system-pools.md` (ms.date 2026-04-30) | system pools need at least 4 vCPUs and 4 GB, no B series, at least two nodes, at least 30 pods |
| `spot-node-pool.md` | a Spot pool can't be the default node pool; system pods have anti-affinity to Spot nodes |
| `use-arm64-vms.md` | no Arm64 Windows pools |

The virtual node's label shape is not verified: the fixture uses a made-up
node object, and a run on a cluster with virtual nodes would confirm it.

**Justification**: every action has to be one AKS accepts.

**Trade-offs accepted**: the autoscaler's settings can't be read without
the ARM API, so the consolidation action names both scaling and the
minimum.

**When this would be wrong**: the in-place resize leaves preview or gains
architecture changes, or AKS allows Spot in system pools.

**Reversal action**: change the action strings and the R15 checks.

**Alternatives discarded**:
- The EKS wording ("node group", "fresh AMI"): it names things AKS doesn't
  have.

## Invariants

- No Azure credentials, no `~/.azure`, no ARM API, at run time or at
  refresh.
- At run time, only the Retail Prices API, which is already used.
- Requests vs allocatable only; same input, same advice.
- An EKS-only cluster gives the same rows, wording and total as before.
- `Update` never waits on analysis or I/O.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Retail Prices API down, 400 or 429 | the rows needing a missing price are absent; the provider retries (fix-azure-retry) | header shows the logged error; note "waiting for the prices of N Azure sizes" |
| Size not in the catalog | no catalog advice for that node | its idle and old-node rows only |
| Upstream table changed shape | `make azure-catalog` output fails to build or a catalog test fails | the monthly refresh PR is red; the old table stays |
| Size absent in the region | no row for that candidate | — |

## Test plan

- `pkg/azure`:
  - the copied table loads, with the R2 samples;
  - family normalisation;
  - the constrained flag;
  - the counterpart lookup, including the D64ps_v5 memory case;
  - `ListPrice` queues a miss; 18 sizes make two requests; the generation
    is bumped;
  - Windows and Spot meters.
- `pkg/recommend`:
  - the AKS fixture end to end: rows, order, `arm` label, $445.30;
  - R4 grouping;
  - R5 idle values;
  - R6 the system floor of 2;
  - R7 with and without a node of the candidate size;
  - R8 the $0.0001 tolerance;
  - R9 Windows and memory;
  - R10 the age limit;
  - R11 regional Spot, system pools, Windows Spot;
  - R12 the virtual node and an unknown size;
  - R14 the mixed fixture ($688.24);
  - R15 limits;
  - the AWS fixture unchanged at $242.94;
  - every table key and target exists in the copied table.
- `pkg/model`:
  - the gate (Azure analysed, the GCP message);
  - `recKey` with the generation;
  - the waiting note;
  - labels into `recommend.Node`;
  - the rewritten `aks_internal_test.go` test.
- Golden screens: the AWS screens unchanged, and a new AKS Recommendations
  screen.
- A run against a cluster: a kind cluster with made-up `azure://` nodes
  carrying region, size, agent pool and mode labels. It is priced through
  the public API without credentials, and the verify task notes what was
  checked.
