# Design: config-views

> Implements [requirements](requirements.md).

## Current state

- `config.yaml` is read by `cmd/kube-node-viewer/settings.go`
  (`readLabelLists`) with `yaml.UnmarshalStrict` into a struct: unknown keys
  are errors without their position, keys match case-insensitively
  (encoding/json), and a field typed `string` turns `yes`/`123` into text.
  List items are decoded as `any` and checked by `patternList`.
- The start view is `resolveView` (`cmd/kube-node-viewer/view.go:32`):
  defaults < `~/.kube-node-viewer` < saved cluster view (replaces both) <
  explicit flags. Each `sessions.Switch` runs it (`session.go:80`); explicit
  flags are consumed by the first one (`session.go:73-75`).
- Named views are `model.Views` (`pkg/model/views.go`), implemented by
  `namedViews` over the state file (`main.go:162`) and by `memViews` in
  tests. `applyView` keeps Search, Collapsed, Header and Screen from the
  current view, calls `SetView` and saves.
- Fields are strings (`pkg/model/view.go`): built-ins, `label:<key>`,
  `usage:<resource>`. `Field.Value` covers only the filter fields and
  `name`/`ip`; other built-ins group as `<none>`.

## Target state

```text
config.yaml ─▶ readSettings ─▶ settings{policy, views []ConfigView, defaultView, startWith}
                                      │
            ┌─────────────────────────┼──────────────────────────────┐
            ▼                         ▼                              ▼
   SetLabelPolicy        sessions.startView(clusterKey)      SetViews(configViews ⊕ namedViews)
                         first connection in the run?          ReadOnly(name) for config views
                         └▶ resolveView(cfg, saved, start, explicit)
```

- `readSettings` decodes the file into `map[string]any` and walks it by
  hand (`labels`, `views`, `default-view`, `start-with`), so every error has
  a path (`views[1].filters[0].values[2]`), keys compare exactly, and scalars
  are type-checked (`describeItem`). The label lists keep their messages.
- `configview.go` (new) turns a view item into a `model.ViewState`: starts
  from `model.DefaultViewState()`, applies each key, maps names to fields
  with the per-key tables of R2.
- The per-key built-in names come from the model: `model.FilterFields`,
  `model.SortFields()` (the fixed part of `sortChoices`) and
  `model.ColumnFields()` (the fixed part of `columnChoices`). The panels use
  the same functions, so the two cannot drift.
- `resolveView(cfg, saved, start, explicit)`: with a start view, the view is
  the start view with Header, Screen and Collapsed taken from `saved`;
  otherwise as today.
- `sessions` keeps the set of cluster keys connected in this run. The start
  view is passed only on the first connection to a key in `default-view`
  mode, or when there is no saved view in `last-view` mode.
- `views.go`: an optional `ReadOnlyViews` interface (`ReadOnly(name) bool`)
  found by type assertion. Rows get ` (config)`; `d` and saving under the
  name notify and do nothing. `combinedViews` lists config names, then state
  names that are not config names.

## Components touched

| Path | Change |
|------|--------|
| `cmd/kube-node-viewer/settings.go` | hand-walked decode of the whole file; `default-view`, `start-with` |
| `cmd/kube-node-viewer/configview.go` (new) | view item → `model.ViewState`; per-key names; validation |
| `cmd/kube-node-viewer/view.go` | `resolveView` start view |
| `cmd/kube-node-viewer/session.go` | start view per first connection; set of connected keys |
| `cmd/kube-node-viewer/main.go` | read settings once; wire views |
| `pkg/model/views.go` | read-only views |
| `pkg/model/panels_view.go`, `view.go` | `SortFields()`, `ColumnFields()` shared by panels and config |
| `README.md`, `CHANGELOG.md` | "Config views" section; Unreleased → Added |

## Decisions

### D1: Views as a YAML list of items with `name`, not a map

**Choice**: `views:` is a list; each item has `name:`.

**Justification, strongest first**:
1. Order: the decoder goes through JSON, so a map loses the order the user
   wrote; the Views panel lists config views in file order.
2. The owner wants block lists (`- item`).

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Repeated names are possible | Rejected at start (R6) |

**When this would be wrong**: views are referenced from many places by name
and a map reads better.

**Reversal action**: accept a map as well, ordered by name.

**Alternatives discarded**: a map keyed by name — loses order.

### D2: `start-with` picks the default view or the last view; the default view applies on the first connection to a cluster in the run

