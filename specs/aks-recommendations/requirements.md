# Requirements: aks-recommendations

> Status: draft · Owner: Carlos Gomes · Reviewed: 2026-11-10 (spec-refuter; 5 HIGH, 5 MEDIUM, 10 LOW; applied)

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

The owner wants the screen on AKS clusters, with no Azure credentials: no
login, no service principal, no managed identity, no IAM role. This spec
adds three things:
1. An Azure size catalog, copied from the Apache-2.0 table that the
   Kubernetes cluster autoscaler ships.
2. Azure prices for candidate sizes.
3. The rules mapped onto AKS:
   - agent pools instead of managed node groups;
   - the Arm series AKS documents instead of Graviton;
   - regional Spot prices;
   - the system-pool limits AKS enforces.

## Usage examples

The fixture is made up and lives in `pkg/recommend/aks_fixture_test.go`.
It describes an AKS cluster in `eastus` with Linux nodes and an age limit
of 30 days. Prices are the `eastus` list prices the Retail Prices API
returned on 2026-11-10; the fixture pins them. Every node carries
DaemonSets requesting 0.25 vCPU / 0.5 GiB (2 pods), and every pool has
max-pods 250.

| Agent pool (mode) | Size, price/h | Allocatable per node | Pods per node (besides DaemonSets) |
|---|---|---|---|
| `system` (system) | 3 × Standard_D4s_v5, $0.192 | 3.86 vCPU / 12.9 GiB | 2 × (1.0 vCPU / 3.0 GiB) |
| `userpool` (user) | 4 × Standard_D8s_v5, $0.384 | 7.82 vCPU / 27.6 GiB | vmss000000–02: 6 × (1.0 / 3.5); vmss000003: 1 × (1.75 / 5.5) |
| `legacy` (user) | 2 × Standard_D4s_v3, $0.192 | 3.86 vCPU / 12.9 GiB | 2 × (1.0 / 3.0); vmss000001 is 40 days old |

Nodes are named `aks-<pool>-12345678-vmss00000N`. The other list prices
the rules use:

| Size | Regular/h | Spot/h |
|---|---|---|
| Standard_D4ps_v5 | $0.154 | |
| Standard_D8ps_v5 | $0.308 | |
| Standard_D4s_v5 | | $0.040531 |
| Standard_D8s_v5 | | $0.081062 |
| Standard_D4s_v3 | | $0.038381 |

No pod is pending.

```text
$ kube-node-viewer
:rec↵
╭ kube-node-viewer · Recommendations ─────────────────────────────────────────────╮
│ Savings: $445.30/month  best non-Spot option per node; estimates from requests   │
│          and list prices, by requests only                                       │
╰─────────────────────────────────────────────────────────────────────────────────╯
  KIND        TARGET                                              ACTION                                             SAVES/MO
  idle        agent pool userpool (4 nodes)                       $377.46/mo requested by nothing                        info
  idle        agent pool system (3 nodes)                         $175.38/mo requested by nothing                        info
  idle        agent pool legacy (2 nodes)                         $116.92/mo requested by nothing                        info
  rightsize   aks-userpool-12345678-vmss000003 (Standard_D8s_v5)  → Standard_D4s_v5 (≈52% cpu, 47% mem)              $140.16
  generation  agent pool legacy (Standard_D4s_v3 ×2)              → Standard_D4s_v5: same price, newer hardware          info
  arm         agent pool userpool (Standard_D8s_v5 ×4)            add a Standard_D8ps_v5 agent pool (arm64 images!)   $221.92
  arm         agent pool system (Standard_D4s_v5 ×3)              add a Standard_D4ps_v5 agent pool (arm64 images!)    $83.22
  arm         agent pool legacy (Standard_D4s_v3 ×2)              add a Standard_D4ps_v5 agent pool (arm64 images!)    $55.48
  old node    aks-legacy-12345678-vmss000001 (40 days)            upgrade the node image                                    —
  spot        agent pool userpool (On-Demand Standard_D8s_v5 ×4)  add a Spot agent pool alongside it (evictions!)     $884.58
  spot        agent pool legacy (On-Demand Standard_D4s_v3 ×2)    add a Spot agent pool alongside it (evictions!)     $224.28
```

There are no consolidation rows: first-fit decreasing at 80% needs 3, 4
and 2 nodes, the size of each pool. There is no Spot row for `system`.
AKS documents a Spot pool only as a secondary pool, never the default
one, and AKS gives its system pods anti-affinity to Spot nodes.

The total counts each node once, with its best non-Spot saving:

