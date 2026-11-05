# Requirements: search-negation

> Status: shipped (v0.14.0, 2026-11-05) · Owner: Carlos Gomes · Reviewed: 2026-11-04 (spec-refuter, two rounds; findings applied)

## Context

The `/` search ([search-regex](../search-regex/requirements.md)) keeps the
nodes whose name, internal IP, instance type or a label value matches the
text. Two questions it cannot answer: "every node but the system ones"
(there is no way to say *not*) and "the nodes that have label X" (label
keys are not searched, and a value alone is ambiguous: `batch` can be a
node pool, a team or a workload). Both were left out of search-regex and
listed in `TODO.md`.

## Usage examples

```text
/!system                        the nodes where nothing contains "system"
/!^system|^infra                neither pool: negation of a regex
/workload=batch                 nodes with a label pair containing "workload=batch"
/^workload=batch$               exactly the label workload=batch
/karpenter.sh/nodepool=         nodes that have the karpenter.sh/nodepool label
/!karpenter.sh/nodepool=        nodes that do not (not managed by Karpenter)
/!                              every node: nothing to negate yet
/\!x                            the literal text "!x" (a regex escape)
/!a(b                           nodes not containing the literal "a(b"; header /!a(b (as text)
```

## Requirements

**R1** WHEN the trimmed search text starts with `!` and something other
than spaces follows THEN the system SHALL keep the nodes for which the rest
(trimmed) matches none of the searched values, the rest being matched
exactly as a search without `!` would match it (literal, regex or invalid,
search-regex R1–R3; label pairs, R2).

- [x] `!search` on nodes labelled `workload=search` and `workload=batch`
  keeps only the `batch` node.
- [x] `!search|batch` keeps neither of them and keeps a `web` node.
- [x] `! search` (space after `!`) is the same as `!search`.
- [x] `!a(b` matches `a(b` as text and keeps the nodes not containing it;
  the footer and header mark it as text, as for `a(b`.
- [x] Filters (`f`) still apply: the search only removes nodes from what
  they keep.

**R2** WHEN the search text (after a leading `!`, if any) contains `=`
THEN the system SHALL also match it against each label of the node written
as `key=value`, besides the name, internal IP, instance type and label
values.

- [x] `workload=batch` keeps the node labelled `workload=batch`, not the
  one labelled `workload=search`.
- [x] `^workload=batch$` keeps the `workload=batch` node and not one
  labelled `workload=batch-2`.
- [x] `example.com/gpu=` keeps every node with that label key, whatever
  its value (also empty), and not a node whose keys do not end in
  `example.com/gpu` (substring: a key `sub.example.com/gpu` also matches;
  README says `^key=` for an exact key).
- [x] `!example.com/gpu=` keeps the nodes without that key.
- [x] `team=a|batch` keeps a node labelled `team=a` and one with the value
  `batch`.
- [x] `key=` and `!key=` also see labels the config file hides from the
  panels (label-visibility: the search matches every label).
- [x] A node with no labels: `example.com/gpu=` drops it, `!example.com/gpu=`
  keeps it.

**R3** WHEN the trimmed search text has no `=` and does not start with `!`
THEN the system SHALL NOT match label keys: plain and regex searches keep
the same nodes as before this change.

- [x] On nodes labelled `topology.kubernetes.io/zone` whose name, IP, type
  and label values do not contain `zone`, `zone` keeps no node.
- [x] The search-regex selection and matcher tests (`TestSearchRegex`,
  `TestSearchMatcher`, `TestSearch` in `view_test.go`) pass unchanged; only
  the help-line assertion changes, to R5.

**R4** WHEN the trimmed search text is `!` alone THEN the system SHALL keep
every node, as an empty search does.

- [x] `!` and `!   ` keep every node.

**R5** WHEN the user opens the help (`?`) or reads the README THEN the
system SHALL say how to negate and how to search a label pair.

- [x] Help: `/` — `search name, IP, type, labels (regex, any case; !x: not x; key=value); esc clears`
  ("instance type" shortened to "type" so the line stays within the
  longest help entry, 84 characters).
- [x] README keys table and Search section: the `!` and `key=value` rows of
  the examples table, `\!` for a leading literal `!`, that only one
  leading `!` negates, `^key=` for an exact key, and the kubectl
  differences: kubectl's `!key` is close to `!key=` here and `key!=value`
  to `!key=value` (substrings); exact: `!^key=` and `!^key=value$`, with
  `.` written `\.`.
- [ ] ~~README export paragraph: the JSON `search` is the text as typed,
  `!` and `=` included.~~

> **Corrected 2026-11-04.** The JSON export writes no `view.search` at all
> (`ViewState.Search` is `json:"-"`), found in the T3 run; an existing bug,
> listed in `TODO.md`, so the export paragraph is left as it is.
>
> **Corrected 2026-11-05.** Fixed by [`fix-export-search`](../fix-export-search/bugfix.md):
> the export writes `view.search`, `!` included.

> **Corrected 2026-11-04 (diff review).** R2: with `=` anywhere in the
> text, every `|` alternative is matched against the `key=value` text too,
> so alternatives match label keys: `team=a|zone` keeps every node with a
> `topology.kubernetes.io/zone` label. Kept on purpose (one rule) and said
> in README. R3: "pass unchanged" means the existing assertions; the
> matcher table gained rows and the empty-text list gained `!`. R5's export
> paragraph now says the JSON has no search text yet (the existing bug in
> `TODO.md`).

## Counter-examples

- MUST NOT negate more than once: `!!x` negates the literal text `!x`
  (`!` is not a regex character), it does not mean `x`.
- MUST NOT treat `!` other than at the start as negation (`a!b` is text,
  and so is `workload!=batch`, which matches nothing: README).
- MUST NOT read `!key` without `=` as "the key is absent", as kubectl does:
  with no `=` it negates a search of values (`!example.com/gpu` keeps nodes
  that have that key, if no value contains the text); README says to write
  `!key=`.
- MUST NOT make `!` alone, or an empty search, keep fewer nodes than no
  search.
- MUST NOT save the search text with the view or the state file.
- MUST NOT search pods, annotations or taints.

## Out of scope

- Combining several terms with AND (`a !b`): one search, one term.
- Matching label keys without `=` (`/gpu` finding the key `example.com/gpu`):
  R3 keeps today's results for every search people type.
- A footer note while negating: the header shows `/!text` as typed.
