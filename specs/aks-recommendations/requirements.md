# Requirements: aks-recommendations

> Status: draft · Owner: Carlos Gomes · Reviewed: — (spec-refuter, YYYY-MM-DD)

## Context

The Recommendations screen ([recommendations](../recommendations/requirements.md))
only covers EKS nodes. When only AKS nodes are shown, it says "Recommendations
cover EKS nodes only for now" ([aks-pricing](../aks-pricing/requirements.md)
R3, `pkg/model/recommendations.go:135-137`). It needs an instance catalog
(vCPU, memory, architecture, family) and list prices for sizes the cluster
does not run. AWS has both: the Price List
(`pkg/aws/zz_generated_instances.go`) and the pricing provider. Azure has
only prices for the sizes on the nodes (`pkg/azure/pricing.go`). The Retail
Prices API carries no vCPU, memory or architecture (see design D1).

The owner wants the screen on AKS clusters, with no Azure credentials: no
login, no service principal, no managed identity, no IAM role. This spec
adds an Azure size catalog and Azure prices for candidate sizes, and maps
each rule onto AKS:
- agent pools instead of managed node groups;
- Arm (Cobalt 100) series instead of Graviton;
- regional Spot prices.

## Usage examples

One fixture (made up). An AKS cluster in `eastus`, three agent pools, Linux
nodes. The prices are the `eastus` list prices the Retail Prices API
returned on 2026-11-10; the fixture pins them.

| Agent pool | Nodes | Mode |
|---|---|---|
| `system` | 3 × Standard_D2s_v5, $0.096/h | system |
| `userpool` | 4 × Standard_D4s_v5, $0.192/h | user |
| `legacy` | 2 × Standard_D4s_v3, $0.192/h | user |

- In `system`, each node has 1.9 vCPU / 5.4 GiB allocatable.
- In `userpool`, the node `aks-userpool-12345678-vmss000003` requests 0.9 vCPU / 3.0 GiB, DaemonSets included.
- In `legacy`, the node `aks-legacy-12345678-vmss000001` is 40 days old.

The Arm counterparts are Standard_D2ps_v6 ($0.0702/h) and Standard_D4ps_v6
($0.14/h). The Spot meters are D4s_v5 $0.040531/h and D4s_v3 $0.038381/h.

```text
$ kube-node-viewer
:rec↵
╭ kube-node-viewer · Recommendations ─────────────────────────────────────────────╮
│ Savings: $316.38/month  best non-Spot option per node; estimates from requests   │
│          and list prices, by requests only                                       │
╰─────────────────────────────────────────────────────────────────────────────────╯
  KIND        TARGET                                             ACTION                                              SAVES/MO
  idle        …                                                                                                      info
  rightsize   aks-userpool-12345678-vmss000003 (Standard_D4s_v5)  → Standard_D2s_v5 in a new agent pool (≈47% cpu, 56% mem)   $70.08
  generation  agent pool legacy (Standard_D4s_v3 ×2)              → Standard_D4s_v5: same price, newer hardware       info
  arm         agent pool userpool (Standard_D4s_v5 ×4)            add a Standard_D4ps_v6 agent pool (arm64 images!)    $151.84
  arm         agent pool legacy (Standard_D4s_v3 ×2)              add a Standard_D4ps_v6 agent pool (arm64 images!)     $75.92
  arm         agent pool system (Standard_D2s_v5 ×3)              add a Standard_D2ps_v6 agent pool (arm64 images!)     $56.50
  old node    aks-legacy-12345678-vmss000001 (40 days)            reimage for a fresh node image                            —
  spot        agent pool userpool (On-Demand Standard_D4s_v5 ×4)  add a Spot agent pool alongside it (evictions!)     $442.29
  spot        agent pool legacy (On-Demand Standard_D4s_v3 ×2)    add a Spot agent pool alongside it (evictions!)     $224.28
```

The total counts each node once, with its best non-Spot saving:

