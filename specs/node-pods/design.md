# Design: node-pods

> Implements [requirements](requirements.md).

## Current state

- `enter` on a node opens the details popup (`openNodeDetails`,
  `pkg/model/nodedetails.go:29`). Its keys are `esc`/`enter` to close and
  scrolling (`nodeDetailsKey`, `:40`); rows are rebuilt every frame from the
  live snapshot (`nodeDetailsPanel`, `:68`).
- Each node keeps the pods bound to it in `n.pods`, and `BindPod` adds every
  bound pod's requests to `n.used` (`pkg/model/node.go:292-306`). `Pods()`
  (`:482`) lists exactly those pods, so their requests add up to the node's
  requested totals.
- A pod keeps namespace, name, phase, summed requests (`Requested`,
  `pkg/model/pod.go:130`) and the kind of its controller (`ownerKind`,
  `:46`). The informer transform keeps only the controller's kind
  (`slimOwners`, `pkg/client/transform.go:103`): the DaemonSet name is
  dropped.
- Terminating, Succeeded and Failed pods never enter the store
  (`pkg/client/controller.go:179`, `:196`, `:240-248`); pods waiting for
  their node bind once it exists (`pkg/model/cluster.go:123`, `:206`).
- `computeRequests` always adds `pods=1` (`pkg/model/pod.go:144`), so "no
  requests" must look at `cpu` and `memory` only.
- Node price: `Price`, `HasPrice`, `PriceIsEquivalent`
  (`pkg/model/node.go:490-518`); PRICE/H formats unknown as `-` and
  equivalents with `~` (`pkg/model/layout.go:764-770`). Details hides the
  price when pricing is disabled (`pkg/model/nodedetails.go:87`).
- The popup (`pkg/model/popup.go`) has a one-line title cut at the box width
  (`:135-136`), scrolling rows with a highlighted cursor (`:147-148`) and
  one hint line from `helpText` (`pkg/model/layout.go:183`, node hint at
  `:205-206`). The box is at most `width-6` wide (`pkg/model/popup.go:94`): 74 columns on an 80-column window. The popup
  opens only from Overview (`pkg/model/uimodel.go:625`).

## Target state

```text
informer ─ slimOwners keeps {Kind, Name} ─▶ Pod.ownerKind, Pod.ownerName
                                                  │
details popup ── tab ──▶ Pods tab ── podRows(node, sort, view) ─▶ rows
                                         └── d: all pods ⇄ daemonsets
node list cursor ──▶ DaemonSetLine(node) ─▶ reserved footer line (R10)
```

> **Corrected 2026-11-03.** The folded DaemonSets group is replaced by the
> all-pods view, the `d` DaemonSets view and the R10 footer line (owner
> change of 2026-11-03). The bullets below describe the new design; the
> group, its `▸`/`▾`/`├─` rows and `enter`/`space` on it are gone.

- `slimOwners` keeps the controller's `Name` as well; `Pod` gets
  `ownerName` and an `Owner() (kind, name string)` accessor.
- `pkg/model/nodepods.go` (new) builds the tab from `n.PodsCopy()`,
  `n.Allocatable()` and the price. In the all-pods view, one `podRow` per
  pod, DaemonSet pods marked ` ds` after the name (cut with the name). In
  the DaemonSets view, one `podRow` per DaemonSet (namespace/name, or one
  `<unnamed>` row) plus a Total row kept out of the sorted rows. Each row has
  requests, percents, share, cost and a `noRequests` flag. Sorting is a
  plain `sort.SliceStable` over the rows.
- `panelView` gains `fixed []string`: lines drawn between the title and the
  rows that never scroll and that the cursor cannot reach. The tab line
  (`Details  [Pods]   view: all pods` and, right-aligned, `sort: cost ↓`),
  the node line and the column header go there; in the DaemonSets view the
  Total line follows the header. Details uses the tab line only.
- The details popup gets `detailsTab` (details or pods), `podsCursor` (a row
  key), `podsSort` (field, descending) and `podsView` (all pods or
  daemonsets), all reset when the popup opens. `tab` switches tabs; in the
  Pods tab a new `nodePodsKey` handles the keys of R8. `helpText` returns the
  active tab's key line, written to fit 74 columns.
- Columns: CPU 5, CPU% 4, MEMORY 6, MEM% 4, SHARE 5, COST/H 8 characters,
  separated by 2 spaces, right-aligned; POD takes what is left, cut with `…`, down to
  12 characters. Below that the two percent columns are dropped. COST/H is
  left out with pricing disabled.
- _Note 2026-11-02 (review):_ when even the percent-less columns leave no
  room for a 12-character name (below 52 columns), CPU and MEMORY are
  dropped as well; POD, SHARE and COST/H stay (`podsTable.values`).
- Rows are rebuilt each frame like Details, so the tab follows the live
  node. The cursor is a row key (`p:` + pod namespace/name, or `ds:` +
  DaemonSet namespace/name): when that row is gone it moves to the row now
  at the same position, or the last row. `d` puts it on the first row.
