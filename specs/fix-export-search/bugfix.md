# Bugfix: export-search

> Status: draft · Reported: 2026-11-04 · Reviewed: —

Found in the [`search-negation`](../search-negation/) cluster run
(2026-11-04) and listed in `TODO.md`.

## Current behavior

Reproduced on 2026-11-04 with `exportDoc` and `json.Marshal` on the demo
cluster with the search `!node-a`: the `"view"` object is
`{"sort":…,"resources":…,"columns":…,"showNodes":true}`, with no
`search` key, although only the nodes the search keeps are exported.

Cause: `exportDoc.View` is a `ViewState` (`pkg/model/export.go`), whose
`Search` field is tagged `json:"-"` (`pkg/model/view.go`) so the state
file never saves it. `exportDoc` sets `doc.View.Search` and the encoder
drops it.

[`search-regex`](../search-regex/requirements.md) R4 promised a
`view.search` field in the export section of the README; the README now
says "without the search text for now". With `!` negation an export cannot
tell whether its nodes are the ones a text matched or the ones it did not.

## Expected behavior

**R1** WHEN the JSON export is written while a search is set THEN the
system SHALL write it as `view.search`, the text exactly as typed (a
leading `!`, an invalid pattern and surrounding spaces included).

- [ ] `/node-a` → `"view": {…, "search": "node-a"}`.
- [ ] `/!node-a` → `"search": "!node-a"`; the nodes are the ones without
  `node-a`.
- [ ] `/a(b` (matched as text) → `"search": "a(b"`.

**R2** WHEN the JSON export is written with no search THEN the system
SHALL leave `search` out of `view`, and WHEN the view is saved to the
state file THEN the system SHALL still not save the search.

- [ ] No search → `view` has no `search` key.
- [ ] The state file written after a search has no `search` key (as
  today).

## Unchanged

- The CSV: no search column.
- The other `view` keys and the rest of the document (`focus`, totals,
  `labelColumns`, `nodes`).
- Which nodes are exported (filters, search, focus).
- `ViewState`'s JSON (the state file and saved views): `Search` stays
  `json:"-"`.
- views-and-export: the search is not exported as a filter.

## Counter-examples

- MUST NOT save the search text in the state file or in a saved view.
- MUST NOT trim or normalise the text written.
