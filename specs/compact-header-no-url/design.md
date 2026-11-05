# Design: compact-header-no-url

> Implements [requirements](requirements.md).

## Current state

`(*UIModel).header` (`pkg/model/header.go:148-216`) builds three headers.
The compact one joins the context, then `u.info.Server` and the region when
not empty, then the counts with the cost (`countsWithCost`, the same text as
the full panel's `Nodes:` row), with ` · ` (`header.go:177-189`). The region
comes from the session and, when empty, from the first node's region label
(`header.go:173-176`). The full panel shows the URL in its `Cluster:` row
(`header.go:193`); the one-line header (below `headerCompactMinHeight`, 14
rows) shows the context, the counts and the view, no URL
(`header.go:164-171`). `boxed` clips each line to the window width
(`header.go:230`). No golden screen covers the compact box today;
`.static/header-compact.png` is drawn by the `header-compact` scene of
`pkg/model/screenshots_test.go:185`, and the `demo.gif` has a `ctrl+e` frame.

## Target state

The compact branch iterates over the region only. Nothing else in the header
changes, and `u.info.Server` is still filled and used by the full panel, the
export and the saved-state key.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/header.go` | compact first line without `u.info.Server` |
| `pkg/model/header_internal_test.go` (new) | `TestCompactHeaderHasNoServerURL`, `TestFullHeaderKeepsServerURL` |
| `pkg/model/panels_internal_test.go` | `TestHeaderPanelAndOneLineFallback` (242-268): the compact header at 20 rows no longer contains the URL; tag also `compact-header-no-url R1` |
| `pkg/model/golden_test.go`, `testdata/screens/header-compact.txt` | a compact golden screen |
| `.static/header-compact.png`, `.static/demo.gif` | regenerated with `scripts/screenshots.sh`; other images restored |
| `README.md` | the sentence above the compact image says what the box shows |
| `CHANGELOG.md` | `[Unreleased]` `Changed` |
| `specs/interactive-tui/requirements.md` | dated note under R16 |
| `TODO.md` | item: ARN-style EKS context names show the account id |

## Decisions

### D1: Remove the URL from the compact box, keep it in the full panel

**Choice**: the compact box's first line drops the API server URL; the full
panel keeps it.

**Justification, strongest first**:
1. The compact box exists to save rows and width; the URL is the widest
   field (52 characters in the fixture, about 72 for a real EKS endpoint)
   and, because the box clips each line, it is what pushes the cost off a
   normal window.
2. The context name is how the user picked the cluster; the URL stays one
   `ctrl+e` away in the full panel and in the JSON export.
3. The one-line header already works this way (context, no URL), so the two
   small headers become consistent.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Contexts with the same name but different servers look alike in the compact box: the same name in different kubeconfig files or runs (the saved state keys clusters by context plus server for this reason, README "Saved view per cluster"), or a cluster recreated behind the same context name | the full panel (`ctrl+e`) and the export show the server; the region still tells clusters in different regions apart |
| A screenshot of the compact box no longer shows the endpoint | the full panel does |

**When this would be wrong**: users report mixing up clusters whose
contexts share a name, or ask which endpoint the compact box is showing.

**Reversal action**: put `u.info.Server` back in the compact branch's list
(one line in `header.go`), regenerate the golden screen and the images; a
few minutes.

**Alternatives discarded**: truncating the URL (`https://0A1B…com`) — still
wide, and the truncated part is the part that identifies the cluster;
showing only the host — still about 60 characters for EKS; a flag or config
key to choose — configuration for a one-field difference the full panel
already covers.

## Invariants

- Requests, never usage; no new network call.
- The full panel and the one-line header render byte-identically.
- `u.info.Server` is still set for every session (the full panel, the export
  and the saved-state key read it).

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Server URL unknown | Same as today: nothing in its place | — |
| Region unknown | Context, counts and cost | No region field |

## Test plan

- `TestCompactHeaderHasNoServerURL`, table-driven: compact by `ctrl+e` on a
  40-row window, and by height at 20 rows with no pick; server and region
  set → first line is `<context> · <region> · …`, without the URL or its
  host; nodes without the region label and no session region →
  `<context> · …`, cost shown.
- `TestFullHeaderKeepsServerURL`: the full panel still has `Cluster:` and
  the URL; at 10 rows the one-line header has no URL.
- `TestHeaderPanelAndOneLineFallback`: the compact assertion becomes "does
  not contain the URL".
- A new golden scene `header-compact` (same fixture as the screenshots);
  the nine existing golden screens unchanged.
- Run on a local kind cluster: `ctrl+e` both ways, a window of about 20
  rows (automatic compact) and of 12 rows (one line).
