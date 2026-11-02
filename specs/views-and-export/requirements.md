# Requirements: views-and-export

> Status: draft · Owner: Carlos Gomes · Reviewed: — (spec-refuter, YYYY-MM-DD)

## Context

After v0.8.2 the owner asked for three things the panels can't do yet:

- filter *out* a value ("everything but Spot"): today a filter only keeps the
  ticked values, so excluding one means ticking all the others, and a new
  value appearing later is silently left out;
- save a view under a name and come back to it ("cost per nodepool", "spot by
  zone") instead of rebuilding it from the panels each time;
- take what is on screen into a spreadsheet or a report.

The tool still only reads the cluster: export writes local files, never to
the cluster.

## Usage examples

```text
# f → open capacity → tick Spot → x on the capacity row: the filter excludes
│ View:     Filters: capacity≠Spot │ Group: nodegroup │ Sort: price ↓ │ cpu+memory
  … only OnDemand, Reserved, Fargate and Unknown nodes are listed

# v → "+ save current view as…" → type "spot by zone" → enter
╭─ Views ─────────────────────────────╮
│ cost per nodepool                   │
│ spot by zone                        │
│ + save current view as…             │
├─────────────────────────────────────┤
│ enter: apply • d: delete • esc: close│
╰─────────────────────────────────────╯

# e: export what is on screen
  exported 4 nodes to ./kube-node-viewer-prod-eks-20261102-153012.csv and .json
```

```text
# e when the directory is not writable
  export failed: open ./kube-node-viewer-…csv: permission denied
```

## Requirements

**R1** WHEN the user presses `x` on a field row of the Filters panel THEN the
system SHALL switch that field's filter between keeping and excluding its
values; an excluding filter SHALL keep a node that has none of the values.

- [ ] Test: `capacity ≠ {Spot}` keeps OnDemand and Reserved nodes, drops Spot.
- [ ] Test: a status filter excluding `Cordoned` drops a node that is Ready
  and Cordoned.
- [ ] The header shows `field≠value` or `field∉{a,b}`; the panel marks the row
  `not`; the choice is saved with the view.

**R2** WHEN the user presses `v` THEN the system SHALL list the saved views by
name, apply the selected one with `enter`, delete it with `d`, and save the
current view under a typed name from `+ save current view as…`.

- [ ] Saved views are shared by all clusters and kept in the state file.
- [ ] Applying a view replaces filters, group-by, folding, sort, resources and
  columns; the search text is left as is; the cluster's own view is then
  saved as usual.
- [ ] Saving under an existing name replaces it; an empty name is refused.
- [ ] While the name is typed, keys go to it (`q`, `f`, `d` are text).

**R3** WHEN the user presses `e` in the main view THEN the system SHALL write
the nodes on screen (after filters and search) to a CSV and a JSON file in the
current directory, named `kube-node-viewer-<context>-<YYYYMMDD-HHMMSS>`, and
say where in the footer for a few seconds, or why it failed.

- [ ] CSV: one row per node with every field (name, group path, instance type,
  capacity, node group, zone, region, IP, status, age, pods, per-resource
  requested / allocatable / percent, price per hour, price kind
  `actual|equivalent|unknown`), whatever columns are shown.
- [ ] JSON: the same nodes, the view, the shown totals and the cluster totals,
  the context and server, and the time.
- [ ] Writing happens off the UI goroutine; keys keep working meanwhile.

## Counter-examples

- SHALL NOT write anything to the cluster, nor a file anywhere but the
  current directory and the state file.
- SHALL NOT export the search text as a filter, nor nodes hidden by filters.
- SHALL NOT let a saved view change watches (`--node-selector` stays the only
  server-side scope).

## Out of scope

- OR across different fields; typed filter syntax in `/`.
- Choosing the export directory or format from the TUI; exporting group rows
  only.
- Sharing views between machines.

## Open questions

- None.
