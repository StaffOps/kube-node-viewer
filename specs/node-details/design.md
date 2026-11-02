# Design: node-details

> Implements [requirements](requirements.md).

## D1. Panel over the cursor's node

`enter` with a node cursor key (`n:<name>`) opens `panelNode`, keeping the
node's name. The panel is rebuilt on every frame from the current snapshot,
so it is live and needs no state beyond the name and a scroll offset.

**Rationale:** the popup machinery already overlays, truncates and draws
the hint; a scroll offset (`panelView.top`) is the only new thing it needs,
since this panel has no selected row.
