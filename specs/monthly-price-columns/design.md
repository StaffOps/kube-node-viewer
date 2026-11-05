# Design: monthly-price-columns

> Implements [requirements](requirements.md).

## Current state

- Fields are strings persisted in the state file (`pkg/model/view.go:30-48`);
  `FieldPrice` (`"price"`) and `FieldIdle` (`"idle"`) are both sort fields
  and columns.
- Column titles come from `columnTitle` (`pkg/model/layout.go:637`), cells
  from `UIModel.cell` (`layout.go` after `render`): `PRICE/H` is
  `$%0.4f`, `~$` for an equivalent, `-` without a price; `IDLE/H` uses
  `Node.IdleHourly` (`view.go`).
- `DefaultColumns` and `columnPriority` (`view.go:529-533`) decide the
  default list and what narrow windows drop.
- `newNodeTable` leaves out `FieldPrice` under `DisablePricing`
  (`layout.go` in `newNodeTable`). `IDLE/H` is not left out and shows
  values: under `--disable-pricing` the session uses the static price table
  (`cmd/kube-node-viewer/session.go`, `pkg/aws/pricing.go`), so nodes have a
  real price.
- Monthly figures elsewhere go through `u.printer` (thousands separators):
  `$%0.3f/month` in summary and headings, `$%0.2f/month` in
  Recommendations (`recommendations.go:144`).
- `DefaultColumns` is used by `DefaultViewState()` when a cluster has no
  saved view, and by `--extra-labels`, which rebuilds the columns from it
  (`cmd/kube-node-viewer/view.go`).
- The month factor is `avgHoursPerMonth = (365*24)/12` = 730
  (`pkg/model/uimodel.go:47-48`), same value as `recommend.HoursPerMonth`.

## Target state

