# Requirements: node-pods

> Status: shipped (v0.13.0, 2026-11-03) · Owner: sule · Reviewed: 2026-11-02 (spec-refuter, two rounds; findings fixed); R6, R8–R10 changed 2026-11-03 (owner), spec-check findings applied 2026-11-03

## Context

The node list shows how full each node is, but not which pods fill it. To
answer "what is eating this node" or "how much do our DaemonSets cost on
every node", the owner has to leave the tool for `kubectl describe node` and
do the math by hand.

This feature adds a Pods tab to the node details popup (`enter` on a node):
each pod bound to the node with its CPU and memory requests, the share of the
node's allocatable they take, and what that share costs. DaemonSet pods are
listed with the others, marked `ds`; `d` switches to a view with one row per
DaemonSet and their total. On the main screen, the only change is one footer
line with the share of the selected node the DaemonSets request (R10).

> **Corrected 2026-11-03.** This paragraph said DaemonSet pods were summed in
> one row and that the main screen did not change; both changed with the
> owner's change of 2026-11-03 (R6, R9, R10).

## Usage examples

`enter` on a node, then `tab`:

```text
 ip-10-0-1-23
 Details  [Pods]   view: all pods                           sort: cost ↓
 m6i.2xlarge $0.3840/h · 23 pods · cpu 6.10/7.91 77% · mem 22.4/30.1Gi 74%
   POD                            CPU        MEMORY      SHARE   COST/H
   search/indexer-0              1.50 19%   8.0Gi 27%     27%  $0.1020
   payments/api-7f9c-x2kq        2.00 25%   4.0Gi 13%     25%  $0.0971
   kube-system/aws-node-x7k2 ds  0.05  1%   128Mi  0%      1%  $0.0024
   batch/report-28911            no requests
 ↑/↓ • d: daemonsets • s/S: sort • tab: details • esc: close
```

The tab line, the node line and the column header stay in place; only the
pod rows scroll. The column widths are in the design.

_Note 2026-11-02 (review):_ the examples above show the content, not the
exact spacing. The implemented layout uses the column widths of the design
(CPU 5, CPU% 4, MEMORY 6, MEM% 4, SHARE 5, COST/H 8, two spaces apart); the
README's Node details section shows it as it renders.

`d` switches to the DaemonSets view:

```text
 Details  [Pods]   view: daemonsets                         sort: cost ↓
 m6i.2xlarge $0.3840/h · 23 pods · cpu 6.10/7.91 77% · mem 22.4/30.1Gi 74%
   DAEMONSET                      CPU        MEMORY      SHARE   COST/H
   Total (6 pods, 5 sets)        0.62  8%   1.1Gi  4%      8%  $0.0301
   kube-system/kube-proxy        0.10  1%   128Mi  0%      1%  $0.0049
   kube-system/aws-node          0.05  1%   128Mi  0%      1%  $0.0024
 ↑/↓ • d: all pods • s/S: sort • tab: details • esc: close
```

> **Corrected 2026-11-03.** The tab line of the all-pods example now names its
> view (R9); the DaemonSets example puts the column header, titled
> `DAEMONSET`, above the Total line, both fixed, and shows its key line.

On the node list, with the cursor on that node:

```text
 ip-10-0-1-23 · DaemonSets: 8% of the node (cpu 620m 8% · mem 1.1Gi 4%)
```

The share of a pod is the larger of its CPU and memory shares. On a node that
costs $1000/month, a pod with 10% of the CPU and 25% of the memory takes 25%
of the node and costs $250/month.

## Requirements

**R1** WHEN the node details popup is open THEN the system SHALL switch
between a Details tab (the current content) and a Pods tab with `tab`, and
SHALL show which tab is active on a fixed line under the popup title.

- [ ] `tab` goes Details → Pods → Details; the active tab is marked `[Pods]` / `[Details]`.
- [ ] The popup still opens on Details; the Details content and keys are unchanged, apart from `tab`.
- [ ] The popup title stays the node name, as today.
- [ ] The key line names the main keys of the active tab and fits in 74 columns (an 80-column window); `?` lists them all.

> **Corrected 2026-11-03.** R2 is scoped to the all-pods view (R6); the
> DaemonSets view is R9. The fixed node line is the same in both views and
> always counts every pod of the node.

**R2** WHEN the Pods tab is shown in the all-pods view THEN the system SHALL
list every pod the node's totals count, one row each, with namespace/name, CPU request and its
percent of the node's allocatable CPU, memory request and its percent of the
node's allocatable memory, share, and hourly cost.

