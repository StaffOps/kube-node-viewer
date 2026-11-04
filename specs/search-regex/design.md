# Design: search-regex

> Implements [requirements](requirements.md).

## Current state

`ViewState.Select` (`pkg/model/view.go:329`) lowercases and trims
`ViewState.Search` and keeps a node when `Node.contains` finds it as a
substring of the lowercased name, internal IP, instance type or a label
value (`view.go:352-366`). `Select` runs for the node list
(`layout.go:69`), the recommendations snapshot (`recommendations.go:84`,
`:197`) and the export (`export.go:114`). `searchKey`
(`pkg/model/panels_view.go:373`) edits the text, and every key re-renders.
While typing, `helpText` shows `type to search • enter: keep • esc: clear`
(`layout.go:227`); after `enter`, the header shows ` · /<text>`
(`header.go:273`). The text is not saved (`views.go:184`, `json:"-"` at
`view.go:235`); the export writes it as `view.search` (`export.go:125`);
the recommendations cache keys on it (`recommendations.go:56`).

## Target state

A `searchMatcher` (new, `pkg/model/search.go`) built from the trimmed text:

```go
type searchMatcher struct {
	re       *regexp.Regexp // nil: words
	words    []string       // lowercased; any of them, as substrings
	invalid  bool           // had regex characters but did not compile
	hasRegex bool           // the text has a regex character
}

func newSearchMatcher(text string) searchMatcher
func (m searchMatcher) match(s string) bool
```

`newSearchMatcher`: text with none of `|()[]{}^$*+?\` → literal (today's
behavior, D1). Words joined by `|` with no other regex character → those
words as substrings (D3). Otherwise `regexp.Compile("(?i)" + text)`; on error →
literal with `invalid = true` (D2). `Select` builds it once per call and
passes it to `Node.contains`. `helpText` and the header call it to show the
invalid note. Export and the recommendations key keep the raw text: the
same text gives the same nodes, so the key stays right.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/search.go` (new) | `searchMatcher`, `newSearchMatcher` |
| `pkg/model/view.go` | `Select` builds the matcher; `Node.contains(m searchMatcher)` |
| `pkg/model/layout.go` | `helpText` while searching: `(regex)`, invalid note |
| `pkg/model/header.go` | ` (as text)` after an invalid search |
| `pkg/model/panels.go` | `helpKeys` `/` line |
| `pkg/model/search_internal_test.go` (new) | matcher table (literal, regex, invalid), footer and header |
| `pkg/model/search_test.go` (new) | `Select` with its own test data: workload labels, `node-1`/`node-12`, a dashed name next to a dotted IP (R1, R2); benchmark |
| `pkg/model/export.go`, `recommendations.go` | unchanged (raw text) |
| `README.md`, `CHANGELOG.md` | keys table, search and export paragraphs, `Changed` entry |

## Decisions

### D1: regex only when the text has a regex character other than `.`

