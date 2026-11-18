# Design: clusters-screen

> Implements [requirements](requirements.md).

> **Revised 2026-11-18.** Added D9 (clusters under their cloud, a total per
> cloud, none across clouds), D10 (nodes first, pods on unfold) and D11
> (`:clusters <cloud>` scope). D1's cost row, D3's limit note, D7's synced
> signal, the components, failure modes and test plan follow them.
>
> **Revised 2026-11-18, after the third review.** D3: the command line is a
> `fleetSync` caller, member changes are coalesced into one redraw, an auth
> error stops retries. D7: the error clears on sync or on the first event
> after it, not on a `Watch` return. D8: no AWS config leaves members
> unpriced. D9: `mixed` and `other`. D10: Fargate pods, the closed list of pod
> figures, a pending unfold. D11: when a member's cloud is final. D12 (new):
> one section per API server. `tab` no longer reaches the screen (R1).
>
> **Corrected 2026-11-18, owner's review of the first build.** D10's default
> is reversed (every member's pods are watched as it connects,
> `clusters.pods: unfolded` keeps the old one), D3 holds a start place until
> the pods synced in that mode, D13 (new) adds the summary lines per
> section. The notes below mark the places that change.
>
> **Revised 2026-11-18, after the fourth review.** D3: a watch stops only on a
> repeated 401, a 403 or an exec failure, per informer; `SetWanted` takes the
> scope and remembers learned clouds. D10: the Fargate watch is selected by
> scheduler name and kept until the screen is left; filters on usage labels
> are gated; snapshot order. D11: an empty synced cluster keeps a known guess.
> D12: de-duplication in `fleetSync`, first by name, normalized URLs.
> `switched()` forces Overview only for R6's `enter`.

## Current state

- One connection at a time: `sessions.Switch` builds clients, a store
  (`model.NewCluster`), a `client.Controller` and a price provider, then
  cancels the previous connection through the single `sessions.cancel` and
  closes the single `sessions.sizes` (`cmd/kube-node-viewer/session.go`).
  `Switch` also consumes `s.explicit` and calls `starts.pick`/`starts.opened`
  (the config default view). `Close` stops only the one connection.
- `main.go` calls `m.Connect(first)` before `SetSwitcher` and before
  `program.Run`; `Connect` returns nothing. `program.Send` blocks until the
  program runs.
- `Controller.Start` runs the pod, node and nodeclaim informers and drops
  them (`_, podController := …`): there is no synced signal and no error
  (`pkg/client/controller.go`). `cache.Controller` has `HasSynced` and no
  error handler (client-go v0.37.2, `tools/cache/controller.go`); the
  informers are built with `cache.NewInformerWithOptions` from a
  `ListerWatcher`.
- `NewController` calls `pricing.OnUpdate(c.RefreshNodePrices)`; both
  providers only append callbacks (`pkg/aws/pricing.go`, `pkg/azure/pricing.go`).
  `sessions.pricing` makes one AWS provider per call, bound to one region.
- `UIModel` holds one `cluster`, `gen`, `stats`, `snapSeq` and `info`;
  messages for another generation are dropped (`pkg/model/uimodel.go`,
  `update`); `Connect(Session)` replaces them (`contexts.go`). Rows read
  `u.cluster.resources`, which `r` sets (`SetResources`).
- Nodes are keyed by provider ID with a `nodesByName` index; pods by
  `namespace/name` (`cluster.go`). Cursor keys are `n:`+name and `g:`+path
  (`layout.go`); `findNode` looks in `u.stats.Nodes` by name
  (`nodedetails.go`); group folds are paths in `view.Collapsed`.
- `currentPage` builds one `nodeTable` and one title line, and `paginate`
  repeats that one title on every page (`layout.go`). `renderContent`
  reserves the DaemonSets line and shows `Waiting…`/`No nodes…` from
  Overview's stats before anything else.
- Screens are `view.Screen` plus the `:` command table (`screens.go`); a
  saved screen is restored by `SetView` inside `Connect`, not by `setScreen`.
  `handleKey` returns early for Recommendations with an Overview-only key
  list.
- `contextsKey` guards a pending switch with `p.switching`; a failed switch
  writes `u.contexts.err`, shown only in the open picker.
- `Switcher.Contexts()` returns names only. `pkg/state` imports `pkg/model`.
- Prices come from `pricing.ByCloud`; its fallback is the AWS provider.

## Target state

