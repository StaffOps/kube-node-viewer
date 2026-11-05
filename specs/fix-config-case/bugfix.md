# Bugfix: config-case

> Status: shipped (v0.14.0, 2026-11-05) · Reported: 2026-11-04 · Reviewed: 2026-11-04 (spec-refuter; 4 MEDIUM, 5 LOW, applied)

> **Corrected 2026-11-05.** Marked shipped by hand: this fix shipped in
> v0.14.0, when `scripts/release-prepare.py` did not ship bugfix specs
> ([fix-spec-tooling](../fix-spec-tooling/bugfix.md)).

Found by the retroactive `spec-refuter` review of
[`config-views`](../config-views/) (2026-11-04: MEDIUM-3, LOW-5, LOW-6).

## Current behavior

Reproduced on 2026-11-04 with `loadSettings` on `oneView(...)` (a view
`a` holding the item):

| Item | Result today |
|------|--------------|
| `columns: [INSTANCE, STATUS]` | loads as `label:INSTANCE`, `label:STATUS`; a label column's title is its key in upper case (`pkg/model/layout.go` `columnTitle`), so the screen shows `INSTANCE` and `STATUS` columns that look like the built-in ones and are empty |
| `group-by: [Zone]`, `filters: [{field: Zone, …}]`, `sort: NAME` | load as the labels `Zone`, `NAME`: one group `<none>`, a filter nothing matches, a sort that orders nothing |
| `group-by: [PRICE]` | loads as the label `PRICE`; `group-by: [price]` is an error (a built-in of another key) |
| `columns: [PRICE/M]` | error `"PRICE/M" is not a valid label key`, with no hint; `sort: PRICE/M` gets one |
| `columns: [instance]` | loads as the label `instance`, titled `INSTANCE` like the built-in column |
| `columns: [node-age]`, `[NODE-AGE]`, `[kube-node-viewer/NODE-AGE]`, `sort: node-cpu-usage` | load as labels titled like the computed `kube-node-viewer/node-age` (or `…/node-cpu-usage`) column, and empty |
| `columns: [Label:zone]`, `sort: USAGE:cpu` | error `"Label:zone" is not a valid label key`, with no hint |
| a key written twice (`name: a` then `name: b` in one view) | refused, as two lines: `error converting YAML to JSON: yaml: unmarshal errors:` then `  line 3: key "name" already set in map` |

## Expected behavior

**R1** WHEN an item of `columns`, `group-by`, `sort` or `filters[].field`
is not lower case and its lower-case form is a built-in name or alias of
that key (`zone`, `price/m`) or a column title of one (`instance` for
`instance-type`) THEN the system SHALL refuse the file with the item's
path, `<name> is not a <key> field; write <name to write>`, followed by
`, or label:<name> for a node label` when the name is a valid label key.
A name whose lower-case form is a built-in of another key only (`group-by:
[PRICE]`) SHALL get the existing message for a built-in of another key
(`write label:PRICE for a node label`).

- [x] `columns: [INSTANCE]` → `INSTANCE is not a column field; write
  instance-type, or label:INSTANCE for a node label`.
- [x] `group-by: [Zone]` → `write zone, or label:Zone for a node label`.
- [x] `columns: [PRICE/M]` → `write price/m` (a `/` after an upper-case
  prefix is not a label key).
- [x] `sort: PRICE/M` keeps its `sort: price` hint.
- [x] `columns: [instance]`, the lower-case form of the title `INSTANCE`,
  → `write instance-type, or label:instance for a node label`.
- [x] A computed label (`kube-node-viewer/node-age`,
  `kube-node-viewer/node-<resource>-usage`) written without its prefix or
  in another case (`node-age`, `NODE-AGE`, `kube-node-viewer/NODE-AGE`)
  → `write kube-node-viewer/node-age for the computed label`, with the
  `label:` way when the name is a valid label key.
- [x] `label:` or (in `sort`) `usage:` in another case (`Label:zone`) →
  `write label:zone`.

> **Corrected 2026-11-04 (spec-refuter review).** R1 first covered only
> names that differ from a built-in by case; the review found the
> lower-case title `instance`, the computed labels and the prefixes in
> another case load the same kind of look-alike or give no hint.

**R2** WHEN `config.yaml` cannot be decoded as YAML (a syntax error, or a
key written twice in one map) THEN the system SHALL report it after the
file name on one line, without the YAML library's `error converting YAML
to JSON: yaml: ` and `unmarshal errors:` prefixes, each error with its line
number when the YAML library gives one and several joined with `; `; an
error without a line starts with `YAML: `.

- [x] `line 3: key "name" already set in map`.
- [x] `views: *a` → `YAML: unknown anchor 'a' referenced`.

**R3** WHEN a name is not a built-in name, alias, column title or computed
label in any case THEN the system SHALL keep reading it as a label key, as
before.

- [x] `MyLabel`, `Team`, `Owner`, `label:INSTANCE`, `label:node-age` and
  `kube-node-viewer/node-age` load as labels; `instance-type` as the
  built-in.

## Unchanged behavior

- Lower-case names other than `instance` and the computed labels:
  built-ins, aliases, labels and every message of
  [fix-config-errors](../fix-config-errors/bugfix.md), sibling hints
  included (`sort: PRICE/M`, `columns: [CREATION]`), except `columns:
  [PRICE/M]`, whose message changes (R1; dated note there).
- The owner's kind of config (lower-case built-in names only) loads as it
  did.
- Errors other than YAML decoding keep their text.

Counter-examples: MUST NOT refuse `MyLabel`; MUST NOT refuse
`label:INSTANCE` or `kube-node-viewer/node-age`; MUST NOT drop the line number from a YAML error.

## Trade-off

This is a deliberate breaking change: a config that names a real node
label whose key is a built-in name in another case (`Zone`, `Status`,
`NodeGroup`, `Name`, `IP`, `Price`), the key `instance`, or a label
`node-age` bare is refused and must write `label:Zone`. CHANGELOG lists it
under `Changed`.

- Why: a look-alike empty column or group is far more likely than a node
  label with such a key; Kubernetes and the clouds use lower-case,
  prefixed keys (`topology.kubernetes.io/zone`).
- When wrong: clusters that do carry such labels and configs that name
  them bare; each one gets the error with the `label:` fix on start.
- Reversal: drop `caseOf`, `computedOf`, `prefixCase` and the lower-case
  `isBuiltin` check in `fieldKind.field`; R1's names load as labels again.

## Root cause

`fieldKind.field` (`cmd/kube-node-viewer/configview.go`) compares names
exactly, then reads anything else that is a qualified name as a label key;
the column titles are only matched lower-cased for the sibling hints and
the four `price/h`… aliases. `readSettingsFile` strips the JSON decoding
prefix but not the one `sigs.k8s.io/yaml` puts before go-yaml errors.

## Fix

- `fieldKind.caseOf`: the lower-case form against the key's built-ins,
  aliases and the screen title `instance` (in any case); checked after the
  sibling hints, so their wording wins. `prefixCase` for `label:` and
  `usage:` in another case; `computedOf` for the computed labels.
  `isBuiltin` also on the lower-case form.
- `yamlError` in `settings.go`: strips both prefixes, joins the listed
  errors on one line, and starts an error without a line with `YAML: `.
- `docs/views.md` field names and errors paragraph; CHANGELOG `Changed`
  (R1) and `Fixed` (R2); dated corrections of `config-views` R6 and
  `fix-config-errors`.
