# Design: label-visibility

> Implements [requirements](requirements.md).

## Current state

- Every panel builds its label rows from `filterFieldChoices`
  (`pkg/model/panels.go:299`): the derived `FilterFields`
  (`pkg/model/view.go:69`), then every label key on the nodes, sorted. It is
  called by `filterRows` (`pkg/model/panels.go:142`), `groupChoices`
  (`pkg/model/panels_view.go:44`), `sortChoices`
  (`pkg/model/panels_view.go:123`) and `columnChoices`
  (`pkg/model/panels_view.go:208`).
- `addLabelKeys` (`pkg/model/node.go:610-616`) adds only the node's real
  labels. Computed labels (`kube-node-viewer/node-age` and the usage labels,
  prefix at `pkg/model/node.go:538`) are not node labels: only
  `columnChoices` adds them, by hand (`pkg/model/panels_view.go:221-224`).
  Filters, Group by and Sort do not offer them. A real node label can carry
  the prefix too, for example `kube-node-viewer/instance-price`
  (`pkg/client/controller.go:212`).
- Filters and Group by rebuild their rows on every key
  (`pkg/model/panels.go:142-151`, `pkg/model/panels_view.go:44-52`); Sort and
  Columns take their field list once, when the panel opens
  (`pkg/model/panels_view.go:150`, `:235`).
- A popup has one hint line, and its rows take `height-4`
  (`pkg/model/popup.go:93-95`, `:121-126`).
- The only config file is `~/.kube-node-viewer`, one `key=value` per line,
  parsed by `loadConfigFile` (`cmd/kube-node-viewer/flag.go:157`). Settings
  the TUI needs at start are passed through setters after `NewUIModel`, for
  example `SetRecommendOptions` (`cmd/kube-node-viewer/main.go:85-92`).
- No YAML parser is a direct dependency. `sigs.k8s.io/yaml v1.6.0` (MIT) is
  already in `go.mod` as indirect, through client-go.

## Target state

```text
~/.config/kube-node-viewer/config.yaml ─┐
--labels-allow / --labels-deny ─────────┤ cmd/kube-node-viewer/settings.go
                                         │   loadSettings → model.LabelPolicy
                                         ▼
                              UIModel.SetLabelPolicy(policy)
                                         │
     filterFieldChoices(nodes, policy, inUse, showAll) → panels
```

- `model.LabelPolicy` holds the compiled allow and deny patterns and answers
  `Shows(key string) bool`. Its zero value shows every key (R3).
- `filterFieldChoices` takes the policy, the set of fields the view uses and
  a show-all switch, and only filters real node label keys: a key is listed
  when the view uses it (R5), or show-all is on, or `policy.Shows(key)`. Real
  labels with the `kube-node-viewer/` prefix follow the policy like any other
  label. Computed labels never pass through it, so Columns keeps adding them
  as today. It also returns how many keys it left out (R6), counted as
  distinct keys on the current nodes, recomputed on every frame.
- The fields the view uses are the filter fields (enabled, disabled and
  excluding filters alike), the group-by levels, the sort field and the
  columns of `UIModel.view`. Named views, `--extra-labels`, `--group-by` and
  `--node-sort` all end up in those fields; `Collapsed` holds values, not
  fields. Each panel also records, when it opens, the fields the view used at
  that moment, and counts them as in use until it closes, so a row the owner
  unticks does not vanish under the cursor.
- Each panel struct gets `showAll bool`, false on open; `a` flips it. For
  Sort and Columns `a` rebuilds the snapshot field list and keeps the cursor
  on the same field. Turning `a` off while the Filters panel has a field open
  that would now be hidden closes that field.
- `popup.go` gains an optional status line above the hint; when present the
  rows take `height-5`. It shows `<n> labels hidden by config • a: show all`
  when keys are hidden, `showing all labels • a: hide` while show-all is on,
  and is absent otherwise.