- The R10 line (`DaemonSetLine`) is built in `pkg/model` from the cursor
  node's bound pods, in a footer line reserved on Overview whenever nodes
  are on screen (D5).

## Components touched

| Path | Change |
|------|--------|
| `pkg/client/transform.go` | `slimOwners` keeps the controller name |
| `pkg/model/pod.go` | `ownerName`, `Owner()` |
| `pkg/model/nodepods.go` (new) | rows, share, cost, `ds` marks, DaemonSets view and Total, sort, R10 line |
| `pkg/model/popup.go` | `fixed` lines between title and rows |
| `pkg/model/nodedetails.go` | tab state, `tab` key, tab line, delegate to the Pods tab |
| `pkg/model/layout.go` | `helpText` per tab and view; reserved R10 footer line |
| `pkg/model/panels.go` | help (`?`) entries for the Pods tab |
| `README.md`, `CHANGELOG.md` | Node details section, keys |

## Decisions

### D1: The pod view lives in the node details popup

**Choice**: A second tab of the existing popup, reached with `tab`.

**Justification, strongest first**:
1. The owner asked for no clutter: the main list and header do not change,
   and the pods of one node are what the popup is already about.
2. The popup already follows the live node, scrolls and closes with `esc`;
   the tab reuses all of it.
3. `tab` inside a popup does not collide with the main `tab`
   (Overview ⇄ Recommendations): popups take keys first.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| One node at a time | The question is "what fills this node"; cluster-wide is out of scope |

**When this would be wrong**: when the owner wants to compare pods across
nodes (top pods of the cluster).

**Reversal action**: Move `nodepods.go` rows into a screen of their own fed
by all nodes; the row code stays.

**Alternatives discarded**: Pod rows under each node in the main list — the
clutter the owner ruled out. A third screen next to Recommendations — loses
the node context.

### D2: Share is the larger of the CPU and memory percents

**Choice**: `share = max(cpu req / cpu alloc, mem req / mem alloc)`,
`cost = share × node price`.

**Justification, strongest first**:
1. The owner's rule: "the pod takes 10% of the node, it costs 10% of the
   price". The larger percent is the part of the node the pod blocks from
   other pods: a pod with 25% of the memory leaves room for only three more
   like it, whatever its CPU.
2. One number per pod, sortable, easy to check by hand.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Pod costs on a node can add up to more than the node price | Each cost answers "what does this pod hold"; the tab shows the node price for reference |
| The Total line's cost differs from the sum of its rows | The Total line (R9) is computed from summed requests, which is what the DaemonSets hold together |

**When this would be wrong**: if the costs are used for chargeback, where
they must add up to the bill.

**Reversal action**: Swap the share function (for example weighted CPU and
memory prices); one function and its tests.

**Alternatives discarded**: Half CPU, half memory — adds up but has no
basis. Percent only, no cost — the owner asked for cost.

### D3: Keep the owner name in the informer transform

**Choice**: `slimOwners` keeps `Name` with `Kind` for the controller
reference.

**Justification, strongest first**:
1. Per-DaemonSet rows need the name; the pod object is slimmed before it
   reaches the store, so there is no other place to get it without a new
   watch.
2. One short string per pod; the controller reference is already kept.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Memory per pod grows by the owner name | Measured in T7 (2026-11-02, see below): +0.15 MiB at 15,000 pods, +2.6 MiB (+0.4%) at 150,000; allocations per op unchanged |

**Measurement** (T7, 2026-11-02, `go test ./pkg/model -run x -bench
BenchmarkHeap -benchmem`, `-count 3` in the tests stage and one more run
here, all agreeing within 0.35 MiB). `fullBenchPod` now has a controller
reference (ReplicaSet `app-N-7d9c8b6f5`, one pod in ten owned by DaemonSet
`ds-N`), and the benchmark keeps the slim pods alive next to the model, as
the informer's store does. `kind` emulates the transform before T1 by
blanking the name after `SlimPod`.

| Pods | no owner | kind (before T1) | kind + name (after T1) |
|------|----------|------------------|------------------------|
| 15,000 | 68.1 MiB | 69.5–69.6 MiB | 69.75–69.85 MiB |
| 150,000 | 662.5 MiB | 676.4 MiB | 679.0 MiB |