```text
 UIModel ── Fleet (interface, set before program.Run)
              │ SetWanted(contexts, scope)   non-blocking, desired state
              │ Contexts()            names + server + guessed cloud
              ▼
   fleetSync() (direct call, no tea.Cmd): from Init, Connect, setScreen and
   the picker → SetWanted(selected, scope) when Clusters is on screen, else nil
              ▼
   sessions (cmd): Fleet under a mutex with an epoch
     member = connect(context) in a goroutine, ≤4 starting, own cancel,
              no `sizes`, price set of this visit
     → program.Send(MemberReadyMsg{Context, Gen, Cluster, Status, Err})
     → one fleet notifier: MembersChangedMsg{Gens} per changeInterval
   UI: slots by context and by generation; a snapshot per slot off the UI
   goroutine → memberSnapshotMsg{Gen, Stats}; Overview's slot is borrowed
   (reads u.stats, Status from Session.Status)
 UIModel.render (Clusters) → per slot: view.Select → heading + blocks;
 total on top; one flat list of blocks that carry their slot
```

## Components touched

| Path | Change |
|------|--------|
| `pkg/client/controller.go` | `Status()` (D7): list/watch wrapper records and clears the last error, `HasSynced` of the node informer (`Synced`) and of the pod informer (`PodsSynced`) under a mutex, a goroutine that calls `Changed()` once synced; errors sanitized (no URL). `StartNodes(ctx)` (node and nodeclaim watches) and `StartPods(ctx)` (once, idempotent) split out of `Start`, which calls both (D10) |
| `cmd/kube-node-viewer/session.go` | `connect(context)` shared by `Switch` and members; `explicit`, `starts.pick`, `starts.opened` stay in `Switch`; the fleet (D3); price set per visit with an AWS provider per region and one Azure provider; `gce` routed to a provider that prices nothing; `Session.Status` filled from the controller; `Close` closes the fleet |
| `pkg/client/client.go` | `GuessCloud(context, server)`; `Contexts` variant returning each context's server |
| `pkg/model/header.go` | export the GKE context patterns as `IsGKEContext`; title and fleet counts on this screen |
| `pkg/model/clusters.go` (new) | `Fleet` (with `WatchPods(context)`), `FleetStore`, `ContextInfo`, messages, slots, `fleetSync` (de-duplicates by server, D12, and passes the scope; the fleet filters by guessed and learned cloud, D11), picker panel grouped by cloud, cloud groups with their totals (D9), heading text, UI-only fold map whose first unfold calls `WatchPods`, `openCluster` |
| `pkg/model/contexts.go` | `Session.Status`; `switchedMsg` gains `fromClusters`, set only by R6's `enter`: only that switch forces Overview (a `c` into a cluster saved on Clusters keeps Clusters, R2); a failed switch is reported as a notice when no panel is open |
| `pkg/model/screens.go` | `ScreenClusters`, command `clusters`/`cl` with an optional cloud word (D11) and its completion, a scope change calls `fleetSync`, `tab` from Clusters to Overview (Overview ↔ Recommendations unchanged), unknown screen → Overview, hints |
| `pkg/model/uimodel.go` | `Init` and `Connect` call `fleetSync`; message routing by generation; `clustersKey` taken before the shared key switch |
| `pkg/model/layout.go` | `clustersBody` taken first in `renderContent` (no DaemonSets line, no Overview empty states); one `nodeTable` per section; `block` carries its slot and title; `paginate` takes the title per block; cursor keys prefixed by context; total line instead of `clusterSummary` |
| `pkg/model/panels*.go` | value lists take the union of the slots' nodes on this screen |
| `cmd/kube-node-viewer/main.go`, `pkg/state/state.go` | `SetFleet` and the `FleetStore` adapter before `program.Run`; optional global `fleet` list and `fleetCloud` scope in the file; old files load as before, an older binary drops them on its next save (accepted) |
| `README.md`, `CHANGELOG.md`, help keys | the screen, `:clusters`, the picker, the keys |

## Decisions

### D1: One store and controller per cluster, joined by the screen

**Choice**: keep a `Cluster` and a `Controller` per context and have the
Clusters screen read all of them, instead of one store holding every
cluster's nodes and pods.

**Justification, strongest first**:
1. `Cluster` keys pods by `namespace/name` and indexes nodes by name
   (`cluster.go`): two clusters collide. Fixing that touches every lookup
   (`GetNodeByName`, cursor keys, node details).
2. A failure, a cancel or a token expiry stays inside one member; the others
   keep running.
