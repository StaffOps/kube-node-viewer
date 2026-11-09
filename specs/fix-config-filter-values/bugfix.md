# Bugfix: config-filter-values

> Status: shipped (v0.15.0, 2026-11-09) · Reported: 2026-11-05 · Reviewed: 2026-11-05 (spec-refuter; 1 HIGH, 4 MEDIUM, 4 LOW, applied), 2026-11-05 (spec-refuter, code; 2 MEDIUM, 5 LOW, applied)

Found by the `spec-refuter` review of
[`fix-config-case`](../fix-config-case/bugfix.md) (2026-11-04, LOW-4), listed
in `TODO.md`.

## Current behavior

Reproduced on 2026-11-05 by reading the code (`readFilters` and
`stringList` in `cmd/kube-node-viewer/configview.go:299-320, 485-490`;
`Filter.Matches` in `pkg/model/view.go:205-217`):

| Item in a view's `filters` | Result today |
|----------------------------|--------------|
| `{field: capacity, values: [spot]}` | loads; no node matches, since a node's capacity is `Spot` (`CapacityType`), so the view shows no node and no error |
| `{field: capacity, exclude: [spot]}` | loads; excludes nothing, so the view shows every node, Spot ones included |
| `{field: capacity, values: [on-demand]}`, `[CAPACITY_BLOCK]`, `[regular]` | load; the label spellings (Karpenter, EKS, AKS) match no node |
| `{field: capacity, values: [Spot/Auto]}`, `{field: status, values: [Ready/Cordoned]}` | load; copied from the CAPACITY or STATUS column, which adds `/Auto` and joins the status flags, they match no node |
| `{field: status, values: [ready]}`, `[Cordon]` | load; no node matches (`Ready`, `Cordoned`) |

The `TODO.md` item said capacity values are lower case; they are not. The
Filters panel (`f`) lists exactly `OnDemand`, `Spot`, `Reserved`,
`Fargate`, `Unknown` for capacity and `Ready`, `NotReady`, `Cordoned`,
`Deleting`, `Launching` for status (`ValueCounts`); the columns show the
same words, but CAPACITY appends `/Auto` on EKS Auto Mode nodes and STATUS
joins a node's flags with `/`.

## Expected behavior

**R1** WHEN a `values` or `exclude` item of a filter on `capacity` or
`status` is not one of that field's values THEN the system SHALL refuse
the file with the item's path and `"<value>" is not a <field> value;`
followed by a hint:

- `write <want>` when the value, compared ignoring case, `-`, `_` and
  spaces, is one of the field's values or a known label spelling of one
  (`CAPACITY_BLOCK` and `capacity-block` → `Reserved`, AKS `regular` →
  `OnDemand`, `SchedulingDisabled` → `Cordoned`);
- for a value with `/` (copied from the column), `write one value per
  item: <want>, <want>`; an `Auto` part instead says `Auto is not a
  capacity: filter on label:eks.amazonaws.com/compute-type`;
- otherwise `use one of <values>`, in the panel's order.

The value is quoted (`%q`), like the existing whitespace error of filter
values, because a value can be any string, spaces included; field names
in fix-config-case are unquoted because they cannot hold spaces.

- [x] `capacity`, `values: [spot]` → `views[0].filters[0].values[0]:
  "spot" is not a capacity value; write Spot`.
- [x] `on-demand` and `ON_DEMAND` → `write OnDemand`; `CAPACITY_BLOCK` →
  `write Reserved`; `regular` → `write OnDemand`.
- [x] `exclude: [Spot/Auto]` → `…exclude[0]: "Spot/Auto" is not a
  capacity value; write Spot; Auto is not a capacity: filter on
  label:eks.amazonaws.com/compute-type`.
- [x] `status`, `values: [Ready/Cordoned]` → `write one value per item:
  Ready, Cordoned`; `ready` → `write Ready`.
- [x] `capacity`, `values: [Burstable]` → `use one of OnDemand, Spot,
  Reserved, Fargate, Unknown`; `status`, `[Cordon]` → `use one of Ready,
  NotReady, Cordoned, Deleting, Launching`.
- [x] The check runs per item, after that item's whitespace and repeat
  checks: `values: [Spot, " ready"]` reports the whitespace of item 1;
  `values: [spot, " x"]` reports item 0's hint.

> **Corrected 2026-11-05 (code review).** More hints: GKE `preemptible`
> and AKS `Low`/`LowPriority` → `write Spot`; a value whose `want` is
> already an earlier item of the same list → `<want> is already listed`;
> repeated parts of a `/` value count once (`Ready/Ready` → `write
> Ready`); a `/` value with a part that is no value (`Spot/Auto/x`) gets
> `use one of …`; `Auto` with two or more values → `write one value per
> item: Spot, OnDemand; Auto is not a capacity: …`. `Preemptible` is no
> longer the unknown example; `Burstable` is.

