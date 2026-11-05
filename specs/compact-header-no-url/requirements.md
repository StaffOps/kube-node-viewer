# Requirements: compact-header-no-url

> Status: draft · Owner: Carlos Gomes · Reviewed: —

## Context

The compact header (`ctrl+e`, or a window from 14 to 27 rows high) is a
two-line box: the cluster and the counts on the first line, the view on the
second. Its first line repeats the API server URL from the full panel
(`pkg/model/header.go:177-189`). An EKS or AKS endpoint is 60 to 80
characters, so on a normal window the URL takes most of the line, pushes the
counts and the cost to the right edge, and adds nothing the context name
does not already say for the owner. The owner asked to drop it from the
compact header. The full panel keeps its `Cluster:` row, so the URL is one
`ctrl+e` away.

## Usage examples

```text
(full panel, unchanged)
 Context:  demo-prod
 Cluster:  https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com
 Region:   us-east-1
 Nodes:    10 of 10 shown · 220 pods · $728.102/month

(compact box, before)
 demo-prod · https://0A1B2C3D4E5F.gr7.us-east-1.eks.amazonaws.com · us-east-1 · 10 of 10 shown · 220 pods · $728.102/month
 Filters: none │ Group: none │ Sort: creation ↓ │ cpu+memory

(compact box, after)
 demo-prod · us-east-1 · 10 of 10 shown · 220 pods · $728.102/month
 Filters: none │ Group: none │ Sort: creation ↓ │ cpu+memory

(compact box, no region known: context and counts only)
 demo-prod · 10 of 10 shown · 220 pods
```

## Requirements

**R1** WHEN the header is the compact box (picked with `ctrl+e`, or chosen
because the window is 14 to 27 rows high) THEN its first line SHALL show the
context name, the pricing region when one is known, and the node and pod
counts with the cost, separated by ` · `, and SHALL NOT show the API server
URL.

- [ ] With a known server URL and region, the compact first line is
  `<context> · <region> · <counts and cost>` and contains no part of the URL.
- [ ] With no region, the compact first line is `<context> · <counts and cost>`.
- [ ] The second line (the view) and the issue line, when there is one, are
  unchanged.

_Test: `// compact-header-no-url R1` on `TestCompactHeaderHasNoServerURL`
(pkg/model) and the golden screen `header-compact.txt`._

**R2** WHEN the header is the full panel or the one-line header THEN it SHALL
be unchanged: the full panel keeps its `Cluster:` row with the URL (or `-`),
and the one-line header (below 14 rows) keeps showing no URL.

- [ ] The full panel's `Cluster:` row still shows the server URL.
- [ ] The golden screens of the full panel are byte-identical.

_Test: `// compact-header-no-url R2` on `TestFullHeaderKeepsServerURL`
(pkg/model); the existing golden screens unchanged._

## Counter-examples

- The system SHALL NOT drop the URL from the full panel, the export, the
  contexts popup (`c`) or the saved state: only the compact box changes.
- The system SHALL NOT shorten or elide the URL (e.g. `https://0A1B…com`):
  it is removed, not truncated.
- The system SHALL NOT drop the region from the compact box.

## Out of scope

- Making the compact fields configurable.
- Changing which window heights pick the compact box.
