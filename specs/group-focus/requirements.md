# Requirements: group-focus

> Status: active · Owner: sule · Reviewed: 2026-11-04 (spec-refuter, three rounds; findings fixed)

## Context

With the node list grouped (for example by `karpenter.sh/nodepool`), the
owner most of the time wants to look at the nodes of one group only. Today
the choices are all groups with all their nodes (`n` unfolded) or every
group folded; folding the other groups one by one with `enter` is slow and
is saved with the view, so it has to be undone afterwards. A filter on the
group's value works but takes several keys in the Filters panel and also
stays in the saved view.

`enter` already means "open what is under the cursor" on a node (its
details). This feature makes it mean the same on a group heading: show that
group's nodes, and nothing else, until `esc`.

## Usage examples

Grouped by nodepool, folded with `n`:

```text
  ▸ default    12 nodes  140 pods  cpu ▇▇▇▇▇▇▇▇▇▇  71%  $1.920/h
▌ ▸ gpu         3 nodes   18 pods  cpu ▇▇▇▇▇▇      38%  $3.018/h
  ▸ system      2 nodes   31 pods  cpu ▇▇▇▇▇▇▇▇    55%  $0.192/h
```

`enter` on `gpu`:

```text
View:  Filters: none │ Focus: karpenter.sh/nodepool=gpu │ Group: karpenter.sh/nodepool │ Sort: creation ↓ │ cpu
  NAME            ...
▌ gpu  3 nodes  18 pods ...
    ip-10-0-12-15 cpu ▇▇▇▇▇▇▇▇  52%  ...
    ip-10-0-12-33 cpu ▇▇▇▇      21%  ...
    ip-10-0-12-61 cpu ▇▇▇▇▇▇▇▇▇ 61%  ...
 ... • esc: back • ...
```

The header counts (`3 of 17 shown`) and the cluster summary cover those
three nodes. `enter` on a node opens its details as before. `esc` goes back
to the three folded groups, with the cursor on `gpu`.

Two levels (nodepool › zone): `enter` on `default` shows its zones and
nodes; `enter` on `default › us-east-1a` then shows only that zone; `esc`
goes back to `default`, and `esc` again to every group.

## Requirements

**R1** WHEN the cursor is on a group heading on Overview and `enter` is
pressed THEN the system SHALL show only the nodes of that group (the
focus), under that group's heading, with its nodes listed even when the
list was folded with `n` or the group was folded on its own.

- [ ] Grouped by nodepool, `enter` on `gpu` lists only the `gpu` nodes, under the `gpu` heading.
- [ ] With the list folded by `n` (no node rows), the focused group's nodes are listed.
- [ ] A group folded on its own (`▸`) shows its nodes once focused.
- [ ] The header counts (`<n> of <total> shown`), the pod count and the cluster summary cover only the focused nodes; the summary keeps its `(filtered: <n> of <total> nodes)` wording.
- [ ] Sub-groups inside the focus that were folded on their own stay folded (three levels: focus on level 1, a level-2 group folded with `space` stays `▸`).
- [ ] A focus whose nodes fill more than one page pages as before: page dots, and the group path repeated with `(cont.)` at the top of a continuing page.
- [ ] Nodes with no value for the group's field (`<none>` group) can be focused like any group.
- [ ] `enter` on a node still opens its details, focused or not.

**R2** WHEN a group is focused THEN the system SHALL show the focus in the
header's View line as `Focus: <field>=<value>`, one pair per level joined
by ` › `, and the key line SHALL read `enter: focus • space: fold` on a
grouped list and start with `esc: back` while focused, without `n: fold`
and `d: details`.

> **Corrected 2026-11-04 (implementation).** `esc: back` comes first, not
> last: the key line is cut at the window's edge, and at 160 columns the
> end of it, where `esc: back` was, did not show.

- [ ] Grouped by nodepool › zone, focusing `default › us-east-1a` reads `Focus: karpenter.sh/nodepool=default › topology.kubernetes.io/zone=us-east-1a` (derived fields by their name, labels by their key).
- [ ] Focused with no group-by (cleared while focused), the key line adds only `esc: back`.
- [ ] Without a focus, the View line is as before, and the key line reads `... • enter: focus • space: fold • n: fold ...` with a group-by.

**R3** WHEN `esc` is pressed on Overview with a group focused and no kept
search THEN the system SHALL leave the deepest focus level and put the
cursor on the heading of the group it left, on the page that heading is on.