Tests: `// fix-config-filter-values R1` on
`TestConfigFilterValuesOfClosedFields` (full messages for every case
above) and `TestViewsDocListsTheClosedValues` (the docs list every
value).

**R2** WHEN a filter is on any other field (`nodegroup`, `zone`, `region`,
`instance-type`, a node label) THEN the system SHALL keep its values as
written and match them exactly, as before.

- [x] `{field: zone, values: [US-EAST-1A]}` and `{field: team, values:
  [Payments]}` load unchanged.
- [x] Every value of the two lists loads, in `values` and in `exclude`.

Tests: `// fix-config-filter-values R2` on
`TestConfigFilterValuesOfOpenFieldsLoad`.

**R3** WHEN `CapacityType()` or `StatusFlags()` can return a value THEN
that value SHALL be in `CapacityValues` or `StatusValues`, the one list
that both the config check and the Filters panel (`ValueCounts`) use.

- [x] Every constant of type `CapacityType`, and every `Status…` string
  constant, in the non-test files of package `model` (found by parsing
  them with `go/ast`: a declared type, `CapacityType("x")`, or the type
  carried down a const block; `iota` blocks are skipped without failing)
  is in its list.
- [x] A table of label sets covering every branch of `CapacityType()`
  (Fargate; Karpenter `on-demand`, `spot`, `reserved`; EKS `ON_DEMAND`,
  `SPOT`, `CAPACITY_BLOCK`; AKS spot and regular; none) and every flag of
  `StatusFlags()` (ready, not ready, cordoned, deleting, launching) gives
  only listed values.
- [x] `ValueCounts` builds its zero counts and its order from the lists.

Tests: `// fix-config-filter-values R3` on
`TestClosedValueListsHoldEveryConstant` and
`TestClosedValueListsCoverEveryBranch`.

## Unchanged behavior

- The interactive Filters panel (`f`) offers the same values in the same
  order (the existing `ValueCounts` tests in `pkg/model` pass unchanged).
- Filters restored from the state file or saved views are not checked:
  they come from the panel. _Test: manual — `readFilters` is the only
  caller of the check; `pkg/state` never calls it._
- A correctly spelled filter keeps the same nodes: matching stays exact
  and case-sensitive in `Filter.Matches` (existing filter tests pass
  unchanged).
- A value with leading or trailing whitespace, or repeated, still gets its
  existing error first (R1 last case).
- The examples in `docs/views.md` load (the existing doc example test),
  and so does the owner's kind of config (checked with the built binary,
  T4).
- Every other `config.yaml` message, those of
  [fix-config-errors](../fix-config-errors/bugfix.md) and
  [fix-config-case](../fix-config-case/bugfix.md) included.

Counter-examples: MUST NOT refuse a value of an open field; MUST NOT
change which nodes a correctly spelled filter keeps; MUST NOT check
filters that did not come from `config.yaml`.

## Root cause

`readFilters` (`configview.go:485-490`) reads `values` and `exclude` with
`stringList` (`configview.go:299-320`), which checks only the type,
surrounding whitespace and repeats. `Filter.Matches` (`view.go:205-217`)
compares each value exactly with `Field.Values(n)`, so a value no node can
have keeps nothing (or, in `exclude`, drops nothing) without a message.

## Fix

- `CapacityValues` and `StatusValues` (`[]string`) in `pkg/model/node.go`,
  next to the constants; `ValueCounts` uses them for its zero counts and
  order.
- `stringList` takes an optional per-item check, run after the item's
  whitespace and repeat checks; `readFilters` passes `closedValue` for
  `capacity` and `status`: an exact match passes, else the hints of R1.

Why only these two fields: their values are fixed by the program, so a
value outside the list can never match, on any cluster. The other fields'
values come from the cluster, which is not known when the file is read,
and Kubernetes label values are case-sensitive (`Payments` and
`payments` may both exist), so neither an error nor a hint is safe there.

A file that started before can now refuse to start: a view with
`exclude: [spot]` used to load (and drop nothing); now `loadSettings`
fails and `main.go` exits with the message (`log.Fatalf`, as for every
config error). Accepted because the filter never worked, so no view that
did what it said stops working; like
[fix-config-errors](../fix-config-errors/bugfix.md), the CHANGELOG lists
it under `Fixed`.

When wrong:

- a new capacity or status value missing from the lists would be refused;
  R3's tests fail when a constant or a branch is added without it;
- a config written for a newer release that knows a new value makes an
  older binary exit with `use one of …`; accepted, since the value is
  unknown to that binary anyway.

Reversal: drop the check passed by `readFilters`; the lists can stay.
