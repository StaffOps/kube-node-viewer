# Requirements: recommendations

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-02 (spec-refuter; 3 BLOCKING, 4 HIGH, 5 MEDIUM, 5 LOW, all applied)

## Context

kube-node-viewer shows what nodes cost and how full they are. The owner wants
it to say what to do about it: a second screen, **Recommendations**, next to
the current one, which becomes **Overview**, reached with a `:` command line
as in sofka or with `tab`. Recommendations come from fixed rules and
arithmetic on what the tool already reads (requests, allocatable, prices,
labels, node age) and an instance catalog — no AI, no new remote service,
nothing installed in the cluster, no new AWS permission. The same input
always gives the same advice, and every row can show its math.

Moving On-Demand to Spot is always the last recommendation: it trades cost
for interruptions the tool can't judge.

Ships in v0.9.0 with `views-and-export` and the filtered summary.

## Usage examples

One fixture, us-east-1 static prices (`pkg/aws/zz_generated_aws.pricing.go`):
NodePool `default` with 4 × m6i.xlarge On-Demand ($0.192/h), one of them
(ip-10-0-12-15) requesting 1.2 vCPU / 5 GiB; managed node group `system-ng`
with 4 × t3.large at 35% cpu and memory ($0.0832/h); managed node group
`legacy-ng` with 2 × t2.large ($0.0928/h); a Spot price for m6i.xlarge of
$0.0768/h in every zone used.

```text
$ kube-node-viewer
:rec↵
╭ kube-node-viewer · Recommendations ───────────────────────────────────────────╮
│ Nodes:  10 of 10 shown · 220 pods · $… /month (global cluster)                │
│ Savings: $268.64/month — best non-Spot option per node, estimates from        │
│          requests and list prices                                              │
╰───────────────────────────────────────────────────────────────────────────────╯
  KIND         TARGET                              ACTION                                SAVES/MO
  idle         NodePool default (4 nodes)          $X/mo requested by nothing            info
  consolidate  node group system-ng (t3.large ×4)  4 nodes → 2 (pods fit at ≤80%)        $121.47
  rightsize    ip-10-0-12-15 (m6i.xlarge)          → m6i.large (≈62% cpu, 70% mem)       $70.08
  generation   node group legacy-ng (t2.large ×2)  → t3.large (Nitro, current gen)       $14.02
  graviton     NodePool default (m6i.xlarge ×4)    allow m7g.xlarge (arm64 images!)      $84.10
  old node     ip-10-0-11-70 (40 days)             recycle for a fresh AMI               —
  spot         NodePool default (On-Demand ×4)     allow Spot (interruptions!)           $336.38
```

The total is not the sum of the rows: rightsize, generation and Graviton on
the same node are alternatives, so each node counts once, with its best
non-Spot saving (ip-10-0-12-15: rightsize $70.08; the other three default
nodes: Graviton 3 × $21.02; system-ng: two nodes removed $121.47; legacy-ng:
$14.02 → $268.64). Idle and Spot never enter the total.

```text
# enter on the rightsize row
  ip-10-0-12-15: m6i.xlarge → m6i.large
  requests 1.2 vCPU / 5 GiB, DaemonSets 0.25 vCPU / 0.4 GiB per node
  m6i.large allocatable ≈ 1.93 vCPU / 7.1 GiB (ratio of this cluster's m6i nodes)
  after the move: cpu 75%, memory 76% ≤ 80% headroom → fits
  saving: ($0.192 − $0.096) × 730 h = $70.08/month
```

```text
:foo↵        → footer: unknown command "foo" (try :overview, :recommendations)
```

## Requirements

**R1** WHEN the user types `:` in the main view THEN the system SHALL open a
command line in the footer that takes `overview` (`ov`) and
`recommendations` (`rec`), completes the word with `tab`, runs it with
`enter`, cancels with `esc`, and reports an unknown command in the footer.