| Nodes | Best saving | Amount |
|---|---|---|
| vmss000003 | rightsize | $140.16, not Arm $55.48 |
| the other three `userpool` nodes | Arm | 3 × $55.48 |
| `system` | Arm | 3 × $27.74 |
| `legacy` | Arm | 2 × $27.74 |
| **Total** | | **$445.30** |

The generation row is `info`, because D4s_v3 and D4s_v5 had the same
`eastus` price. All figures were checked by hand: a script replaying the
rules' arithmetic, kept in the session, not in the repository.

```text
# enter on the rightsize row
  aks-userpool-12345678-vmss000003: Standard_D8s_v5 → Standard_D4s_v5
  requests 2.00 vCPU / 6.00 GiB (DaemonSets 0.25 / 0.50 included)
  Standard_D4s_v5 allocatable ≈ 3.86 vCPU / 12.90 GiB (allocatable of this cluster's Standard_D4s_v5 nodes)
  after the move: cpu 52%, memory 47% ≤ 80%; pods 3 of 250
  Standard_D2s_v5 doesn't fit: ≈ 1.82 vCPU / 3.60 GiB (catalog capacity minus this node's reservation; may understate)
  the pool has one VM size: resize it in place (preview) only if every node fits, else move the workloads to a pool of that size
  saving: ($0.3840 − $0.1920) × 730 h = $140.16
```

Failure examples:
- **The candidate prices have not arrived yet.** The rows that need them are
  absent, and a note says `waiting for the prices of 7 Azure sizes`. The
  rows appear when the answer arrives.
- **The Retail Prices API fails.** The rows that need it are absent, and the
  header shows the logged error, as Overview does.
- **A size the catalog lacks** (newer than the copied table), **or an ACI
  virtual node** (a made-up node object in the fixture:
  `type=virtual-kubelet`, no `node.kubernetes.io/instance-type`). That node
  gets idle and old-node rows only.
- **A GKE cluster.** The screen says `Recommendations cover EKS and AKS
  nodes only for now; the nodes shown run on GCP.`

## Requirements

**R1** WHEN the nodes shown include Azure nodes (provider ID `azure://`)
THEN the Recommendations screen SHALL analyse them with the rules of
recommendations R5–R11, as mapped by R5–R11 below, instead of saying it
covers EKS nodes only.

- [ ] When no AWS or Azure node is shown, the screen says it covers EKS and
  AKS nodes only, and names the cloud shown.
- [ ] An EKS-only cluster gives the same rows, wording and total as before.
  Test: the recommendations fixture still totals $242.94, and the AWS
  golden screens are unchanged.

**R2** WHEN a rule uses an Azure size THEN it SHALL take that size's family,
vCPU count, memory and architecture from the Azure size catalog, and the
math popup SHALL show them.
- The catalog is the cluster autoscaler's Azure instance-type table, copied
  into the binary (design D1).
- A size missing from it gets no catalog-based advice (R12).

- [ ] Family: the table's `SkuFamily`, lower-cased, without the `standard`
  prefix and the `Family` suffix:
  - `standardDSv5Family` → `dsv5`;
  - `StandardDpsv6Family` → `dpsv6`;
  - `standardDav6Family` → `dav6`, which holds the Dasv6 sizes.

  The family is never parsed from the size name. Names such as
  `Standard_DS2_v2`, `Standard_B2ms` and `Standard_B1ls2` don't follow one
  pattern.
- [ ] Constrained-vCPU sizes (`Standard_E8-4ds_v5`) are flagged by the `-`
  in the name. The table gives them their full vCPU count (8). They are
  never a candidate or a target, and no row recommends one.
- [ ] Test: the copied table loads, and `Standard_D64ps_v5` has 64 vCPU /
  208 GiB (212992 MB), arm64. The memory is not 4 GiB × vCPU.

**R3** WHEN a rule needs the list price of an Azure size in a region THEN the
system SHALL take it from the Retail Prices API, as aks-pricing R1 prices a
node:
- in the node's region;
- for the node's OS (the Windows meter for a Windows node);
- from the regular meter, or the `… Spot` meter for Spot.

- [ ] Prices not cached are asked for in the background, batched with the
  provider's other wanted sizes of that region. Each request holds at most
  17 sizes, the cap of [fix-azure-price-batch](../fix-azure-price-batch/bugfix.md),
  reused rather than redefined: the API refuses a filter of more than 20
  comparisons, and three are the fixed clauses (design D3). The requests run one after another.
  Test: 18 wanted sizes make two requests.
- [ ] The analysis runs again when they arrive. Until then, the rows that
  need them are absent, and a note gives the number of sizes awaited.
- [ ] A size the API doesn't know in that region gives no row, and it is not
  asked again in the session (aks-pricing R2).
