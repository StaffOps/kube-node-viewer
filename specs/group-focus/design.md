# Design: group-focus

> Implements [requirements](requirements.md).

## Current state

- Main key switch: `pkg/model/uimodel.go` `handleKey`. `enter` and `space`
  share one case: on a node `enter` opens details (`cursorOnNode`,
  `openNodeDetails`); otherwise both call `toggleGroupAtCursor`, which
  folds the heading's group (`ViewState.ToggleCollapsed`) and saves through
  `setView` → `onViewChange` (interactive-tui R17). `esc` clears a kept
  search (`u.view.Search = ""`), nothing else.
- Cursor: `u.cursorKey` is `n:<node>` or `g:<path joined by " › ">`;
  `u.cursorPath` is the heading's path; `resolveCursor`
  (`pkg/model/layout.go`) puts a cursor whose row is not on the current
  page on that page's first row.
- Selection: `render` (`pkg/model/layout.go`) calls `u.view.Select`, and the
  header, the cluster summary and the list all read that result;
  `exportDoc` (`pkg/model/export.go`) and the Recommendations screen
  (`pkg/model/recommendations.go`) call `Select` on their own.
- Groups are built from `Field.Value(n)` per group-by level
  (`groupLevel`, `pkg/model/view.go`); a missing value is `UngroupedValue`.
- `SetView` (`pkg/model/uimodel.go`) replaces the view on a context switch
  (`pkg/model/contexts.go`) and when a named view is applied
  (`pkg/model/views.go`).
- Saved state is the JSON of `ViewState` (`pkg/state`).

## Target state

```text
enter on heading ─▶ push a step: the heading's levels not in the focus yet
render (Overview) ─▶ nodes = focusNodes(Select(...)) ─▶ header, summary, list
                     list drawn from displayView(): nodes shown, focus unfolded
esc (Overview) ─▶ search kept? clear it : pop one step, cursor back on its heading
```

