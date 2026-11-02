# Design: views-and-export

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Filters | `Filter{Field, Values, Enabled}`; `Matches` keeps a node having any value | `pkg/model/view.go` |
| Filters panel | fields and values listed up front; space ticks, d clears | `pkg/model/panels.go` |
| State file | `{version, clusters: {key: ViewState}}`, debounced atomic writer | `pkg/state/state.go` |
| Keys | f g s o c / r n d ? q in the main view | `pkg/model/uimodel.go` |

## Target state

- `Filter` gains `Excluded []string` (JSON `excluded`, omitted when empty). A
  filter keeps `Values` or excludes `Excluded`; `x` moves the ticked values
  from one field to the other. Code that only knows `values` sees an empty
  filter in an excluding one and filters nothing, instead of the opposite.
- The state file gains `views: {name: ViewState}` beside `clusters` (`Load`
  keeps either when the other is missing); the UI reaches it through a small
  `Views` interface set by `cmd`, whose saves report failures back as a
  footer notice.
- A Views popup (`v`) with a one-line name input, like the search line.
- `e` builds rows on the UI goroutine from `Select` over every watched node
  (all pages, folded groups included) and returns a `tea.Cmd` that writes both
  files (exclusive create, `-2`… on a clash, state directory when the current
  one is not writable) and answers with a message the footer shows for 5 s.
- Applying a named view uses `SetView` (resources and the `r` cycle) and then
  reports the change, like any TUI change.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/view.go` | `Filter.Exclude`, matching, header text |
| `pkg/model/panels.go` | `x` on a field row; `not` marker |
| `pkg/model/views.go` (new) | Views popup, name input |
| `pkg/model/export.go` (new) | rows, CSV and JSON writers, footer notice |
| `pkg/state/state.go` | `views` in the file; get/put/delete |
| `cmd/kube-node-viewer/main.go` | wire views to the store and saver |
| `README.md`, `CHANGELOG.md` | keys and files |

## Decisions

### D1: Exclusion is a property of one filter

**Choice**: a filter either keeps or excludes its values (`x` toggles); fields
still combine with AND.

**Justification, strongest first**:
1. It answers "everything but Spot" in one key, and stays right when new
   values appear.
2. It keeps the filter model of interactive-tui D4; no query language.

**Trade-offs accepted**: no OR across fields.

**When this would be wrong**: queries mixing fields with OR become common —
then the deferred typed syntax.

**Alternatives discarded**: an `Exclude bool` on the filter — an older
binary reading it would keep only the excluded values (review 2026-11-02).

**Reversal action**: drop `excluded`; older code already ignores it.

### D2: Named views are global, in the state file

**Choice**: views are saved once for all clusters, in the same state file,
written by the same debounced atomic writer.

**Justification, strongest first**:
1. The owner's examples ("cost per nodepool") are habits, not facts of one
   cluster.
2. One file, one writer: no new file to corrupt or document.

**Trade-offs accepted**: a view filtering on a value one cluster lacks shows
no nodes there; the panel still lists the value (interactive-tui behaviour)
so it can be unticked.

**When this would be wrong**: the owner wants per-cluster views — key them by
cluster like `clusters`.

**Alternatives discarded**: per-cluster views (the examples are habits);
a separate views file (a second file to write atomically and document).

**Reversal action**: move `views` under each cluster.

### D3: Export writes CSV and JSON to the current directory

**Choice**: `e` writes both formats next to where the tool was started, named
by context and time; no prompt.

**Justification, strongest first**:
1. One key, no dialog: the footer says where the files went.
2. CSV opens in a spreadsheet; JSON keeps groups, totals and the view for
   scripts.

**Trade-offs accepted**: files pile up in the directory. Two exports in the
same second get `-2`. When the current directory is not writable (a launch
from `/`), files go to the state directory instead and the footer says so.

**When this would be wrong**: users want to pick the place — then a prompt.

**Alternatives discarded**: a save dialog (a text input for a path on every
export); one format only (CSV loses groups and totals, JSON is awkward in a
spreadsheet).

**Reversal action**: remove the key.

## Invariants

- Requests vs allocatable only; read-only towards the cluster.
- `Update` never waits on the disk (export and view saves run off the UI
  goroutine).

## Test plan

- Unit: exclusion matching (single, multi-value, status flags); header text;
  state round-trip of views and of `exclude`; CSV/JSON content for a filtered
  view; export failure message.
- Key-driven: `x`; `v` save, apply, replace, delete, empty name, typed `q`;
  `e` writes files into a temp dir.
