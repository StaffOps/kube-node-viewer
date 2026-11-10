# Bugfix: node-name-lock

> Status: active · Reported: 2026-11-10 UTC ([hide-cloud-ids](../hide-cloud-ids/requirements.md) review, `TODO.md`) · Reviewed: 2026-11-10 (spec-refuter; 2 HIGH, 5 MEDIUM, 6 LOW, applied)

## Current behavior

Locking faults in `Node` (`pkg/model/node.go`), found by auditing every method
that takes `n.mu`, and one in its AWS caller:

1. **A recursive read lock.** `Name` holds `n.mu.RLock` (`node.go:269-276`) and,
   for a node with no name yet (known only from its NodeClaim), calls
   `InstanceID` (`:306-318`), which calls `ProviderID` (`:286-290`), which takes
   `n.mu.RLock` again. Today it is the only nesting: `InstanceType` and
   `PodsCopy` hold the read lock and call `IsFargate` and `Pods`, which take
   none; `LabelValue` and `NotReadyTime` release theirs before calling a
   locking method. Making `Pods` lock without changing `PodsCopy` would add a
   second one.
2. **Readers that take no lock.** `IsOnDemand`, `IsSpot`, `IsFargate`,
   `IsAuto` and `Labels` read `n.node.Labels`, and `Pods` ranges over
   `n.pods`, with no lock (`:95-111`, `:259-261`, `:520-526`). `Update`
   replaces `n.node`, and `BindPod` and `DeletePod` write `n.pods`, under the
   node's write lock and under `Cluster.mu.Lock` (`cluster.go:83,103,124,206,222`).
   The races are between informer goroutines: the node informer's
   `UpdateFunc` calls `updatePrice` → `NodePrice` → `Labels`, `IsFargate`,
   `Pods` after `UpdateNode` released `Cluster.mu` (`pkg/client/controller.go:154-160`),
   while the pod informer runs `BindPod`; and the pod informer's
   `node.IsFargate()` (`controller.go:184`) runs while the node informer runs
   `Update`. The pricing goroutine is not one of them: `RefreshNodePrices`
   reaches the readers through `ForEachNode` under `Cluster.mu.RLock`
   (`controller.go:233-236`), which excludes those writers.
3. **Two acquisitions for one decision.** `NodePrice` in `pkg/aws/pricing.go:84-85`
   tests `len(n.Pods()) == 1` and then reads `n.Pods()[0]`: a `DeletePod`
   between the two leaves an empty list and the index panics.
4. **A field read outside the lock (latent).** `NotReadyTime` releases the read
   lock and then reads `n.nodeclaimCreationTime` (`:607-611`), which `Ready`
   writes under the write lock (`:429-433`). Every caller of `Ready`, `Created`
   and `NotReadyTime` runs on the UI goroutine today (`layout.go:887,893`,
   `view.go:81,102`, `export.go:198` per `export.go:113-117`,
   `recommendations.go:96-104`), so no user sees it; fixing it keeps the rule
   below true for every method.

Reproduction, on `main` at 280b965, with throwaway tests (not committed):

- (1) Four goroutines calling `Name` on a nameless node for one second while one
  calls `Update` in a loop: no progress within 3 seconds, three runs out of
  three. The same test on a named node finishes every time.
- (2) `go test -race` with one goroutine calling `Update` and `BindPod` and one
  calling `IsFargate`, `Labels()[…]` or `Pods`: `WARNING: DATA RACE` for each.
- (4) `go test -race` with a writer that flips a NodeClaim node's Ready
  condition with `Update` while it calls `Ready`, and a reader calling
  `NotReadyTime`: `DATA RACE` between `node.go:431` and `:610` (spec-refuter).

What the user sees: with Karpenter, while a NodeClaim has no node yet, the
screen can freeze for good (fault 1): the view reads `Name` on every render
and the informers update the node. Since `Update` runs under `Cluster.mu.Lock`,
a writer stuck behind it also stops every informer and the stats goroutine
(`uimodel.go:397`); `SetPrice` from `RefreshNodePrices` is a second writer.
Fault 2 can make Go abort with `concurrent map read and map write` when a
Fargate node's price is looked up while a pod is bound; fault 3 can panic the
same way.

> **Corrected 2026-11-10 (code review).** The runtime error fault 2 can cause
> is `fatal error: concurrent map iteration and map write` (`Pods` ranging over
> `n.pods` while `BindPod` writes it; reproduced on `main` without `-race`), not
> `concurrent map read and map write`. The label readers race on the `n.node`
> struct, which only `-race` reports: nothing writes into the label map. Dates
> in this spec are UTC.

