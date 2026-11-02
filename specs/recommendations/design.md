# Design: recommendations

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Pages | one screen; panels as popups | `pkg/model/layout.go`, `popup.go` |
| Pod data | slimmed at the informer: requests, phase, node; no owner | `pkg/client/transform.go` (`SlimPod`) |
| Node pods | each node keeps its pods with their requests | `pkg/model/node.go` (`pods`) |
| Prices | on-demand per type (Price List API + static table), spot per type and zone (EC2 spot history, all types) | `pkg/aws/pricing.go` |
| Instance specs | none: only the type name label | — |
| Command mode | out of scope in interactive-tui | `specs/interactive-tui/requirements.md` |

## Target state

```
store (nodes, pods with requests + isDaemonSet) ──► recommend.Analyze(nodes, catalog, prices, cfg)
                                                        │  pure functions, no I/O
catalog: embedded table (+ optional DescribeInstanceTypes refresh)
                                                        ▼
                                  []Recommendation{Kind, Target, Action, Saving, Reason, Math}
UI: page = Overview | Recommendations; ':' command line; tab switches
```

- `pkg/recommend/` (new): rules as pure functions over plain inputs, table
  tested; no Kubernetes or AWS imports.
- `pkg/aws/catalog`: `zz_generated_instances.go` generated like the price
  tables (vCPU, memory, arch, hypervisor, current generation), plus an
  optional refresh through `ec2:DescribeInstanceTypes` when allowed.
- `SlimPod` keeps whether the pod is owned by a DaemonSet (a boolean in an
  annotation-free field), so packing can leave those pods in place.
- Analysis runs off the UI goroutine on each snapshot while the page is
  shown, and its result is cached by snapshot generation.

## Components touched

| Path | Change |
|------|--------|
| `pkg/recommend/` (new) | rules, packing, family tables, savings |
| `pkg/aws/` | instance catalog (generated + optional API) |
| `pkg/client/transform.go`, `pkg/model/pod.go` | DaemonSet flag |
| `pkg/model/` | pages, command line, Recommendations page, IDLE/H column, idle sort, idle in group headings |
| `cmd/kube-node-viewer/` | config keys, catalog wiring |
| `README.md`, `CHANGELOG.md`, screenshots | new page and keys |

## Decisions

### D1: Rules, not AI

**Choice**: every recommendation is a fixed rule over requests, allocatable,
prices, labels, age and the catalog.

**Justification, strongest first**:
1. Same input, same advice: testable with fixed cases, explainable row by row.
2. Works offline; nothing leaves the machine.

**Trade-offs accepted**: no judgment about workloads (interruption tolerance,
arm64 images) — those become warnings.

**When this would be wrong**: advice needs workload knowledge the cluster
doesn't expose.

**Alternatives discarded**: an LLM summarising the screen (non-deterministic,
needs a remote service and sends cluster data out).

**Reversal action**: none needed.

### D2: Catalog embedded, API optional

**Choice**: an instance table generated at release time and embedded; when
`ec2:DescribeInstanceTypes` is allowed, it refreshes the table at start.

**Justification, strongest first**:
1. Works without new AWS permissions, like the static price table.
2. The API keeps new types current between releases where allowed.

**Trade-offs accepted**: a type newer than the release and without the API is
skipped (no recommendation involving it).

**When this would be wrong**: catalog drift causes wrong advice — then make
the API mandatory.

**Alternatives discarded**: API only (needs a permission most users lack).

**Reversal action**: drop the API refresh.

### D3: Packing by first-fit decreasing, DaemonSets fixed

**Choice**: per group, pods sorted by their larger normalised request, placed
first-fit into nodes of the group's type at ≤ headroom; DaemonSet pods are
replaced by a fixed overhead per node.

**Justification, strongest first**:
1. Standard, deterministic, within a node of optimal for this size.
2. Removing a node removes its DaemonSet pods; counting them as movable
   would understate the need.

**Trade-offs accepted**: ignores affinity, taints, topology spread, PDBs and
volumes; said in the reason ("by requests only").

**When this would be wrong**: clusters with heavy pod constraints — then
show consolidation as information only.

**Alternatives discarded**: exact bin packing (slow, no practical gain).

**Reversal action**: remove the rule.

### D4: Defaults the owner can change

**Choice**: headroom 80%, old node after 30 days, Graviton included with a
warning, Spot last — set in config keys `recommend-headroom` and
`recommend-node-age`.

**Justification**: the owner's stated preferences (2026-11-02), and Spot last
by explicit request.

**When this would be wrong**: clusters where 80% leaves too little room for
bursts — the key lowers it.

**Alternatives discarded**: per-cluster settings (no need yet).

**Reversal action**: change the defaults.

### D5: One command line, two pages

**Choice**: `:` opens a command line (`overview`, `recommendations`, short
forms, `tab` completion); `tab` switches pages; the page is part of the
cluster's view.

**Justification**: the owner asked for sofka's way; one key flips pages.

**Trade-offs accepted**: interactive-tui listed command mode as out of
scope; this spec brings it in (dated note there).

**When this would be wrong**: more pages arrive and `tab` cycling gets slow
— the command line already covers that.

**Alternatives discarded**: a page per number key (`1`, `2`) — not
discoverable.

**Reversal action**: drop the command line, keep `tab`.

## Invariants

- Read-only: no change to the cluster or AWS.
- Requests vs allocatable only.
- `Update` never waits on analysis or I/O.

## Test plan

- `pkg/recommend`: table tests per rule (fits / doesn't fit at the headroom,
  DaemonSet overhead, generation table, graviton only when cheaper, spot never
  for Reserved/Fargate/Spot, spot last in ordering, unknown prices → `—`).
- Catalog: generated table loads; API refresh merges.
- UI: key-driven `:` command line (completion, unknown command, typed `q`),
  `tab`, page saved per cluster, row selection and the math popup; IDLE/H
  column and idle sort.
- Benchmark: analysis on 500 nodes / 15,000 pods under 50 ms (unmeasured).
