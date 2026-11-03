# Design: sort-invert

> Implements [requirements](requirements.md).

## Current state

The direction is `SortSpec.Descending` (`pkg/model/view.go:218`). Only the
Sort panel changes it: `space`, or `enter` on the current field
(`pkg/model/panels_view.go:164-180`). `SortSpec.apply` sorts by creation
when the field is empty and keeps nodes without a value last in both
directions (`view.go:375-415`). The header draws the arrow from
`Descending` (`header.go:286-292`).

## Target state

`handleKey` (`pkg/model/uimodel.go`) gets `case "I"`: copy the view, negate
`Sort.Descending`, `setView`. `setView` resets the page and reports the
change for saving, as the Sort panel does. Panels, search, command line and
the view-name prompt get keys before the main switch, so R3 needs no code.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/uimodel.go` | `case "I"` in the main switch; `I` added to the keys the Recommendations screen swallows |
| `pkg/model/layout.go` | Overview footer: `I: flip ↑/↓` after `s: sort` |
| `pkg/model/panels.go` | `helpKeys`: `I` after `s` |
| `pkg/model/header.go` | unchanged (header key box stays at 10 entries) |
| `README.md`, `CHANGELOG.md` | keys table, Sort section, Unreleased entry |

## Decisions

### D1: `I` flips `SortSpec.Descending`, the same as `space` in the Sort panel

**Choice**: one key on the main screen negates the existing direction flag.

**Justification, strongest first**:
1. One state, one meaning of "direction". The Sort panel, the header arrow,
   saved state and named views already agree on `Descending`; a second
   "reversed" flag could make them disagree (`price ↓` shown over an
   ascending list).
2. No state format change.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Nodes without a value stay last, so `I` is not a literal reversal of the screen | It is the existing sort contract (README, Sort) and keeps `<none>` rows out of the way |

**When this would be wrong**: users expect `I` to reverse the screen
literally, missing values or group order included.

**Reversal action**: remove the `case "I"` and the help and footer entries;
nothing persisted changes.

**Alternatives discarded**: a separate `Reversed` flag — see justification 1;
`space` on the main screen — it already folds groups (`uimodel.go:616-620`).

### D2: Uppercase `I`, not `i`

**Choice**: shift+i.

**Justification, strongest first**:
1. The owner asked for shift+i.
2. Panels already use shifted letters (`K`/`J`) for secondary actions;
   lowercase `i` stays free.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Needs shift | A flip is not typed in bursts |

**When this would be wrong**: a terminal or layout cannot send shift+i as
`I`. Bubble Tea reports it as the rune `I` (`KeyRunes`).

**Reversal action**: rebind the case to another key.

## Invariants

- Nodes without a value last in both directions; ties on the node name.
- Group order unaffected by the node sort direction.
- Only `Sort.Descending` changes; the field is kept.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Pasted text containing `I` | Bubble Tea delivers a paste as one `[...]` key, which matches nothing | nothing happens |

## Test plan

- Key-driven tests in `pkg/model`: flip both ways from a named sort and from
  the default; empty field; page reset; save callback; no-op on
  Recommendations; no-op with Sort and Filters panels open; typed as text in
  search, command line and view-name prompt; group order unchanged; missing
  values stay last; help and footer text (width 0) with `I` after `s`.
- `scripts/verify.sh`. The binary was not run against a cluster unless the
  owner names one.
