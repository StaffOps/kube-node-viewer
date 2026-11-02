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
ip-10-0-2-7  m5.xlarge  OnDemand  workers (in the managed node group colour)  us-east-1b  ...
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
  `Unknown`, from `eks.amazonaws.com/compute-type=fargate`, then
  `karpenter.sh/capacity-type` (`on-demand`, `spot`, `reserved`), then
  `eks.amazonaws.com/capacityType` (`ON_DEMAND`, `SPOT`, `CAPACITY_BLOCK` →
  `Reserved`). EKS Auto Mode nodes show their capacity type with an `/Auto`
  suffix (`Spot/Auto`).

  > **Corrected 2026-10-31.** `Auto` was a capacity value; Auto Mode nodes
  > also carry `karpenter.sh/capacity-type`, so making `Auto` a value would
  > hide whether they are spot or on-demand, which the cost filter needs.
- [ ] Node group is the `karpenter.sh/nodepool` value, else the
  `eks.amazonaws.com/nodegroup` value shown in a distinct colour, else `-`.

  > **Corrected 2026-11-02.** The `ManagedNodeGroup:` prefix made the column
  > too wide for generated names (`karpenter-20261031…`); the owner asked for
  > the plain name in another colour. Help (`?`) explains the colour. A
  > NodePool and a managed node group with the same name share one filter and
  > group value.
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

> **Corrected 2026-11-02 (T11).** The pricing region comes from the EKS API
> server URL (`https://<id>.gr7.<region>.eks.amazonaws.com`), known before any
> node is listed, and falls back to the AWS SDK's configured region for other
> servers. Command-line view flags apply to the cluster the run starts on; a
> switched-to cluster opens with its saved state, then the config file.

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

> **Corrected 2026-11-02 (T10).** `ctrl+c` quits from panels too: it is the
> terminal's interrupt key and no panel takes text that needs it. `q` is still
> typed into panels.

**R13** WHEN the user presses `n` THEN the system SHALL show or hide the node
list; the group summary shows whenever a group-by is set. Both are never
hidden at once.

**R14** WHEN the screen is smaller than the content THEN the system SHALL
paginate by lines, repeat the group path on a page that continues a group
(`default › us-east-1a (cont.)`), and drop lowest-priority columns first,
showing how many are hidden.

> **Added 2026-11-02 (T10b).** A node's usage bar is at least 36 cells (the
> owner asked for 50% more than the first 24) and grows into width the columns
> leave free, up to 60. Columns are dropped before the bar shrinks.

> **Corrected 2026-11-02.** Resizing the window (zoom) left old lines below
> the frame, and summary and group lines could be wider than the window, so
> they wrapped and the frame scrolled. A resize now clears the screen and every
> line is cut to the window width. Internal IP is no longer a default column;
> it stays available in the Columns panel (owner's request).

> **Corrected 2026-11-02.** On the owner's request, panels float as a popup
> over the node list (as in sofka), with the selected row highlighted. Filters
> and Group by list every field up front, like Sort and Columns: Filters opens
> a field in place to tick its values, each tick filtering at once; Group by
> ticks up to three fields as levels and reorders them with `K`/`J`. Default
> columns are instance, price, capacity, node group, zone, status.

**R15** WHEN existing flags or config keys are used THEN they SHALL map to
the view state as in the design's flag table (D8), and `--node-selector`
SHALL stay the only server-side scope.

- [ ] One test per row of the flag table.

**R16** WHEN the screen has room THEN the system SHALL show a header panel
above the nodes with the context name, API server URL, pricing region, nodes
shown of total with the pod count, the active view (filters, group-by, sort,
resources) and the main keys; WHEN the screen is too short THEN it SHALL fall
back to one header line with the view only.

- [ ] No network call is made to fill it (no "new version" check, no logo).

> **Added 2026-11-02.** The owner asked for a sofka-like header panel; read-only
> actions (cordon, drain) and the update check are left out.

> **Corrected 2026-11-02 (T10).** The one-line header took over below 30 rows
> and looked unseparated on a normal terminal. The header is now always boxed:
> the full panel from 28 rows, a two-line box (cluster and counts, then the
> view) from 14 rows, and one plain line only below that.

> **Added 2026-11-02.** `ctrl+e` switches between the full panel and the
> compact box, as in sofka, from the main view or a panel; the pick is saved
> with the cluster's view state.

## Counter-examples

- SHALL NOT list or show individual pods; pods only feed per-node totals
  (requests and pod counts).
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
- Pod counts stay on screen (per node, per group, cluster); individual pods
  are never listed.

> **Corrected 2026-10-31.** Pod counts were briefly removed from the screen,
> then the owner asked to keep them.
