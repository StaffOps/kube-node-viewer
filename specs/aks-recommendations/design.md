# Design: aks-recommendations

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Screen gate | analysis runs only when the shown cloud is `aws`; otherwise "EKS nodes only" | `pkg/model/recommendations.go:63,135-137,196-207` |
| Advisor | one, AWS only: catalog lookups, on-demand price, Spot price in a zone | `cmd/kube-node-viewer/session.go:240-282`, `pkg/model/recommendations.go:28-34` |
| Rule input | `Spec(type)`, `OnDemand(type)`, `SpotIn(type, zone)`, `Types()`; no cloud, region or OS | `pkg/recommend/recommend.go:97-114` |
| Families | `family.size` split on `.`; fixed AWS tables | `pkg/recommend/recommend.go:227-230`, `tables.go` |
| Group | `karpenter.sh/nodepool`, else `eks.amazonaws.com/nodegroup` | `pkg/model/recommendations.go:102-107` |
| Azure prices | per (region, size) for node sizes only; lazy, batched, one request per region | `pkg/azure/pricing.go:93-127,153-230` |
| Azure catalog | none; the Retail Prices API has no vCPU/memory/arch fields | see D1 |

## Target state

```
store ── nodes ──► recommendInput: each node tagged with cloud, region, OS
                    │
recommend.Analyze(in) ── in.Advisor(cloud) ──► aws advisor  (as today)
                                           └─► azure advisor:
                                                 catalog  pkg/azure/zz_generated_sizes.go
                                                 prices   azure.Provider.ListPrice(region, size, os, spot)
                                                          miss → queued, OnUpdate → advisor generation++
UI: recKey includes the advisor generation → re-analysis when prices arrive
```

## Components touched

| Path | Change |
|------|--------|
| `hack/azcatalog/` (new) | generator: reads the size tables of the public docs repository, writes the table |
| `pkg/azure/zz_generated_sizes.go` (new) | the size table (name, series, vCPU, memory GiB, arch, constrained) |
| `pkg/azure/catalog.go` (new) | lookups: by name, by series, counterpart (series, vCPU, min memory) |
| `pkg/azure/pricing.go` | `ListPrice(region, size, windows, spot)` that queues a miss like `NodePrice`; a generation counter bumped on every answer |
| `pkg/recommend/` | `Node` gets `Cloud`, `Region`, `Windows`, `System`; price functions take the node; family split per cloud; Azure tables; kind label per cloud; agent pool wording |
| `pkg/model/recommendations.go` | advisor per cloud; the gate accepts `aws` and `azure`; agent pool, mode and `type` labels into `recommend.Node`; the generation in `recKey`; the "waiting for N prices" note |
| `cmd/kube-node-viewer/session.go` | `azureAdvisor` wired from the `ByCloud` Azure provider |
| `NOTICE`, `README.md`, `CHANGELOG.md` | attribution of the docs data (if D1 stands), Recommendations on AKS |
| `Makefile`, `.github/workflows/refresh-data.yaml` | `make azure-catalog`, run by the monthly refresh |
| `specs/aks-pricing/requirements.md`, `specs/recommendations/requirements.md` | dated notes: R3 and the Azure out-of-scope line are superseded |

## Decisions

### D1: Catalog from the public Azure docs tables, generated into the binary

**Choice**: `hack/azcatalog` reads the size pages of
`MicrosoftDocs/azure-compute-docs`. Those are public, need no credentials
and are licensed CC BY 4.0. The generator takes from them:
- the `| Size Name | vCPUs (Qty.) | Memory (GiB) |` table;
- arm64 from the `p` feature letter, cross-checked against the series
  summary;
- the constrained flag from the `-` in the name.

It writes `pkg/azure/zz_generated_sizes.go`. The node's own capacity still
wins for sizes present in the cluster (R7). NOTICE carries the attribution.

**Evidence** (2026-11-10):

1. **Retail Prices API.** `curl -G https://prices.azure.com/api/retail/prices
   --data-urlencode "$filter=… armSkuName eq 'Standard_D4ps_v5' …"` returned
   items with the fields `armRegionName, armSkuName, currencyCode,
   effectiveStartDate, isPrimaryMeterRegion, location, meterId, meterName,
   productId, productName, retailPrice, serviceFamily, serviceId,
   serviceName, skuId, skuName, tierMinimumUnits, type, unitOfMeasure,
   unitPrice`. There is no vCPU, memory or architecture field.
