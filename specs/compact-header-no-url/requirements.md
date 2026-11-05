# Requirements: compact-header-no-url

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-05 (spec-refuter; 3 HIGH, 6 MEDIUM, 5 LOW, applied)

## Context

The compact header (`ctrl+e`, or a window from 14 to 27 rows high) is a
two-line box: the cluster and the counts on the first line, the view on the
second. Its first line repeats the API server URL from the full panel
(`pkg/model/header.go:177-189`). The URL is the widest field: 52 characters
for the demo fixture's EKS endpoint, about 72 for a real one (`https://`, a
32-character cluster id, `.gr7.<region>.eks.amazonaws.com`). The box clips
each line to the window (`MaxWidth`, `header.go:230`), so on a normal window
the URL pushes the cost off the right edge. The owner asked to drop it from
the compact box. The full panel keeps its `Cluster:` row, so the URL is one
`ctrl+e` away.

## Usage examples

Rendered from the screenshot fixture (`pkg/model/screenshots_test.go`):

```text
(full panel, unchanged)
 Context:  demo-prod
 Cluster:  https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com
 Region:   us-east-1
 Nodes:    10 of 10 shown · 220 pods · $728.102/month (+1 reserved, not priced) (global cluster)

(compact box, before)
 demo-prod · https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com · us-east-1 · 10 of 10 shown · 220 pods · $728.102/month (+1 reserved, not priced) (global cluster)
 Filters: none │ Group: none │ Sort: creation ↓ │ cpu+memory

(compact box, after)
 demo-prod · us-east-1 · 10 of 10 shown · 220 pods · $728.102/month (+1 reserved, not priced) (global cluster)
 Filters: none │ Group: none │ Sort: creation ↓ │ cpu+memory

(compact box, no region known: no region in the URL and no node region label)
 demo-prod · 10 of 10 shown · 220 pods · $728.102/month (+1 reserved, not priced) (global cluster)
```

## Requirements

**R1** WHEN the header is the compact box (picked with `ctrl+e`, or chosen
because the window is 14 to 27 rows high and nothing was picked) THEN its
first line SHALL be the context name, then the pricing region when one is
known, then the same counts-and-cost text as the full panel's `Nodes:` row
(node and pod counts, ` (N pending)`, and the cost as shown there:
`$…/month`, `>= $…`, ` (+N reserved, not priced)`, ` (global cluster)`,
`prices unknown`, or nothing with pricing off), separated by ` · `, and
SHALL NOT show the API server URL.

- [x] With a known server URL and region, the compact first line is
  `<context> · <region> · <counts and cost>` and contains neither the server
  URL nor its host.
- [x] With no region (none in the URL, no node region label), the compact
  first line is `<context> · <counts and cost>`, the cost still shown.
- [x] The same holds whether the box was picked with `ctrl+e` on a tall
  window or chosen at about 20 rows with no pick.
- [x] The second line (the view) and the issue line, when there is one, are
  unchanged.

_Test: `// compact-header-no-url R1` on `TestCompactHeaderHasNoServerURL`
and `TestHeaderPanelAndOneLineFallback` (pkg/model), and the golden screen
`header-compact.txt`._

**R2** WHEN the header is the full panel or the one-line header THEN it SHALL
be unchanged: the full panel keeps its `Cluster:` row with the URL (or `-`),
and the one-line header (below 14 rows) keeps showing no URL.

- [x] The full panel's `Cluster:` row still shows the server URL.
- [x] The one-line header (below 14 rows) does not contain the server URL.
- [x] The golden screens of the full panel are byte-identical.

_Test: `// compact-header-no-url R2` on `TestFullHeaderKeepsServerURL`
(pkg/model); the existing golden screens unchanged._

## Counter-examples

- The system SHALL NOT drop the URL from the full panel or the JSON export:
  only the compact box changes.
- The system SHALL NOT shorten or elide the URL (e.g. `https://0A1B…com`):
  it is removed, not truncated.
- The system SHALL NOT drop the region from the compact box.
- The system SHALL NOT shorten the context name.

## Out of scope

- Making the compact fields configurable.
- Changing which window heights pick the compact box.
- Shortening an ARN-style context name (`arn:aws:eks:<region>:<account>:cluster/<name>`):
  both headers show it as it is (`TODO.md`).
