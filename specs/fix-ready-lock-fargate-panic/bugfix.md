# Bugfix: ready-lock-fargate-panic

> Status: shipped (v0.16.0, 2026-11-11) · Reported: 2026-11-10 UTC ([fix-node-name-lock](../fix-node-name-lock/bugfix.md) review, `TODO.md`) · Reviewed: 2026-11-10 (spec-refuter; 1 HIGH, 3 MEDIUM, 4 LOW, applied)

## Current behavior

Two faults in `pkg/model`, found while fixing the node locks:

1. **`Ready` writes on every call.** `Node.Ready` (`pkg/model/node.go:446-462`)
   reads the Ready condition under the read lock and then, for a Ready node,
   takes the write lock to clear the NodeClaim creation time, every call,
   whether or not there is one to clear. It is reached through `StatusFlags`
   (`node.go:263`) from the Status column (`layout.go:886-887`), the status
   filter and group-by (`view.go:81`, `:102`) and the export
   (`export.go:198`), so with the Status column shown it takes the write lock
   once per Ready node on every render: it waits for the readers in flight
   (the view, pricing) and holds back new ones, and it competes with the
   informers' writes (`Update`, `BindPod`). The cost is unmeasured. The
   clearing also depends on those callers: with the Status column hidden and no
   status filter or group, `Ready` is never called, and a node that registered
   from a NodeClaim and went Ready keeps its NodeClaim's creation time for good
   in the Age column (`layout.go:902`), the sort by age (`view.go:123`), the
   details' created line (`nodedetails.go:123`) and the old-node
   recommendation (`recommendations.go:103`); one that went Ready and back to
   NotReady between two renders keeps it in `NotReadyTime` too.
2. **A malformed Fargate annotation panics.** `Pod.FargateCapacityProvisioned`
   (`pkg/model/pod.go:161-183`) logs a `CapacityProvisioned` annotation that
   does not match `(.*?)vCPU (.*?)GB` and then indexes the failed match
   (`match[1]`, `:173`): `index out of range [1] with length 0`. Its callers,
   `Node.InstanceType` (`node.go:486`) and the AWS provider's `NodePrice`
   (`pkg/aws/pricing.go:88`), run on the UI goroutine and on the informer and
   pricing goroutines, so a Fargate pod with, say, `CapacityProvisioned: 2vCPU`
   ends the program.

Reproduction, on `main` at e553dc8, with the tests below before the fix:
`TestReadyTakesNoWriteLock` times out (fault 1: `Ready` blocks while a reader
holds the lock), `TestReadyClearedOnUpdate` reads the NodeClaim time from
`Created` (fault 1), and `TestFargateCapacityMalformed` panics (fault 2).

## Expected behavior

**R1** WHEN `Ready` is called THEN the system SHALL only read the node, under
the read lock, and return whether its Ready condition is True, as today.

_Test: `// fix-ready-lock-fargate-panic R1` on `TestReadyTakesNoWriteLock`,
pkg/model: the test holds the node's read lock and calls `Ready` on a Ready
NodeClaim node from another goroutine; it fails when `Ready` has not returned
within 5 seconds._

> **Corrected 2026-11-10 (code review).** "Under the read lock" is checked by
> `TestReadyReadsUnderTheLock` (`// fix-ready-lock-fargate-panic R1`), pkg/model:
> `Ready` against `Update` flipping the condition, under `go test -race`;
> `Ready` without its lock gives `DATA RACE`. `TestReadyTakesNoWriteLock` only
> catches a write lock.

**R2** WHEN `Update` applies a node object whose Ready condition is True THEN
the system SHALL clear the NodeClaim creation time, under the same write lock,
so `Created` and `NotReadyTime` use the node's own times from then on, whether
or not the node was rendered, and also after it goes NotReady again.

_Test: `// fix-ready-lock-fargate-panic R2` on `TestReadyClearedOnUpdate`,
pkg/model: a NodeClaim node, updated NotReady (NodeClaim time kept), then Ready
(`Created` is the node's creation time with no `Ready` call), then NotReady
(`NotReadyTime` is the condition's transition time); and on
`TestReadyClearedThroughTheStore`, pkg/model: the same through
`Cluster.AddNode` merging a Ready node into its NodeClaim node, and through
`Cluster.UpdateNode`._

**R3** WHEN a pod's `CapacityProvisioned` annotation does not match
`<cpu>vCPU <memory>GB` THEN the system SHALL log it as today and report no
provisioned capacity (`0, 0, false`) instead of panicking, so the node shows
`Fargate` as its instance type and no price.

_Test: `// fix-ready-lock-fargate-panic R3` on `TestFargateCapacityMalformed`,
pkg/model (`2vCPU` and `big` panic on the old code; `vCPU GB` matches with
empty groups and already returned `0, 0, false`, kept as a guard), and on `TestFargatePriceMalformed`,
pkg/aws (the node's price is unknown, no panic)._

## Unchanged behavior

- `Ready` returns what it returns today for the same node: any `NodeReady`
  condition with status True. `StatusFlags` gives the same flags, Launching
  included (`node.go:261-276`).
- A node never built from a NodeClaim is unaffected. A Ready node gets a
  NodeClaim time back only when it is deleted and re-added from its NodeClaim
  (`controller.go:89-101`), as today.
- The export's Created and Age columns are unchanged: the export calls
  `StatusFlags` before `Created` (`export.go:198-199`), so it already saw the
  cleared time.
- A NodeClaim node that has not been Ready keeps its NodeClaim creation time in
  `Created` and `NotReadyTime` (`TestNodeLocksKeepTheirResults` and
  `TestReadyClearedOnUpdate`).
- A well-formed annotation (`0.25vCPU 0.5GB`) still gives `0.25, 0.5, true`
  (`TestFargateCapacity`); `InstanceType` still shows `0.25vCPU-0.5GB`.
- `TestNodeReadersAreLocked` still passes; its comment and writer are updated,
  since `Update`, not `Ready`, now writes the NodeClaim time.
- The fix-node-name-lock rule holds: no exported `Node` method calls another
  exported one while it holds `n.mu`; `Update` uses an unexported helper.
- No new exported method; the golden screens are unchanged.

## Root cause

1. The clearing lives in a reader. Upstream put "when the node goes ready,
   remove the nodeclaim creation ts" in `Ready` because that is where it
   noticed readiness; the only place the node's conditions change is `Update`,
   which already holds the write lock.
2. The `len(match) != 3` branch logs and falls through instead of returning.

The fix: `Update` sets the node and, when an unexported `isReady` helper sees
the Ready condition True, clears the NodeClaim creation time; `Ready` takes the
read lock and calls `isReady`. `FargateCapacityProvisioned` returns
`0, 0, false` after the log. A `sync.Once` or an atomic flag in `Ready` was the
alternative to moving the write; it keeps a write in a reader and still ties
the clearing to rendering.