**Choice**: `default-view` mode opens each cluster with the default view
the first time the run connects to it; TUI changes are saved per cluster in
both modes.

**Justification, strongest first**:
1. The owner chose to let the user pick, with the default view winning by
   default.
2. First connection only: switching away with `c` and back keeps what was
   done in this run, instead of resetting it.
3. Saving in both modes keeps `last-view` meaningful if the user switches
   modes later.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A cluster's saved view is written but not shown at the next start | That is the mode the user picked; `last-view` shows it |

**When this would be wrong**: users expect `c` back to a cluster to reset
it, or `default-view` mode to never write the state file.

**Reversal action**: drop the connected-keys set (reset on every
connection), or skip saving in `default-view` mode.

**Alternatives discarded**: resetting on every connection — loses work
within a run; not saving in `default-view` mode — `last-view` would bring
back stale views.

### D3: Per-key built-in names from the model; any other name is a label

**Choice**: each key accepts the built-in names of the panel that sets it;
other names are labels; `label:` forces a label.

**Justification, strongest first**:
1. A built-in in the wrong key (group by `price`) gives a screen of
   `<none>` or `-`; rejecting it at start is clearer.
2. Labels are free text and cannot be validated; the panels accept any.
3. Taking the names from the functions the panels use keeps them in step
   (monthly columns appear when merged).

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A typo in a label (`karpenter.sh/nodepol`) is an empty column or a `<none>` group | Visible at once; built-in typos (`zon`) are labels too, listed in the README |

**When this would be wrong**: typos are reported often.

**Reversal action**: warn at start for names that are not built-in and have
no `/` and no `label:` prefix.

**Alternatives discarded**: one list of names for every key — accepts
combinations that render nothing.

### D4: Config views are read-only in the Views panel and win name clashes

**Choice**: listed first with `(config)`; `d` and saving under their name
are refused; a state view with the same name is hidden, not deleted.

**Justification, strongest first**:
1. The TUI never writes `config.yaml`; deleting a row that comes back at the
   next start would be a lie.
2. One name, one view: `enter` is never ambiguous.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A state view can be hidden by a new config view | It stays in the file; renaming the config view brings it back |

**When this would be wrong**: users want to tweak a config view in the TUI
and keep the tweak under the same name.

**Reversal action**: let a state view override the config view of the same
name.

**Alternatives discarded**: listing both — `enter` ambiguous.

### D5: Hand-walked decoding instead of a typed struct

**Choice**: decode to `map[string]any` and check every key and value.

**Justification, strongest first**:
1. Errors with the item's path, exact-case keys and strict scalars, which
   `UnmarshalStrict` into structs cannot give (it reports `unknown field`
   without the item, matches `Name`, turns `yes` into `"true"`).
2. One code path for the whole file, labels included.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| More code than a struct | Small helpers (`str`, `list`, `boolean`) and table tests |
| `yes`/`on` still read as `true` for `groups-only` | The YAML→JSON step erases the difference; documented |

**When this would be wrong**: the file grows many more keys.

**Reversal action**: move to `go.yaml.in/yaml/v3` nodes (positions and
exact types), listed as deferred in label-visibility.

**Alternatives discarded**: typed struct with `any` scalars — still no
paths for unknown keys.

## Invariants

- Without `views` and `default-view`, behavior is exactly as before.
- `config.yaml` is never written; state named views are never deleted by
  this feature.
- Precedence at a connection: defaults < `~/.kube-node-viewer` < (saved view
  or start view, per D2) < explicit flags (first cluster only).

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Invalid view or setting | exit 1 before the TUI | error with file and path |
| Typo in a label name | empty column / `<none>` group | on screen |
| A view uses a hidden label | applied and listed (R7) | — |

## Test plan

- `cmd/kube-node-viewer`: decoding of every key, block and flow lists; every
  R6 error with its path; per-key names, `label:` prefix, `usage:cpu`;
  default `order` per field and `order` without `sort`; `resolveView` in
  both modes with and without a saved view, Header/Screen/Collapsed kept,
  explicit flags on top; `sessions` first-connection rule with `c` and back;
  every README YAML example decodes.
- `pkg/model`: Views panel lists config views first with the marker, hides a
  state view of the same name, `enter` applies, `d` and save refused.
- `scripts/verify.sh`, `make lint`. Not run against a cluster unless the
  owner names one.
