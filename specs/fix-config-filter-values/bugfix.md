# Bugfix: config-filter-values

> Status: draft · Reported: 2026-11-05 · Reviewed: —

Found by the `spec-refuter` review of
[`fix-config-case`](../fix-config-case/bugfix.md) (2026-11-04, LOW-4), listed
in `TODO.md`.

## Current behavior

Reproduced on 2026-11-05 by reading the code (`readFilters` in
`cmd/kube-node-viewer/configview.go`, `Filter.Matches` in
`pkg/model/view.go`):

| Item in a view's `filters` | Result today |
|----------------------------|--------------|
| `{field: capacity, values: [spot]}` | loads; no node matches, since a node's capacity is `Spot` (`CapacityType`), so the view shows no node and no error |
| `{field: capacity, exclude: [spot]}` | loads; excludes nothing, so the view shows every node, Spot ones included |
| `{field: status, values: [ready]}`, `[Cordon]` | loads; no node matches (`Ready`, `Cordoned`) |
| `{field: capacity, values: [Preemptible]}` | loads; no node matches: no node can have that value |

The `TODO.md` item said capacity values are lower case; they are not:
`OnDemand`, `Spot`, `Reserved`, `Fargate`, `Unknown`, as the CAPACITY
column and the Filters panel show them.

## Expected behavior

**R1** WHEN a `values` or `exclude` item of a filter on `capacity` or
`status` is not one of that field's values THEN the system SHALL refuse
the file with the item's path and `"<value>" is not a <field> value;`
followed by `write <value>` when the value matches one in another case,
else `use one of <values>` listing them in display order.

- [ ] `capacity`, `values: [spot]` → `views[0].filters[0].values[0]:
  "spot" is not a capacity value; write Spot`.
- [ ] `capacity`, `exclude: [ONDEMAND]` → `…exclude[0]: "ONDEMAND" is not a
  capacity value; write OnDemand`.
- [ ] `capacity`, `values: [Preemptible]` → `"Preemptible" is not a
  capacity value; use one of OnDemand, Spot, Reserved, Fargate, Unknown`.
- [ ] `status`, `values: [ready]` → `write Ready`; `[Cordon]` → `use one of
  Ready, NotReady, Cordoned, Deleting, Launching`.

**R2** WHEN a filter is on any other field (`nodegroup`, `zone`, `region`,
`instance-type`, a node label) THEN the system SHALL keep its values as
written and match them exactly, as before.

- [ ] `{field: zone, values: [US-EAST-1A]}` and `{field: team, values:
  [Payments]}` load unchanged.
- [ ] Every correctly spelled `capacity` and `status` value loads.

## Unchanged behavior

- The interactive Filters panel (`f`): it offers the values the nodes
  have, so it cannot produce a wrong one; filters restored from the state
  file or saved views are not checked.
- Matching stays exact and case-sensitive (`Filter.Matches`).
- Every other `config.yaml` message, those of
  [fix-config-errors](../fix-config-errors/bugfix.md) and
  [fix-config-case](../fix-config-case/bugfix.md) included; a value with
  leading or trailing whitespace still gets its existing error first.
- A config whose `capacity` and `status` values are spelled as displayed
  (the examples in `docs/views.md`, the owner's kind of config) loads as
  before.

Counter-examples: MUST NOT refuse a value of an open field; MUST NOT
change which nodes a correctly spelled filter keeps; MUST NOT check
filters that did not come from `config.yaml`.

## Root cause

`readFilters` reads `values` and `exclude` with `stringList`, which checks
only the type and surrounding whitespace. `Filter.Matches` compares each
value exactly with `Field.Values(n)`, so a value no node can have keeps
nothing (or, in `exclude`, drops nothing) without a message.

## Fix

- A closed value list per field in `pkg/model`: `CapacityValues`
  (`OnDemand`, `Spot`, `Reserved`, `Fargate`, `Unknown`, the
  `CapacityType` constants) and `StatusValues` (the `Status…` flags), next
  to the constants so a new value is added in one place.
- `readFilters` checks each `values`/`exclude` item of a `capacity` or
  `status` filter against that list: an exact match passes; a
  case-insensitive match gives `write <value>`; anything else `use one of
  …`.

Why only these two fields: their values are fixed by the program, so a
value outside the list can never match, on any cluster. The other fields'
values come from the cluster, which is not known when the file is read,
and Kubernetes label values are case-sensitive (`Payments` and
`payments` may both exist), so neither an error nor a hint is safe there.

Not a breaking change: a refused value never kept a node (in `values`) or
dropped one (in `exclude`), so no config that worked stops working; the
CHANGELOG lists it under `Fixed`.

When wrong: a future capacity or status value missing from the list would
be refused; keeping the list next to the constants, with a test that every
constant is in it, guards that. Reversal: drop the check in `readFilters`.