- `cmd/kube-node-viewer` loads the YAML file, applies the flags and calls
  `SetLabelPolicy` before the program starts. A load error goes through
  `log.Fatalf` like `recommendOptions` does today, so it exits 1 before the
  TUI (R8). The message is `config file <path>: <problem>`, with the
  `error unmarshaling JSON: while decoding JSON: json: ` prefix of
  `sigs.k8s.io/yaml` removed.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/labelpolicy.go` (new) | `LabelPolicy`, `NewLabelPolicy(allow, deny []string) (LabelPolicy, error)`, `Shows`, glob match |
| `pkg/model/panels.go` | `filterFieldChoices` filtering, hidden count; Filters panel `showAll`, `a`, footer |
| `pkg/model/panels_view.go` | Group by, Sort, Columns: pass the policy, `showAll`, `a`, footer |
| `pkg/model/popup.go` | optional status line above the hint; rows take `height-5` when present |
| `pkg/model/uimodel.go` | `labelPolicy` field, `SetLabelPolicy`, help (`?`) entry for `a` |
| `cmd/kube-node-viewer/settings.go` (new) | path resolution (R9), strict YAML load (R8) |
| `cmd/kube-node-viewer/flag.go` | `--labels-allow`, `--labels-deny` (R7) |
| `cmd/kube-node-viewer/main.go` | wire the policy |
| `go.mod` | `sigs.k8s.io/yaml` from indirect to direct |
| `README.md`, `CHANGELOG.md` | config file, flags, `a` key |

## Decisions

### D1: A new YAML file, next to the old key=value file

**Choice**: Read `labels.allow` and `labels.deny` from
`$XDG_CONFIG_HOME/kube-node-viewer/config.yaml` with
`sigs.k8s.io/yaml.UnmarshalStrict`, and leave `~/.kube-node-viewer` as it is.

**Justification, strongest first**:
1. The owner asked for YAML lists in `~/.config/kube-node-viewer/`; lists of
   10 patterns read better as YAML items than as one comma-separated line.
2. `sigs.k8s.io/yaml` is already in the module graph, MIT licensed, so the
   binary gains no new module and the license allowlist holds.
3. Strict decoding turns a typo such as `lables:` into an error instead of a
   silent no-op (R8). Measured on v1.6.0: unknown fields, a string where a
   list belongs and duplicate keys are rejected, but list items `123`, `yes`
   and `~` decode into strings (`"123"`, `"true"`, `""`) without error. So the
   lists are decoded as `[]any` and every item that is not a string, or is
   `""`, is rejected with its position (`labels.allow[2]`). `allow: null`
   means no list (R3).
4. Keeping the old file avoids breaking every existing setup in this change.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Two config files with different formats | The YAML file only holds the new lists; moving the rest is a separate, deferred change |
| `sigs.k8s.io/yaml` converts YAML to JSON first, so error positions are by field, not line | Messages still name the file and the field, which is enough for a 10-line file |

**When this would be wrong**: when more options move to the YAML file and
the two files start to disagree, or when users ask for line numbers in
errors.

**Reversal action**: Replace the loader with a `go.yaml.in/yaml/v3` decoder
with `KnownFields(true)` (already indirect) for line numbers; about an hour.
Moving everything to YAML is the deferred task below.

**Alternatives discarded**: TOML — needs a new module. Extending
`~/.kube-node-viewer` with `labels-allow=` lines — not the location or the
list format the owner asked for.

### D2: Only `*`, matching across `/` and `.`

**Choice**: A pattern is literal text where `*` matches any run of
characters, the whole key must match, case-sensitive (R4).

**Justification, strongest first**:
1. Label keys look like `prefix.domain/name`; the useful patterns
   (`*.amazonaws.com/*`, `karpenter.sh/*`) need `*` to cross `.` and `/`,
   which `path.Match` and `filepath.Match` do not do.
2. Treating `?`, `[` and `\` as literal means any real key written in full
   matches itself; nobody has to escape anything.
3. A dozen lines of code with table tests, no regexp compile errors to report.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| No `?` or character classes | Not needed for label keys so far |
| No negation inside allow | The deny list covers it |

**When this would be wrong**: when owners need to pick keys that differ in
the middle in ways `*` cannot express.

**Reversal action**: Add `?` and classes to the matcher; existing patterns
using those characters literally would change meaning, so it needs a
CHANGELOG note.

**Alternatives discarded**: Regular expressions — harder to write for a
label key and easy to get wrong (`.` matches anything). `path.Match` — `*`
stops at `/`.

### D3: The lists only shape the panels, never the view

**Choice**: The policy is applied when panel rows are built; a label the view
uses always stays listed and applied (R5), and computed labels are never
hidden.

**Justification, strongest first**:
1. A saved view or a flag that silently stops working after editing the
   config file would look like a bug in filtering or grouping.
2. The owner must be able to see and remove what the view uses; if it were
   hidden, the only way out would be editing the state file.
3. Keeping it in `pkg/model` panel code leaves `ViewState`, the state file
   and export untouched.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A denied label can still appear in a panel | Only while the view uses it, and removing it makes it disappear |

**When this would be wrong**: if the owner wants deny to be a hard rule that
also drops such labels from saved views.

**Reversal action**: Drop the in-use exemption in `filterFieldChoices` and
remove hidden labels from the view at load; the state file is rewritten on
the next save.

**Alternatives discarded**: Filtering node labels in the informer or store —
would also hide them from details, search and export (counter-examples).

### D4: Flags replace a list, they do not add to it

**Choice**: `--labels-allow` replaces the file's allow list and
`--labels-deny` its deny list, for the run only (R7).

**Justification, strongest first**:
1. Same rule as every other flag here: an explicit flag wins over the config
   file (`AGENTS.md`, D8 of interactive-tui). Unlike the other flags, their
   defaults are not read from `~/.kube-node-viewer` (`flag.go:78-106`); a flag
   applies only when it is in `flags.Explicit` (`flag.go:114-117`), which is
   what tells `--labels-allow ''` apart from no flag. `resolveView` only reads
   `viewKeys` (`cmd/kube-node-viewer/view.go:27`, `:46-54`) and `ViewState` has
   no field for the lists, so neither flag reaches the state file.
2. Replacing makes `--labels-allow ''` a simple way to see every label for
   one run.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| To add one pattern for a run, the whole list must be typed | `a` in a panel covers the quick look |

**When this would be wrong**: if owners routinely want "file list plus one".

**Reversal action**: Merge flag patterns into the file's list; one function.

**Alternatives discarded**: Merging — makes clearing a list from the command
line impossible.

## Invariants

- Requests, never usage: the feature does not touch the totals.
- Flag > config file; neither flag is saved to the state file.
- Derived fields, and the computed labels in the panels that offer them today, are always listed; real node labels follow the lists whatever their prefix.
- A field the view uses is always applied and listed.
- Node details, search and export behave as before.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| No file or no directory | No lists, every label shown | Panels as before |
| YAML syntax error, unknown field, wrong type, empty pattern | Exit 1 before the TUI | Message naming the file and the problem |
| File not readable (permissions) | Exit 1 before the TUI | Message naming the file and the OS error |
| Allow list matches no label | Only derived fields, computed labels and in-use labels listed | Footer count, `a` shows all |
| Pattern hides a label used by a saved view | Label still applied and listed | Nothing changes until the owner removes it |

## Test plan

- `pkg/model/labelpolicy_test.go`: table tests for the matcher (R4) and
  allow/deny combination (R1–R3), including empty lists and the computed
  prefix.
- `pkg/model` panel tests: each panel with a policy, in-use labels (R5),
  footer count and `a` (R6), no policy (R3).
- `cmd/kube-node-viewer/settings_test.go`: `XDG_CONFIG_HOME` set, unset,
  empty, relative (R9); missing file; `allow: null`; invalid YAML, unknown
  field, string instead of list, items `123`, `yes`, `~`, `''`, unreadable
  file (R8); flags replacing lists, `''` clearing one (R7).
- Panel tests for unticking an in-use hidden label (row stays until close),
  `a` on Sort/Columns keeping the cursor, `a` off closing a hidden open
  Filters field, a 10-line window, no nodes yet.
- `scripts/verify.sh` green; coverage of the new files ≥ 90%.
- Manual run against a cluster the owner names, with the owner's real label
  set.
