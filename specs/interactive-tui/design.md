# Design: interactive-tui

> Implements [requirements](requirements.md).

## Current state

| Area | Today | Where |
|------|-------|-------|
| Watches | three `cache.NewInformer` (nodes with server-side selector, all pods cluster-wide, nodeclaims if the CRD answers); handlers write straight into `model.Cluster` | `pkg/client/controller.go:70-160` |
| Locking | one global `sync.RWMutex` on `Cluster`, one per `Node`; `GetPod` takes the write lock for a read; `SetPrice` writes `n.Price` unlocked while the view reads it | `pkg/model/cluster.go:25-30,128`, `node.go` `SetPrice` |
| Lookups | `GetNodeByName` is a linear scan, called on every pod add/delete; `DeleteNode` scans all pods | `cluster.go:59-63,85-94,105,116` |
| Redraw | `tickCmd` every 100 ms; each `View()` runs `Stats()` over all pods and nodes, re-sorts, regroups and renders an extra node to measure page size | `uimodel.go:158,175,392-396,410-437` |
| Filters | only `--node-selector` (server-side); `--group-by a=b,c` values are merged into it; `Node.Visible`/`Hide` exists but is unused | `main.go:93,139-161`, `cluster_test.go:268` |
| Runtime keys | `q`, `g` (toggle summary), `n` (toggle nodes), paging | `uimodel.go:423-434` |
| Capacity type | `IsSpot`/`IsOnDemand` from Karpenter or EKS labels; `reserved` unknown; managed node group not shown | `node.go:81-97` |
| Config | read-only `~/.kube-node-viewer` (or legacy), flag > config | `cmd/kube-node-viewer/flag.go` |

## Target state

```
client-go informers ──events──► model.Store (indexed, own lock)
        │                             │
        └── coalescer: 1 "changed" msg per ≤100 ms burst ──► tea.Program.Send
                                      │
UI (Bubble Tea) ◄─────────────────────┘
  ViewState {filters, groupBy[≤3], sort, resources, columns, search}
  on change msg or key: snapshot = store.Snapshot()  (O(nodes), pod totals precomputed)
                        rows = filter → group → sort → page   (O(nodes log nodes))
  panels: f Filters · g Group by · s Sort · o Columns · c Context · / search · r resources · n nodes · ? help
state file ◄── debounced writer (goroutine) ── ViewState per context
```

- **Pod totals are maintained incrementally**: on pod add/update/delete the
  store adjusts the owning node's request totals and pod count. Rendering never
  walks pods.
- **Node rows carry derived facts** computed once per node update: capacity
  type, node group, zone, region, internal IP, instance type, status.