3. `sessions` already builds exactly this per context; this reuses it.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| N sets of node informers, and a pod watch per unfolded cluster (D10); corrected 2026-11-18: per member with `clusters.pods: all` | closed on leaving the screen (D3); memory unmeasured, measured in T12 |
| The screen sums per slot instead of reading one list | `newGroupStats` already sums any node slice |

**When this would be wrong**: a feature needs one list across clusters (a
global sort of nodes). Then the keys must be made cluster-scoped.

**Reversal action**: remove `clusters.go` and the fleet in `session.go`; the
single-session path is untouched, so it is a deletion, about a day.

**Alternatives discarded**: one merged store with cluster-scoped keys — wider
change to the store, node details and cursor, for no need in this spec.

### D2: A screen of the current view, not a new mode

**Choice**: `ScreenClusters` in `view.Screen`, reached like Recommendations,
using the live `ViewState` of the context on screen for every section.

**Justification, strongest first**:
1. Filters, sort, search and group-by already exist and are what the owner
   asked to keep working.
2. No new saved state per cluster.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A filter on a label only some clusters have drops the others' nodes | the heading says `(filtered: 0 of N)` |
| The view is saved with the context on screen | same as Recommendations |
| Group folds and focus are not offered inside a section | their state is by path, so one fold would fold every cluster; Overview keeps them |
| Sections use a copy of the view with the nodes shown, no depth, no details, no folds | a saved folded Overview view would otherwise hide rows with no key to open them |

**When this would be wrong**: owners want a different view per cluster side
by side, or folding of groups here.

**Reversal action**: drop the constant and the command; a saved `screen:
clusters` then opens Overview (tested).

**Alternatives discarded**: a separate state per cluster — more state, no
request for it.

### D3: Desired-state fleet, started by one hook, 4 starting at a time

**Choice**: the UI calls `Fleet.SetWanted(contexts, scope)` from a single
`fleetSync()`: with Clusters on screen it passes the selected contexts,
less those sharing an API server with another (D12), and the cloud scope
(D11); otherwise nil. `SetWanted` does not block: it records the set under a mutex
with an epoch, cancels members no longer wanted, and starts the new ones in
goroutines, 4 starting at once. A result of an older epoch is closed at once.
`fleetSync()` runs from `Init` (the start-up with a saved screen), `Connect`
(after `c` or `enter`), `setScreen`, the command line (a scope change while
on the screen, D11) and the picker's `enter`. Overview's store is
borrowed as a slot and never owned. The first time (no saved list) the
screen opens the picker and connects only Overview's slot until `enter` in it.
A slot of a member frees its start place when its node watch is synced, on
its first recorded error, or when it is cancelled. Members' change notices are coalesced: one redraw per
`changeInterval` for the whole fleet, not one per member. Each
informer of a member has its own context. An informer stops, keeping its
error, on a 403 (RBAC on that resource: the other watches go on, R7), on a
401 that repeats after the exec authenticator's forced refresh (client-go
refreshes on the first 401, `plugin/pkg/client/auth/exec/exec.go`), or on an
exec plugin failure, told by its `getting credentials:` prefix (wrapped with
`%v`, so only the text tells it; a test pins it to the client-go version).
Other errors keep the reflector's backoff. Overview's own controller is not
changed: it keeps retrying as today.

**Justification, strongest first**:
1. `Connect` is called before the program runs, from a void function, and
   `program.Send` blocks until then; a non-blocking desired-state call from
   `Init` avoids threading a `tea.Cmd` through `setScreen`, `runCommand` and
   `commandKey`.
2. The epoch makes a quick `tab` out of the screen cancel connects still in
   flight, so no watch is left after leaving (R9).
3. A pod watch across all namespaces is the heaviest thing the tool does;
   keeping N of them while looking at Overview is waste.