- [ ] The pods listed are the pods bound to the node in the store: the sum of their requested CPU and memory quantities equals the node's requested totals (checked on quantities in a test, not on the rounded numbers on screen).
- [ ] The tab follows the live node: pods bound or removed while it is open appear or disappear.
- [ ] A fixed line above the rows shows the instance type, the node price, the pod count and the requested / allocatable CPU and memory with percentages; a fixed column header follows it.
- [ ] On a narrow window the name column is cut with `…` first (down to 12 characters); below that the percent columns are dropped; COST/H and SHARE stay.
- [ ] _Note 2026-11-02 (review, option a):_ below the width that holds a 12-character name and every column but the percents (52 columns), CPU and MEMORY are dropped too, so POD, SHARE and COST/H stay down to a 37-column window.

**R3** WHEN a share is computed THEN the system SHALL use the larger of the
CPU percent and the memory percent of allocatable, and the cost SHALL be the
share times the node's hourly price.

- [ ] A pod with 10% CPU and 25% memory has share 25%; on a $0.40/h node its cost is $0.10/h.
- [ ] A percent whose allocatable is zero or missing shows `-` and is left out of the share; when both are, share and cost show `-`.
- [ ] On a Fargate node the share is taken against the node's allocatable, like any node.

**R4** WHEN the node's price is unknown or only an on-demand equivalent, or
pricing is disabled THEN the system SHALL show `-` for unknown, prefix
equivalent costs with `~` as the PRICE/H column does, and with pricing
disabled SHALL leave out the COST/H column and the cost sort.

- [ ] Unknown price: COST/H shows `-`, the share is still shown.
- [ ] Reserved node: costs show `~$…`. A price from the override label counts as real.
- [ ] `--disable-pricing`: no COST/H column, no price on the node line, `s` skips cost and the default sort is CPU.

**R5** WHEN a pod requests neither `cpu` nor `memory` THEN the system SHALL
show `no requests` in place of its numbers and SHALL sort it after every pod
with requests, for every sort field and direction, name included.

- [ ] A pod requesting only memory shows its memory columns and `-` for CPU, and is not marked `no requests`.
- [ ] A pod requesting only a GPU (or another extended resource) shows `no requests`.
- [ ] _Corrected 2026-11-03:_ a DaemonSet row or the Total line (R9) whose summed requests have neither CPU nor memory shows `no requests` the same way; such a DaemonSet row sorts last.

> **Corrected 2026-11-03 (owner).** The closed `DaemonSets` row that opened
> into one row per DaemonSet is replaced: the Pods tab lists every pod,
> DaemonSet pods included, and `d` switches to a DaemonSets view (R9). The
> node list shows the DaemonSets' share of the selected node (R10).

**R6** WHEN the Pods tab is in its default view THEN the system SHALL list
every pod the node's totals count as its own row, DaemonSet pods included,
and SHALL mark each DaemonSet pod with `ds` after its name.

- [ ] DaemonSet pods are sorted among the other pods by the same sort.
- [ ] A pod whose controller is a DaemonSet with no name recorded is listed and marked like the others.
- [ ] There is no row that groups or folds pods in this view.

**R7** WHEN `s` is pressed in the Pods tab THEN the system SHALL cycle the
sort between cost, CPU, memory and name, and `S` SHALL flip its direction;
the current sort SHALL be shown in the tab.

- [ ] Default: cost, biggest first. Cost, CPU and memory start biggest first; name starts A to Z.
- [ ] Without a known price, sorting by cost orders by share.
- [ ] _Corrected 2026-11-03:_ the sort applies to the pod rows of the all-pods view and to the DaemonSet rows of the DaemonSets view; the Total line does not move.
- [ ] Ties break on namespace/name.
- [ ] _Note 2026-11-02 (review):_ a row without the sorted value (no share for cost, no CPU request for CPU, no memory request for memory) sorts after the rows that have it, in both directions.

**R8** WHEN the Pods tab is open THEN the keys SHALL be: `↑`/`↓` (`k`/`j`)
move the cursor over the rows, `pgup`/`pgdown`/`home` move by ten rows or to
the top, `d` switches the view (R9), `s` and `S` sort, `tab` goes to
Details, `esc` closes the popup; the list SHALL scroll to keep the cursor
visible.

- [ ] `enter` and `space` do nothing in the Pods tab and do not close the popup (in Details, `enter` still closes and `space` still pages down).
- [ ] The cursor never lands on the tab line, the node line or the column header.
- [ ] When the cursor's row goes away, the cursor moves to the row now at the same position, or the last row.
- [ ] With no pods, the tab shows `no pods` under the column header and the cursor keys do nothing.
- [ ] When the node leaves the cluster while the tab is open, the popup shows the same message as Details does today.
- [ ] The view, sort, cursor and tab reset when the popup opens again.
- [ ] _Corrected 2026-11-03:_ `d` puts the cursor on the first row of the view it switches to; `pgup`/`pgdown`/`home` work the same in both views. `d` does nothing in the Details tab.

**R9** WHEN `d` is pressed in the Pods tab THEN the system SHALL switch
between the all-pods view and a DaemonSets view, and SHALL name the active
view on the tab line (`view: all pods` / `view: daemonsets`).