- [ ] With `--disable-pricing` nothing is fetched. The rows that need a price
  are absent, as on EKS.
- [ ] The UI goroutine never waits on a request.

**R4** WHEN an Azure node has `kubernetes.azure.com/agentpool` and no
`karpenter.sh/nodepool` THEN the rules SHALL treat its agent pool as a
managed node group (recommendations R13). Targets read `agent pool <name>`.
A node with `karpenter.sh/nodepool` (AKS node auto-provisioning) SHALL be
treated as a Karpenter NodePool.

- [ ] Test: the fixture's targets read `agent pool <name>`. A node with
  both labels groups under its NodePool.

**R5** (idle) WHEN Azure nodes have idle cost THEN the system SHALL list it
per agent pool or NodePool, as recommendations R5.

- [ ] Test: the fixture's three idle rows ($377.46, $175.38, $116.92).

**R6** (consolidate) WHEN the pods of an agent pool fit on fewer nodes of its
size at the headroom (recommendations R6 packing) THEN the system SHALL
recommend it. The action is `scale agent pool <name> to N nodes (or lower
its autoscaler minimum)`, and the saving is the price of the removed nodes.
A node auto-provisioning NodePool gives an `info` row, as Karpenter does.

- [ ] A pool of mode `system` (`kubernetes.azure.com/mode=system`) is never
  recommended below 2 nodes: AKS requires at least two.
- [ ] Test: the fixture gives no row. A system pool of 3 that packs into 1
  is recommended down to 2.

**R7** (rightsize) WHEN an Azure node's requests fit a smaller size of the
same family and architecture at the headroom THEN the system SHALL recommend
the cheapest such size that fits, with the price difference. "Smaller"
means fewer vCPU, or less memory, in the catalog.

- [ ] The candidate's allocatable, in this order:
  - the allocatable of a node of that size in the cluster;
  - else the candidate's catalog capacity minus the node's reservation, per
    resource. The reservation is the node's catalog capacity minus its
    allocatable.

  The math shows which, with "may understate" for the second (design D4).
- [ ] The candidate's pod capacity is that of a node of that size in the
  cluster, else the node's own allocatable pods. The math says which.
- [ ] The math says the pool has one VM size. It can be resized in place
  (preview, same architecture) only if every node fits; otherwise the
  workloads move to a pool of that size.
- [ ] For a node of a `system` pool, the candidate also honours R15.
- [ ] Test: vmss000003 → D4s_v5 at $140.16. With no D4s_v5 node in the
  cluster, the estimate is 3.82 vCPU / 11.60 GiB, which still fits. D2s_v5
  (≈ 1.82 vCPU / 3.60 GiB) does not fit.

