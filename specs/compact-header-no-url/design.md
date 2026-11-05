# Design: compact-header-no-url

> Implements [requirements](requirements.md).

## Current state

`(*UIModel).header` (`pkg/model/header.go:148-200`) builds three headers.
The compact one joins the context, then `u.info.Server` and the region when
not empty, then the counts with the cost, with ` · ` (`header.go:177-189`).
The full panel shows the URL in its `Cluster:` row (`header.go:193`); the
one-line header (below `headerCompactMinHeight`, 14 rows) shows only the
context, the counts and the view (`header.go:164-171`). No golden screen
covers the compact box today; `.static/header-compact.png` is drawn by the
`header-compact` scene of `pkg/model/screenshots_test.go:185`.

## Target state

The compact branch iterates over the region only. Nothing else in the header
changes, and `u.info.Server` is still filled and used by the full panel, the
contexts popup and the export.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/header.go` | compact first line without `u.info.Server` |
| `pkg/model/header_internal_test.go` (new) or an existing header test | R1, R2 tests |
| `pkg/model/golden_test.go`, `testdata/screens/header-compact.txt` | a compact golden screen |
| `.static/header-compact.png` | regenerated with `scripts/screenshots.sh` |
| `README.md` | the sentence above the compact image says what the box shows |
| `CHANGELOG.md` | `[Unreleased]` `Changed` |
| `specs/interactive-tui/requirements.md` | dated note under R16 |

## Decisions

### D1: Remove the URL from the compact box, keep it in the full panel

**Choice**: the compact box's first line drops the API server URL; the full
panel keeps it.

**Justification, strongest first**:
1. The compact box exists to save rows and width; the URL is the widest
   field (60 to 80 characters for EKS and AKS) and the least read once the
   context is known.
2. The context name already identifies the cluster to the person who picked
   it; the URL stays one `ctrl+e` away in the full panel.
3. The one-line header already works this way (context, no URL), so the two
   small headers become consistent.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Two kubeconfig contexts with the same name but different servers look alike in the compact box | rare; the full panel and the contexts popup (`c`) still show the server |
| A screenshot of the compact box no longer shows which endpoint it is | the full panel does; sharing an endpoint is usually unwanted anyway |

**When this would be wrong**: users ask which cluster the compact box is
showing, or report mixing up clusters whose contexts share a name.

**Reversal action**: put `u.info.Server` back in the compact branch's list
(one line in `header.go`), regenerate the golden screen and the image; a
few minutes.

**Alternatives discarded**: truncating the URL (`https://0A1B…com`) — still
wide and the truncated part is the part that identifies the cluster; showing
only the host — still 50+ characters for EKS; a flag or config key to choose
— configuration for a one-field difference the full panel already covers.

## Invariants

- Requests, never usage; no new network call.
- The full panel and the one-line header render byte-identically.
- `u.info.Server` is still set for every session (the contexts popup and
  the export read it).

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Server URL unknown | Same as today: nothing in its place | — |
| Region unknown | Context and counts only | No region field |

## Test plan

- `TestCompactHeaderHasNoServerURL`: compact box with server and region set;
  first line equals `<context> · <region> · …` and does not contain the
  server; with no region, `<context> · …`.
- `TestFullHeaderKeepsServerURL`: full panel still has `Cluster:` and the URL.
- A new golden scene `header-compact` (same fixture as the screenshots);
  the nine existing golden screens unchanged.
- Run on a local kind cluster: `ctrl+e` both ways, and a window resized to
  about 20 rows (automatic compact) and to 12 rows (one line).