- [ ] The DaemonSets view shows a fixed `Total (<n> pods, <m> sets)` line with the summed requests, percents, share and cost of every DaemonSet pod on the node, then one row per DaemonSet (namespace/name) with the same columns.
- [ ] The Total line is computed from the summed requests, not by adding the rows' shares.
- [ ] A DaemonSet row sums all its pods on the node (normally one); `<m>` is the number of distinct namespace/name pairs.
- [ ] DaemonSet pods with no name recorded count in Total and are listed under one `<unnamed>` row, which `<m>` does not count.
- [ ] With no DaemonSet pods, the view shows `no DaemonSets` and the cursor keys do nothing.
- [ ] _Corrected 2026-11-03:_ `s` and `S` sort the DaemonSet rows with the same rules as pods. The cursor key is the DaemonSet namespace/name; when that row goes away, the R8 fallback applies.
- [ ] _Corrected 2026-11-03:_ the column header of this view is titled `DAEMONSET` and comes before the Total line; both are fixed and the cursor never lands on them.

**R10** WHEN the cursor is on a node in the node list THEN the system SHALL
show above the key hints one line with the node name and the share of the
node the DaemonSet pods request:
`<node> · DaemonSets: <share>% of the node (cpu <c>m <c>% · mem <m> <m>%)`.

> **Corrected 2026-11-03 (owner).** The line also shows what the DaemonSets
> request in absolute terms: CPU in millicores, memory as on the Pods tab
> (`cpu 620m 8% · mem 1.1Gi 4%`). A resource the DaemonSets do not request
> shows `0m` / `0`.

- [ ] The share follows R3 on the summed DaemonSet requests.
- [ ] A node with no DaemonSet pods shows `<node> · no DaemonSets`.
- [ ] No cost is shown on this line.
- [ ] The line is absent when the cursor is on a group heading, on the Recommendations screen, or while a panel, the search or the command line is open.
- [ ] On a narrow window the line is cut with `…`.
- [ ] _Corrected 2026-11-03:_ on Overview with nodes on screen, one footer line is always reserved for this line (blank when the cursor is not on a node), so the node list gets the window height minus the header, the summary and three footer lines, still at least 3. Pages do not change as the cursor moves between headings and nodes.
- [ ] _Corrected 2026-11-03 (review):_ the frame is never taller than the window. When the reserved line is shown and the window is so short that three footer lines would leave fewer than 3 list lines, the list drops to 2 lines (column title and one row) instead of the line being dropped or the frame overflowing. _Superseded 2026-11-03 (refuter):_ two lines cannot hold a group heading and the selected node, so the cursor's node went off screen at heights 7 to 9. Instead, WHEN three footer lines would leave fewer than 3 list lines THEN the system SHALL drop the DaemonSets line on that window and keep the list at least 3 lines.

> **Added 2026-11-03 (owner).** The DaemonSets' share must be visible as soon
> as the details open, without `tab` and `d`.

**R11** WHEN the node details popup opens on the Details tab THEN the system
SHALL show, after Resources, a DaemonSets section with the number of
DaemonSet pods and sets and their summed CPU and memory requests, each absolute and as a percent of allocatable, and SHALL
show CPU in millicores everywhere in the Details tab.

- [ ] `2 pods, 2 sets`, then `cpu  400m (10%)` and `memory  4.0Gi (25%)`.
- [ ] A node with no DaemonSet pods shows `none on this node`.
- [ ] A resource with no allocatable shows `-` for its percent.
- [ ] _Owner 2026-11-03:_ the section shows no share of the node; the per-resource percents are enough.
- [ ] Resources shows CPU as `7516m (95%) / 7910m / 8000m`, never `8`.
- [ ] _Owner 2026-11-03:_ a resource whose allocatable equals its capacity shows requested / allocatable only.
- [ ] _Owner 2026-11-03:_ the Instance line shows the monthly price after the hourly one (`$0.1287/h · $93.95/mo`).
- [ ] _Owner 2026-11-03:_ pods are not a row of Resources; a Pods section before DaemonSets shows `23 (40%) / 58` (on the node / max).

## Counter-examples

- The system SHALL NOT add columns, node rows or keys to the main node list; R10 adds only the status line.
- The system SHALL NOT read actual usage (metrics-server): every number is a request.
- The system SHALL NOT make any new call to the cluster or a new watch.
- The system SHALL NOT count pods the node's totals leave out.

## Out of scope

- A cluster-wide DaemonSet cost (for example in Recommendations); the popup still opens only from Overview.
- Pods with a deletion timestamp: the store does not keep them, so the tab does not list them (unlike `kubectl describe node`).
- Grouping pods by other owners (Deployment, StatefulSet, Job).
- GPUs and other extended resources in the share.
- Search or filters inside the Pods tab; exporting the pod list.
- Saving the Pods tab sort in the state file.

## Open questions

- None.
