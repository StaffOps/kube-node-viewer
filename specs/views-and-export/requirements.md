# Requirements: views-and-export

> Status: shipped (v0.9.0, 2026-11-02) · Owner: Carlos Gomes · Reviewed: 2026-11-02 (spec-refuter; 4 HIGH, 7 MEDIUM, 4 LOW, all applied)

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
# f → open capacity → tick Spot → x: the filter now excludes Spot
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

**R1** WHEN the user presses `x` on a field row (or one of its value rows) of
the Filters panel and the field has a filter THEN the system SHALL switch it
between keeping and excluding its values; an excluding filter SHALL keep a
node that has none of the values.

- [ ] Test: `capacity` excluding `{Spot}` keeps OnDemand and Reserved nodes,
  drops Spot.
- [ ] Test: a status filter excluding `Cordoned` drops a node that is Ready
  and Cordoned.
- [ ] Test: ticking or unticking a value keeps the filter's mode; unticking
  the last value removes the filter as today.
- [ ] `x` on a field with no filter does nothing.
- [ ] The header shows `field≠value` or `field∉{a,b}`; the panel marks the row
  `not`; the mode is saved with the view.
- [ ] Test: a state file written with an excluding filter, read by code that
  only knows `values`, filters nothing (excluded values live in their own
  field, `excluded`).

**R2** WHEN the user presses `v` THEN the system SHALL list the saved views by
name, apply the selected one with `enter`, delete it with `d`, and save the
current view under a typed name from `+ save current view as…`.

- [ ] Saved views are shared by all clusters and kept in the state file next
  to the per-cluster views.
- [ ] A view carries filters, group-by, sort, resources, columns, the node
  list toggle, the folding depth and details. It does not carry the search
  text, groups folded one by one (their paths are one cluster's values) or the
  header size (a screen preference).
- [ ] Applying a view goes through the same path as loading a cluster's view
  (resources and the `r` cycle follow), then saves it as the cluster's view.
  Test: applying a view with a custom resource list shows those resources.
- [ ] Saving under an existing name replaces it; an empty or whitespace-only
  name is refused with a footer notice.
- [ ] The name input takes printable keys (`q`, `f`, `d` are text); `enter`
  saves, `esc` cancels, `backspace` deletes, `ctrl+u` clears. `ctrl+c` still
  quits and `ctrl+e` still toggles the header, as everywhere.
- [ ] A view save that fails to reach the disk shows a footer notice.
- [ ] Tests: a state file with views and no clusters keeps its views; a file
  from v0.8.x without `views` loads with none.

**R3** WHEN the user presses `e` in the main view THEN the system SHALL write
every node the filters and search keep (all pages, folded groups included) to
a CSV and a JSON file, and say where in the footer, or why it failed.

- [ ] Files go to the current directory, or to the state directory when the
  current one is not writable; test both.
- [ ] Name: `kube-node-viewer-<context>-<YYYYMMDD-HHMMSS>.csv` and `.json`,
  the context reduced to `[A-Za-z0-9._-]` (others become `_`); test with an
  ARN context `arn:aws:eks:us-east-1:111122223333:cluster/prod`. A name that
  exists already gets `-2`, `-3`… (files are created exclusively).
- [ ] CSV (`encoding/csv`): one row per node with name, group path, instance
  type, capacity, node group, zone, region, IP, status, age, pods, for each
  resource requested / allocatable / percent, price per hour, price kind
  (`actual`, `equivalent`, `unknown`, or `disabled` with `--disable-pricing`,
  price left empty then), then the label columns chosen in Columns. A cell
  starting with `=`, `+`, `-` or `@` is prefixed with `'` so spreadsheets
  don't run it.
- [ ] JSON: the same nodes, the view, the totals of the nodes exported and of
  the cluster, the context and server, and the time.
- [ ] Rows are built on the UI goroutine from the frame's data; only plain
  values go to the command that writes, which runs off the UI goroutine.
- [ ] The footer notice lasts 5 seconds. Footer priority: the search line,
  then a notice, then the page dots and hidden-columns note.

## Counter-examples

- SHALL NOT write anything to the cluster.
- SHALL NOT write files other than the exports (current directory, or the
  state directory as fallback), the state file and the log file.
- SHALL NOT export the search text as a filter, nor nodes hidden by filters
  or search.
- SHALL NOT let a saved view change watches (`--node-selector` stays the only
  server-side scope).

## Out of scope

- OR across different fields; typed filter syntax in `/`.
- Choosing the export directory or format from the TUI; exporting group rows
  only.
- Sharing views between machines.

## Open questions

- None.