| Nodes | Best saving | Amount |
|---|---|---|
| vmss000003 | rightsize | $70.08 |
| the other three `userpool` nodes | Arm | 3 × $37.96 |
| `legacy` | Arm | 2 × $37.96 |
| `system` | Arm | 3 × $18.834 |
| **Total** | | **$316.38** |

The generation row is `info`: on 2026-11-10, D4s_v3 and D4s_v5 had the same
`eastus` price. `system` has no Spot row, because AKS does not allow a Spot
system pool.

```text
# enter on the rightsize row
  aks-userpool-12345678-vmss000003: Standard_D4s_v5 → Standard_D2s_v5
  requests 0.90 vCPU / 3.00 GiB (DaemonSets included)
  Standard_D2s_v5 allocatable ≈ 1.90 vCPU / 5.40 GiB (allocatable of this cluster's Standard_D2s_v5 nodes)
  after the move: cpu 47%, memory 56% ≤ 80%; pods 14 of 30
  an agent pool's VM size can't change: create a pool of the new size and move the workloads
  saving: ($0.1920 − $0.0960) × 730 h = $70.08
```

Failure examples:
- **The candidate prices have not arrived yet.** The rows that need them are
  absent, and a note says `waiting for 9 Azure prices`. They appear when the
  answer arrives.
- **The Retail Prices API fails.** The rows that need it are absent, and the
  header shows the logged error, as Overview does.
- **A size the catalog lacks** (newer than the release, or an ACI virtual
  node with no size). That node gets no catalog-based advice, only idle and
  old node.
- **A GKE cluster.** The screen says `Recommendations cover EKS and AKS
  nodes only for now; the nodes shown run on GCP.`

## Requirements

**R1** WHEN the nodes shown include Azure nodes (provider ID `azure://`)
THEN the Recommendations screen SHALL analyse them with the rules of
[recommendations](../recommendations/requirements.md) R5–R11 as mapped by
R5–R11 below, instead of saying it covers EKS nodes only.

- [ ] When no AWS or Azure node is shown, the screen says it covers EKS and
  AKS nodes only, naming the cloud shown (today's message for `gce`, `kind`
  and the rest).
- [ ] An EKS-only cluster gives the same rows and total as before. Test:
  the recommendations fixture still totals $242.94.

**R2** WHEN an Azure node's VM size (`node.kubernetes.io/instance-type`) is
in the Azure size catalog THEN the system SHALL know its series, vCPU count,
memory (GiB) and architecture from that catalog. The catalog is a table
generated from public data and compiled into the binary (design D1). A size
missing from it gets no catalog-based advice (R12).

- [ ] Series: the size name without the vCPU count, e.g. `Standard_D4ps_v6`
  → `Dps_v6`. Constrained-vCPU sizes (`Standard_E8-4ds_v5`) are in the
  catalog with their active vCPUs and flagged; they are never a candidate
  or a target.
- [ ] Architecture: arm64 for the sizes the generator marks Arm. That means
  the `p` feature letter, cross-checked against the series summary (design
  D1). Every other size is x86_64.
- [ ] Test: the generated table loads; `Standard_D64ps_v5` has 64 vCPU /
  208 GiB, arm64. The memory is not 4 GiB × vCPU, so it is not derived from
  the name.

**R3** WHEN a rule needs the list price of an Azure size in a region THEN the
system SHALL take it from the Retail Prices API, as
[aks-pricing](../aks-pricing/requirements.md) R1 prices a node:
- in the node's region;
- for the node's OS (the Windows meter for a Windows node);
- with the regular meter for regular capacity and the `… Spot` meter for
  Spot.

- [ ] Prices not cached are asked for in the background. They are batched
  with the provider's other wanted sizes, one request per region, in no more
  than 20 sizes per request.
- [ ] The analysis runs again when they arrive. Until then, the rows that
  need them are absent and a note gives the number of prices awaited.
- [ ] A size the API doesn't know in that region gives no row, and it is not
  asked again in the session (aks-pricing R2).
