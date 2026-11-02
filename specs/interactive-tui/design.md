# Design: interactive-tui

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Watches | three `cache.NewInformer` (deprecated in client-go v0.37): nodes and nodeclaims with the server-side selector, all pods cluster-wide; handlers write straight into `model.Cluster`; nodes with an empty `providerID` are dropped | `pkg/client/controller.go:69-194,72,116,125` |
| Startup | `controller.Start` runs before the Bubble Tea program exists and does a blocking nodeclaims `Get` | `main.go:106-110`, `controller.go:64` |
| Locking | one global `sync.RWMutex` on `Cluster`, one per `Node`; `GetPod` takes the write lock for a read; `SetPrice` writes `n.Price` unlocked while the view reads it | `cluster.go:25-30,128`, `node.go` `SetPrice` |
| Lookups | `GetNodeByName` is a linear scan per pod event; `DeleteNode` scans all pods; a pod seen before its node is never bound to it | `cluster.go:39-48,59-63,85-94,105-108,116` |
| Redraw | `tickCmd` every 100 ms; each `View()` runs `Stats()` over all pods and nodes, re-sorts, regroups and renders an extra node to measure page size | `uimodel.go:158,175,392-396,410-437` |
| Pod fields used | requests incl. init containers and pod-level `Spec.Resources`; `deletionTimestamp`; annotation `CapacityProvisioned` for Fargate type and price | `pod.go` `Requested`, `FargateCapacityProvisioned`; `controller.go:224`; `node.go:257-260`; `pkg/aws/pricing.go:79-85` |
| Filters | only `--node-selector` (server-side); `--group-by a=b,c` values are merged into it | `main.go:93,139-161` |
| Runtime keys | `q`/`esc`/`ctrl+c` quit, `g` toggle summary, `n` toggle nodes, paging | `uimodel.go:423-434` |
| Flags vs config | config values become flag defaults, so explicit flags are indistinguishable after parsing; built-in defaults `resources=cpu`, `node-sort=creation=dsc` | `flag.go:76-105` |
| Pricing | `Provider.NodePrice → (float64, bool)`; reserved returns NaN; region from the AWS SDK config | `pkg/pricing/pricing.go`, `pkg/aws/pricing.go:60-87,145-158` |

## Target state

```
main: program := tea.NewProgram(ui)  ──►  controller.Start(ctx, sender=program.Send) in a goroutine
client-go informers (WithOptions + Transform) ──events{gen}──► model.Store (indexed, one lock)
        │                                                              │
        └── coalescer: ≤1 "changed{gen}" per 100 ms burst ──► program.Send
pricing OnUpdate ─────────────────────────────────────────────────────┘
UI (Bubble Tea): ViewState{filters, groupBy[≤3], sort, resources, columns, search, showNodes}
  on changed{gen==current} or key: rows = filter → group → sort over store.Snapshot()
  1 s tick: re-render ages/durations only
  panels: f g s o c / r n ? ; esc closes; q/ctrl+c quit from main view
state: debounced atomic writer ◄── ViewState per cluster key
```

