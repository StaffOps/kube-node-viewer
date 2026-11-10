# Requirements: aks-recommendations

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-10 (spec-refuter, second pass; 2 HIGH, 5 MEDIUM, 13 LOW; applied), earlier 2026-11-10 (spec-refuter; 5 HIGH, 5 MEDIUM, 10 LOW; applied); diff 2026-11-10 (spec-refuter, two passes; 1 HIGH, 9 MEDIUM, 15 LOW; applied)

## Context

The Recommendations screen ([recommendations](../recommendations/requirements.md))
covers EKS nodes only.
- **AKS nodes only:** the screen says "Recommendations cover EKS nodes only
  for now" ([aks-pricing](../aks-pricing/requirements.md) R3,
  `pkg/model/recommendations.go:135-137`).
- **Mixed with AWS nodes:** the gate passes (`shownCloud` returns `aws`,
  `pkg/model/recommendations.go:201-204`). Every selected node goes in
  (`:84-86`), so Azure nodes get AWS-worded idle and old-node rows ("fresh
  AMI") and nothing else.

The rules need an instance catalog (vCPU, memory, architecture, family) and
list prices for sizes the cluster does not run. AWS has both. Azure has only
the prices of the sizes on the nodes (`pkg/azure/pricing.go`), and the
Retail Prices API carries no vCPU, memory or architecture (design D1).

> **Corrected 2026-11-10 (azure-live-skus).** Once azure-live-skus ships, the size
> catalog (point 1 below) is read at run time from Azure Resource Manager with the
> machine's Azure CLI sign-in, by the owner's decision; prices stay credential-free
> ([azure-live-skus](../azure-live-skus/requirements.md) Context).

The owner wants the screen on AKS clusters, with no Azure credentials: no
login, no service principal, no managed identity, no IAM role. This spec
adds three things:
1. An Azure size catalog, copied from the Apache-2.0 table that the
   Kubernetes cluster autoscaler ships.
2. Azure prices for candidate sizes, fetched through the batch split of
   [fix-azure-price-batch](../fix-azure-price-batch/bugfix.md).
3. The rules mapped onto AKS:
   - agent pools instead of managed node groups;
   - the Arm series AKS documents instead of Graviton;
   - regional Spot prices;
   - the system-pool limits AKS enforces;
   - targets that keep the node's temp disk.

## Usage examples

The fixture is made up and lives in `pkg/recommend/aks_fixture_test.go`.
It describes an AKS cluster in `eastus` with an age limit of 30 days and no
pending pod. Prices are the `eastus` list prices the Retail Prices API
returned on 2026-11-10; the fixture pins them.

Every node has:
- the labels `kubernetes.io/os=linux` and `topology.kubernetes.io/region=eastus`;
- a zone among `eastus-1`, `eastus-2` and `eastus-3`;
- no `kubernetes.azure.com/scalesetpriority` label, so regular priority;
- an age of 10 days, unless stated;
- DaemonSets requesting 0.25 vCPU / 0.5 GiB (2 pods);
- max-pods 250 (allocatable pods).

Allocatable follows the AKS reservation for max-pods 250. Node names are
`aks-<pool>-12345678-vmss00000N`.

| Agent pool (`kubernetes.azure.com/mode`) | Size, price/h | `status.capacity` / allocatable per node | Pods per node (besides DaemonSets) |
|---|---|---|---|
| `system` (`system`) | 3 × Standard_D4s_v5, $0.192 | 4 vCPU / 15.6 GiB; 3.86 / 11.9 | 2 × (1.0 vCPU / 3.0 GiB) |
| `userpool` (`user`) | 4 × Standard_D8s_v5, $0.384 | 8 / 31.3; 7.82 / 27.2 | vmss000000–02: 6 × (1.0 / 3.5); vmss000003: 1 × (1.75 / 5.5) |
| `legacy` (`user`) | 2 × Standard_D4s_v3, $0.192 | 4 / 15.6; 3.86 / 11.9 | 2 × (1.0 / 3.0); vmss000001 is 40 days old |

The other list prices the rules use:

| Size | Regular/h | Spot/h |
|---|---|---|
| Standard_D2s_v5 | $0.096 | |
| Standard_D2s_v3 | $0.096 | |
| Standard_D4ds_v5 | $0.226 | |
| Standard_D8ps_v5 | $0.308 | |
| Standard_D4pds_v5 | $0.181 | |
| Standard_D8s_v5 | | $0.081062 |
| Standard_D4s_v3 | | $0.038381 |

```text
$ kube-node-viewer
:rec↵
╭ kube-node-viewer · Recommendations ─────────────────────────────────────────────╮
│ Savings: $322.66/month  best non-Spot option per node; estimates from requests   │
│          and list prices, by requests only                                       │
╰─────────────────────────────────────────────────────────────────────────────────╯
  KIND        TARGET                                              ACTION                                             SAVES/MO
  idle        agent pool userpool (4 nodes)                       $377.46/mo requested by nothing                        info
  idle        agent pool system (3 nodes)                         $175.38/mo requested by nothing                        info
  idle        agent pool legacy (2 nodes)                         $116.92/mo requested by nothing                        info
  rightsize   aks-userpool-12345678-vmss000003 (Standard_D8s_v5)  → Standard_D4s_v5 (≈52% cpu, 50% mem)              $140.16
  arm         agent pool userpool (Standard_D8s_v5 ×4)            add a Standard_D8ps_v5 agent pool (arm64 images!)   $221.92
  arm         agent pool legacy (Standard_D4s_v3 ×2)              add a Standard_D4pds_v5 agent pool (arm64 images!)   $16.06
  old node    aks-legacy-12345678-vmss000001 (40 days)            upgrade the node image                                    —
  spot        agent pool userpool (On-Demand Standard_D8s_v5 ×4)  add a Spot agent pool alongside it (evictions!)     $884.58
  spot        agent pool legacy (On-Demand Standard_D4s_v3 ×2)    add a Spot agent pool alongside it (evictions!)     $224.28
```

Some rows are missing on purpose:
- **Consolidation:** none. First-fit decreasing at 80% needs 3, 4 and 2
  nodes, the size of each pool.
- **Generation:** none. `legacy`'s D4s_v3 has a temp disk, so its target is
  D4ds_v5 (`ddsv5`, with a temp disk), which costs more ($0.226), not D4s_v5
  (design D5).
- **Arm and Spot for `system`:** none. AKS's Arm64 page doesn't document
  Arm system pools (R15). AKS documents a Spot pool only as a secondary pool,
  and gives system pods anti-affinity to Spot nodes.
- **Arm target for `legacy`:** D4pds_v5, which keeps the temp disk, not
  D4ps_v5.

The total counts each node once, with its best non-Spot saving:

| Nodes | Best saving | Amount |
|---|---|---|
| vmss000003 | rightsize | $140.16, not Arm $55.48 |
| the other three `userpool` nodes | Arm | 3 × $55.48 |
| `legacy` | Arm | 2 × $8.03 |
| `system` | none | — |
| **Total** | | **$322.66** |

All figures were checked by hand: a script replaying the rules' arithmetic,
kept in the session, not in the repository.

```text
# enter on the rightsize row
  aks-userpool-12345678-vmss000003: Standard_D8s_v5 → Standard_D4s_v5
  requests 2.00 vCPU / 6.00 GiB (DaemonSets 0.25 / 0.50 included)
  Standard_D4s_v5 allocatable ≈ 3.86 vCPU / 11.90 GiB (allocatable of this cluster's Standard_D4s_v5 nodes)
  after the move: cpu 52%, memory 50% ≤ 80%; pods 3 of 250 (a Standard_D4s_v5 node's)
  Standard_D2s_v5 doesn't fit: ≈ 1.82 vCPU / 3.20 GiB (catalog capacity minus this node's reservation; may understate)
  the pool has one VM size: resize it in place (preview) only if every node fits, else move the workloads to a pool of that size
  saving: ($0.3840 − $0.1920) × 730 h = $140.16
```

Failure examples:
- **The candidate prices have not arrived yet.** The rows that need them are
  absent, and a note says `waiting for the prices of 5 Azure sizes`. Those
  are the sizes asked that no node runs: D2s_v5, D2s_v3, D4ds_v5, D8ps_v5
  and D4pds_v5. A node's own size has all its meters cached already. The
  rows appear when the answer arrives.
- **The Retail Prices API fails.** The rows that need it are absent, and the
  header shows the logged error, as Overview does.
- **`--disable-pricing`.** Only the consolidation rows (saving `—`) and the
  old-node rows show, with the note `pricing disabled: Azure rows that need a
  price are not shown` (R3).
- **A size the catalog lacks** (newer than the copied table). That node gets
  idle and old-node rows only.
- **An ACI virtual node.** In the fixture this is a made-up node object:
  `type=virtual-kubelet`, no `node.kubernetes.io/instance-type`. It gets no
  row except idle, and none when it has no price.
- **A GKE cluster.** The screen says `Recommendations cover EKS and AKS
  nodes only for now; the nodes shown run on GCP.`

  > **Corrected 2026-11-10 (T4).** The message names the cloud as the
  > screen already does for GCE nodes, `Google Cloud`, not `GCP`.

## Requirements

**R1** WHEN the nodes shown include Azure nodes (provider ID `azure://`)
THEN the Recommendations screen SHALL analyse them with the rules of
recommendations R5–R11, as mapped by R5–R11 below, instead of saying it
covers EKS nodes only.

- [ ] When nodes are shown and none runs on AWS or Azure, the screen says it
  covers EKS and AKS nodes only, and names the cloud shown. With no node
  shown, the screen says there is nothing to recommend, as today
  (`recommendations.go:199-200`). Test: the GCP message, and an empty
  cluster.
- [ ] An EKS-only cluster gives the same rows, wording and total as before.
  Test: the recommendations fixture still totals $242.94, and the AWS
  golden screens are unchanged.

**R2** WHEN a rule uses an Azure size THEN it SHALL take that size's family,
vCPU count, memory and architecture from the Azure size catalog, and the
math popup SHALL show them.
- The catalog is the cluster autoscaler's Azure instance-type table, copied
  into the binary (design D1).
- A size missing from it gets no catalog-based advice (R12).
- An entry with 0 vCPU or 0 memory counts as missing.

- [ ] Family: the table's `SkuFamily`, lower-cased, without the `standard`
  prefix and the `Family` suffix:
  - `standardDSv5Family` → `dsv5`;
  - `StandardDpsv6Family` → `dpsv6`;
  - `standardDav6Family` → `dav6`, which holds the Dasv6 sizes.

  The family is never parsed from the size name. Names such as
  `Standard_DS2_v2`, `Standard_B2ms` and `Standard_B1ls` don't follow one
  pattern.
- [ ] Architecture: the table's `amd64` is read as `x86_64` and `arm64` as
  `arm64`, the words the rules use.
- [ ] Constrained-vCPU sizes (`Standard_E8-4ds_v5`) are flagged by the `-`
  in the name. The table gives them their full vCPU count (8). They are
  never a candidate or a target, and no row recommends one.
- [ ] Memory is the table's `MemoryMb` / 1024. The table truncates to whole
  GB:
  - `Standard_DS1_v2` reads 3 GiB, though it is 3.5;
  - `Standard_B1ls` reads 0, so it counts as missing.
- [ ] Test:
  - the copied table loads;
  - `Standard_D64ps_v5` has 64 vCPU / 208 GiB (212992 MB), arm64, so the
    memory is not 4 GiB × vCPU;
  - `Standard_DS1_v2` reads 3 GiB;
  - `Standard_B1ls` is missing.

> **Corrected 2026-11-10 (azure-live-skus).** Superseded once azure-live-skus ships: the
> family, vCPU, memory and architecture come live from the Azure Resource SKUs
> API ([azure-live-skus](../azure-live-skus/requirements.md) R1, R2), with fractional
> memory and usable vCPUs of constrained sizes; the copied table goes (its R12).

**R3** WHEN a rule needs the list price of an Azure size in a region THEN the
system SHALL take it from the Retail Prices API, as aks-pricing R1 prices a
node:
- in the node's region;
- for the node's OS (the Windows meter for a Windows node);
- from the regular meter, or the `… Spot` meter for Spot.

- [ ] Prices not cached are asked for in the background, through the fetch
  loop's split of [fix-azure-price-batch](../fix-azure-price-batch/bugfix.md).
  That fix sets the cap of 17 sizes a request, and this spec reuses it
  rather than redefining it. Test: candidate sizes beyond 17 go out in two
  requests.
- [ ] The analysis runs again when the prices arrive. Until then, the rows
  that need them are absent, and a note gives the number of sizes awaited.
  Test: an analysis before the answer has the note; after the answer, the
  rows appear.
- [ ] A size the API doesn't know in that region gives no row, and it is not
  asked again in the session (aks-pricing R2). Test: a second analysis makes
  no request for it.
- [ ] With `--disable-pricing`:
  - the Azure advisor still has the catalog, and its price lookups answer
    "no price" without ever queuing a request;
  - there is no waiting note; a note says `pricing disabled: Azure rows that
    need a price are not shown`;
  - the rows that show are consolidation (saving `—`) and old node.

  Test: session and model tests make no request and give those rows only.
- [ ] The UI goroutine never waits on a request. Test: an analysis with a
  price source that blocks returns at once.

> **Corrected 2026-11-10 (diff review M3).** Two R3 checks have no test
> yet; they are T2's. These are "candidate sizes beyond 17 go out in two
> requests" and "a size the API doesn't know is not asked again". Until T2,
> spec-trace sees R3 cited by the advisor and model tests only. The same goes
> for R16's `Test: ci` marker: it waits for T6 (`refresh-data.yaml`).
>
> **Corrected 2026-11-10 (T2).** Both R3 checks now exist:
> `TestListPriceBatches` and `TestListPriceQueuesAndAnswers`. R16 still
> waits for T6.

**R4** WHEN an Azure node has `kubernetes.azure.com/agentpool` and no
`karpenter.sh/nodepool` THEN the rules SHALL treat its agent pool as a
managed node group (recommendations R13). Targets read `agent pool <name>`.
A node with `karpenter.sh/nodepool` (AKS node auto-provisioning) SHALL be
treated as a Karpenter NodePool.

- [ ] A node with no `kubernetes.azure.com/mode` label counts as `user`.
- [ ] Test:
  - the fixture's targets read `agent pool <name>`;
  - a node with both labels groups under its NodePool;
  - a node with no mode label gets a Spot row.

**R5** (idle) WHEN Azure nodes have idle cost THEN the system SHALL list it
per agent pool or NodePool, as recommendations R5.

- [ ] Test: the fixture's three idle rows ($377.46, $175.38, $116.92).

**R6** (consolidate) WHEN the pods of an agent pool fit on fewer nodes of its
size at the headroom (recommendations R6 packing) THEN the system SHALL
recommend it.
- The action is `scale agent pool <name> to N nodes (or lower its
  autoscaler minimum)`, and the saving is the price of the removed nodes.
- A node auto-provisioning NodePool gives an `info` row, as Karpenter does.

- [ ] A pool of mode `system` (`kubernetes.azure.com/mode=system`) is never
  recommended below 2 nodes, because AKS requires at least two. When the row
  says 2, its math adds that AKS recommends three.
- [ ] Test: the fixture gives no row. A system pool of 3 that packs into 1
  is recommended down to 2, with the note.

**R7** (rightsize) WHEN an Azure node's requests fit a smaller size of the
same family and architecture at the headroom THEN the system SHALL recommend
the cheapest such size that fits, with the price difference. "Smaller"
means fewer vCPU, or less memory, in the catalog.

- [ ] The candidate's allocatable, in this order:
  - the allocatable of a node of that size in the cluster;
  - else the candidate's catalog capacity minus the node's reservation, per
    resource (design D4). The reservation is the larger of two figures: the
    node's catalog capacity minus its allocatable, and its `status.capacity`
    minus its allocatable.

  The math shows which, with "may understate" for the second. An estimate
  of 0 or less in any resource means the candidate doesn't fit.
- [ ] The candidate's pod capacity is that of a node of that size in the
  cluster, else the node's own allocatable pods. The math says which. Test:
  each source shows in the math.
- [ ] The math says the pool has one VM size and how it can change:
  - a VMSS pool (provider ID with `virtualMachineScaleSets`) can be resized
    in place (preview) only if every node fits and nothing else changes,
    otherwise the workloads move to a pool of that size;
  - a Virtual Machines pool (provider ID with `/virtualMachines/` and no
    scale set) can add the size.

  Test: both provider-ID forms.
- [ ] On Azure rows only, the math lists the cheaper smaller sizes that
  don't fit, with their estimate. AWS math is unchanged, so the AWS golden
  screens are too.
- [ ] For a node of a `system` pool, the candidate also honours R15.
- [ ] Test:
  - vmss000003 → D4s_v5 at $140.16;
  - with no D4s_v5 node in the cluster, the estimate is 3.82 vCPU / 11.20
    GiB, which still fits; D2s_v5 (≈ 1.82 vCPU / 3.20 GiB) does not fit;
  - a Standard_B2s node with tiny requests is never rightsized to
    Standard_B1ls, which counts as missing.

**R8** (generation) WHEN an Azure node's family has a newer x86 family of the
same class and features in the fixed table (design D5) THEN the system SHALL
recommend the size of the target family with the same vCPU count and no
less memory, per agent pool or NodePool. Features include the temp disk: a
family with a temp disk maps only to a family with one.
- Cheaper: a priced row, with the saving.
- Within $0.0001/h of the same price: an `info` row ("same price, newer
  hardware"), outside the total. Decided by the owner, 2026-11-10.

- [ ] A dearer target, or one with no size of that vCPU count and at least
  that memory, gives no row.
- [ ] Test:
  - legacy D4s_v3 ×2 → D4ds_v5 at $0.226 gives no row;
  - with the target priced at $0.19205/h (synthetic), it is `info`;
  - at $0.1925/h, no row;
  - at $0.180/h, a priced row.

**R9** (arm) WHEN an x86_64 Linux Azure node's family has an Arm counterpart
in the fixed table (design D5) THEN the system SHALL recommend that
counterpart, per agent pool or NodePool, warning that images must run on
arm64.
- The table holds only the families AKS documents for Arm64 node pools:
  `dpsv5`, `dpdsv5`, `dplsv5`, `dpldsv5`, `epsv5`, `epdsv5`. Decided by the
  owner, 2026-11-10.
- A family with a temp disk maps only to an Arm family with one (`dpdsv5`,
  `dpldsv5`, `epdsv5`).
- The target is the counterpart size with the same vCPU count and no less
  memory, and it must be cheaper.

- [ ] The kind reads `arm` on Azure rows and stays `graviton` on AWS rows,
  in the same place in the order (design D6; decided by the owner,
  2026-11-10).
- [ ] An agent pool's action is `add a <size> agent pool (arm64 images!)`,
  because the architecture can't change in place. A NodePool's action is
  `allow <size> in the NodePool (arm64 images!)`.
- [ ] No row for a Windows node: AKS has no Arm64 Windows pools.
- [ ] No row for a `system` pool (R15).
- [ ] Test:
  - userpool D8s_v5 ×4 → D8ps_v5, $221.92;
  - legacy D4s_v3 ×2 → D4pds_v5, $16.06;
  - a D64s_v5 node gets no row, because D64ps_v5 has 208 GiB, less than
    256 GiB.

**R10** (old node) WHEN an Azure node is older than the age limit THEN the
system SHALL recommend `upgrade the node image`, with no saving
(recommendations R10). The math names
`az aks nodepool upgrade --node-image-only`, which recycles the pool's
nodes. An ACI virtual node (`type=virtual-kubelet`) gets no old-node row:
it has no node image and no pool.

- [ ] Test:
  - legacy vmss000001 (40 days) gets the row;
  - a 10-day node gets none;
  - a 40-day virtual node gets none.

**R11** (spot) WHEN a regular Azure agent pool or NodePool has a Spot price
for its size in the nodes' region THEN the system SHALL recommend Spot last
(recommendations R11), with that saving.
- An agent pool's action is `add a Spot agent pool alongside it
  (evictions!)`.
- A NodePool's action is `allow spot in the NodePool (evictions!)`.

- [ ] Azure Spot prices are per region. The zone is not used, and the math
  says the price is regional.
- [ ] No Spot row for an agent pool of mode `system`. The AKS Spot page says
  a Spot pool can only be a secondary pool, and that system pods have
  anti-affinity to Spot nodes.
- [ ] A Windows node takes the Windows Spot meter.
- [ ] Test: userpool → $884.58, legacy → $224.28, system → no row.

**R12** WHEN an Azure node can't be compared THEN it SHALL get no
catalog-based advice (rightsize, generation, arm, Spot), as recommendations
R15. That covers:
- a node with no `node.kubernetes.io/instance-type`;
- a size missing from the catalog (R2);
- a node labelled `type=virtual-kubelet` (ACI virtual node, R10).

- [ ] A node with the `kube-node-viewer/instance-price` label is compared
  using that price, said in the math. Test: a labelled Azure node's
  rightsize math shows the label price.
- [ ] Test:
  - the made-up virtual node gets no row except idle when priced;
  - a made-up size absent from the table gets idle and old-node rows only.

> **Corrected 2026-11-10 (azure-live-skus).** Once azure-live-skus ships, a size the
> live answer lacks or restricts for the node also counts here
> ([azure-live-skus](../azure-live-skus/requirements.md) R2, R3), and when the sizes
> can't be read at all the screen says why (its R5).

**R13** WHEN the total saving is shown THEN it SHALL count each Azure node
once, as recommendations R14. These rows stay out of it:
- `info` rows: idle, NodePool consolidation, and same-price generation;
- Spot rows.

- [ ] Test: the fixture totals $322.66.

**R14** WHEN AWS and Azure nodes are shown together THEN each node SHALL be
advised from its own cloud's catalog, prices and wording.
- No candidate or target crosses clouds.
- Groups never mix clouds.
- The single total covers both.

- [ ] Test: a mixed fixture gives the rows of both fixtures, and the sum of
  their totals ($242.94 + $322.66 = $565.60).

**R15** WHEN a rule recommends a size for a pool of mode `system` THEN the
size SHALL meet AKS's system-pool limits:
- at least 4 vCPU and 4 GiB of memory;
- not a B-series family (`bs`, `bsv2`, `basv2`, `bpsv2`);
- a pod capacity of at least 30, the pod capacity R7 uses.

Beyond those limits:
- No Arm row is offered to a system pool. AKS's Arm64 page doesn't say a
  system pool can be Arm64, so that is deferred until it does.
- A generation target for a system pool reads `add a system-mode pool of
  <size>`.

- [ ] Test:
  - a system-pool D4s_v5 node needing 1 vCPU is not rightsized to D2s_v5
    (2 vCPU);
  - a B-series target is never offered to a system pool;
  - a candidate whose pod capacity is 20 is refused.

**R16** WHEN a maintainer refreshes the Azure catalog THEN the target SHALL
copy the cluster autoscaler's table with no credentials, and the monthly
data-refresh workflow SHALL run it with the price tables.

- [ ] `make azure-catalog` downloads
  `cluster-autoscaler/cloudprovider/azure/azure_instance_types.go` from
  `kubernetes/autoscaler` verbatim. It is already `package azure` and
  carries its Apache-2.0 header, so nothing in it changes and it gets no
  modification note. A run with no upstream change leaves the file
  unchanged.

> **Corrected 2026-11-10 (diff review M2).** The workflow clause ("the
> monthly data-refresh workflow SHALL run it") is deferred to its own PR
> (tasks, Deferred). R16 now holds for `make azure-catalog` run by hand.
> _Test: manual — `make azure-catalog` by hand: the table is identical to
> upstream (commit `c1239b4`), with no credentials._

> **Corrected 2026-11-10 (azure-live-skus).** Superseded by [azure-live-skus](../azure-live-skus/requirements.md)
> R9: the copied table and `make azure-catalog` are removed, and so is the deferred
> workflow clause; nothing is refreshed by hand.

## Counter-examples

> **Corrected 2026-11-10 (azure-live-skus).** Once azure-live-skus ships, the first two
> counter-examples hold for prices only: the size catalog is read from Azure Resource
> Manager with the machine's `az` sign-in (this tool still never reads `~/.azure`
> itself), and Resource Manager joins the Retail Prices API as a run-time service
> ([azure-live-skus](../azure-live-skus/requirements.md) R1, R4).

- SHALL NOT ask for Azure credentials, read `~/.azure`, or call the Azure
  Resource Manager API, at run time or when refreshing the catalog.
- At run time, SHALL NOT call any service other than the Retail Prices API,
  which aks-pricing already uses.
- SHALL NOT derive memory or family from the size name, and SHALL NOT use a
  table entry with 0 vCPU or 0 memory.
- SHALL NOT recommend:
  - a target that drops the node's temp disk;
  - for a system pool: Spot, Arm, consolidation below 2 nodes, or a size
    under 4 vCPU or 4 GiB or of the B series;
  - Spot for a Spot node, or for a Windows node without a Windows Spot
    price;
  - Arm for a Windows node, or an Arm family AKS doesn't document;
  - a constrained-vCPU size, or a target with less memory than the node's
    size.
- SHALL NOT give an ACI virtual node an old-node row.
- SHALL NOT claim a zone-level Spot price on Azure.
- SHALL NOT change any row, word or total of an EKS-only cluster.

## Out of scope

- Azure Reservations and Savings Plans.
- Swaps between vendors of the same architecture, such as Intel `Dsv5` to
  AMD `Dasv5`, which is often cheaper (a deferred idea).
- Arm v6 (Cobalt 100) targets, which AKS's Arm64 page doesn't name yet.
  Deferred by the owner, 2026-11-10 (design D5).
- Arm system pools, until AKS documents them.
- Generation or Arm targets for:
  - the GPU (`N*`), HPC (`H*`), confidential (`DC*`, `EC*`), isolated
    (`ei*`), storage (`L*`, `eb*`), M and `Av2` families;
  - `Fsv2`, because only AMD is newer;
  - the v7 families;
  - the B series (`bs` has a temp disk, and no B-series v2 family has one).
- Reading the cluster autoscaler or agent pool settings, which needs the ARM
  API.
- Disk, network and control-plane tier costs.
- GKE (stage 2).

## Decided

Decided by the owner, 2026-11-10:
- **Generation at the same price** (design D5): an `info` row, outside the
  total.
- **Arm targets** (design D5): the v5 families AKS documents only (Dpsv5,
  Dplsv5, Epsv5 and their `d` variants). v6 Cobalt stays deferred.
- **Kind label** (design D6): `arm` on Azure rows; `graviton` stays on AWS
  rows.

Decided technically by the reviewer, 2026-11-10: targets keep the node's
temp disk (design D5).