- [ ] Keys typed in the command line are text (`q`, `f` don't act); `ctrl+c`
  quits and `ctrl+e` toggles the header, as everywhere.
- [ ] The command line and the search line are never open together.

**R2** WHEN the user presses `tab` in the main view THEN the system SHALL
switch between Overview and Recommendations; the header title names the
screen; filters and search apply to both.

- [ ] The screen is saved with the cluster's view, but not in named views.

**R3** WHEN Recommendations is shown THEN the system SHALL list, for the
nodes the filters and search keep, the recommendations of R5–R11 ordered by
kind (idle, consolidate, rightsize, generation, graviton, old node, spot) and
within a kind by monthly saving, each with kind, target, action, monthly
saving or `info`/`—`, and a reason; and the header SHALL show the total
saving defined in R14.

- [ ] Spot recommendations are always last, whatever their saving.
- [ ] Keys: `↑`/`↓` select, `enter` opens the row's math, `←`/`→` page,
  `f`, `/`, `c`, `v`, `e`, `?`, `:`, `tab`, `q` as in Overview; `g`, `s`,
  `o`, `n`, `d`, `r` and `space` do nothing there.
- [ ] A saving that can't be priced shows `—`, never `$0`.
- [ ] With nothing to recommend the screen says so.
- [ ] Analysis runs off the UI goroutine and is reused until the snapshot or
  the view changes.

**R4** WHEN Overview shows nodes THEN it SHALL offer an `IDLE/H` column
(price × (1 − the larger of cpu and memory usage)) and an idle-cost sort, and
group headings SHALL show their idle cost.

- [ ] Reserved nodes (equivalent price), nodes without a price and Fargate
  nodes have no idle cost.

**R5** (idle) WHEN a node group or NodePool has idle cost THEN
Recommendations SHALL list it as `info`: no action, not in the total.

**R6** (consolidate) WHEN the pods of a managed node group of one instance
type fit on fewer nodes of that type with every node at or below the
headroom THEN the system SHALL recommend the smaller count and count the
removed nodes' price as the saving.

- [ ] First-fit decreasing over pods, ordered by the larger of their cpu and
  memory requests as a share of the node's allocatable; a pod fits when cpu,
  memory and pod count all stay within the headroom (pods within allocatable
  pods).
- [ ] DaemonSet and static (Node-owned) pods are not moved; each kept node
  carries the group's largest per-node sum of their requests.
- [ ] No consolidation while any pod in the cluster is pending.
- [ ] For a Karpenter NodePool the row is `info` ("Karpenter consolidation
  should reach N nodes; check the NodePool's consolidationPolicy") and is not
  in the total.
- [ ] Test: 4 nodes at 35% fit in 2 at ≤80% → remove 2; 4 at 50% → 3; 4 at
  70% → none.

**R7** (rightsize) WHEN a node's requests plus its DaemonSet overhead fit a
smaller size of the same family at or below the headroom, by cpu, memory and
pods THEN the system SHALL recommend that size and the price difference.

- [ ] A candidate's allocatable is that of a node of that type in the
  cluster when there is one; otherwise its catalog capacity times the
  allocatable/capacity ratio of the cluster's nodes of the same family, or
  0.90 cpu / 0.85 memory when there are none, said in the math. Its pod
  capacity is that of a node of that type, else not checked and said so.
- [ ] No rightsize while any pod in the cluster is pending.

**R8** (generation) WHEN a node's family is previous generation in the
catalog, or one of the Xen families (`a1, c1, c3, c4, d2, g2, g3, i2, i3,
m1, m2, m3, m4, p2, p3, r3, r4, t1, t2, x1, x1e`) THEN the system SHALL
recommend the newest current-generation family of the same class and
architecture from a fixed table (`t2→t3`, `m4/m5→m7i`, `c4/c5→c7i`,
`r4/r5→r7i`, …) in the same size, with the price difference.

- [ ] No row when that size doesn't exist in the target family, or the
  target is not cheaper.
- [ ] Test: legacy-ng t2.large ×2 → t3.large, $14.02.

**R9** (graviton) WHEN an x86 node's family has an arm64 counterpart of the
same class in the fixed table (`m6i/m7i→m7g`, `c6i/c7i→c7g`, `r6i/r7i→r7g`,
`t3→t4g`, …) that is cheaper in the same size THEN the system SHALL recommend
it, with the warning that images must run on arm64.

**R10** (old node) WHEN a node is older than the age limit (default 30 days)
THEN the system SHALL recommend recycling it, with no saving.

**R11** (spot) WHEN a group of On-Demand nodes has a Spot price for its type
in each node's own zone THEN the system SHALL recommend Spot last, with the
saving and the warning that workloads must tolerate interruptions; a node
whose zone has no Spot price makes the saving `—`.

- [ ] Test: a type with Spot data in another zone only gives `—`, not the
  full on-demand price.
- [ ] For a managed node group the action is "add a Spot node group" (its
  capacity type can't change in place); for a NodePool, "allow spot".

**R12** WHEN `recommend-headroom` (percent, default 80) or
`recommend-node-age` (`30d`, `12h` or any Go duration; default 30d) are set
in the config file THEN the system SHALL use them; there are no flags for
them.

**R13** WHEN a node belongs to a Karpenter NodePool or an EKS managed node
group THEN its generation, Graviton and Spot advice SHALL target that pool or
group; rightsize and old-node advice stays per node.

**R14** WHEN the total saving is shown THEN it SHALL count each node once,
with the best of its non-Spot savings (removed by a managed-node-group
consolidation, rightsize, generation, Graviton share), leaving out idle,
Karpenter consolidation and Spot.

- [ ] Test: the usage-example fixture totals $268.64.

**R15** WHEN a node can't be compared THEN it SHALL get no catalog-based
advice (rightsize, generation, Graviton, Spot): Fargate nodes, EKS Auto Mode
nodes (built-in pools, prices with a management fee), nodes without an
instance-type label or with a type missing from the catalog. A node with the
`kube-node-viewer/instance-price` label is compared using that price, said in
the math.

## Counter-examples

- SHALL NOT change anything in the cluster or in AWS; advice only.
- SHALL NOT call an AI model or any service beyond the AWS Price List and
  EC2 spot-price APIs the tool already uses; no new AWS permission.
- SHALL NOT recommend Spot for Reserved, Fargate, Auto Mode or Spot nodes,
  nor put it anywhere but last.
- SHALL NOT present a saving as certain; the header says they are estimates
  from requests and list prices, by requests only (affinity, taints, spread
  constraints, disruption budgets and volumes are not considered).
- SHALL NOT use actual usage (metrics-server).

## Out of scope

- GCP and Azure catalogs and families (stage 2).
- Savings Plans, Reserved Instances, commitments.
- Pod-level rightsizing (requests that are too high).
- Applying a recommendation.