2. **The docs repository.** `gh api repos/MicrosoftDocs/azure-compute-docs`
   reports license `CC-BY-4.0`, and the repository was updated that day.
   `articles/virtual-machines/sizes/` holds 162 `*-series.md` pages, in
   these categories:

   | Category | Pages |
   |---|---|
   | general-purpose | 58 |
   | memory-optimized | 55 |
   | gpu-accelerated | 21 |
   | compute-optimized | 13 |
   | storage-optimized | 8 |
   | high-performance-compute | 6 |
   | fpga-accelerated | 1 |

   The pages checked (`dpsv5`, `dpsv6`, `dsv3`, `dsv5`, `bsv2`, `fasv6`,
   `epsv6`) share the same table header. `dpsv5` lists
   `Standard_D64ps_v5 | 64 | 208`.
3. **Resource SKUs API.** `curl` on
   `management.azure.com/subscriptions/<made-up id>/providers/Microsoft.Compute/skus`
   answers `SubscriptionNotFound`. It needs a real subscription and a
   signed-in identity.
4. **Pricing calculator endpoint.** The unofficial
   `azure.microsoft.com/en-us/api/v2/pricing/virtual-machines/calculator/`,
   which used to return cores and RAM, answers 404.
5. **Azure Karpenter provider.** `Azure/karpenter-provider-azure` (Apache
   2.0) ships `known_skus.yaml`, which has names and families only, no
   specs. Its `pkg/fake/zz_generated.sku.*.go` hold 20 test SKUs, not a
   catalog.

**Justification, strongest first**:
1. It needs no credentials, either at run time or to regenerate, so the
   monthly workflow can refresh it (R15). That is unlike `make catalog` for
   AWS, which is run by hand.
2. Microsoft publishes it as the reference for size specs. Exact numbers,
   such as 208 GiB, come from it rather than from a guess.
3. Like the AWS catalog, it is compiled in, so no request is made at run
   time.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| CC BY 4.0 data, not a code license on the allowlist | the allowlist is for Go modules; the table holds facts (names, counts), attributed in NOTICE as CC BY asks; the owner confirms (open question) |
| Parsing Markdown that Microsoft may reformat | the generator fails loudly on a page it can't parse; the monthly PR shows the diff |
| A size newer than the last refresh is missing | no catalog advice for it (R12), as with AWS |

**When this would be wrong**: the owner rejects CC BY data in the
repository, or the docs stop publishing the tables. Then use alternative B.

**Reversal action**: delete the generated table and the generator. The
Azure catalog rules then show no rows, and the advisor stays wired.

**Alternatives discarded**:
- **B: Resource SKUs API (`az vm list-skus`) run by a maintainer.** It is
  authoritative (vCPUs, MemoryGB, CpuArchitectureType), but it needs an
  Azure subscription and login when the table is generated. The monthly
  workflow can't run it without an Azure identity, which the owner rules
  out. It is kept as the fallback for D1.
- **C: parsing the size name and deriving memory from a per-family ratio.**
  It needs no data file, but the memory is wrong:
  `Standard_D64ps_v5` is 208 GiB, not 64 × 4. It also can't size the B or
  M series, or constrained sizes. It is wrong advice presented as math.
- **D: the nodes' own capacity only.** It is exact, but rightsize and the
  swaps need sizes the cluster doesn't run.
- **E: third-party catalogs or the calculator endpoint.** They are
  unofficial, their license is unclear, and the endpoint is already gone
  (evidence 4).

### D2: An advisor per cloud; the rules see the node's cloud, region and OS

**Choice**: `recommend.Node` gains `Cloud`, `Region`, `Windows` and `System`
(the AKS pool mode). The price functions take the node and a size:
`OnDemand(n, size)` and `SpotIn(n, size)`. `Spec` keeps its single
argument; Azure names start with `Standard_`, so they never collide with
AWS types. `model` builds one advisor per cloud and routes each call by
`n.Cloud`. Groups, families and candidates are keyed by cloud.

**Justification, strongest first**:
1. Azure prices depend on the region and the OS (aks-pricing R1). Today's
   `OnDemand(type)` can't express that.
2. It makes mixed clusters correct (R14), where today an AWS node on screen
   silently drops every Azure node from the analysis.

**Trade-offs accepted**: the AWS advisor gets new signatures and ignores the
extra fields. That is a mechanical change, guarded by the unchanged $242.94
fixture.

**When this would be wrong**: AWS prices would also need the node's region.
That is the case in principle (fix-aws-pricing-region), and this shape
already allows it.

**Reversal action**: put the old signatures back and keep the Azure advisor
behind a session-level choice, which drops R14.

**Alternatives discarded**:
- An Azure advisor fixed to one region per session. It is simpler, since an
  AKS cluster is in one region, but it still can't tell a Windows node's
  price from a Linux one. It also keeps the "first cloud wins" gate that
  breaks R14.

### D3: Candidate prices on demand, analysis again when they arrive