**R8** (generation) WHEN an Azure node's family has a newer x86 family of the
same class and features in the fixed table (design D5) THEN the system SHALL
recommend the size of the target family with the same vCPU count and no
less memory, per agent pool or NodePool:
- cheaper, with the saving;
- within $0.0001/h of the same price, as `info` ("same price, newer
  hardware"), outside the total.

- [ ] A dearer target, or one with no size of that vCPU count and at least
  that memory, gives no row.
- [ ] Test: legacy D4s_v3 ×2 → D4s_v5, `info`. The same target at
  $0.19205/h is `info`; at $0.1925/h it gives no row.

**R9** (arm) WHEN an x86_64 Linux Azure node's family has an Arm counterpart
in the fixed table (design D5) THEN the system SHALL recommend that
counterpart, per agent pool or NodePool, warning that images must run on
arm64. The fixed table holds only the families AKS documents for Arm64 node
pools: `dpsv5`, `dpdsv5`, `dplsv5`, `dpldsv5`, `epsv5`, `epdsv5`. The target
is the counterpart size with the same vCPU count and no less memory, and it
must be cheaper.

- [ ] The kind reads `arm` on Azure rows and stays `graviton` on AWS rows,
  in the same place in the order (design D6).
- [ ] An agent pool's action is `add a <size> agent pool (arm64 images!)`,
  because the architecture can't change in place. A NodePool's action is
  `allow <size> in the NodePool (arm64 images!)`.
- [ ] No row for a Windows node: AKS has no Arm64 Windows pools.
- [ ] A `system` pool's target honours R15.
- [ ] Test:
  - userpool D8s_v5 ×4 → D8ps_v5, $221.92;
  - a D64s_v5 node gets no row, because D64ps_v5 has 208 GiB, less than
    256 GiB.

**R10** (old node) WHEN an Azure node is older than the age limit THEN the
system SHALL recommend `upgrade the node image`, with no saving
(recommendations R10). The math names
`az aks nodepool upgrade --node-image-only`, which recycles the pool's
nodes.

- [ ] Test: legacy vmss000001 (40 days) gets the row; a 10-day node gets
  none.

**R11** (spot) WHEN a regular Azure agent pool or NodePool has a Spot price
for its size in the nodes' region THEN the system SHALL recommend Spot last
(recommendations R11), with that saving.
- An agent pool's action is `add a Spot agent pool alongside it
  (evictions!)`.
- A NodePool's action is `allow spot in the NodePool (evictions!)`.

- [ ] Azure Spot prices are per region; the zone is not used, and the math
  says the price is regional.
- [ ] No Spot row for an agent pool of mode `system`. The AKS Spot page
  says a Spot pool can only be a secondary pool, and that system pods have
  anti-affinity to Spot nodes.
- [ ] A Windows node takes the Windows Spot meter.
- [ ] Test: userpool → $884.58, legacy → $224.28, system → no row.

**R12** WHEN an Azure node can't be compared THEN it SHALL get no
catalog-based advice (rightsize, generation, arm, Spot), as recommendations
R15. That covers:
- a node with no `node.kubernetes.io/instance-type`;
- a size missing from the catalog;
- a node labelled `type=virtual-kubelet` (ACI virtual node).

- [ ] A node with the `kube-node-viewer/instance-price` label is compared
  using that price, said in the math.
- [ ] Test: the fixture's made-up virtual node and a made-up size absent
  from the table get only idle and old-node rows.

**R13** WHEN the total saving is shown THEN it SHALL count each Azure node
once, as recommendations R14. These rows stay out of it:
- `info` rows: idle, NodePool consolidation, and same-price generation;
- Spot rows.

- [ ] Test: the fixture totals $445.30.

**R14** WHEN AWS and Azure nodes are shown together THEN each node SHALL be
advised from its own cloud's catalog, prices and wording.
- No candidate or target crosses clouds.
- Groups never mix clouds.
- The single total covers both.

- [ ] Test: a mixed fixture gives the rows of both fixtures, and the sum of
  their totals ($242.94 + $445.30 = $688.24).

**R15** WHEN a rule recommends a size for a pool of mode `system` THEN the
size SHALL meet AKS's system-pool limits:
- at least 4 vCPU and 4 GiB of memory;
- not a B-series family (`bs`, `bsv2`, `basv2`, `bpsv2`);
- a pod capacity of at least 30. When the pool keeps its max-pods, the
  node's own allocatable pods count.

- [ ] Test: a system-pool D4s_v5 node needing 1 vCPU is not rightsized to
  D2s_v5 (2 vCPU). A B-series target is never offered to a system pool.

**R16** WHEN a maintainer refreshes the Azure catalog THEN the target SHALL
copy the cluster autoscaler's table with no credentials, and the monthly
data-refresh workflow SHALL run it with the price tables.

- [ ] `make azure-catalog` downloads
  `cluster-autoscaler/cloudprovider/azure/azure_instance_types.go` from
  `kubernetes/autoscaler`, as `make prices` copies Karpenter's tables. It
  rewrites the package line, adds the modification note, and keeps the
  license header. A run with no upstream change leaves the file unchanged.
  _Test: ci — `refresh-data.yaml` opens the PR._

## Counter-examples

- SHALL NOT ask for Azure credentials, read `~/.azure`, or call the Azure
  Resource Manager API, at run time or when refreshing the catalog.
- At run time, SHALL NOT call any service other than the Retail Prices API,
  which aks-pricing already uses.
- SHALL NOT derive memory or family from the size name.
- SHALL NOT recommend:
  - for a system pool: Spot, consolidation below 2 nodes, or a size under 4
    vCPU or 4 GiB or of the B series;
  - Spot for a Spot node, or for a Windows node without a Windows Spot
    price;
  - Arm for a Windows node, or an Arm family AKS doesn't document;
  - a constrained-vCPU size, or a target with less memory than the node's
    size.
- SHALL NOT claim a zone-level Spot price on Azure.
- SHALL NOT change any row, word or total of an EKS-only cluster.

## Out of scope

- Azure Reservations and Savings Plans.
- Swaps between vendors of the same architecture, such as Intel `Dsv5` to
  AMD `Dasv5`, which is often cheaper (a deferred idea).
- Arm v6 (Cobalt 100) targets, which AKS's Arm64 page doesn't name yet.
  Deferred by the owner, 2026-11-10 (design D5).
- Generation or Arm targets for:
  - the GPU (`N*`), HPC (`H*`), confidential (`DC*`, `EC*`), storage
    (`L*`), M, `Av2` and `Fsv2` families (only AMD is newer than `Fsv2`);
  - the B series to Arm (`bpsv2` is not in AKS's Arm64 list).
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