**Choice**: text made only of characters with no regex meaning, plus `.`,
is matched literally; anything with `| ( ) [ ] { } ^ $ * + ? \` is a Go RE2
expression, case-insensitive (`(?i)`) and unanchored.

**Justification, strongest first**:
1. `.` is in almost every search people type today (IPs, `m5.large`,
   dotted host names and label values). As a regex, `203.0.113.5`
   also keeps a node named `ip-203-0-113-5.example.com` (checked with Go's
   `regexp`); keeping `.` literal keeps every such search as it is.
2. The other characters are rare in node names, IPs, instance types and
   label values (Kubernetes allows `[A-Za-z0-9._-]` in label values and
   names), so their presence says the user means a pattern.
3. RE2 runs in linear time: a pattern typed by hand cannot hang the UI.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Inside a pattern, `.` is any character (`203.0.113.5|x` is wide) | standard regex meaning; README says to write `\.` |
| Two modes to explain | one sentence in help and README; plain text needs no change of habit |

**When this would be wrong**: users search `.` expecting "any character"
with no other regex character, or label values start carrying `|`, `(`,
`$` as ordinary text.

**Reversal action**: drop the literal check in `newSearchMatcher` (one
`if`) so every text is a regex; tests in `search_internal_test.go` change.
Minutes.

**Alternatives discarded**: always regex — breaks dotted searches (point
1); split on `|` and OR substrings — no `^`, `$`, groups, a second syntax;
glob — no alternation.

### D2: an invalid pattern is matched as literal text, and marked

**Choice**: on a compile error, match the text as a literal and show
`invalid regex: as text` in the footer while typing and ` (as text)` after
the header's `/<text>`.

**Justification, strongest first**:
1. The list filters on every keystroke, so `a(` is on screen before `a(b)`
   is finished; an empty list or an error there would flicker while typing.
2. Literal matching is the old behavior, so nothing is lost; the mark says
   why `(` did not group.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A typo in a pattern silently searches literal text | the footer and header say so |

**When this would be wrong**: users want a broken pattern to show nothing,
to notice the typo.

**Reversal action**: return a matcher that matches nothing on error; the
marks stay. Minutes.

**Alternatives discarded**: keep the last valid pattern's result (the list
does not follow the text on screen); an error popup (blocks typing).

### D3: compile once per `Select`, no cache

**Choice**: build the matcher at the top of `Select` (and once in
`helpText` and the header).

**Justification**: a cache keyed on the text would add state to
`ViewState`, which is copied by value. Compile cost: unmeasured; the T1
benchmark measures `Select` at 5,000 nodes × 30 labels, regex against
substring.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| a few compiles per frame | short patterns; the benchmark decides |

**When this would be wrong**: the benchmark shows `Select` with a regex
more than 2× slower than the substring at that size, or above 5 ms.

**Reversal action**: the matching, not the compile, is the cost, so:
first match the regex against the name, IP and type and only then the
labels (already the order); if still over the bound, give the regex path a
literal prefilter (`regexp.LiteralPrefix`, skip values without it) and, as
the last step, cache the kept node set per text and store generation in
`UIModel`. Hours.

**Alternatives discarded**: compile in `searchKey` and store it in
`ViewState` — a pointer in a value type that is copied, compared and
written to JSON.

> **Corrected 2026-11-04 (T1 benchmark).** `BenchmarkSelectSearch`, 5,000
> nodes × 30 labels, Apple M-series laptop: substring `nomatch` 6.4 ms (so
> the 5 ms bound above was already exceeded by today's search: the bound
> is now "within 2× of the substring"); a plain compiled `nomatch|other`
> 65 ms, over it. The reversal's first steps were taken: a pattern made
> only of words joined by `|` (no other regex character, no `.`) is matched
> as those words (`searchMatcher.words`), 8.0 ms; `^nomatch$` 7.1 ms; a
> group such as `nomatch\.(a|b)` stays a regex, 35 ms (5.5×), accepted: it
> is ~3.5 ms at 500 nodes, and every other pattern is within the bound.
> A joined-values prefilter was tried and dropped (65 ms, no gain).

## Invariants

- The search filters; it never changes totals of the nodes it keeps,
  requests, prices or the filters.
- The search text is never saved with a view or the state file.
- An empty or whitespace-only search keeps every node.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Pattern does not compile | literal match | footer `invalid regex: as text`; header ` (as text)` |
| Pattern matches the empty string (`a|`, `()`, `.*`), e.g. mid-typing `a|` | every node kept, like an empty search | the list shows all nodes until the next character |
| Pattern matches nothing | `No nodes match the filters or search.` (today's message) | the message |
| Spaces inside a pattern (`search | batch`) | they count: `"search "` or `" batch"`; only the ends are trimmed | fewer nodes than expected; README says to write `search|batch` |

## Test plan

- Table tests for `newSearchMatcher` (literal, regex, invalid, case,
  `(?-i)`, anchors, empty alternative) and for `Select` with the R1–R3
  data, including the dashed name next to the dotted IP.
- Key-driven tests: footer while typing (valid and invalid), header after
  `enter`, help line.
- `BenchmarkSelectSearch`: 5,000 nodes × 30 labels, substring vs regex.
- A run on a cluster the owner allows (T4).