- [ ] Focus `default › us-east-1a`, `esc` → focus `default`, cursor on `default › us-east-1a`; `esc` again → no focus, cursor on `default`.
- [ ] With a kept search, the first `esc` clears the search as before; the next one leaves the focus.
- [ ] With no focus and no search, `esc` does nothing, as before.
- [ ] When the group left is on a later page, that page is shown.
- [ ] A cursor move pending when `esc` arrives does not move the cursor off the heading left.
- [ ] When the heading left no longer exists (group-by changed), the cursor goes to the first row of the first page.
- [ ] `esc` undoes the last `enter`: when one `enter` added several levels (R4), one `esc` removes them all.

**R4** WHEN a group is focused and `enter` is pressed on a group heading
THEN the system SHALL add to the focus the heading's levels whose field is
not in the focus yet, keeping the levels it has, and SHALL do nothing when
every level of the heading is already in the focus (the focused group's own
heading, or one above it).

- [ ] Nodepool › zone, focused on `default`: `enter` on `default › us-east-1a` focuses it.
- [ ] `enter` on the `default` heading while `default` is focused changes nothing.
- [ ] Focused on `default › us-east-1a`, `enter` on the `default` heading changes nothing.
- [ ] Focused on nodepool `gpu`, then grouped by zone instead: `enter` on `us-east-1a` focuses `nodepool=gpu › zone=us-east-1a`, and the list still holds only `gpu` nodes.

**R5** WHEN `space` is pressed on a group heading THEN the system SHALL
fold or unfold that group on its own and save it with the view, as `enter`
did before this feature.

- [ ] `space` on `OnDemand` folds it (`▸`) and the fold is saved; `space` again unfolds it.
- [ ] Inside a focus, `space` on a sub-group heading folds that sub-group.
- [ ] Inside a focus, `space` on the focused group's heading, or a heading above it, does nothing and saves nothing.
- [ ] `space` on a node does nothing, as before.

**R6** WHEN a group is focused THEN the system SHALL keep the focus out of
the saved view state and out of named views, keep it across changes of
filters, sort, columns, resources, search and group-by, and clear it when
the context changes or a named view is applied.

- [ ] The view passed to the save callback has no trace of the focus; the state file format is unchanged.
- [ ] A view saved with `v` while focused does not focus when applied.
- [ ] Switching the context (`c`) or applying a view (`v`) shows every group again.
- [ ] Changing the group-by while focused keeps listing only the nodes whose values match the focus, and the View line still shows it.
- [ ] When the filters and search leave nodes but none matches the focus, the list says `No nodes in the focused group.`; when the filters or search leave none, it says `No nodes match the filters or search.` as before; `esc` still leaves the focus.

**R7** WHEN a group is focused THEN `n` and `d` SHALL do nothing and the key
line SHALL not offer them, since the focus always lists its nodes.

- [ ] Focused, `n` leaves the list and the saved view unchanged.
- [ ] After `esc` out of the focus, `n` folds as before, and the fold state saved before the focus is back.

**R8** WHEN the screen is Recommendations THEN the system SHALL ignore the
focus (rows, header counts, `e` and `esc`), and WHEN `e` exports from
Overview while focused THEN it SHALL export the focused nodes only and SHALL
write the focus in the JSON as `"focus": [{"field": …, "value": …}]`.

- [ ] Focused on `gpu`, `tab` to Recommendations shows the same rows and header counts as with no focus; `esc` there leaves the focus as it is; `tab` back shows the focus again.
- [ ] Focused on `gpu` on Overview, `e` writes only the `gpu` nodes, and the JSON has `focus` with `karpenter.sh/nodepool` = `gpu`; with no focus the key is absent; the CSV columns do not change.
- [ ] On Recommendations, `e` writes the same nodes as with no focus, with no `focus` key.

## Counter-examples

- The system SHALL NOT add a filter to the view, the Filters panel or the state file when a group is focused.
- The system SHALL NOT change what `enter` does on a node, in any panel or on Recommendations.
- The system SHALL NOT change the order of the groups or of the nodes inside the focus.
- The system SHALL NOT let `esc` quit (interactive-tui R12).

## Out of scope

- Remembering the focus across runs or per cluster.
- Focusing on Recommendations rows.
- A key to focus a group from a panel (the Filters panel already filters by value).
- Mouse clicks.

## Open questions

- None.