4. The limit of 4 keeps a 30-cluster list from opening 30 TLS and
   exec-auth sessions (`aws eks get-token`) together.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The limit is on starting, not on steady state: a list of 30 is 30 node watches (corrected 2026-11-18: and 30 pod watches with `clusters.pods: all`; the start place is then held until a member's pods synced or its first error, so at most 4 pod lists run at once besides those that failed and retry) |
| A pod list that failed gives its place back and retries beside 4 others | its failure costs little; holding the place would let 4 unreachable clusters block the queue | [ASSUMPTION] node lists are small next to pod lists (D10, T12); the list is the user's choice, shown in the picker; first use asks |
| A stopped watch does not recover by itself after a fixed token or role | leaving and opening the screen again reconnects it (R7) |
| Matching the exec error by text | pinned by a test to the client-go version |
| Every visit pays the sync and the price set again | seconds per cluster (unmeasured, T12); sections fill as they arrive |
| `enter` opens the cluster through `Switch` while its member is still open | a few seconds of two watches of one cluster; `Switch` leaves the fleet alone and the screen change closes it |

**When this would be wrong**: the owner keeps the screen open all day on a
large fleet, or opens it often, so sync cost dominates — then keep members
connected with a timeout and add an unregister to `OnUpdate`; or `enter`
should promote the member's store to Overview instead of reconnecting.

**Reversal action**: stop passing nil in `fleetSync`'s else branch; small.

**Alternatives discarded**: connect everything at start — slows start and
loads clusters nobody looks at. A `tea.Cmd` per call — see 1.

### D4: An unknown price is a lower bound, never zero

**Choice**: nodes without a price, clusters not connected and GCE nodes add
nothing to the total, and the total and the heading say so. `gce` (only) is
routed to a provider that prices nothing.

**Justification, strongest first**:
1. A total that looks complete and is not misleads a cost decision; Overview
   already uses `>=`.
2. `ByCloud` falls back to the AWS provider for a cloud it does not know
   (`pricing.go`), so nothing today stops it pricing a `gce://` node. Other
   unknown schemes (`kind://`) keep their current behavior, so Overview does
   not change for them.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The total of a mixed fleet with GKE reads as a bound | it is one |

**When this would be wrong**: a GCE provider ships; the route is then
replaced by it.

**Reversal action**: remove the `gce` route.

**Alternatives discarded**: hide unpriced clusters from the total silently —
the figure would look final. Route every unknown scheme to nothing — changes
Overview for `kind://`.

### D5: First-use picker, default ticks by guessed cloud

**Choice**: with no saved `fleet` list, the screen opens the picker with the
context on screen and the contexts of its guessed cloud ticked (of the
scope's cloud when the visit was `:clusters <cloud>`); `enter` saves
and connects them. Later visits connect the saved list. `unknown` matches
nothing, so a context on screen of unknown cloud ticks only itself. The
context on screen is always a section, ticked or not.

**Justification, strongest first**:
1. It is the owner's stated use ("everything in EKS") and the user sees what
   will be opened before N watches start.
2. The guess runs on the kubeconfig only (server URL for EKS/AKS, context
   name for GKE), before any connection.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A custom-named context with an IP server or a private EKS endpoint is `unknown` | the picker lists it unticked; the user ticks it |
| One extra step the first time | it prevents opening every cluster by surprise |

**When this would be wrong**: guesses are wrong often enough that users untick
half the list; then start unticked.

**Reversal action**: tick nothing by default; one line.

**Alternatives discarded**: connect first, then classify — connects every
context in the kubeconfig.

### D6: Sections paginated as one flat list of blocks

**Choice**: `clustersBody` builds, per slot, a heading block, its own
`nodeTable` rows (column widths per section) and group blocks; `paginate` is
changed to take the title row per block, so a page that starts inside a
section repeats that section's heading line (`name (cont.)`) and column
titles. One paginator, one cursor, one selected row, with keys `c:<context>`
(heading), `n:<context>/<node>`, `g:<context>/<path>`; a fold is a UI-only
set of contexts.

**Justification, strongest first**:
1. The cursor, the page, the selected-lines marker and `resolveCursor` are
   single-instance; one list keeps them.
2. Prefixed keys remove the name collisions between clusters.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| `paginate` and `block` change, which Overview also uses | golden screens of Overview must not change (T7) |
| Per-section column widths differ between clusters | each section aligns its own rows |

**When this would be wrong**: a spike (T7 starts with it) shows `paginate`
cannot carry a per-block title without a rewrite — then write a separate
paginator for this screen and leave Overview's alone.

**Reversal action**: remove `clustersBody`; restore `paginate`'s signature.

**Alternatives discarded**: one table for all clusters with a cluster
column — loses per-cluster sections and column widths.

### D7: Controller reports sync and errors by wrapping the list/watch

**Choice**: each informer's `ListerWatcher` is wrapped: a failed `List` or
`Watch` records the error (URL removed); the error is cleared when the
informer has synced or delivers its first event after the error, not when a
`Watch` call returns (a stream can open and then fail before any event).
client-go v0.37.2 turns `WatchListClient` on, so the initial sync goes over
`Watch` and falls back to `List` (`tools/cache/reflector.go`); the tests
cover both paths.
`Controller.Status()` returns `{Synced, PodsSynced, Err}`, `Synced` being
`HasSynced` of the node informer and `PodsSynced` that of the pod informer
(false while it is not started, D10), read under a mutex and false until they
are assigned. A goroutine waits for each sync and calls `Changed()`, so an
empty cluster leaves `connecting…`. A member frees its start place on
`Synced` or its first recorded error (D3).

**Justification, strongest first**:
1. `NewInformerWithOptions` returns a `cache.Controller` with no error
   handler, and `InformerOptions` has none; a wrapper works with the calls as
   they are and also tells recovery (a later success clears the error).
2. Without it "connecting", "ok" and "failed" (R2, R3, R7) cannot be told
   from a cluster that has no nodes.
3. The 4-at-a-time limit needs to know when one finished starting.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A wrapper type in `pkg/client` | small, tested with a fake clientset |
| Retry errors flicker the heading | the last error stays until a success |

**When this would be wrong**: transient errors that heal in a second make
headings noisy — then show the error only after N seconds.

**Reversal action**: drop `Status()`; headings show `ok` once a node is seen.

**Alternatives discarded**: infer from node count — an empty or unreachable
cluster look the same. Build a `cache.Config` by hand with
`WatchErrorHandlerWithContext` — more code, no recovery signal either.

### D8: Prices: a set per visit, AWS per region

**Choice**: each visit builds one price set for its members: one AWS
provider per region (EKS server URLs name it; otherwise the regional
provider, as `sessions.pricing` does), one Azure provider, the static
provider with `--disable-pricing`. The set lives with the visit's context and
is dropped when the fleet closes.

**Justification, strongest first**:
1. Providers only append `OnUpdate` callbacks; a set that outlives the visit
   would call into every closed store forever. A per-visit set is discarded
   with them.
2. AWS providers are bound to one region, so one provider for all is wrong.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Prices are fetched again on each visit | unmeasured (T12); sections show `prices unknown` and fill |
| A member with no AWS credentials is unpriced | the heading and the total say so (D4) |
| No AWS config at all (`LoadDefaultConfig` fails) | the visit's AWS members are unpriced, not failed; `Switch` keeps failing hard as today |

**When this would be wrong**: the refetch is slow enough to annoy — then
keep the set across visits and give `OnUpdate` an unregister.

**Reversal action**: hoist the set to the sessions object and add the
unregister.

**Alternatives discarded**: one provider per member — repeats downloads per
cluster of the same region.

### D9: Clusters under their cloud, a total per cloud, none across clouds

**Choice**: the screen groups the sections by cloud (`aws`, `azure`, `gce`,
then `mixed` for a cluster with nodes of two schemes, `other` for another
scheme, `-` for not known yet); the last three have no total. Each cloud has a total line; the header carries a
cost only when one cloud is on screen. No figure adds two clouds.

**Justification, strongest first**:
1. The prices are of different kinds: AWS on-demand list prices from the
   pricing API or the static tables, Azure retail prices from a public API,
   none for GCE. A sum of them reads as one bill and is not one.
2. The owner's use is one cloud at a time ("all the EKS clusters"); a cloud
   total is the figure they asked for.
3. The `>=` marks (D4) stay readable: a missing AKS cluster does not mark the
   AWS total.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| No single figure for a mixed fleet | the owner agreed (2026-11-18); a sum by hand is two numbers |
| A context of unknown cloud has no total until its nodes are seen | [ASSUMPTION] a node watch is quick (D10, T12); the header counts them |
| A cluster spanning providers has no total | it is rare; its section still shows its cost |

**When this would be wrong**: owners want a company-wide figure and accept
mixed price sources — then add a total line marked `mixed sources`.

**Reversal action**: add the line across clouds; the per-cloud lines stay.

**Alternatives discarded**: one total with a `mixed clouds` warning — the
number would be quoted without the warning. One screen per cloud — the same
as the scope (D11) with more code.

### D10: Nodes first, pods when a section is unfolded

> **Corrected 2026-11-18.** The owner chose the requests of every cluster
> at a glance over the lighter screen, which is this decision's "when this
> would be wrong". The default is now `clusters.pods: all`: a member starts
> its pod watch right after its node watch, in its own goroutine, inside the
> same 4-at-a-time limit. `clusters.pods: unfolded` keeps the choice below.
> The gate of pod figures (`podsReady`) and the Fargate-only watch apply in
> both modes; in `all` mode the Fargate watch runs until `StartPods` has
> started, as before. Memory of `all` on a large fleet is unmeasured (T12).

**Choice**: a member starts its node and nodeclaim watches only
(`Controller.StartNodes`). Its pod watch starts the first time its section is
unfolded (`Fleet.WatchPods` → `Controller.StartPods`, idempotent), and stays
until the screen is left. `WatchPods` on a member still connecting is
recorded and applied once its controller exists; a member closed by a scope
change forgets it. Sections start folded. Overview's borrowed slot already has
its pods.

Until `PodsSynced`, every figure made from pods reads `…` and orders nothing.
The list is closed (R15): idle (`IdleHourly` returns the whole price when no
pod is used, `view.go`, so it is gated, not trusted), pod counts
(`NumPods`), requests and bars (`Used`), the computed
`kube-node-viewer/node-<resource>-usage` labels (`node.go`), and the group
line's pods, usage and idle (`group.go`, `layout.go`). The gate is one
`podsReady` flag per slot read by the renderer and by the sort, not a change
to `Node`.

Pod figures also gate filters: until `PodsSynced`, a filter on a computed
usage label keeps no node and the section says `(filtered: … pods not
loaded)`, so the cost totals never come from a filter that cannot be
evaluated; the cloud's total says `N clusters pods not loaded`. A section's "name order" is a sort override applied by the
renderer while its pods are not ready (the default sort is creation time).
A member's snapshot reads `PodsSynced` first, then the stats, and carries
both in `memberSnapshotMsg`, so a frame never pairs "ready" with stats taken
before the pods arrived. Overview's borrowed slot reads `podsReady` from
`Session.Status().PodsSynced` like any slot (at start-up its pods have not
synced yet). On Overview the gate is always open, so its golden screens do
not change.

Fargate nodes are priced from their pod (`pkg/aws/pricing.go`), so a member
with a node labelled `eks.amazonaws.com/compute-type=fargate` also starts a
pod watch with the field selector `spec.schedulerName=fargate-scheduler`
([ASSUMPTION] the EKS Fargate scheduler's name, checked in T12): Fargate pods
only, one per Fargate node. It feeds the same store and runs until the
screen is left, also after `StartPods`: two informers writing one store by
`namespace/name` converge, and stopping one would leave a pod it added and
never deleted. With `--disable-pricing` it is not started.

**Justification, strongest first**:
1. The cost, the node count, the cloud and the group costs come from the
   nodes and their prices (Fargate aside, above); only idle, pod counts and
   requests need the pods.
2. [ASSUMPTION] A pod list across all namespaces is the heaviest call the
   tool makes, in memory here and on the API server, and a cluster has far
   fewer nodes than pods (T12 measures it). N clusters at once multiplies it;
   the owner asked for no bottleneck.
3. The informers are already separate (`startNodeWatch`, `startPodWatch` in
   `controller.go`), so the split is small.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Idle and pod figures read `…` until a section is unfolded and its pods synced | it says so; never `$0` or the whole price (R3, R15) |
| Unfolding waits for a pod list | seconds per cluster, unmeasured (T12); prices show at once |
| A sort on a pod field waits for the pods in that section | the section keeps name order until then (R15) |
| Unfolded pod watches stay until the screen is left | bounded by what the user unfolds; T12 decides if fold should close them |
| A Fargate-only watch per Fargate cluster | one pod per Fargate node; none without Fargate (R16) |

**When this would be wrong**: owners need idle for every cluster at a glance
more than they need a light screen — then start pods for all members, still
4 at a time. Or Fargate pods are not labelled as assumed — then Fargate nodes
stay unpriced until unfold, counted as such.

**Reversal action**: call `StartPods` in `connect` for members; one line.

**Alternatives discarded**: pods for all members — the cost the owner wants
to avoid. Pod counts from a paged list without a watch — another code path,
stale numbers. A field selector on `spec.nodeName` per Fargate node — one
watch per node instead of one per cluster.

### D11: `:clusters <cloud>` scopes the screen to one cloud

**Choice**: the `clusters` command takes an optional cloud word. With a word
only the selected contexts of that cloud are wanted; with none, all of them.
The scope is saved next to the `fleet` list.

**Justification, strongest first**:
1. The owner's main use: one cloud at once, without unticking the others in
   the picker each time.
2. Leaving the other clouds out closes their connections, so a scoped view
   is also the lightest one.
3. No new key: `:` already exists and has completion.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Contexts of unknown cloud are connected (nodes only) to learn their cloud | [ASSUMPTION] a node watch is small (T12); closed once synced when the cloud is not the scope's (R14) |
| A member that fails before syncing may belong to the scope or not | it stays under `-`, counted in the header, so the total is not silently complete |
| The command line now takes an argument | only `clusters`; others reject one as an unknown command |

**When this would be wrong**: users switch clouds so often that typing hurts —
then add a key that cycles the scope.

**Reversal action**: ignore the word; the saved scope is dropped on load.

**Alternatives discarded**: a cloud filter in `f` — filters act on nodes
inside sections, and would keep the other clouds' clusters connected.

A member's cloud is final once its node watch has synced (R3, R14): a guessed
cloud that turns out wrong, `other` and `mixed` leave a scope that is not
theirs, and the member is closed. A synced cluster with no node keeps its
guessed cloud when the guess is known (an EKS cluster scaled to zero stays
under `aws`), and is closed under a scope only when the guess was
`unknown`. A member that fails before it synced keeps the guess (or `-`) and
stays, counted. An empty synced cluster with an `unknown` guess is
remembered as "no cloud". The fleet remembers each context's learned cloud for the
visit and does not restart a context while the scope excludes its learned
cloud, so a repeated `fleetSync` does not connect it again; a scope that
includes that cloud (or none) connects it again. A first visit given a scope ticks the
scope's cloud by default (D5).

> **Corrected 2026-11-18.** The learned clouds are kept by the model
> (`fleetState.left`, read by `wantedMembers` in `fleetSync`), not by the cmd
> fleet: the fleet judges a synced member against the scope it is given and
> closes it with `MemberLeftMsg`, and the model leaves it out of the wanted
> list while the scope excludes that cloud. The behaviour is the one above.

### D12: One section per API server

**Choice**: `fleetSync` drops, before `SetWanted`, every selected context
whose API server (`ContextInfo.Server`, normalized: lower-case host, no
trailing `/`, no default `:443`) equals Overview's (`u.info.Server`) or a
context kept before it. Overview's context wins, otherwise the first by name
(the kubeconfig's contexts are a map, so file order is lost). The picker marks
the others `same cluster as <name>`. The cmd fleet stays keyed by context.

**Justification, strongest first**:
1. An EKS ARN context and a short alias of the same cluster are common in one
   kubeconfig, and D5 ticks a whole cloud; per-context sections would count
   the same nodes twice in the cloud total.
2. The server is in the kubeconfig, so it is known before connecting.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Two contexts with one server and different users (other RBAC) show one view | the cost is the cluster's, not the user's |
| A server reached through two URLs (a private and a public endpoint) is still counted twice | [ASSUMPTION] rare; the headings show both |

**When this would be wrong**: two contexts share a URL but reach different
clusters (a local proxy, `127.0.0.1` tunnels) — then key by the cluster's
`kube-system` namespace UID once connected.

**Reversal action**: key members by context name again.

**Alternatives discarded**: de-duplicate by the node provider IDs once
connected — needs both connections first.

### D13: A summary block per section, aligned across sections

**Choice**: each section's heading line is followed by Overview's summary
lines for its nodes (`writeClusterSummary`'s format: requested/allocatable,
percentage, resource, bar, and the cost on the first line). The lines of
every section go through one tab writer, so their columns line up down the
screen, then are split back into each section's heading block.

**Justification, strongest first**:
1. The owner asked for "the same format as the other": one format to read
   on both screens.
2. One tab writer gives aligned columns without a width pass of its own.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A section takes 1 + resources lines folded, not 1 | the cursor moves by sections; pages hold fewer clusters (`r` to cpu only gives 2 lines each) |
| A wide cost column on long lines | lines are cut at the window edge as elsewhere |

**When this would be wrong**: owners with 30+ clusters want them on one
page — then a compact one-line mode with a percentage per resource.

**Reversal action**: drop the block from `clustersBody`; the heading line
alone is the first build.

**Alternatives discarded**: percentages on the heading line only — not the
format asked for.

## Invariants

- Requests, never usage; read only.
- The Overview path (`c`, one connection) behaves as before; its golden
  screens do not change.
- A member's failure never stops another's or the UI.
- Overview's connection is never closed or restarted by the fleet.
- Each member has its own generation; a message of an unknown or closed
  generation is dropped, and member snapshots never write `u.stats` or
  `snapSeq`.
- Error text shown in a heading has no API server URL.
- No figure adds costs of two clouds.
- A member's full pod watch exists only after its section was unfolded
  (the Fargate-only watch aside, R16). Corrected 2026-11-18: with
  `clusters.pods: unfolded`; with `all`, at most 4 members are listing their
  pods at once, besides those whose list failed and retries.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Context missing from kubeconfig | member `error` | heading shows the reason |
| Token expired / unreachable | member `error`, others continue | heading reason, `>=` total |
| No AWS credentials for prices | nodes unpriced; one issue in the log | `prices unknown`, `>=` total |
| No `nodeclaims` CRD (not Karpenter) | as Overview: no nodeclaim watch | none |
| Empty cluster | synced with no nodes | `ok`, 0 nodes |
| Prices not yet loaded | nodes unpriced, total `>=`, updates as they arrive | `prices unknown`, then filled |
| Window narrower than the table | columns drop by priority as in Overview | `N columns hidden` |
| Many clusters, slow sync | sections fill as they come, 4 starting at a time | `connecting…` |
| `enter` twice, or on the context on screen | a pending switch ignores it; the context on screen just shows Overview | none |
| `enter` fails to connect | footer notice with the error | notice |
| Quick `tab` out while connecting | the epoch cancels the connects | no leftover watches |
| Pod watch fails after unfold | cost kept, idle `…` | heading shows the error |
| Unknown-cloud context under a scope | connected for nodes, closed once synced if not the scope's cloud | appears, then leaves |
| Unknown-cloud context fails before syncing | kept under `-` | header counts it as not connected |
| 403 on one resource, a repeated 401, exec plugin failure | that informer stopped, the others go on | heading shows the error |
| Two contexts, one API server | one section (D12) | picker says `same cluster as …` |
| Fargate nodes, folded section | Fargate-only pod watch prices them (R16) | none |

## Test plan

- `pkg/model`: key-driven tests of every row of R13 and of R1, R5, R6, R8,
  R11, R12 with a fake `Fleet`; routing by generation and that `u.stats` is
  untouched; Overview's slot is borrowed; panels offer the union of values;
  golden screens for the Clusters screen and Overview unchanged
  (`TestGoldenScreens`); total and `>=` cases (R4); cursor and fold in one
  section do not touch a same-named one in another; unknown saved screen
  opens Overview; saved `clusters` screen reaches `fleetSync` from `Init` and
  from `c`; `enter` twice sends one switch and forces Overview; cloud groups,
  one total per cloud and none across (R4, R12); `:clusters aws` scope,
  unknown word, completion, saved scope (R14); first unfold calls `WatchPods`
  once, idle `…` until pods sync (R11, R15); picker `space` on a cloud line
  (R8); pending ticks, `esc` and first-use `esc` (R8); every pod figure of
  R15's list reads `…` and sorts nothing before `PodsSynced`; header in its
  three sizes (R1); `tab` from Overview still reaches Recommendations (R1);
  `mixed`/`other`/`-` buckets have no total (R4); a scope change on the
  screen calls `fleetSync` (R14).
- `cmd`: fleet tests with a fake connector: never above 4 starting (R2), a
  failed or cancelled start frees its place, a result of an older epoch is
  closed, Overview's store not closed (R2, R9), close on leave and on quit
  (R9), one failing member (R7), `gce` node never priced by the AWS provider
  (R4), `Switch` still applies the config default view, a member starts no
  pod watch until `WatchPods` (R15; with `clusters.pods: unfolded`), pod
  lists four at a time with `all` (R18), scope change closes the other clouds'
  members (R14); a member synced with another cloud or no nodes is closed
  under a scope, one failing before sync is kept (R14); `:clusters aws` twice gives one
  connect for an excluded unknown context, and `:clusters azure` or a bare
  `:clusters` after it connects it again; an empty synced cluster keeps a
  known guess; two contexts with one
  server make one member (R17); a Fargate node prices with only the Fargate
  watch (R16); `WatchPods` before connect is applied after; a 403 stops one informer only, a 401 stops after a repeat, an exec
  failure is told by its prefix; member changes give one redraw per interval.
- `pkg/client`: controller `Status()` with a fake clientset (synced, empty
  cluster, error, recovery, no URL in the error, `PodsSynced` false until
  `StartPods`); `StartPods` twice starts one watch; error cleared on sync, not on a `Watch`
  return, over both the watch-list and the `List` path; `GuessCloud` table.
- `pkg/state`: old file without `fleet` or `fleetCloud` loads; both round-trip.
- `scripts/kind-it.sh`: unchanged.
- Manual (T12): two or more of the owner's EKS contexts from a scratch
  directory; note what was checked and the memory and time measured (node-only
  members, then with sections unfolded), not the cluster names.
