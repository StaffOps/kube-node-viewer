# Design: search-negation

> Implements [requirements](requirements.md).

## Current state

`newSearchMatcher` (`pkg/model/search.go`) turns the trimmed text into a
literal, words or a compiled regex, marking invalid patterns.
`ViewState.Select` (`pkg/model/view.go`) builds it once and keeps a node
when `Node.contains` finds a match in the name, internal IP, instance type
or a label value. The footer and header ask `searchInvalid(text)`; the help
line is in `helpKeys` (`pkg/model/panels.go`).

## Target state

```go
type searchMatcher struct {
	re       *regexp.Regexp
	words    []string
	invalid  bool
	hasRegex bool
	negate   bool // a leading `!` (R1)
	pairs    bool // the text has `=`: also match key=value (R2)
}
```

`newSearchMatcher(text)`: trim; if it starts with `!`, set `negate` and
trim the rest; an empty rest returns the empty matcher (R4, `negate`
false). `pairs = strings.Contains(rest, "=")`. The rest is then built as
today. `match(s)` is unchanged (one value). `Node.contains(m)` computes
`hit` over the name, IP, type and label values, and when `m.pairs` also over
`k + "=" + v` for each label; it returns `hit != m.negate`. `empty()` stays
"no words and no regex", so `!` alone keeps every node.

`searchInvalid` keeps working: the invalid flag is about the rest.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/search.go` | `negate`, `pairs` in `newSearchMatcher` |
| `pkg/model/view.go` | `Node.contains`: label pairs, negation |
| `pkg/model/panels.go` | `helpKeys` `/` line |
| `pkg/model/search_internal_test.go` | matcher table rows for `!`, `=`; footer/header for `!a(b`; the help-line assertion (R5) |
| `pkg/model/labelpanels_internal_test.go` | hidden labels matched by `key=` |
| `pkg/model/search_test.go` | `Select` cases R1–R4; benchmark row for a pair search |
| `README.md`, `CHANGELOG.md` | keys table, Search section, export paragraph, `Added` |
| `specs/search-regex/requirements.md` | dated note: label keys are searched with `=` now |

## Decisions

### D1: `=` turns on label pairs

**Choice**: only text containing `=` is matched against `key=value`; text
without it is matched as today.

**Justification, strongest first**:
1. Kubernetes rejects `=` in label keys, label values and node names
   (`IsQualifiedName`, `IsValidLabelValue`, `IsDNS1123Subdomain`), so in
   practice a literal with `=` matched nothing before, and a pattern with `=` can only
   change by matching more, since pairs are added and nothing is removed
   (R3).
2. Searching keys always would make `zone`, `node`, `kubernetes` keep every
   node (each has `topology.kubernetes.io/zone`, `kubernetes.io/hostname`),
   so common searches would stop filtering.
3. `key=value` is the syntax of `kubectl -l`, so it reads without a manual;
   `key=` for "has the key" follows from substring matching.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| `gpu=` also matches a key `example.com/no-gpu` (substring) | anchor with a regex: `(^|/)gpu=`; README example `^key=` |
| A pair is matched unanchored: `workload=batch` keeps `other-workload=batch-2` | the same as every other search; `^…$` for exact |
| One string per label per node built while searching with `=` | measured in the benchmark (bound below) |
| `=` written as an escape (`workload\x3dbatch`) does not turn pairs on | the gate is the character `=`; nobody types `\x3d` |

**When this would be wrong**: users expect `/gpu` to find a label key. Then
match keys too, behind a prefix such as `key:`.

**Reversal action**: drop the `pairs` flag (one line in `newSearchMatcher`,
one loop in `contains`). Minutes.

**Alternatives discarded**: always match `key=value` (breaks R3, point 2);
a separate `label:` prefix (a second syntax; `=` is what people type).

### D2: one leading `!` negates the whole search

**Choice**: a `!` at the start of the trimmed text inverts the result of the
rest; one level only; `\!` (a valid RE2 escape) for a literal leading `!`.

**Justification**: `!` reads as "not" in shells, k9s filters and kubectl
selectors (where `!key` means "key absent", a different meaning: README
says to write `!key=` here); one term keeps the matcher one boolean; RE2 has
no negative lookahead, so negation cannot be a regex feature. Kubernetes
rejects `!` in label values and names, so a literal leading `!` loses
nothing.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| `!!x` is not `x` | stated in README; nobody needs it |
| no AND of terms (`a !b`) | filters (`f`) narrow first; out of scope |
| kubectl `!key` and `key!=value` give other results here | README rows; `!key=` and `!key=value` |

**When this would be wrong**: users ask for several terms. Then split on
spaces and AND them, each with its own `!`.

**Reversal action**: drop the `!` branch. Minutes.

**Alternatives discarded**: a `not:` prefix (longer, a word to remember);
`-x` as search engines do (people search fragments that start with `-`,
such as `-spot`, `-1`, `-worker`); reading `key!=value` as kubectl does (a second operator for one
term).

### D3: no footer change

**Choice**: footer while typing stays `type to search (regex) • …`; the
header shows `/!text` as typed.

**Justification**: the footer is already long, and the header shows the
text with its `!`; help and README explain it.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| nothing on the footer says the search is negated | the typed `!` is on the footer line itself while typing |

**Alternatives discarded**: `• not` on the footer while negated (the text
being typed already starts with `!`).

**When this would be wrong**: users miss that `!` negates while typing.

**Reversal action**: add `• !: not` to the negated footer in `helpText`.
Minutes.

## Invariants

- An empty, whitespace-only or `!`-only search keeps every node.
- A search without `!` and without `=` keeps the same nodes as before.
- For `x` non-empty after trimming and not starting with `!`, `!x` keeps
  exactly the nodes that `x` does not, among those the filters keep.
- The search text is never saved.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| `!` alone, mid-typing | every node | the list is unchanged until the next character |
| `!.*`, `!a|` (rest matches everything) | no node | `No nodes match the filters or search.` |
| `!` + invalid pattern | the rest as text, negated | footer `invalid regex: as text`, header `(as text)` |
| `key = value` with spaces | spaces count, matches nothing | no node; README says no spaces |
| kubectl `!key` (no `=`) | values searched, negated: most nodes kept | README: write `!key=` |
| kubectl `key!=value` | literal text, no node | `No nodes match…`; README: write `!key=value` |
| `=` alone | every pair contains `=`: every node with at least one label | the list barely changes |
| `!=` (kubectl habit) | only nodes with no labels | usually `No nodes match…` |
| `!`, `!   ` and `` give different recommendation cache keys | a recompute, same result | nothing |

## Performance

`BenchmarkSelectSearch` gets a pair search (`nomatch=x`). Bound: within 3×
of the plain substring row, since each label adds one string build and
lowercase [ASSUMPTION: the review's scratch copy measured about 2.8×; T1
measures it in the real benchmark]. Over it: match key and value separately for literal words
(split the word at `=`) instead of building `k=v`.

## Test plan

- Matcher table rows: `!x` sets `negate`, `!` alone is empty, `\!x`
  matches `!x`, `a=b` sets `pairs`, `!a(b` invalid and negated.
- `Select` table on made-up nodes with `workload`, `example.com/gpu`,
  `sub.example.com/gpu`, `topology.kubernetes.io/zone` labels and one node
  with no labels: every R1–R4 criterion, plus `!!x`, `a!b`,
  `workload!=batch`, `!example.com/gpu`, `=`, `!=`, an empty node list.
- A hidden label (label-visibility) matched by `key=` and `!key=`.
- Key-driven footer and header test for `!a(b`; help line test.
- A run on a cluster (T3): a local throwaway kind cluster.
