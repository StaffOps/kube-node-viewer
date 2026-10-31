# Requirements: interactive-tui

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-10-31 (spec-refuter; 4 HIGH, 8 MEDIUM, all applied)

## Context

Today every view choice is a startup flag (`--node-selector`, `--extra-labels`,
`--node-sort`, `--group-by`, `--resources`). Changing what you look at means
quitting and retyping the command. The only runtime keys are `g`/`n` (show or
hide the group summary and the node list), paging, and `q`/`esc`/`ctrl+c` to
quit.

The goal is a tool you start with `kube-node-viewer` and then drive from inside
the TUI: filter, group and sort nodes, switch cluster and choose which node
facts and resources to see. It should feel as responsive as `sofka`, whose way
of working (event-driven redraw, filters inside the TUI, no flags, remembered
state) is the reference, not its code.

This is stage 1 of three. Stage 2 is multi-cloud pricing (GCP, Azure); stage 3
covers GPU and saved views. The tool looks at nodes only: pods are watched only
to add up their requests per node and are never listed.

## Usage examples

```text
$ kube-node-viewer
# current kube context; last filters/grouping/sort used on this context restored
Filters: capacity=Spot · nodegroup∈{default,gpu} │ Group: nodegroup › zone │ Sort: price ↓ │ cpu+memory

default › us-east-1a   3 nodes  41 pods  cpu 61%  mem 48%  $0.312/h
  ip-10-0-1-5   m6i.large  Spot  default  us-east-1a  10.0.1.5   cpu ███░ 72%  mem ██░░ 51%  $0.104/h  Ready
  ...
```

```text
# press f → Filters panel
┌ Filters ─────────────────────────────┐
│ [x] capacity     Spot                │
│ [x] nodegroup    default, gpu        │
│ [ ] zone         us-east-1a   (off)  │
│ + add filter                         │
└ space on/off · d remove · enter edit · esc close ┘

# "+ add filter" → pick field → pick values (with node counts)
  capacity:  ( ) OnDemand 12   (x) Spot 9   ( ) Reserved 2   ( ) Unknown 0
```

```text
# g → Group by: up to 3 ordered keys     s → Sort: any column or label, ↑/↓
# / → type "10.0.1" → only matching nodes, live; esc clears
# r → resources: cpu → memory → cpu+memory     c → context picker
```

```text
# A cluster with no Karpenter and an EKS managed node group:
ip-10-0-2-7  m5.xlarge  OnDemand  ManagedNodeGroup:workers  us-east-1b  ...
# A reserved node: price shown as an on-demand equivalent, left out of totals
ip-10-0-3-9  p5.48xlarge  Reserved  gpu  us-east-1a  ...  ~$98.32/h
gpu   4 nodes  ...  $12.10/h  (+1 reserved, not priced)
```

## Requirements

**R1** WHEN the user runs `kube-node-viewer` with no flags THEN the system
SHALL connect to the current kubeconfig context and restore the view state
(filters, group-by, sort, resources, columns, node list visibility) last
saved for that cluster; WHEN none is saved THEN it SHALL use the config file
values, then built-in defaults.

- [ ] Test: state saved for cluster A is restored for A and not applied to B.
- [ ] Test: config file values do not override saved state; built-in flag
  defaults never override saved state.

**R2** WHEN the user changes the view state from the TUI THEN the system
SHALL save it for that cluster without blocking the UI; WHEN flags were
passed explicitly THEN their values apply to that run only and are not saved
unless the user then changes the view from the TUI.

- [ ] A corrupt or unreadable state file is ignored with one stderr warning
  before the TUI starts; the tool starts with defaults.

**R3** WHEN the user presses `f` THEN the system SHALL open a Filters panel
where filters can be added, edited, toggled on/off and removed, on these
fields: any node label (key, then values), capacity type, node group, zone,
region, instance type, status.

- [ ] Value pickers list the values present among all watched nodes with the
  number of nodes for each.
- [ ] Filters on different fields combine with AND; several values of one
  field combine with OR.
- [ ] Disabled filters stay in the list and in saved state but do not filter.
- [ ] Status is a set of flags per node — `Ready` or `NotReady`, plus
  `Cordoned`, `Deleting`, `Launching` (nodeclaim with no node yet) — and a
  status filter matches a node that has any selected flag.

**R4** WHEN the user presses `g` THEN the system SHALL let them choose up to
three ordered group-by keys (any field from R3) and show nested groups with
node count, pod count, resource usage and cost subtotals per level.

- [ ] Nodes without a key value go to a `<none>` group that sorts last.