- [ ] With `--disable-pricing` nothing is fetched. The catalog-based rows
  that need a price are absent, as on EKS.
- [ ] The UI goroutine never waits on a request (recommendations
  invariants).

**R4** WHEN an Azure node has `kubernetes.azure.com/agentpool` and no
`karpenter.sh/nodepool` THEN the rules SHALL treat its agent pool as a
managed node group (recommendations R13). Targets read `agent pool <name>`.
A node with `karpenter.sh/nodepool` (AKS node auto-provisioning) SHALL be
treated as a Karpenter NodePool.

**R5** (idle) WHEN Azure nodes have idle cost THEN the system SHALL list it
per agent pool or NodePool, as recommendations R5.

**R6** (consolidate) WHEN the pods of an agent pool fit on fewer nodes of its
size at the headroom (recommendations R6 packing) THEN the system SHALL
recommend it.
- The action is `scale agent pool <name> to N nodes (or lower its autoscaler
  minimum)`.
- The saving is the price of the removed nodes.
- For a node auto-provisioning NodePool, the row is `info`, as for
  Karpenter.

- [ ] A pool of mode `system` (`kubernetes.azure.com/mode=system`) is never
  recommended below one node. _Test: the packing already returns at least
  one node; one case checks it on a system pool._

**R7** (rightsize) WHEN an Azure node's requests fit a smaller size of the
same series and architecture at the headroom THEN the system SHALL
recommend the cheapest such size that fits, with the price difference.
- "Smaller" means fewer vCPU, or less memory, in the catalog.
- The action says that the size changes through a new agent pool.

- [ ] The candidate's allocatable, in this order:
  - the allocatable of a node of that size in the cluster, when there is one;
  - else the candidate's catalog capacity minus the node's own reservation,
    which is its catalog capacity minus its allocatable, per resource (design
    D4).

  The math says which.
- [ ] The candidate's pod capacity is that of a node of that size in the
  cluster, else the node's own allocatable pods (the pool's max-pods
  setting), said in the math.
- [ ] Test: a D4s_v5 node requesting 0.9 vCPU / 3.0 GiB, with a D2s_v5 node
  in the cluster, gets → D2s_v5 at $70.08. With no D2s_v5 node, the
  estimate is D2s_v5 capacity minus the D4s_v5 node's reservation.

**R8** (generation) WHEN an Azure node's series has a newer x86 series of the
same family and features in the fixed table (design D5: `Dsv3/Dsv4 → Dsv5`,
`Esv3/Esv4 → Esv5`, …) THEN the system SHALL recommend the size with the
same vCPU count and no less memory in the target series, per agent pool or
NodePool:
- cheaper, with the saving;
- at the same price, as `info` ("same price, newer hardware"), outside the
  total.

- [ ] A target that costs more, or has no size of that vCPU count and at
  least that memory, gives no row.
- [ ] Test: legacy D4s_v3 ×2 → D4s_v5, `info`, at the fixture prices.

**R9** (arm) WHEN an x86_64 Linux Azure node's series has an Arm counterpart
in the fixed table (design D5: `Dsv5 → Dpsv6`, `Ddsv5 → Dpdsv6`,
`Esv5 → Epsv6`, `Bsv2 → Bpsv2`, …) THEN the system SHALL recommend it, per
agent pool or NodePool, warning that images must run on arm64. The target
is the size with the same vCPU count and no less memory, and it must be
cheaper.

- [ ] The kind reads `arm` on Azure rows and stays `graviton` on AWS rows,
  in the same place in the order (design D6).
- [ ] An agent pool's action is `add a <size> agent pool (arm64 images!)`.
  A NodePool's action is `allow <size> in the NodePool (arm64 images!)`.
- [ ] No row for a Windows node.
- [ ] Test: userpool D4s_v5 ×4 → D4ps_v6 at $151.84; a target with less
  memory at that vCPU count gives no row.