**Choice**: `azure.Provider.ListPrice` reads the cache. On a miss it queues
the (region, size) in `wanted`, just as `NodePrice` does, so the existing
fetch loop batches it. It also bumps a generation counter whenever an
answer is stored. `recKey` includes that counter, so the next update after
prices arrive runs the analysis again. The fetch splits a region's sizes
into requests of at most 20 sizes.

**Justification, strongest first**:
1. It reuses the provider's retry, back-off and "unknown size" handling
   (aks-pricing R2, fix-azure-retry). No second HTTP path.
2. The candidates of a node are few: the smaller sizes of its series plus
   two targets, about 10 per series. So the first answer is one or two
   requests.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Rows appear a moment after the screen opens | the note says what is awaited; Overview did the same for prices (aks-pricing) |
| The 20-sizes split is a guess at the filter length limit | unmeasured; a 400 answer is logged and the split can shrink |

**When this would be wrong**: a cluster with many series makes many requests
when the screen opens. In that case, fetch a whole series by its product
name instead. One request returned all seven `Dpsv5` sizes with
`productName eq 'Virtual Machines Dpsv5 Series'`, checked 2026-11-10.

**Reversal action**: prefetch every catalog size of the cluster's regions at
start (dozens of pages per region).

**Alternatives discarded**:
- Fetching the whole region at start: thousands of items in dozens of pages,
  most of them never used.
- Static Azure price tables like the AWS ones: prices per region, per OS and
  per Spot change daily (the Spot meters had dates of 2026-11-01).

### D4: Allocatable of a size the cluster doesn't run: the same absolute reservation

**Choice**: for an Azure candidate with no node of that size in the
cluster, the allocatable is the candidate's catalog capacity minus the
measured node's reservation. The reservation is the measured node's catalog
capacity minus its allocatable, for cpu and memory separately. The
measuring node is the node being rightsized. The AWS rule (a ratio, else
0.90/0.85) is unchanged.

**Justification, strongest first**:
1. AKS reserves cpu on a regressive scale. It reserves memory either by a
   fixed amount per max-pods plus eviction (newer clusters) or by a
   regressive share (older ones). In every one of these, a smaller node
   reserves no more in absolute terms than a larger one. Keeping the larger
   node's absolute reservation never overstates the smaller node's room.
2. A ratio taken from the larger node overstates a small node's allocatable
   under the per-max-pods formula, which is the risky direction for a "it
   fits" verdict.

**Trade-offs accepted**: it may understate the room, so some rightsizes that
would fit are missed. That is the safe side.

**When this would be wrong**: the candidate goes to a pool with a different
max-pods or OS than the node. Then its reservation differs. The math says
which node it was measured on.

**Reversal action**: use the AWS ratio rule for Azure too.

**Alternatives discarded**:
- Coding AKS's reservation formula. It changes with the Kubernetes version
  and the OS, and the version isn't on the node in a reliable form.

### D5: Fixed Azure tables; generation at the same price is information

**Choice**: two tables in `pkg/recommend`, both reference data. The final
entries are checked against the generated catalog in T3.

| Generation (x86 → newer x86, same family and features) | Arm counterpart (Linux) |
|---|---|
| `Dv3`, `Dv4` → `Dv5` | `Dv5`, `Dsv3`, `Dsv4`, `Dsv5`, `Dsv6`, `Dasv5`, `Dasv6` → `Dpsv6` |
| `Dsv3`, `Dsv4` → `Dsv5` | `Ddsv4`, `Ddsv5`, `Ddsv6`, `Dadsv5` → `Dpdsv6` |
| `Ddv4` → `Ddv5`; `Ddsv4` → `Ddsv5` | `Dlsv5`, `Dlsv6`, `Dalsv6` → `Dplsv6` |
| `Dasv4` → `Dasv5`; `Dadsv4` → `Dadsv5` | `Dldsv5`, `Dldsv6` → `Dpldsv6` |
| `Ev3`, `Ev4` → `Ev5`; `Esv3`, `Esv4` → `Esv5` | `Esv3`, `Esv4`, `Esv5`, `Esv6`, `Easv5`, `Easv6` → `Epsv6` |
| `Edsv4` → `Edsv5`; `Easv4` → `Easv5` | `Edsv4`, `Edsv5`, `Edsv6`, `Eadsv5` → `Epdsv6` |
| `Bs` (v1) → `Bsv2` | `Bsv2`, `Basv2` → `Bpsv2` |

The target is the size with the same vCPU count and no less memory. A
generation row at the same price is `info` ("same price, newer hardware");
a cheaper one is priced; a dearer one gives no row. Generation targets are
v5, not v6. On 2026-11-10 in `eastus`:

| Size | Price/h |
|---|---|
| D4s_v3 | $0.192 |
| D4s_v5 | $0.192 |
| D4s_v6 | $0.202 |
| E4s_v3 | $0.252 |
| E4s_v5 | $0.252 |

