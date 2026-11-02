# Design: recommendations

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Screens | one; panels are popups; `u.page` is pagination | `pkg/model/layout.go`, `uimodel.go` |
| Pod data | slimmed: requests, phase, node; no owners | `pkg/client/transform.go:38-45` |
| Terminal pods | removed from the store | `pkg/client/controller.go:196` |
| Pending pods | counted, requests not kept by node | `pkg/model/cluster.go` |
| Prices | on-demand per type (Price List API, static fallback); spot per type and zone, default 0 for unseen zones | `pkg/aws/pricing.go:236,535` |
| Instance specs | none | — |

## Target state

```
store ── nodes (+ pods with requests, owner kind) ──► recommend.Analyze(input) ──► []Rec
catalog: static table (hack/catalog → pkg/aws/zz_generated_instances.go)
         + refreshed from the Price List products the pricing provider already reads
UI: screen = overview | recommendations; ':' command line; tab switches
```

- `pkg/recommend/` (new): pure functions over plain structs (no Kubernetes or
  AWS imports): rules, packing, family tables, total.
- `pkg/aws/`: the catalog (`instanceType → vCPU, memory, arch, family,
  current generation`) and a spot lookup that reports "no price for this
  zone" instead of 0.
- `SlimPod` keeps `OwnerReferences` reduced to kind and controller flag;
  `model.Pod` gets `Owner kind` (DaemonSet, Node, other).
- `UIModel.screen` (not `page`) is saved in the cluster's `ViewState`
  (`screen`), cleared by `forSaving()` for named views.
- Analysis input is built on the UI goroutine from the snapshot (plain
  values); a `tea.Cmd` runs it; the result is cached by a snapshot counter
  (incremented on every `snapshotMsg`) plus a hash of the view.

## Components touched

| Path | Change |
|------|--------|
| `pkg/recommend/` (new) | rules, packing, tables, total |
| `hack/catalog/` (new) | generator for the instance table |
| `pkg/aws/` | `zz_generated_instances.go`, catalog refresh, zone-aware spot lookup |
| `pkg/client/transform.go`, `pkg/model/pod.go` | owner kind |
| `pkg/model/` | screens, command line, Recommendations screen, `IDLE/H`, idle sort and heading |
| `cmd/kube-node-viewer/` | config keys, catalog wiring |
| `README.md`, `CHANGELOG.md`, screenshots, interactive-tui note | docs |

## Decisions

### D1: Rules, not AI

**Choice**: every recommendation is a fixed rule over requests, allocatable,
prices, labels, age and the catalog.

**Justification, strongest first**:
1. Same input, same advice: testable, explainable row by row.
2. Works offline; nothing leaves the machine.

**Trade-offs accepted**: no judgment about workloads — warnings instead.

**When this would be wrong**: advice needs workload knowledge the cluster
doesn't expose.

**Alternatives discarded**: an LLM over the screen (non-deterministic, needs a
remote service, sends cluster data out).

**Reversal action**: none needed.

### D2: Catalog from the Price List, no new permission

**Choice**: the AWS Price List products for EC2 carry `vcpu`, `memory`,
`processorArchitecture`, `currentGeneration` and `instanceFamily`.
`go run ./hack/catalog` (maintainer, with AWS credentials allowed
`pricing:GetProducts`) writes `pkg/aws/zz_generated_instances.go`, about
1,000 types (~60 KB). At run time the pricing provider, which already reads
those products for on-demand prices, refreshes the table from the same
responses. Nitro is not in the Price List; the Xen families are a fixed list
(R8).

**Justification, strongest first**:
1. No new AWS permission; same data path as prices.
2. Facts about AWS products, not third-party data: no license or NOTICE
   change.

**Trade-offs accepted**: a type newer than the release, run without pricing,
is missing → no advice for it (R15).

**When this would be wrong**: the Price List drops those attributes — then
`ec2:DescribeInstanceTypes`.

**Alternatives discarded**: `ec2:DescribeInstanceTypes` (new permission most
users lack); third-party catalogs (license, drift).

**Reversal action**: delete the table; catalog rules show no rows.

### D3: Packing by first-fit decreasing, DaemonSets fixed

**Choice**: per managed node group of one type, pods sorted by their larger
normalised request, placed first-fit into nodes at ≤ headroom on cpu,
memory and pods; DaemonSet and static pods replaced by the group's largest
per-node sum.

**Justification, strongest first**:
1. Standard, deterministic, close to optimal for this size.
2. Removing a node removes its DaemonSet pods; moving them would understate
   the need.

**Trade-offs accepted**: ignores affinity, taints, spread, PDBs, volumes —
said on the screen. Karpenter pools are `info`: Karpenter consolidates
itself and mixes types.

**When this would be wrong**: heavy pod constraints — then `info` everywhere.

**Alternatives discarded**: exact bin packing (slow, no practical gain).

**Reversal action**: remove the rule.

### D4: One total, each node once

**Choice**: the total takes, per node, the best non-Spot saving among the
rows that touch it; idle, Karpenter consolidation and Spot stay out.

**Justification**: rows on the same node are alternatives; summing them
promises money that can't be saved twice.

**When this would be wrong**: the owner wants Spot in a "maximum" figure —
then a second, labelled total.

**Alternatives discarded**: sum of rows (double counting); no total (the
owner asked for "how much").

**Reversal action**: drop the header line.

### D5: Defaults the owner can change

**Choice**: headroom 80%, old after 30 days, Graviton included with a
warning, Spot last; config keys `recommend-headroom`, `recommend-node-age`
(accepts `Nd` and Go durations).

**Justification**: the owner's preferences (2026-11-02); Spot last by
explicit request.

**When this would be wrong**: bursty clusters — lower the headroom.

**Alternatives discarded**: flags (config is enough; flags apply to one run).

**Reversal action**: change the defaults.

### D6: A command line and two screens

**Choice**: `:` opens a command line (`overview`, `recommendations`, short
forms, `tab` completion); `tab` in the main view switches screens; the
screen is kept per cluster.

**Justification**: the owner asked for sofka's way; one key flips screens.

**Trade-offs accepted**: interactive-tui put command mode out of scope; this
spec brings it in (dated note there).

**When this would be wrong**: many screens — the command line covers them.

**Alternatives discarded**: number keys per screen (not discoverable).

**Reversal action**: drop the command line, keep `tab`.

## Invariants

- Read-only towards the cluster and AWS; no new AWS permission.
- Requests vs allocatable only.
- `Update` never waits on analysis or I/O.

## Test plan

- `pkg/recommend`: the usage-example fixture end to end (rows, order, $268.64
  total); per rule fits / doesn't fit at the headroom with DaemonSet overhead
  and pod count; consolidation 35% / 50% / 70%; pending pods block packing;
  generation and Graviton only when the size exists and is cheaper; spot zone
  miss → `—`; Spot always last; R15 exclusions; override-label price.
- Catalog: generated table loads; refresh from Price List products.
- `SlimPod` keeps owner kinds; `model.Pod` reports DaemonSet / Node.
- UI: `:` (completion, unknown command, typed `q`, not with `/`), `tab`,
  screen saved per cluster and not in named views, keys on Recommendations,
  the math popup; `IDLE/H` and idle sort.
- Benchmark: analysis on 500 nodes / 15,000 pods (target unmeasured until
  measured; recorded in tasks).