## Expected behavior

**R1** WHEN `Name`, `PodsCopy` or `InstanceType` (a Fargate node) is called for
a node with no name while other goroutines call `Update` and `SetPrice` on it
THEN the system SHALL return from every call, with the same value as today (the
instance id on AWS, the provider ID as hide-cloud-ids R5 to R7 show it
otherwise).

_Test: `// fix-node-name-lock R1` on `TestNodeLocksDoNotDeadlock`, pkg/model:
readers and writers loop for one second, and the test fails when they have not
all returned 10 seconds later. On the old code `Name` fails it within a second,
three runs out of three; it is a timing test, so a pass on the old code is
possible in principle but was not seen. The goroutines stuck on the old code
are not released; the test binary exits with them._

**R2** WHEN `IsOnDemand`, `IsSpot`, `IsFargate`, `IsAuto` or `Labels` is called
while another goroutine runs `Update` on the same node, or `Pods` while another
runs `BindPod` and `DeletePod`, THEN the system SHALL make the read under the
node's lock, so `go test -race` reports no data race, and SHALL return what it
returns today.

_Test: `// fix-node-name-lock R2` on `TestNodeReadersAreLocked`, one subtest per
reader with the writer it races, pkg/model. It proves the absence of the race
only under `-race`, which `scripts/verify.sh:37` and the `verify` job use;
without `-race` it passes either way._

**R3** WHEN the AWS provider prices a Fargate node THEN the system SHALL read the
node's pods once, so a pod deleted meanwhile cannot make it index an empty
list.

_Test: `// fix-node-name-lock R3` on `TestFargatePriceReadsThePodsOnce`, pkg/aws:
a Fargate node's price while another goroutine binds and deletes its pod, under
`-race`, with no panic._

**R4** WHEN `NotReadyTime` is called while `Ready` clears the NodeClaim creation
time THEN the system SHALL read that time under the lock, with the result it
gives today.

_Test: `// fix-node-name-lock R4` on `TestNodeReadersAreLocked` (a writer flips a
NodeClaim node's Ready condition with `Update` and calls `Ready`; a reader calls
`NotReadyTime`), under `-race` as R2, pkg/model._

## Unchanged behavior

- Every `Node` method returns what it returns today for the same node; the
  existing tests in `pkg/model`, `pkg/aws` and `pkg/client` pass unchanged.
- `InstanceType` of a Fargate node with one pod still reads its provisioned
  size (`"0.25vCPU-0.5GB"`), and `"Fargate"` without one
  (`TestNodeLocksKeepTheirResults`).
- `NotReadyTime` of a NotReady node with a NodeClaim creation time still returns
  that time (`TestNodeLocksKeepTheirResults`).
- `Labels` still returns the node's own map, not a copy. Sharing it is safe
  because nothing writes into it: `Update` replaces `n.node` whole, and the map
  is the one the informer cache holds (`node.go:65`, `:79`), read-only by the
  cache's contract. Its doc comment says so.
- `PodsCopy` is unchanged for its callers.
- No new exported method; the golden screens are unchanged.

## Root cause

`sync.RWMutex` is not reentrant for readers. Once a goroutine calls `Lock`,
every later `RLock` waits until that writer is done, and the writer waits for
the readers already in. A goroutine that holds `RLock` and asks for it again
(`Name` → `InstanceID` → `ProviderID`) therefore blocks behind a writer that
arrived between its two calls, and the writer blocks behind it: a deadlock
(https://pkg.go.dev/sync#RWMutex: "If any goroutine calls Lock while the lock
is already held by one or more readers, concurrent calls to RLock will block
until the writer has acquired (and released) the lock"). Faults 2 and 4 come
from helpers kept lock-free so `InstanceType` and `PodsCopy` could call them
under their own lock, which left them unlocked for everyone else.

The rule the fix makes true: **no exported `Node` method calls another exported
`Node` method while it holds `n.mu`.** Each piece of state is read by an
unexported helper that assumes the lock is held (`isFargate`, `podList`,
`instanceID` on a provider ID string…), and each exported method takes the lock
once and calls only those helpers. `Name` reads the name and the provider ID
under one read lock. Lock order stays acyclic (`Cluster.mu` → `Node.mu` →
`Pod.mu`; pricing locks before `Node.mu`; `Node` never calls `Cluster`).

Helpers over the alternatives: copy-on-write with `atomic.Pointer` would change
every method and the write path for a fault in a few readers; reading the
provider ID inline in `Name` alone fixes fault 1 but leaves the unlocked
helpers that cause faults 2 and 4.
