# Requirements: sort-invert

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-02 (code-review agent; spec-refuter not run)

## Context

Flipping the sort between ascending and descending takes three keys today:
`s` to open the Sort panel, `space` (or `enter` on the current row), `esc`.
Comparing the cheapest and the priciest nodes, or the emptiest and fullest,
means doing it over and over. One key on the main screen does it in place.

## Usage example

```text
Sort: price ↓        node list: priciest first
I                    → Sort: price ↑   node list: cheapest first
I                    → Sort: price ↓
```

## Requirements

**R1** WHEN the user presses `I` (shift+i) on the Overview screen with no
panel, search or command line open THEN the system SHALL flip the direction
of the current sort, keeping its field.

- [x] The header shows the new arrow (`↑`/`↓`) and the list redraws in the
  new order, back on the first page.
- [x] Default view (`creation ↓`) → `I` → `creation ↑`.
- [x] A saved view with an empty sort field sorts by creation; `I` flips it.
- [x] The flip is a view change like any other: it is saved for the cluster.
- [x] Nodes without a value for the sort field stay last, and ties still
  break on the node name, as with the Sort panel.

**R2** WHEN `I` is pressed on the Recommendations screen THEN the system SHALL
do nothing.

- [x] The sort is unchanged after `tab`, `I`.

**R3** WHEN a panel, the search, the command line or the view-name prompt is
open THEN the system SHALL treat `I` as that input's key, not as the flip.

- [x] With the Sort or Filters panel open, `I` leaves the sort unchanged.
- [x] In the search, the command line and the view-name prompt, `I` is
  added to the typed text and the sort is unchanged.

**R4** WHEN the user opens the help (`?`) or reads the Overview footer THEN
the system SHALL list `I` with what it does.

- [x] Help lists `I` next to `s`.
- [x] The Overview footer shows `I: flip ↑/↓` right after `s: sort`, so it
  stays visible on narrow windows where the end of the line is cut.

## Counter-examples

- MUST NOT change the order of the groups: groups sort by value with
  `<none>` last, whatever the node sort direction.
- MUST NOT bind lowercase `i`.
- MUST NOT add `I` to the header key box (one more entry adds a row).

## Out of scope

- Reversing the group order; a separate "reverse" flag on top of the sort.
- Keeping the selection on the same node after the flip: the cursor stays on
  the same row, like after any other sort change.
- The Sort panel shows no arrow when the saved field is empty
  (`panels_view.go:188`); pre-existing, not changed here.
