# Bugfix: export-search

> Status: active · Reported: 2026-11-04 · Reviewed: 2026-11-05 (spec-refuter; 1 HIGH, 5 MEDIUM, 4 LOW, applied) · 2026-11-05 (spec-refuter, code 2026-11-05; 5 LOW, applied)

Found in the [`search-negation`](../search-negation/) cluster run
(2026-11-04) and listed in `TODO.md`.

## Current behavior

Reproduced on 2026-11-04 with `exportDoc` and `json.Marshal` on the demo
cluster with the search `!node-a`: the `"view"` object is
`{"sort":…,"resources":…,"columns":…,"showNodes":true}`, with no
`search` key, although only the nodes the search keeps are exported.

[`search-regex`](../search-regex/requirements.md) R4 promised a
`view.search` field in the export section of the README; the README now
says "without the search text for now". With `!` negation an export cannot
tell whether its nodes are the ones a text matched or the ones it did not.

## Expected behavior

**R1** WHEN the JSON export is written while the search text is not empty
THEN the system SHALL write it as `view.search`, the text exactly as typed:
a leading `!`, surrounding spaces and an invalid pattern included. An
invalid pattern is written raw, without the header's ` (as text)` mark, on
purpose: the mark is display, not part of the text.

- [x] `/node-a` → `"view": {…, "search": "node-a"}`.
- [x] `/!node-a` → `"search": "!node-a"`.
- [x] `/a(b` (matched as text) → `"search": "a(b"`.
- [x] `/ node-a ` → `"search": " node-a "`.
- [x] A text that keeps every node is still written as typed, as the header
  shows it (`header.go`, `u.view.Search != ""`): `/!` → `"search": "!"`,
  two spaces → `"search": "  "`.

**R2** WHEN the JSON export is written with an empty search THEN the system
SHALL leave `search` out of `view`.

- [x] No search → `view` has no `search` key, and the document is the same
  as today's.

**R3** WHEN a view is saved to the state file, as a cluster's view or as a
named view, THEN the system SHALL NOT save the search text.

- [x] The raw `state.json` written after a search has no `search` key, for
  the cluster's view and for a named view saved while a search was set.

## Unchanged behavior

- Which nodes are exported (filters, search, focus): `/!node-a` exports the
  nodes without `node-a`, as today.
- The CSV: no search column.
- The existing `view` keys keep their names and order; `search` comes last.
- The rest of the document (`focus`, totals, `labelColumns`, `nodes`).
- `ViewState`'s JSON (the state file and saved views): `Search` stays
  `json:"-"`.
- views-and-export: the search is not exported as a filter.

## Root cause

`exportDoc.View` is a `ViewState` (`pkg/model/export.go`), whose `Search`
field is tagged `json:"-"` (`pkg/model/view.go`) so the state file never
saves it. `exportDoc` sets `doc.View.Search` and the encoder drops it.

## Fix

`exportDoc.View` becomes an `exportView`: a struct embedding `ViewState`
and adding `Search string \`json:"search,omitempty"\``. `encoding/json`
promotes the embedded fields in their order and the outer `Search` wins
over the inner one (which `json:"-"` already hides), so the existing keys
are unchanged and `search` follows them.

- Why not a top-level `search` key: search-regex R4 promised `view.search`,
  and the search is part of the view the export describes.
- Why not a custom `MarshalJSON` on `ViewState`: the state file uses the
  same type and must never write the search (R3); a method would have to
  tell the two apart.
- When wrong: if `ViewState` ever gains a `MarshalJSON`, embedding would
  promote it and `search` would vanish again; R1's test catches that.
- Reversal: change `exportDoc.View` back to `ViewState`.

## Counter-examples

- MUST NOT save the search text in the state file or in a saved view.
- MUST NOT trim or normalise the text written.
