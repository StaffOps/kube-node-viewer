# Requirements: search-regex

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-04 (spec-refuter, two rounds)

## Context

The `/` search keeps the nodes whose name, internal IP, instance type or a
label value contains the typed text (case-insensitive substring,
`pkg/model/view.go:352`). One search holds one string only: keeping the
nodes of two workloads, `/search|batch`, keeps none, because no node
contains the literal text `search|batch`. Tools used next to this one (k9s
`/`, `kubectl get … | grep -E`) take a regular expression there.

## Usage example

```text
/batch            contains "batch" (as today)
/203.0.113.5      contains the literal text "203.0.113.5" (as today: no regex character but ".")
/search|batch     matches either word
/^node-1$         the node named node-1, not node-12
/m7g\.(large|xl)  m7g.large and m7g.xlarge, not m7g.medium
/a(b              not a valid regular expression: matched as the literal text "a(b",
                  and the footer and header say so
```

## Requirements

**R1** WHEN the search text holds a regular-expression character other than
`.` (one of `| ( ) [ ] { } ^ $ * + ? \`) and is a valid Go RE2 expression
THEN the system SHALL keep the nodes for which it matches, anywhere and
ignoring case, the node name, the internal IP, the instance type or any
label value.

- [x] `search|batch` keeps a node labelled `workload=search` and one
  labelled `workload=batch`, and no other.
- [x] `^node-1$` keeps `node-1` and not `node-12`.
- [x] `m7g\.(large|xl)` keeps `m7g.large` and `m7g.xlarge` nodes, not
  `m7g.medium`.
- [x] Matching ignores case: `BATCH|x` keeps the `batch` node; `(?-i)BATCH|x`
  does not.
- [x] It is not anchored: `atc|x` keeps the `batch` node.

**R2** WHEN the search text holds no regular-expression character other than
`.` THEN the system SHALL match it as literal text, ignoring case, as before
this change.

- [x] `203.0.113.5` keeps the node with IP `203.0.113.5` and not a node
  named `ip-203-0-113-5.example.com` (which a regex `.` would keep).
- [x] Every existing search test passes unchanged.

**R3** WHEN the search text holds a regular-expression character but is not
a valid expression (`a(b`, `[`, `*a`, a trailing `\`) THEN the system SHALL
match it as literal text, and SHALL say so.

- [x] `a(b` keeps the nodes containing the literal `a(b`; no error and no
  popup.
- [x] While typing, the footer reads `type to search (regex) • invalid
  regex: as text • enter: keep • esc: clear`; otherwise `type to search
  (regex) • enter: keep • esc: clear`. `(regex)` says the search accepts
  one, also while the text is still plain (R2); kept short on purpose.
- [x] After `enter`, the header's search part reads `/a(b (as text)`; a
  valid or plain search reads `/<text>` as today.

**R4** WHEN the user opens the help (`?`) or reads the README THEN the
system SHALL say the search takes a regular expression.

- [x] Help: `/` — `search name, IP, instance type, labels (regex, any case); esc clears`.
- [x] README keys table, the search paragraph and the export section (the
  `view.search` field) say the same, with one `a|b` example and how to
  escape (`\.`).

> **Corrected 2026-11-05.** The export never wrote `view.search`: the
> field was dropped by `json:"-"` on `ViewState.Search`, so the box above
> held for the README text only. Fixed by
> [`fix-export-search`](../fix-export-search/bugfix.md).

> **Corrected 2026-11-04.** The header mark ` (as text)` follows the text,
> so it shows while typing an invalid pattern too, not only after `enter`.
>
> **Corrected 2026-11-04.** R3 listed `x{2` as invalid; Go's RE2 reads an
> unfinished `{` as a literal, so `x{2` is a valid pattern matching `x{2`.
> Replaced by `*a` (repeat with nothing to repeat).

> **Corrected 2026-11-04.** [search-negation](../search-negation/requirements.md)
> adds a leading `!` (negation) and, for text with `=`, label keys as
> `key=value`; the counter-example "not label keys" and the out-of-scope
> negation item below are superseded there. Searches without `!` or `=`
> are unchanged.

## Counter-examples

- MUST NOT change what is searched (name, internal IP, instance type, label
  values; not label keys, not pods).
- MUST NOT save the search text with the view or the state file (it is not
  saved today).
- MUST NOT make an empty or whitespace-only search keep fewer nodes than no
  search.
- MUST NOT turn an invalid pattern into an empty list or an error popup.
- A valid pattern that matches the empty string (`a|`, `()`, `.*`) keeps
  every node, like an empty search: accepted, not special-cased (design,
  Failure modes).

## Out of scope

- Negation (`!text`) and searching label keys: one TODO.md item.
- Regex in the Filters panel (`f`): filters pick values from a list.