**R10** (old node) WHEN an Azure node is older than the age limit THEN the
system SHALL recommend `reimage for a fresh node image`, with no saving
(recommendations R10).

**R11** (spot) WHEN a regular Azure agent pool or NodePool has a Spot price
for its size in the nodes' region THEN the system SHALL recommend Spot last
(recommendations R11), with that saving.
- An agent pool's action is `add a Spot agent pool alongside it
  (evictions!)`.
- A NodePool's action is `allow spot in the NodePool (evictions!)`.

- [ ] Azure Spot prices are per region. The zone is not used, and the math
  says the price is regional.
- [ ] No Spot row for an agent pool of mode `system`: AKS does not allow a
  Spot system pool.
- [ ] A Windows node takes the Windows Spot meter.
- [ ] Test: userpool → $442.29, legacy → $224.28, system → no row.

**R12** WHEN an Azure node can't be compared THEN it SHALL get no
catalog-based advice (rightsize, generation, arm, Spot), as recommendations
R15. That covers:
- a node with no `node.kubernetes.io/instance-type`;
- a size missing from the catalog;
- an ACI virtual node (`type=virtual-kubelet`).

- [ ] A node with the `kube-node-viewer/instance-price` label is compared
  using that price, said in the math.

**R13** WHEN the total saving is shown THEN it SHALL count each Azure node
once, as recommendations R14. `info` rows (idle, NodePool consolidation, and
same-price generation) and Spot rows stay out of it.

- [ ] Test: the usage-example fixture totals $316.38.

**R14** WHEN AWS and Azure nodes are shown together THEN each node SHALL be
advised from its own cloud's catalog and prices.
- No candidate or target crosses clouds.
- Groups never mix clouds.
- The single total covers both.

- [ ] Test: a mixed fixture gives the rows of the AWS fixture plus the rows
  of the Azure fixture, and the sum of the two totals.

**R15** WHEN a maintainer regenerates the Azure catalog THEN the generator
SHALL need no credentials, and the monthly data-refresh workflow SHALL run
it with the price tables.

- [ ] `make azure-catalog` (or the generator's `go run`) writes the table
  from the public source. A run with no change leaves the file unchanged.
  _Test: ci — `refresh-data.yaml` opens the PR._

## Counter-examples

- SHALL NOT ask for Azure credentials, read `~/.azure`, or call the Azure
  Resource Manager API, at run time or when generating the catalog.
- SHALL NOT call any service other than the Retail Prices API, which
  aks-pricing already uses.
- SHALL NOT derive memory from the size name, because that is wrong for
  Standard_D64ps_v5 (208 GiB, not 256).
- SHALL NOT recommend:
  - Spot for a system pool, a Spot node or a Windows node without a Windows
    Spot price;
  - Arm for a Windows node;
  - a constrained-vCPU size;
  - a target with less memory than the node's size.
- SHALL NOT claim a zone-level Spot price on Azure.
- SHALL NOT change any row or the total of an EKS-only cluster.

## Out of scope

- Azure Reservations and Savings Plans.
- Swaps between vendors of the same architecture, such as Intel `Dsv5` to
  AMD `Dasv5`, which is often cheaper (a deferred idea).
- Generation or Arm targets for:
  - the GPU (`N*`), HPC (`H*`), confidential (`DC*`, `EC*`), storage (`L*`)
    and M-series sizes;
  - `Fsv2`, because the only newer F series is AMD.
- Reading the cluster autoscaler or agent pool settings, which needs the ARM
  API.
- Disk, network and control-plane tier costs.
- GKE (stage 2).

## Open questions

- [TO CONFIRM] Catalog source: the public Azure docs tables (CC BY 4.0,
  attributed in NOTICE), or `az vm list-skus` run by a maintainer with an
  Azure login (design D1).
- [TO CONFIRM] Generation rows at the same price as `info` rather than no
  row (design D5).
- [TO CONFIRM] The kind label `arm` on Azure rows, keeping `graviton` on AWS
  rows (design D6).
