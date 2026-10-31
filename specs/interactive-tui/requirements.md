# Requirements: interactive-tui

> Status: draft · Owner: Carlos Gomes · Reviewed: — (spec-refuter pending)

## Context

Today every view choice is a startup flag (`--node-selector`, `--extra-labels`,
`--node-sort`, `--group-by`, `--resources`). Changing what you look at means
quitting and retyping the command. The only runtime keys are `g`/`n` (show or
hide the group summary and the node list) and paging.

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
Filters: capacity=spot · nodepool∈{default,gpu} │ Group: nodepool › zone │ Sort: price ↓ │ cpu+memory

default › us-east-1a   3 nodes  41 pods  cpu 61%  mem 48%  $0.312/h
  ip-10-0-1-5   m6i.large  Spot  default  us-east-1a  10.0.1.5   cpu ███░ 72%  mem ██░░ 51%  $0.104/h  Ready
  ...
```

```text
# press f → Filters panel
┌ Filters ─────────────────────────────┐
│ [x] capacity     spot                │
│ [x] nodepool     default, gpu        │
│ [ ] zone         us-east-1a   (off)  │
│ + add filter                         │
└ space on/off · d remove · enter edit ┘

# "+ add filter" → pick field → pick values (with node counts)
  capacity:  ( ) on-demand 12   (x) spot 9   ( ) reserved 2   ( ) unknown 0
```

```text
# press g → Group by: up to 3 ordered keys
  1. nodepool   2. zone   3. —
# press s → Sort: any column or label, ↑/↓
# press / → type "10.0.1" → only matching nodes, live
# press r → resources cycle: cpu → memory → cpu+memory
# press c → context picker → switch cluster without restarting
```

```text
# A cluster with no Karpenter and an EKS managed node group:
ip-10-0-2-7  m5.xlarge  OnDemand  ManagedNodeGroup:workers  us-east-1b  ...
```

## Requirements

**R1** WHEN the user runs `kube-node-viewer` with no flags THEN the system
SHALL connect to the current kubeconfig context and restore the filters,
group-by, sort, resources and visible columns last used on that context, or
defaults when none were saved.

- [ ] Test: state saved for context A is restored for A and not applied to B.
- [ ] Every existing flag still works and overrides the restored state for that run.

**R2** WHEN the user changes filters, group-by, sort, resources or columns
THEN the system SHALL save them per context, without blocking the UI, so the
next start restores them.

- [ ] A corrupt or unreadable state file is ignored with one stderr warning;
  the tool starts with defaults.

**R3** WHEN the user presses `f` THEN the system SHALL open a Filters panel
where filters can be added, edited, toggled on/off and removed, on these
fields: any node label (key, then values), capacity type, node group
(nodepool or managed node group), zone, region, instance type, node status
(Ready/NotReady/Cordoned).

- [ ] Value pickers list the values present in the cluster with the number of
  nodes for each.
- [ ] Filters on different fields combine with AND; several values of one
  field combine with OR.
- [ ] Disabled filters stay in the list and in saved state but do not filter.

**R4** WHEN the user presses `g` THEN the system SHALL let them choose up to
three ordered group-by keys (any field from R3) and show nested groups with
node count, pod count, resource usage and cost subtotals per level.

- [ ] Nodes without a key value go to a `<none>` group that sorts last
  (current behaviour of `--group-by`).

**R5** WHEN the user presses `s` THEN the system SHALL let them sort the node
list by any visible column or any label, ascending or descending.

- [ ] Columns sortable at least: name, instance type, capacity type, node
  group, zone, region, internal IP, price, cpu %, memory %, age, status.

**R6** WHEN the user presses `/` and types THEN the system SHALL show only
nodes whose name, internal IP, instance type or any label value contains the
text, updating on each keystroke; `esc` clears it.

**R7** WHEN nodes are shown THEN the system SHALL offer these columns, each
toggleable from a Columns panel (`o`): instance type, capacity type
(`OnDemand`/`Spot`/`Reserved`/`Fargate`/`Unknown`), node group
(Karpenter nodepool name, `ManagedNodeGroup:<name>` for an EKS managed node
group, `-` otherwise), zone, region, internal IP, age, status, price, and any
extra label.

- [ ] Capacity type reads `karpenter.sh/capacity-type` (`on-demand`, `spot`,
  `reserved`) and `eks.amazonaws.com/capacityType` (`ON_DEMAND`, `SPOT`).
- [ ] A `Reserved` node shows the on-demand list price marked as an
  equivalent (`~$0.096/h`), never as an actual cost.

**R8** WHEN the user presses `r` THEN the system SHALL cycle the shown
resources between cpu, memory and cpu+memory, for node rows and group
subtotals.

**R9** WHEN the user presses `c` THEN the system SHALL show the kubeconfig
contexts and, on selection, switch to that cluster without restarting,
loading that context's saved state.

- [ ] The previous cluster's watches are stopped; no data from it remains on screen.

**R10** WHEN cluster objects change THEN the system SHALL redraw only after a
change (batched), not on a fixed timer, and keypresses SHALL get visual
feedback without waiting for cluster data.

- [ ] Benchmark on a synthetic cluster (sizes in the design test plan): time
  to apply an event and time to render a frame recorded before and after;
  render time does not grow with the pod count. Numbers are `unmeasured`
  until T1 runs.

**R11** WHEN a key panel is open THEN the system SHALL show the keys it
accepts, and `?` SHALL show all keys.

**R12** WHEN the user presses `n` THEN the system SHALL show or hide the node
list; the group summary shows whenever a group-by is set. Both are never
hidden at once (current rule).

## Counter-examples

- SHALL NOT list or show pods; pods only feed per-node request totals.
- SHALL NOT restart the node/pod watches when a filter, group or sort changes;
  those apply to data already in memory. Only `--node-selector` (a hard scope
  set at start) and a context switch restart watches.
- SHALL NOT write state into `~/.kube-node-viewer` (the config file stays
  user-owned and read-only for the tool).
- SHALL NOT present a reserved, savings-plan or committed-use price as real
  cost.
- SHALL NOT require anything installed in the cluster.

## Out of scope

- GCP and Azure pricing (stage 2 spec).
- GPU columns and allocation (stage 3, roadmap).
- Saved named views (`v`), command mode (`:`) and a typed filter syntax in `/`
  — revisit after using stage 1.
- Actual (metrics-server) usage; the tool stays on requests vs allocatable.

## Resolved questions

Answered by the owner on 2026-10-31.

- Up to three group-by levels; validate in use and revisit.
- No pod view; pods only feed per-node totals.
- No command mode in stage 1; panels instead.