**Justification, strongest first**:
1. Azure prices an older and a newer Intel generation the same (evidence
   above). Keeping AWS's "only when cheaper" rule would make the generation
   rule show nothing on Azure. The upgrade is still worth knowing, as free
   newer hardware and an exit from series headed for retirement.
2. v6 Intel costs more than v5, so a v6 target would never pass the price
   check.
3. Matching the memory as well as the vCPU count avoids
   `Standard_D64ps_v5`-style traps.

**Trade-offs accepted**: `info` generation rows add lines with no money. Like
idle rows, they stay out of the total.

**When this would be wrong**: the owner wants only rows that save money.
Then drop the `info` case (one condition). Or Azure starts pricing v6 below
v5. Then move the targets.

**Reversal action**: edit the tables, or remove the `info` case.

**Alternatives discarded**:
- Reading generation from a "previous generation" list. The docs' lifecycle
  pages cover retirements, not which series is the newest.
- Targets across vendors (Intel → AMD, often cheaper). That is out of
  scope, as a deferred idea.

### D6: The kind reads `arm` on Azure rows

**Choice**: the kind keeps its place in the order (after generation, before
old node). It shows `graviton` for AWS rows and `arm` for Azure rows.

**Justification**: Graviton is the AWS product name. On Azure the
counterparts are Cobalt 100 and Ampere Altra, and calling them Graviton
would be wrong.

**Trade-offs accepted**: a mixed screen shows two labels for the same kind.

**When this would be wrong**: the owner prefers one neutral label (`arm`)
everywhere. That would change the AWS screen and the golden files.

**Reversal action**: one `String()` change per cloud.

**Alternatives discarded**:
- Renaming the AWS kind to `arm` as well. That changes released behaviour
  without being asked.

### D7: AKS wording and limits

**Choice**:

| Rule | Azure wording or limit |
|---|---|
| Consolidation | scales the agent pool |
| Rightsize | names a new agent pool, because an agent pool's VM size is fixed when it is created |
| Spot | adds a Spot agent pool; no Spot row for `mode=system` pools |
| Old node | reimages to a fresh node image |
| Exclusions | ACI virtual nodes (`type=virtual-kubelet`) are excluded |

**Justification**: the action has to be one AKS can actually do. A Spot
system pool is refused by AKS.

**Trade-offs accepted**: the cluster autoscaler's settings can't be read
without the ARM API. The consolidation action names both scaling and the
autoscaler minimum.

**When this would be wrong**: AKS starts allowing an agent pool's size to
change in place, or Spot system pools.

**Reversal action**: change the action strings.

**Alternatives discarded**:
- Reusing the EKS wording ("node group"). It names things AKS doesn't have.

## Invariants

- No Azure credentials, no `~/.azure`, no ARM API, at run time or at
  generation.
- No new remote service: only the Retail Prices API, already used.
- Requests vs allocatable only; same input, same advice.
- An EKS-only cluster gives the same rows and total as before.
- `Update` never waits on analysis or I/O.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Retail Prices API down or 429 | the rows that need a missing price are absent; the provider retries (fix-azure-retry) | header shows the logged error; note "waiting for N Azure prices" |
| Size not in the catalog | no catalog advice for that node | its idle and old-node rows only |
| Docs page format changed | the generator fails, naming the page | the monthly refresh PR fails, and the old table stays |
| A size missing in the region | no row for that candidate | the math of other rows; nothing for it |

## Test plan

- `pkg/azure`:
  - the generated table loads (`Standard_D64ps_v5` is 64 vCPU / 208 GiB,
    arm64, and constrained sizes are flagged);
  - the counterpart lookup;
  - `ListPrice` queues a miss, batches at most 20 sizes and bumps the
    generation;
  - Windows and Spot meters.
- `hack/azcatalog`: parses fixture pages (a normal table, a constrained
  size, an Arm summary); fails on a page with no table.
- `pkg/recommend`:
  - the usage-example fixture end to end: rows, order, the `arm` label,
    and the $316.38 total;
  - R7 with and without a node of the candidate size (D4);
  - R8 `info` at the same price, and no row when dearer;
  - R9 with no row for Windows and none when the target has less memory;
  - R11 regional Spot, no Spot row for a system pool, the Windows Spot
    meter;
  - R12 exclusions;
  - R14 the mixed fixture;
  - the AWS fixture unchanged at $242.94.
- `pkg/model`:
  - the gate (`azure` analysed, `gce` message);
  - the generation in `recKey` re-runs the analysis;
  - the "waiting for N" note;
  - the agent pool, mode and `type` labels into `recommend.Node`.
- Golden screens: a Recommendations screen of the made-up AKS cluster.
- A run against a cluster: a kind cluster with fake `azure://` nodes
  carrying region and size labels. It prices them through the public API,
  without credentials. The verify task notes what was checked.