Two column-only fields, `FieldPriceMonthly` (`"price-month"`) and
`FieldIdleMonthly` (`"idle-month"`). `cell` derives them from the same
values as the hourly columns × `avgHoursPerMonth`, formatted
`u.printer.Sprintf("$%0.2f")` (as Recommendations).
They are not sort, filter or group fields.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/view.go` | two fields; `DefaultColumns`; `columnPriority` |
| `pkg/model/layout.go` | `columnTitle` (`PRICE/M`, `IDLE/M`); `cell`; `newNodeTable` leaves all four price columns out when pricing is disabled |
| `pkg/model/panels_view.go` | no change: `columnChoices` already offers every `DefaultColumns` entry, then `IDLE/H` |
| `README.md`, `CHANGELOG.md` | columns table, Columns panel sentence, troubleshooting; Unreleased entry (Added, and Changed for `IDLE/H` under `--disable-pricing`) |
| `.static/` | regenerated with `scripts/screenshots.sh`: the main, groups, columns and compact-header scenes show the default list |

## Decisions

### D1: Separate column fields, not a per-column unit switch

**Choice**: `price-month` and `idle-month` are their own fields, toggled in
the Columns panel like any column.

**Justification, strongest first**:
1. The owner wants to show hourly, monthly or both; independent columns give
   that with the existing panel and no new key.
2. Columns are already a list of fields in saved state and named views; two
   more values need no format change.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Two more columns compete for width | `columnPriority` drops the least important first, and the footer counts hidden columns |

**When this would be wrong**: more units are wanted (per day, per year), and
a column per unit per metric multiplies.

**Reversal action**: remove the two fields and their cases. Saved state that
lists them still loads; an unknown field falls through `cell` to `-`
(`layout.go`, end of `cell`) under its raw name as title, so drop them from
saved columns or keep the names reserved.

**Alternatives discarded**: a global "per hour / per month" toggle — cannot
show both at once, which the owner asked for.

### D2: No monthly sort fields; the Sort panel keeps `price` and `idle`

**Choice**: the new fields have no `number()` value and are not in
`sortChoices`.

**Justification, strongest first**:
1. ×730 preserves order, so a monthly sort would duplicate `price`/`idle`.
2. The owner asked for the sort name to stay `price`.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| No monthly entry in the Sort panel | Its `price` and `idle` give the same order; `--node-sort` only takes `creation` or a label anyway |

**When this would be wrong**: a sort that is not a monotone function of the
hourly price is wanted per month (none today).

**Reversal action**: add the fields to `number()` and `sortChoices`.

A hand-edited state that sorts or groups on them gets one `<none>` group and
name order: harmless.

### D3: No migration of saved columns

**Choice**: only `DefaultColumns` changes; clusters and named views with
saved columns keep them.

**Justification, strongest first**:
1. The owner chose to add the columns through `o`.
2. The README already says saved columns win over defaults; a migration
   would override a choice the user made and need a state version bump.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Existing clusters do not show the new columns until added with `o` | One `o`, `space` per cluster; documented in Troubleshooting |

**When this would be wrong**: users miss the columns and report them absent.

**Reversal action**: add a version-2 migration in `pkg/state` that inserts
the columns after `price`.

### D4: Narrow windows keep `PRICE/M` before `PRICE/H`, and `CAPACITY` before both hourly columns

**Choice**: `columnPriority` = `STATUS`, `PRICE/M`, `CAPACITY`, `PRICE/H`,
`IDLE/M`, `INSTANCE`, `IDLE/H`, `NODEGROUP`, `ZONE`, `IP`, `AGE`, `REGION`.

**Justification, strongest first**:
1. The owner reads cost per month first. `PRICE/H` is `PRICE/M` ÷ 730, so it
   is the first thing worth losing when space is short.
2. `CAPACITY` (Spot / OnDemand / Reserved, coloured) is information no price
   column repeats. Measured on the demo cluster, cpu only: putting both
   price columns ahead of it would drop it at 120 columns.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| More columns hidden at 140–160 columns than today (two more defaults) | The footer counts hidden columns; all eight fit from 168 columns; columns can be hidden with `o` |
| `INSTANCE` drops before today | Instance type is also in node details and the search |

> **Corrected 2026-11-05.** The widths above (120, 140–160 and 168
> columns; "about 172" in the README and the 0.11.0 CHANGELOG) were never
> recorded as measured. With the `realisticUI` test fixture all default
> columns show only from about 192 columns wide (cpu) or 195 (cpu and
> memory), measured by the retroactive review; the real figure depends on
> the length of the node and node-group names. The README now says so
> instead of a number.

**When this would be wrong**: the owner prefers the hourly price on narrow
windows, or `INSTANCE` is missed.

**Reversal action**: reorder `columnPriority`.

## Invariants

- Monthly = hourly × 730; the same 730 the summary and group headings use.
- `PRICE/H` and `IDLE/H` unchanged.
- `-` exactly where the hourly column shows `-`; `~$` exactly where
  `PRICE/H` shows `~$`.
- Under `--disable-pricing`, no price or idle column is drawn. The Columns
  panel still lists them (as it lists `PRICE/H` today); ticking one draws
  nothing.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Price unknown | `-` in both price columns | header `>=` / `prices unknown`, as today |
| Pricing disabled | price and idle columns absent | as `PRICE/H` today |

## Test plan

- Table-driven tests in `pkg/model`: cell values for real, equivalent,
  unknown price and no idle; two decimals and a value ≥ $1,000 with its
  separator; titles; default columns and order
  on a cluster with no saved columns; saved columns untouched; Columns panel
  lists and toggles each; `DisablePricing` leaves out all four; narrow-window
  priority (120 columns keeps `PRICE/M`, `CAPACITY`, `STATUS`); Sort panel
  unchanged (`price`, `idle`); `--extra-labels` gives the new defaults plus
  labels (`cmd/kube-node-viewer`); Columns panel order for an old saved view.
- `scripts/verify.sh`, `make lint`. Not run against a cluster unless the
  owner names one.
