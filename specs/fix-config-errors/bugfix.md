# Bugfix: config-errors

> Status: shipped (v0.14.0, 2026-11-05) · Reported: 2026-11-03 · Reviewed: spec-refuter 2026-11-04 (findings applied)

> **Corrected 2026-11-05.** Marked shipped by hand: this fix shipped in
> v0.14.0, when `scripts/release-prepare.py` did not ship bugfix specs
> ([fix-spec-tooling](../fix-spec-tooling/bugfix.md)).

Found by the `spec-refuter` review of [`config-views`](../config-views/)
(2026-11-03), listed in `TODO.md` ("config-views error polish").

## Current behavior

Reproduced on 2026-11-04 with a throwaway test in `cmd/kube-node-viewer`
that calls `loadSettings(writeSettings(t, oneView(...)), nil)` for each item
below (`oneView` puts it under `views: - name: a`) and prints the error or
the loaded view:

| Item | Result today |
|------|--------------|
| `columns:` then `  - label: zone` (a space after the colon) | error `views[0].columns[0]: map[label:zone] is not a string; quote it` — Go's map syntax, and quoting is not the fix |
| `columns: ["label: zone"]` | loads: a column for the label `" zone"`, which no node can have |
| `group-by: ["zone "]` | loads: groups by the label `"zone "`, not by zone |
| `sort: " price"` | loads: sorts by the label `" price"`, not by price |
| `resources: ["cpu "]`, `resources: [usage:cpu]`, `resources: [label:zone]` | load: resources no pod can request |
| `group-by: ["zone,region"]`, `columns: [PRICE/M]` | load as labels no node can have |
| `filters: [{field: zone, values: ["a "]}]` | loads: keeps value `"a "`, which no node has |
| `labels: {allow: ["team "]}` | loads, never matches; `--labels-allow "team "` is trimmed and works |
| `sort: price/m` (also `price/h`, `price-month`, `idle/h`, `idle/m`, `idle-month`) | error `price/m is not a sort field; write label:price/m for a node label` — the fix is `sort: price` |
| `sort: age`; `columns: [creation]` | error `write label:age` / `write label:creation` — the fixes are `sort: creation` and the column `age` (both are the node's creation time, `pkg/model/view.go`) |
| `views: {a: b}`; `columns:` then `  price: 1` | `map[a:b] is not a list`, `map[price:1] is not a list` |
| `columns: [1000000]` | `1e+06 is not a string; quote it` |

> **Corrected 2026-11-04 ([fix-config-case](../fix-config-case/bugfix.md)).**
> `columns: [PRICE/M]` now says `PRICE/M is not a column field; write
> price/m` instead of `"PRICE/M" is not a valid label key`.

## Expected behavior

**R1** WHEN a label key (a bare name read as a label, or the key after
`label:`) in `columns`, `group-by`, `sort` or `filters[].field`, or a
resource name in `resources` or after `usage:` in `sort`, is not a
Kubernetes qualified name (the rule of label keys and resource names: no
whitespace, no `:` or `,`, a lower-case DNS prefix before `/`) THEN the
system SHALL refuse the file with the item's path and the quoted value:
`has leading or trailing whitespace` when trimming would make it valid,
otherwise `is not a valid label key` (or `resource name`). The checks for
an empty value (`empty`) and for `label:` / `usage:` with nothing after
them (`names no label`, `names no resource`) come first and keep their
text.

**R2** WHEN a filter value or excluded value, or a `labels.allow` /
`labels.deny` pattern, has leading or trailing whitespace THEN the system
SHALL refuse the file with the path, the quoted value and `has leading or
trailing whitespace`. Whitespace inside a value is not checked.

**R3** WHEN an item of `columns`, `group-by`, `sort` or `filters[].field` is
a map of one key, `label` or `usage`, whose value is a non-empty string
(`- label: zone`) THEN the system SHALL refuse the file with the path and
`write label:zone with no space after the colon` (`usage:cpu` for usage, in
`sort` only). Any other map there (empty value, more keys) is reported by
R4.

**R4** WHEN an error shows a decoded value THEN maps SHALL read
`{key: value}` (keys sorted), lists `[a, b]`, and whole numbers without an
exponent (`1000000`); "quote it" SHALL follow only a scalar (number,
boolean, null), and a map or list where a string is expected SHALL read
`is a map, not a string` (or `is a list, not a string`). This applies to
every message that shows a value, `labels.*` included.

**R5** WHEN `sort` names a price or idle column (`price/h`, `price/m`,
`price-month`, `idle/h`, `idle/m`, `idle-month`) THEN the error SHALL say to
write `sort: price` (or `sort: idle`), which orders both the hourly and the
monthly column; WHEN `sort` is `age` THEN it SHALL say `sort: creation`;
WHEN `columns` has `creation` THEN it SHALL say to use the column `age`.

## Unchanged behavior

Each becomes a test asserting the whole message after the file path:

- Configs that load today and break none of R1–R2 load to the same views
  (the existing `TestConfigView*` tests).
- `columns: [yes]` → `views[0].columns[0]: true is not a string; quote it`;
  `123` and `~` likewise (`123`, `null`).
- `columns: ["  "]` → `views[0].columns[0]: empty`.
- `columns: ["label:"]` → `views[0].columns[0]: "label:" names no label`;
  `sort: "usage:"` → `views[0].sort: "usage:" names no resource`.
- `group-by: [price]` → `views[0].group-by[0]: price is not a group-by
  field; write label:price for a node label`.
- `columns: ["label:nvidia.com/gpu.product"]`, `resources: [nvidia.com/gpu]`,
  `sort: usage:nvidia.com/gpu` load.
- A filter value with an inner space (`values: ["a b"]`) loads.
- `order`, `start-with` and `default-view` messages:
  `views[0].order: " descending" is not ascending or descending`,
  `start-with: "last-view " is not default-view or last-view`,
  `default-view: no view is named " a"` (they already refuse padding).
- A padded view name: `views[0].name: "a " has leading or trailing spaces`.
- An inline `sort: label: zone` stays a YAML syntax error from the parser.
- The command line flags (`-group-by`, `-resources`, `-extra-labels`,
  `-node-sort`, `--labels-allow`, `--labels-deny`) and the legacy
  `~/.kube-node-viewer` keep trimming their values.

## Root cause

`str` (`cmd/kube-node-viewer/settings.go`) checks only that a value is a
non-empty string; `fieldKind.field`, `stringList` and the `label:` /
`usage:` prefix cut (`configview.go`) and `patternList` (`settings.go`) use
the string as is, and nothing checks the shape of label keys and resource
names. `describeItem` prints decoded values with `fmt.Sprint`, and `str`
says "quote it" for any non-string. `fieldKind.field` gives every built-in
name of another key the `label:` hint, including the price, idle, age and
creation fields, which have a sibling in the right key.