- Pod totals are maintained incrementally per node; rendering never walks pods.
- Pods whose node is not known yet are indexed by node name and bound when the node (or its nodeclaim's registered name) appears.
- Derived node facts (capacity type, node group, zone, region, IP, status flags) are computed once per node update.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/store.go` (new) | indexes: nodes by providerID and name, pods by key, pending pods by node name; per-node totals; generation; `Snapshot()` |
| `pkg/model/cluster.go`, `node.go`, `pod.go` | fold into the store; fix `SetPrice`/`GetPod` locking; derived facts |
| `pkg/model/view.go` (new) | `ViewState`, filters, nested grouping, sort, search, column priority |
| `pkg/model/uimodel.go`, `pkg/model/panels/` (new) | panels, keys, header, pagination by lines, 1 s tick |
| `pkg/client/controller.go` | `cache.NewInformerWithOptions` with `Transform`; events tagged with generation; coalescer with a sender; nodeclaims probe moved off the UI path |
| `pkg/pricing/pricing.go`, `pkg/aws/pricing.go` | price kind (actual vs equivalent); reserved → on-demand equivalent; provider re-created per region on context switch |
| `pkg/state/` (new) | per-cluster state file, debounced atomic writer, tolerant reader |
| `cmd/kube-node-viewer/main.go`, `flag.go` | program-first wiring; explicit-flag detection; flag table (D8) |
| `README.md`, `CHANGELOG.md` | keys and panels, state file, changed `g` and `esc` |

## Decisions

### D1: View changes never restart watches

**Choice**: watch all nodes (within `--node-selector`) and every pod; filters,
grouping, sort and search work on memory.

**Justification, strongest first**:
1. Restarting informers re-lists everything: seconds of blank screen on big clusters; in memory it is O(nodes).
2. Value pickers need counts for values currently filtered out.

**Trade-offs accepted**: memory for nodes outside the current filter.

**When this would be wrong**: clusters too large to watch fully from a laptop — then `--node-selector` is the documented way to scope.

**Reversal action**: push label filters into the informer `LabelSelector` and restart on change.

### D2: Change-driven, coalesced redraw plus a slow tick

**Choice**: events update the store and mark it dirty; a coalescer sends one
`changed{gen}` per burst (≤ every 100 ms) through `program.Send`; pricing
`OnUpdate` does the same; a 1 s tick only re-renders ages and durations.

**Justification, strongest first**:
1. Today every 100 ms tick recomputes stats over all pods; idle clusters cost CPU and big clusters lag.
2. Same pattern as sofka: drain queued changes, draw once.

**Trade-offs accepted**: ≤100 ms between an event and the screen.

**When this would be wrong**: if Bubble Tea's frame limiter makes the coalescer redundant — then send on every event.

**Reversal action**: restore the 100 ms `tickCmd`.

### D3: Pods reduced at the informer to what totals need

**Choice**: `cache.NewInformerWithOptions` with a `Transform` that keeps
namespace, name, `spec.nodeName`, `status.phase`, `deletionTimestamp`, the
`CapacityProvisioned` annotation, pod-level `spec.resources`, `spec.overhead`,
and for containers and init containers only `resources.requests` and
`restartPolicy`; tombstones (`DeletedFinalStateUnknown`) pass through.

**Justification, strongest first**:
1. The tool never shows pods; full pod objects are most of the memory (stored twice today).
2. Totals make render cost independent of pod count (R11).

**Trade-offs accepted**: a future pod feature must widen the transform.

**When this would be wrong**: if a feature needs pod details — then widen for that feature.

**Reversal action**: remove the transform.

### D4: Filter model — AND across fields, OR within a field, toggleable

**Choice**: a filter is `{field, values[], enabled}`; fields are a label key or
a derived fact; status matches on any selected flag.

**Justification**: matches how people phrase it and Kubernetes `in` selectors; toggling keeps comparisons quick.

**Trade-offs accepted**: no OR across fields, no negation in stage 1.

**When this would be wrong**: real queries need negation or cross-field OR — add negation per filter first.

**Reversal action**: none needed; the model extends.

### D5: Node group and capacity type derivation

**Choice**: node group = `karpenter.sh/nodepool`, else
the `eks.amazonaws.com/nodegroup` name (coloured on screen; see the R7
correction of 2026-11-02), else `-`. Capacity type
from the labels in R7, Fargate/Auto from `compute-type` first.

**Justification**: these are the labels EKS and Karpenter v1 set (Karpenter `pkg/apis/v1/labels.go`).

**When this would be wrong**: GKE/AKS pools (stage 2) — add `cloud.google.com/gke-nodepool`, `kubernetes.azure.com/agentpool`.

**Reversal action**: drop the managed node group branch.

### D6: State file per cluster, separate from config

**Choice**: `$XDG_STATE_HOME/kube-node-viewer/state.json` (default
`~/.local/state/kube-node-viewer/state.json`), keyed by context name plus API
server URL (merged kubeconfigs can repeat context names), written atomically
(temp file + rename) by a debounced goroutine.

**Justification, strongest first**:
1. The config file is user-owned; generated state would rewrite their comments.
2. Atomic write survives being killed mid-write.

**Trade-offs accepted**: a second file; documented in README.

**When this would be wrong**: users want versioned defaults — they keep the config file.

**Reversal action**: stop writing; delete the file.

### D7: Key scheme

**Choice**: `f` Filters, `g` Group by, `s` Sort, `o` Columns, `c` Context,
`/` search, `r` resources, `n` node list, `?` help; panels and inputs capture
keys; `esc` closes; `q`/`ctrl+c` quit from the main view only.

**Justification**: the owner chose `f`/`g`; one key per panel; `esc` is needed to close panels.

**Trade-offs accepted**: `g` (toggle summary) and `esc` (quit) change meaning; CHANGELOG and README say so.

**When this would be wrong**: if hiding the summary alone is needed often — add it to the Group by panel.

**Reversal action**: remap keys in one table.

### D8: Startup precedence and flag table

**Choice**: explicit flag (detected with `flagSet.Visit`) > saved state >
config file > built-in default. Explicit flags apply to the run and are saved
only after a TUI change (R2).

| Flag / config key | View state | Saved |
|---|---|---|
| `--node-selector` | server-side scope (not view state) | no |
| `--group-by key` | group-by level 1 = `key` | via state |
| `--group-by key=a,b` | group-by level 1 = `key` + filter `key ∈ {a,b}` (in memory, no longer merged into the selector) | via state |
| `--groups-only` | `showNodes=false` | via state |
| `--node-sort creation\|<label>[=asc\|dsc]` | sort | via state |
| `--extra-labels a,b` (incl. computed names, both prefixes) | extra columns | via state |
| `--resources list` | resources (R9 fourth entry when not cpu/memory) | via state |
| `--context`, `--kubeconfig` | cluster selection | no |
| `--style`, `--disable-pricing` | run options | no |

**Justification**: flags are explicit for this run; saved state is the last TUI choice; config is a standing default.

**When this would be wrong**: users expect the config file to win — add a reset key that clears the cluster's state.

**Reversal action**: swap the middle layers.

### D9: Generations against stale events

**Choice**: each context gets a generation number; informer handlers, the
coalescer and pricing tag what they send; the store and UI drop anything with
an older generation. Watches start in a `tea.Cmd`, never inside `Update`.

**Justification**: cancelling informers does not stop handlers already running; without tags old-cluster data can reappear (R10).

**When this would be wrong**: never for correctness; only if switching is removed.

**Reversal action**: remove context switching.

### D10: Price kind

**Choice**: `pricing.Provider` gains a price kind (`Actual`, `Equivalent`);
reserved nodes get the on-demand price as `Equivalent`; totals sum `Actual`
only and count `Equivalent` nodes.

**Justification**: R8 forbids showing reserved cost as real; the current `(float64, bool)` can't express it.

**When this would be wrong**: once billing data is integrated — then reserved gets `Actual`.

**Reversal action**: drop the kind; reserved shows unknown again.

> **Corrected 2026-10-31 (T7).** The kind lives on `model.Node`
> (`SetEquivalentPrice`, `PriceIsEquivalent`), set by the controller from the
> node's capacity type, instead of in `pricing.Provider`. Whether a price is
> real depends on how the node is bought, not on the price source, and stage 2
> providers then need no extra method. An `instance-price` override label
> stays an actual price.

## Invariants

- Requests vs allocatable only, never actual usage.
- Existing flags, config keys and label prefixes keep working (D8).
- Render cost does not depend on pod count.
- No in-cluster dependency; no new runtime dependency beyond Bubble Tea `bubbles` components already used.

## Test plan

- `scripts/verify.sh` and `go test -race ./pkg/... ./cmd/...` green after every task.
- Unit: store totals (add/update/delete/move, pod before node, nodeclaim then node, Fargate annotation kept); transform output; filters; grouping 1–3 with `<none>`; sort; search; derived facts incl. `CAPACITY_BLOCK`; price kind and totals; state round-trip, corrupt file, per-cluster key; flag table rows; generation drops stale events.
- UI: key-driven tests through `Update(tea.KeyMsg)` (no `teatest` dependency); a held store lock does not block a panel opening.
- Benchmark (T1 before, T12 after): synthetic store with 500 nodes and 15 000 / 150 000 pods; event apply time, snapshot+render time, heap.
- Manual: a real cluster and a context switch, named by the owner.