- **Filters, grouping, sort and search run over the node snapshot** in the UI
  goroutine. Watches are untouched by view changes.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/store.go` (new) | indexed store: nodes by name and providerID, pods by key → node, per-node running totals; one lock; `Snapshot()` |
| `pkg/model/cluster.go`, `node.go`, `pod.go` | fold into the store; fix `SetPrice` race; derived node facts (`CapacityType`, `NodeGroup`, zone, region, IP, status) |
| `pkg/model/view.go` (new) | `ViewState`, filter evaluation (AND across fields, OR within), nested grouping ≤3, sort by column/label, search |
| `pkg/model/uimodel.go` + `pkg/model/panels/` (new) | panels and keys from R3–R12, header line with active state, change-driven redraw |
| `pkg/client/controller.go` | write to the store; coalesced change notification; pod informer transform that keeps only fields needed for totals; context switch = stop and restart watches |
| `pkg/state/` (new) | per-context state file, debounced writer, tolerant reader |
| `cmd/kube-node-viewer/main.go`, `flag.go` | flags become overrides of restored state; context picker wiring |
| `README.md`, `CHANGELOG.md` | keys, panels, state file, changed meaning of `g` |

## Decisions

### D1: View changes never restart watches

**Choice**: watch all nodes (within `--node-selector` if given) and every
pod; apply filters, grouping, sort and search in memory.

**Justification, strongest first**:
1. Restarting informers on every filter change means a full re-list and seconds of blank screen on big clusters; in memory it is O(nodes).
2. Value pickers need counts for values that are currently filtered out.

**Trade-offs accepted**: memory for nodes outside the current filter.

**When this would be wrong**: clusters so large that watching all nodes is too
heavy for a laptop — then make `--node-selector` (server-side) the documented
way to scope.

**Reversal action**: push the label part of filters into the informer's
`LabelSelector` and restart on change.

### D2: Change-driven, coalesced redraw instead of a 100 ms tick

**Choice**: the controller sends one `changed` message per burst (at most every
100 ms) through `tea.Program.Send`; `View()` renders a cached row list rebuilt
only on `changed` or a key.

**Justification, strongest first**:
1. Today every tick recomputes stats over all pods; idle clusters cost CPU and big clusters lag.
2. Same pattern as sofka (drain queued messages, then draw once).

**Trade-offs accepted**: a ≤100 ms delay between a cluster event and the screen.

**When this would be wrong**: if Bubble Tea's own frame rate makes the coalescer redundant — then drop it and send on every event.

**Reversal action**: restore `tickCmd`.

### D3: Pods are reduced to request totals at the informer

**Choice**: an informer `TransformFunc` keeps only namespace, name, node name,
phase and container/init/overhead requests; the store keeps per-node totals.

**Justification, strongest first**:
1. The tool never shows pods; full pod objects are most of today's memory (they are stored twice: informer cache and `Cluster.pods`).
2. Totals make render cost independent of pod count (R10).

**Trade-offs accepted**: adding a pod view later means widening the transform.

**When this would be wrong**: if a later feature needs pod details (logs, owner) — then widen the transform for that feature only.

**Reversal action**: remove the transform.

### D4: Filter model — AND across fields, OR within a field, toggleable

**Choice**: a filter is `{field, values[], enabled}`; fields are a label key or
a derived fact (capacity type, node group, zone, region, instance type,
status).

**Justification, strongest first**:
1. Matches how people say it ("spot nodes in default or gpu"), and how Kubernetes `in` selectors work.
2. Toggling without deleting makes comparisons quick.

**Trade-offs accepted**: no OR across different fields, no negation in stage 1.

**When this would be wrong**: if real use needs "spot OR nodepool=gpu" or "NOT zone=a" — then add negation per filter first, OR groups later.

**Reversal action**: none needed; the model extends.

### D5: Node group column

**Choice**: `karpenter.sh/nodepool` value; else `ManagedNodeGroup:<eks.amazonaws.com/nodegroup>`; else `-`.

**Justification**: those are the two ways EKS nodes get grouped; both are labels already on the node.

**When this would be wrong**: GKE/AKS node pools (stage 2) — then add their labels (`cloud.google.com/gke-nodepool`, `kubernetes.azure.com/agentpool`) to the same column.

**Reversal action**: drop the managed node group branch.

### D6: State file per context, separate from config

**Choice**: `$XDG_STATE_HOME/kube-node-viewer/state.json` (default
`~/.local/state/kube-node-viewer/state.json`), keyed by context name, written
atomically (temp file + rename) by a debounced goroutine.

**Justification, strongest first**:
1. The config file is user-owned (R2 counter-example); mixing generated state into it would rewrite their comments.
2. Atomic write avoids a half-written file if the tool is killed.

**Trade-offs accepted**: two files to know about; documented in README.

**When this would be wrong**: if users want to version their defaults — then
they keep using the config file, which still applies under flags.

**Reversal action**: stop writing; delete the file.

### D7: `g` opens the Group by panel

**Choice**: `g` changes meaning from "toggle summary" to "Group by panel"; the
summary shows whenever a group-by is set and `n` hides/shows the node list.

**Justification**: the owner chose `f`/`g` as the main keys; one key per panel
keeps the scheme learnable.

**Trade-offs accepted**: a key users of `--group-by` learned changes; noted in CHANGELOG and README.

**When this would be wrong**: if hiding the summary alone is needed often —
then add it to the Group by panel.

**Reversal action**: map the toggle back to `g` and move the panel.

### D8: Initial precedence

**Choice**: flag > restored state > config file > defaults.

**Justification**: flags are explicit for this run; restored state is the
user's last choice; the config file is a standing default.

**When this would be wrong**: if users expect the config file to win over
remembered state — then add a `reset` key that clears state for the context.

**Reversal action**: swap the two middle layers.

## Invariants

- Requests vs allocatable only, never actual usage.
- Existing flags, config keys, label prefixes (`kube-node-viewer/`, `eks-node-viewer/`) keep working.
- Rendering cost does not depend on pod count.
- No in-cluster dependency.

## Test plan

- `scripts/verify.sh` green after every task; `go test -race ./pkg/... ./cmd/...` green (the `SetPrice` race is fixed in T2).
- Unit: store totals under add/update/delete/move pods; filter AND/OR/disabled; grouping 1–3 levels with `<none>`; sort per column and label; search; capacity type and node group derivation; state round-trip, corrupt file.
- UI: key-driven tests with `teatest` (or `Update` with `tea.KeyMsg`) for each panel.
- Benchmark (T1 before, T9 after): synthetic store with 500 nodes / 15 000 pods and 5 000 nodes / 150 000 pods; event apply time, snapshot+render time, heap. Sizes are test inputs, not targets; results recorded in tasks.md.
- Manual: one real cluster named by the owner.
