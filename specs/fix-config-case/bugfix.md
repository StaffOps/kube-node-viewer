# Bugfix: config-case

> Status: draft · Reported: 2026-11-04 · Reviewed: —

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

**R2** WHEN `config.yaml` cannot be decoded as YAML (a syntax error, or a
key written twice in one map) THEN the system SHALL report it after the
file name on one line, without the YAML library's `error converting YAML
to JSON: yaml: ` and `unmarshal errors:` prefixes, each error with its line
number and several joined with `; `.

- [x] `line 3: key "name" already set in map`.

**R3** WHEN an upper-case name is not a built-in in any case THEN the system
SHALL keep reading it as a label key, as before.

- [x] `MyLabel`, `Team`, `Owner` and `label:INSTANCE` load as labels;
  `instance-type` as the built-in.

## Unchanged behavior

- Lower-case names: built-ins, aliases, labels and every message of
  [fix-config-errors](../fix-config-errors/bugfix.md), sibling hints
  included (`sort: PRICE/M`, `columns: [CREATION]`).
- The owner's kind of config (lower-case built-in names only) loads as it
  did.
- Errors other than YAML decoding keep their text.

Counter-examples: MUST NOT refuse `MyLabel`; MUST NOT refuse
`label:INSTANCE`; MUST NOT drop the line number from a YAML error.

## Root cause

`fieldKind.field` (`cmd/kube-node-viewer/configview.go`) compares names
exactly, then reads anything else that is a qualified name as a label key;
the column titles are only matched lower-cased for the sibling hints and
the four `price/h`… aliases. `readSettingsFile` strips the JSON decoding
prefix but not the one `sigs.k8s.io/yaml` puts before go-yaml errors.

## Fix

- `fieldKind.caseOf`: the lower-case form against the key's built-ins,
  aliases and the screen title `instance`; checked after the sibling hints,
  so their wording wins. `isBuiltin` also on the lower-case form.
- `yamlError` in `settings.go`: strips both prefixes and joins the listed
  errors on one line.
- `docs/views.md` errors paragraph; CHANGELOG `Fixed`; dated correction of
  `config-views` R6.
