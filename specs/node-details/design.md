# Design: node-details

> Implements [requirements](requirements.md).

## D1. Panel over the cursor's node

`enter` with a node cursor key (`n:<name>`) opens `panelNode`, keeping the
node's name. The panel is rebuilt on every frame from the current snapshot,
so it is live and needs no state beyond the name and a scroll offset.

**Rationale:** the popup machinery already overlays, truncates and draws
the hint; a scroll offset (`panelView.top`) is the only new thing it needs,
since this panel has no selected row.

> **Corrected 2026-11-05.** The decision format asks for more (retroactive
> review). Trade-offs: rebuilding the rows on every frame costs a little
> per redraw while the popup is open. Invariant: the panel reads only the
> watched node, never the API server. **When this would be wrong**: a
> section needs data the snapshot does not hold (events, annotations).
> **Reversal action**: a separate screen with its own state instead of a
> popup.
