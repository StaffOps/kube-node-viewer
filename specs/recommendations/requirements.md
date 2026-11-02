# Requirements: recommendations

> Status: draft · Owner: Carlos Gomes · Reviewed: — (spec-refuter, YYYY-MM-DD)

## Context

kube-node-viewer shows what nodes cost and how full they are. The owner wants
it to say what to do about it: a second page, **Recommendations**, next to
the current one, which becomes **Overview**, reached with a `:` command line
as in sofka. Recommendations are worked out with fixed rules and arithmetic
on what the tool already reads (requests, allocatable, prices, labels, node
age) plus an instance catalog — no AI, no remote service, nothing installed
in the cluster. The same input always gives the same advice, and every row
can show its math.

Moving On-Demand to Spot is always the last recommendation: it trades cost
for interruptions the tool can't judge.

Ships in v0.9.0 with `views-and-export` and the filtered summary.

## Usage examples

```text
$ kube-node-viewer
:rec↵
╭ kube-node-viewer · Recommendations ───────────────────────────────────────────╮
│ Nodes:  10 of 10 shown · 220 pods · $728.102/month (global cluster)           │
│ Savings found: $213.40/month on the nodes shown                               │
╰───────────────────────────────────────────────────────────────────────────────╯
  #  KIND          TARGET                         ACTION                                  SAVES/MO
  1  idle          NodePool default (4 nodes)     $96.30/mo requested by nothing          —
  2  consolidate   default · m6i.xlarge           4 nodes → 3 (pods fit at ≤80%)          $140.16
  3  rightsize     ip-10-0-12-15 (m6i.xlarge)     → m6i.large (fits at 76% cpu, 71% mem)  $70.08
  4  generation    system-ng (t3.large ×2)        → t3a.large / m7i-flex.large            $12.41
  5  graviton      default · m6i.xlarge           → m7g.xlarge (needs arm64 images)       $27.74
  6  old node      ip-10-0-11-70 (40 days)        recycle for a fresh AMI                 —
  7  spot          default · OnDemand ×3          → Spot (interruptions!)                 $301.12
  enter: why • tab/:ov: Overview
```

```text
# enter on a row shows the math
  m6i.xlarge → m6i.large on ip-10-0-12-15
  requests 2.45 vCPU / 9.9 GiB; m6i.large allocatable 1.93 vCPU / 7.1 GiB…
  does not fit: memory 139% > 80% headroom — not recommended
```

```text
:foo↵        → footer: unknown command "foo" (try :overview, :recommendations)
```

## Requirements

**R1** WHEN the user types `:` in the main view THEN the system SHALL open a
command line in the footer that takes `overview` (`ov`) and
`recommendations` (`rec`), completes with `tab`, runs with `enter`, cancels
with `esc`, and reports an unknown command in the footer.

- [ ] Keys typed in the command line are text (`q`, `f` don't act).
- [ ] `ctrl+c` quits and `ctrl+e` toggles the header, as everywhere.

**R2** WHEN the user presses `tab` in the main view THEN the system SHALL
switch between Overview and Recommendations; the header title names the
page; filters, search and the cluster are shared by both pages.

- [ ] The page is saved with the cluster's view.

**R3** WHEN Recommendations is shown THEN the system SHALL list, for the
nodes the filters and search keep, the recommendations of R5–R11 ordered by
kind (idle, consolidate, rightsize, generation, graviton, old node, spot) and
within a kind by monthly saving, each with kind, target, action, monthly
saving and a one-line reason, with the total saving at the top.

- [ ] Spot recommendations are always last, whatever their saving.
- [ ] `↑`/`↓` select a row and `enter` shows its math in a popup.
- [ ] A recommendation whose saving can't be priced shows `—`, never `$0`.
- [ ] With nothing to recommend the page says so.

**R4** WHEN a node or group has requests below its allocatable THEN Overview
SHALL offer an `IDLE/H` column (price × (1 − the larger of cpu and memory
usage)) and a sort on idle cost, and group headings SHALL show their idle
cost.

- [ ] Reserved nodes (equivalent price) and nodes without a price have no
  idle cost.

**R5** (idle) WHEN a group (node group or NodePool) has idle cost THEN
Recommendations SHALL list it with its monthly idle cost, as information
(no action, no saving counted in the total).

**R6** (consolidate) WHEN the pods of a group (same node group or NodePool
and instance type) fit on fewer nodes of that type with every node at or
below the headroom THEN the system SHALL recommend removing the extra nodes
and count their price as the saving.

- [ ] Packing is first-fit decreasing on each pod's cpu and memory requests,
  per pod; DaemonSet pods are not moved — each kept node carries the group's
  largest DaemonSet requests as fixed overhead.
- [ ] Test: 4 nodes at 50% fit in 3 at ≤80% → remove 1; at 70% they don't.

**R7** (rightsize) WHEN a node's requests plus its DaemonSet overhead fit a
smaller instance type of the same family and architecture at or below the
headroom THEN the system SHALL recommend that type and its price difference.

**R8** (generation) WHEN a node's family is a previous generation or not
Nitro (catalog) THEN the system SHALL recommend the current-generation
family of the same class and size from a fixed table, with the price
difference; an equal or higher price is not recommended.

**R9** (graviton) WHEN an x86 node's family has an arm64 equivalent of the
same class and size that is cheaper THEN the system SHALL recommend it, with
the warning that images must run on arm64.

**R10** (old node) WHEN a node is older than the age limit (default 30 days)
THEN the system SHALL recommend recycling it, with no saving.

**R11** (spot) WHEN a group of On-Demand nodes (not Reserved, not Fargate)
has a known Spot price for its type in its zones THEN the system SHALL
recommend Spot last, with the saving and the warning that workloads must
tolerate interruptions.

**R12** WHEN the headroom or the age limit are set in the config file
(`recommend-headroom=80`, `recommend-node-age=30d`) THEN the system SHALL use
them.

**R13** WHEN a node is managed by a Karpenter NodePool THEN its
recommendations SHALL target the NodePool (e.g. "allow m7g in NodePool
default"), and for an EKS managed node group the node group, not the single
node; rightsize and old-node advice stays per node.

## Counter-examples

- SHALL NOT change anything in the cluster or in AWS; advice only.
- SHALL NOT call an AI model or any remote service but the AWS pricing and
  EC2 APIs the tool already uses (plus `ec2:DescribeInstanceTypes`, optional).
- SHALL NOT recommend Spot for Reserved, Fargate or already-Spot nodes, nor
  put Spot anywhere but last.
- SHALL NOT present a saving as certain: every saving is an estimate from
  requests and list prices, said once on the page.
- SHALL NOT use actual usage (metrics-server); requests only, as everywhere.

## Out of scope

- GCP and Azure catalogs and families (stage 2).
- Savings Plans, Reserved Instances, commitments.
- Pod-level rightsizing (requests that are too high).
- Applying a recommendation (editing NodePools or node groups).

## Open questions

- None: the owner's open choices (catalog source, headroom, Graviton, age)
  are taken as the defaults in the design and can be changed there.