**R5** WHEN the user presses `s` THEN the system SHALL let them sort the node
list by creation time, any column of R7, or any label, ascending or
descending.

**R6** WHEN the user presses `/` and types THEN the system SHALL show only
nodes whose name, internal IP, instance type or any label value contains the
text, updating on each keystroke; `esc` clears it.

**R7** WHEN nodes are shown THEN the system SHALL offer these columns, each
toggleable from a Columns panel (`o`): instance type, capacity type, node
group, zone, region, internal IP, age, status, price, the computed labels
(`node-age`, `node-<resource>-usage`, either prefix) and any extra label.

- [ ] Capacity type is one of `OnDemand`, `Spot`, `Reserved`, `Fargate`,
  `Auto`, `Unknown`, from `karpenter.sh/capacity-type` (`on-demand`, `spot`,
  `reserved`), `eks.amazonaws.com/capacityType` (`ON_DEMAND`, `SPOT`,
  `CAPACITY_BLOCK` → `Reserved`) and `eks.amazonaws.com/compute-type`
  (`fargate`, `auto`).
- [ ] Node group is the `karpenter.sh/nodepool` value, else
  `ManagedNodeGroup:<eks.amazonaws.com/nodegroup>`, else `-`.
- [ ] A nodeclaim-only node shows `-` for IP and the instance ID as its name.

**R8** WHEN a node is `Reserved` THEN its price SHALL show as the on-demand
list price marked as an equivalent (`~$98.32/h`), and cluster and group
totals SHALL leave it out and say how many reserved nodes were not priced.

**R9** WHEN the user presses `r` THEN the system SHALL cycle the shown
resources between cpu, memory and cpu+memory; WHEN `--resources` names other
resources THEN that list SHALL be a fourth entry in the cycle.

**R10** WHEN the user presses `c` THEN the system SHALL list the kubeconfig
contexts and, on selection, switch to that cluster without restarting,
loading its saved state and re-scoping AWS pricing to the new cluster's
region (`topology.kubernetes.io/region` of its nodes).

- [ ] No object from the previous cluster appears after the switch, even if
  its watches deliver late events.

**R11** WHEN cluster objects or prices change THEN the system SHALL redraw
after a batched change notification instead of a 100 ms timer; node ages and
NotReady durations SHALL still advance (1 s tick).

- [ ] Benchmark: at the same node count, snapshot+render time with 150 000
  pods is within 20% of the time with 15 000 pods.
- [ ] Test: with the store's write lock held, a key still opens its panel
  (`Update` never waits on cluster data or the API server).

**R12** WHEN a panel or text input is open THEN keys SHALL go to it
(`f`, `g`, `q` are typed, not commands) and `esc` SHALL close it; in the main
view `q` and `ctrl+c` quit and `?` shows all keys. `esc` no longer quits.

**R13** WHEN the user presses `n` THEN the system SHALL show or hide the node
list; the group summary shows whenever a group-by is set. Both are never
hidden at once.

**R14** WHEN the screen is smaller than the content THEN the system SHALL
paginate by lines, repeat the group path on a page that continues a group
(`default › us-east-1a (cont.)`), and drop lowest-priority columns first,
showing how many are hidden.

**R15** WHEN existing flags or config keys are used THEN they SHALL map to
the view state as in the design's flag table (D8), and `--node-selector`
SHALL stay the only server-side scope.

- [ ] One test per row of the flag table.

## Counter-examples

- SHALL NOT list or show pods; pods only feed per-node totals.
- SHALL NOT restart the node/pod watches when a filter, group or sort changes.
  Only `--node-selector` (set at start) and a context switch restart watches.
- SHALL NOT write to `~/.kube-node-viewer` or `~/.eks-node-viewer`.
- SHALL NOT present a reserved, savings-plan or committed-use price as real
  cost, nor add it to totals.
- SHALL NOT require anything installed in the cluster.
- SHALL NOT drop the `eks-node-viewer/` prefix for `instance-price` or the
  computed labels.

## Out of scope

- GCP and Azure pricing; nodes with an empty `providerID` (dropped today,
  `controller.go:125`) — stage 2.
- GPU columns and allocation (stage 3, roadmap).
- Saved named views (`v`), command mode (`:`), typed filter syntax in `/`.
- Actual (metrics-server) usage; the tool stays on requests vs allocatable.

## Resolved questions

Answered by the owner on 2026-10-31.

- Up to three group-by levels; validate in use and revisit.
- No pod view; pods only feed per-node totals.
- No command mode in stage 1; panels instead.
- All product and repository text in English.