- `pkg/model/focus.go` (new):
  - `focusLevel{Field Field; Value string}` and
    `focusStep{levels []focusLevel; heading string}`; `u.focus []focusStep`
    on `UIModel`, one step per `enter`, with the heading key it came from.
  - `focusLevels()` is every step's levels in order; `hasField(f)` tells
    whether a level for `f` exists.
  - `focusNodes(nodes)` keeps nodes with `Field.Value(n) == Value` for every
    level, in their order. It uses `Value`, not `Values`, because groups are
    built from `Value`.
  - `inFocus(path)`: every `{GroupBy[i], path[i]}` of the heading is a focus
    level (the focused group's heading or one above it).
  - `enterFocus(path)`: the step is `{GroupBy[i], path[i]}` for each `i`
    whose field is not `hasField` yet (R4); nothing happens when it is
    empty. Otherwise the step is pushed, the cursor goes to the heading and
    the page to 0.
  - `leaveFocus()`: pops one step (R3), sets `cursorDelta` to 0, the page to
    0, the cursor key to the step's heading and `u.cursorFind`.
  - `displayView()`: a copy of `u.view` with `ShowNodes` on, `GroupDepth` 0,
    `Details` off, and `Collapsed` cloned (`append([]string(nil), …)`)
    without the headings `inFocus`. `u.view` is not changed, so nothing is
    saved.
- `render`: on Overview with a focus, `u.view` is set to `displayView()`
  for the frame and restored with `defer` (D5); the node set is
  `focusNodes(Select(...))`. On Recommendations it is `Select(...)` and the
  view is not swapped.
- `handleKey`:
  - `enter` on a node → details; on a heading on Overview → `enterFocus`.
  - `space`: inside `toggleGroupAtCursor`, after the pending move is laid
    out, a heading that is `inFocus` returns without folding (R5);
    otherwise it folds as today.
  - `esc`: a kept search is cleared first; otherwise, on Overview only,
    `leaveFocus` when focused (R3, R8).
  - `n` and `d` return early while focused (R7).
- `SetView` clears the focus (R6).
- `resolveCursor`: `cursorFind` is checked before the "not on this page"
  case. When the cursor's row exists, the page becomes that row's page; the
  flag is cleared either way. A missing row falls back to the first row of
  the current page, which `leaveFocus` sets to 0.
- `viewSummary` adds `Focus: f=v › f=v` after Filters when focused (R2).
  `helpText` reads `enter: focus • space: fold • n: fold` with a group-by,
  and `enter: focus • space: fold • esc: back` while focused (no `n`, `d`);
  _Corrected 2026-11-04:_ while focused it starts with
  `esc: back • enter: focus • space: fold`, as R2's correction says;
  focused with no group-by, it adds only `esc: back`;
  the help panel rows for `enter`, `space` and `esc` follow.
- `exportDoc`: on Overview the nodes are `focusNodes(Select(...))` and the
  doc gets `Focus []exportFocus` (`field`, `value`) with
  `json:"focus,omitempty"`; on Recommendations neither (R8). CSV columns do
  not change.
- `render` passes the `Select` count to `renderContent`, which shows `No
  nodes in the focused group.` when `Select` left nodes and `focusNodes`
  none; `No nodes match the filters or search.` when
  `Select` left none.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/focus.go` (new) | focus state helpers |
| `pkg/model/uimodel.go` | `focus`, `cursorFind` fields; keys `enter`, `space`, `esc`, `n`, `d`; `SetView` clears |
| `pkg/model/layout.go` | `render` focus and display view; empty message; `resolveCursor` follow; `helpText` |
| `pkg/model/header.go` | `Focus:` in the View line |
| `pkg/model/export.go` | export the focused nodes |
| `pkg/model/panels.go` | help rows for `enter`, `space`, `esc` |
| `pkg/model/view.go` | `Collapsed` comment: folded with `space` |
| `pkg/model/panels_internal_test.go`, `nodedetails_internal_test.go`, `screenshots_test.go` | fold with `space`; test names and demo captions |
| `specs/interactive-tui/requirements.md`, `specs/node-details/requirements.md` | dated corrections: `enter` on a heading focuses, `space` folds |
| `README.md`, `CHANGELOG.md` | keys table, Group by section, export format |

## Decisions

### D1: `enter` on a heading focuses; folding moves to `space`

**Choice**: `enter` on a group heading shows only that group; `space`, which
already folds a heading, keeps folding it.

**Justification, strongest first**:
1. The owner asked for `enter` on a nodepool to show its nodes.
2. `enter` then means "open what is under the cursor" on both kinds of row
   (a node's details, a group's nodes).
3. `space` already folds a heading today, so folding keeps a key and no
   new key is needed.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Users who fold with `enter` now focus instead | `esc` undoes it; the CHANGELOG lists it under Changed; `space` folds |
| interactive-tui R17 and node-details R1 change | Recorded as dated corrections in those specs |

**When this would be wrong**: if folding one group is used far more often
than looking at one group.

**Reversal action**: swap the two cases in `handleKey` and the help texts;
the focus code stays, reachable from another key.

**Alternatives discarded**: a new key (`F`) for the focus — the owner asked
for `enter`, and it is one more key to learn. `enter` adding a filter — see
D2.

### D2: The focus is UI state, not a filter and not saved

**Choice**: `u.focus` lives on `UIModel`; `ViewState` and the state file
do not change.

**Justification, strongest first**:
1. It is a look at one group, undone with `esc`; a saved filter would still
   be there on the next start and in the Filters panel.
2. Every later view change (`setView`) saves the whole `ViewState`; a
   filter or a `json:"-"` field on it would leak into saves or named views
   (`SetView` replaces the struct when a view is applied).
3. Matching on `Field.Value` with the group's own value focuses exactly the
   group the heading showed, `<none>` included; a filter uses
   `Field.Values` and would differ for `status`.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The focus is lost on restart | Out of scope; a filter or a named view covers "always this group" |
| The focus survives a group-by change and may no longer match a heading | It still narrows the nodes and shows in the header; `esc` leaves it |

**When this would be wrong**: if the owner wants the focus back after a
restart.

**Reversal action**: add the levels to `ViewState` with a json tag and save
them; the matching code stays.

**Alternatives discarded**: a temporary `Filter` (saved by the next
`setView`, shown in the Filters panel); a `json:"-"` field on `ViewState`
(carried by `SetView` and named views in memory).

### D3: The focused group is drawn unfolded, `n` and `d` are off

**Choice**: while focused, the list is drawn from a copy of the view with
the node list shown and the focused group unfolded; `n` and `d` do nothing.

**Justification, strongest first**:
1. The point of the focus is to see that group's nodes (R1); a folded list
   would show one heading.
2. Drawing from a copy leaves the saved fold state alone, so `esc` brings
   the folded view back as it was (R7).
3. With nodes always shown, `n` and `d` would have no visible effect;
   doing nothing is clearer than changing a hidden state.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| No folded view of the sub-groups inside a focus | `space` folds a sub-group inside it |

**When this would be wrong**: if owners focus on a big nodepool with many
sub-groups and want them folded.

**Reversal action**: let `n` act on the copy kept with the focus.

**Alternatives discarded**: unfolding by editing `u.view` — saved, and lost
on `esc`.

### D4: `esc` clears a kept search before leaving the focus

**Choice**: the first `esc` clears a kept search; the next one leaves one
focus level.

**Justification, strongest first**:
1. `esc` clears the search today (interactive-tui R6, search-regex); that
   keeps working the same.
2. A search typed inside a focus narrows it; undoing the last narrowing
   first matches "go back a step" in the help.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A search kept before focusing is cleared before the focus is left | Both are shown in the View line |

**When this would be wrong**: if owners expect `esc` to leave the focus at
once.

**Reversal action**: swap the two branches of the `esc` case.

**Alternatives discarded**: `backspace` to leave the focus — not used on the
main screen for anything else, but nobody would guess it.

### D5: Render draws from a copy swapped into `u.view`

**Choice**: while focused on Overview, `render` sets `u.view` to
`displayView()` and restores it with `defer`; the drawing code is unchanged.

**Justification, strongest first**:
1. The list, group lines, bars, `helpText` and panels all read `u.view`;
   passing a view through each would touch most of `layout.go`.
2. Nothing in `render` writes `u.view` or calls `setView`: the writes are on
   the key path (header size, search, views), so the copy cannot be saved.
3. `defer` restores the view even if drawing panics.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A future write to `u.view` during render would hit the copy | A test checks that a focused frame leaves `u.view` and the saved view unchanged |
| `render` also runs from `cursorOnNode` on the key path | It restores before returning, and keys run on the same goroutine |

**When this would be wrong**: once render starts changing view state, or
runs on another goroutine.

**Reversal action**: pass the view to `currentPage`, `groupName`,
`groupTable` and `helpText` instead.

**Alternatives discarded**: editing `u.view` and undoing on `esc` — any
`setView` in between would save it.

## Invariants

- Requests, never usage; no new watch or API call.
- The saved view state and its file format are unchanged.
- Groups and nodes keep their order.
- `esc` never quits.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| The focused group's nodes all go away | `No nodes in the focused group.` | Message, `Focus:` still in the header |
| Group-by changed while focused | Nodes still narrowed by the focus values | `Focus:` in the header |
| Heading left by `esc` no longer exists | Cursor on the first row of page 0 | Cursor position |

## Test plan

- `pkg/model/focus_internal_test.go`: R1–R8 by keys on `capacityUI` and
  labeled nodes with two and three group levels: focus and counts, folded
  list and folded group, folded sub-groups kept, `<none>`, header and key
  line text, `esc` order, a pending move and the cursor, page follow,
  sub-group and ancestor `enter`, a group-by change then `enter`, one `esc`
  per `enter`, `space` folds and saves (and does nothing on headings in
  focus, also with a pending move), nothing saved by the focus and `u.view` untouched after a frame,
  `SetView` clears, both empty messages, focus with a search, a focus over
  two pages, `n`/`d` off, Recommendations (rows, counts, `e`, `esc`), and
  export with and without the `focus` key.
- Existing tests that fold with `enter` move to `space`.
- `scripts/verify.sh`, `make lint`, a run on a cluster.