The increase is not material. The README figure ("150,000 pods take about
150 MiB") counts the model alone, without the slim pods: measured the same
way with the slim pods dropped, it is 154.4 MiB before T1 and 157.0 MiB
after, so the figure still holds and is unchanged. The 679 MiB above also
counts the informer's slim pods, which the README figure never included.

**When this would be wrong**: if the measurement shows a material increase.

**Reversal action**: Keep only the name for DaemonSet owners.

**Alternatives discarded**: Deriving the DaemonSet from the pod name
(`<ds>-<hash>`) — guesswork.

### D4: Static pods are not in the DaemonSets group

**Choice**: Pods owned by a Node (static pods) are listed as normal pods,
with no group.

**Justification, strongest first**:
1. The owner asked about DaemonSets; a static pod has no DaemonSet to name.
2. Few nodes run them under EKS or AKS.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Recommendations count static pods as fixed overhead, the tab does not group them | Different questions; the tab names what the owner can change |

**When this would be wrong**: on clusters where static pods are a large,
fixed share of every node.

**Reversal action**: Add a "Static pods" group next to DaemonSets.

**Alternatives discarded**: One "fixed overhead" group for both — mixes
things the owner manages differently.

### D5: A reserved footer line for the DaemonSets share (R10)

**Choice**: On Overview with nodes on screen, one footer line between the
page/notice line and the key hints is always reserved. It shows
`<node> · DaemonSets: <share>% of the node (cpu <c>m <c>% · mem <m> <m>%)` (or
`<node> · no DaemonSets`) when the cursor is on a node and no panel, search
or command line is open, and is blank otherwise. The text is built by
walking the bound pods of the cursor node only, once per frame. While a
panel is open the popup takes the reserved line (`renderContent` gives it
the height without it), so popups keep the room they had.

**Justification, strongest first**:
1. Pagination fixes the page height before the cursor is resolved; a line
   that appears only on nodes would change the height after the cursor
   lands, repaginate and move the cursor. A line that is always there keeps
   pages stable as the cursor moves.
2. The owner asked for the DaemonSets share of the selected node without
   opening the popup; one line is the smallest place for it.
3. One node's pods (tens, at most a few hundred) per frame is negligible
   next to the frame; every other node keeps the per-node running totals.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The node list is one line shorter on Overview, even when the cursor is on a heading | Stable pages are worth one line; the list keeps at least 3 lines, or 2 (column title and one row) on a window too short for both (corrected 2026-11-03: the frame must not grow past the window). Superseded 2026-11-03 (refuter): two lines hid the selected node under a group heading, so on a window too short for both the DaemonSets line is dropped and the list keeps 3 lines |
| A frame walks the pods of one node, where the README says frames never walk pods | Only the cursor node, bounded by its pod count; the README is reworded to say so |

**When this would be wrong**: when a node holds thousands of pods (the walk
would show in the frame time), or when the owner wants the line hidden.

**Reversal action**: Keep a per-node running DaemonSet total in the store
(like `n.used`), or drop the line and the reserved row.

**Alternatives discarded**: A line only when the cursor is on a node —
circular with pagination. A column in the node list — the clutter D1 rules
out.

## Invariants

- Requests, never usage.
- The listed pods are the pods the node totals count; their requests add up
  to the node's requested CPU and memory.
- No new watch, no new API call; on the main screen only the R10 footer
  line is added.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Node price unknown | Cost `-`, share shown, cost sort uses share | `-` in COST/H |
| Reserved node | Costs `~$…` | `~` prefix |
| Node gone while the tab is open | Same message as Details today | Popup text |
| Pod with no requests | `no requests`, sorted last | Row text |
| Zero allocatable for a resource | Percent `-`, left out of the share; both zero: share and cost `-` | Row text |
| Pricing disabled | No COST/H column, cost sort skipped | Column missing |
| Name too long for the window | POD cut with `…`, then percents dropped | Row text |

## Test plan

- `pkg/model/nodepods_internal_test.go`: share and cost (R3), unknown and
  equivalent price (R4), no requests (R5), `ds` marks in the all-pods view
  (R6), `d` toggle, Total line and DaemonSet rows (R9), sort cycle, flip,
  ties, no-requests last in every sort (R7), cursor, `enter`/`space` doing
  nothing, cursor fallback, node gone (R8), requested quantities add up to
  `Node.Used()` (R2), live updates, pricing disabled, 80 and 40 columns;
  `DaemonSetLine` text and its absence (R10); page boundaries equal with
  the cursor on a heading and on a node (R10).

> **Corrected 2026-11-03.** The test plan above replaces the DaemonSet group
> and `enter`/`space` toggling with the `ds` marks, the `d` toggle, the Total
> line, `DaemonSetLine` and the page-stability test.
- `pkg/model/nodedetails_internal_test.go`: `tab` switches, Details
  unchanged, reset on reopen (R1).
- `pkg/client` transform test: owner name kept.
- `scripts/verify.sh` and `make lint` green; coverage ≥ 90% on new code.
- Manual run on a cluster, noting what was checked, never the cluster's name.

> **Corrected 2026-11-02.** The popup drops the fixed lines after the first
> (node line, column header) before the key line, so a short window keeps the
> panel's keys and the tab line; found by the refuter at heights up to 14
> (`TestNodePodsShortWindowKeepsTheKeyLine`). When the cursor's DaemonSet row
> goes away and the group is still there, the cursor moves to the group row
> (`TestNodePodsCursorOnVanishedDaemonSetGoesToTheGroup`).

> **Corrected 2026-11-03.** The second half of the 2026-11-02 note above is
> superseded: there is no group any more, and a vanished DaemonSet row
> follows the R8 fallback. The first half stands, and the Total line of the
> DaemonSets view is one of the fixed lines dropped on short windows, with
> the node line and the column header.
