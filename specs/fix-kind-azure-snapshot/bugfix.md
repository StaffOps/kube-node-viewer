# Bugfix: kind-azure-snapshot

> Status: active · Reported: 2026-11-24 UTC (the `kind` job of three PR runs, 2026-11-18 to 2026-11-23, each passing on a rerun) · Reviewed: 2026-11-24 (spec-refuter; 4 LOW, applied)

## Current behavior

`TestKindClusterAzureSizes` (`cmd/kube-node-viewer/kind_integration_test.go`)
fails now and then, in CI and locally, with

    kind_integration_test.go:484: no permission line on the Recommendations screen:
    … Nodes:    0 of 0 shown · 0 pods (2 pending) · $0.000/month …
      Nothing to recommend for the nodes shown.

The screen lists no node at all, although the store holds kind's three nodes
and the two made-up AKS nodes: `connectAzure` waited for them
(`first.Cluster.Stats().NumNodes >= 5`) before it returned. With no AKS node
on screen there is no Azure pair to read, so the fake Resource Manager is never
asked and the permission line never comes; the test waits its minute and fails.

Seen in the `kind` job of the pull-request runs 44364267057 (2026-11-18),
44792713162 (2026-11-20) and 45273205953 (2026-11-23), all with the same screen. Reproduced
on 2026-11-24 on a local three-node kind cluster made by `scripts/kind-it.sh`
(the first run failed the same way), then with `go test -tags integration -race
-count=10 -run 'TestKindClusterAzureSizes$'` and a throwaway log of the
`Nodes:` header right after `connectAzure`: see the dated note in
[tasks](tasks.md).

## Expected behavior

**R1** WHEN `TestKindClusterAzureSizes` has connected and the store holds every
node THEN the test SHALL apply a snapshot of the store taken after that point to
the model before it opens the Recommendations screen, and SHALL fail at once,
with the screen, when the header does not list every node of the store.

_Test: `// fix-kind-azure-snapshot R1` on `TestKindClusterAzureSizes`, run by
`scripts/kind-it.sh` and the `kind` job; seen failing before the fix and
passing after it in the dated note of [tasks](tasks.md)._

## Unchanged behavior

- The product code is untouched: only the integration test changes.
- `TestKindCluster` and its `connect` helper are unchanged; `connect` already
  runs the command its `ChangedMsg` returns.
- What `TestKindClusterAzureSizes` checks after connecting is unchanged: the
  permission line, the request it makes, the retry within a minute (R6 of
  [azure-live-skus](../azure-live-skus/requirements.md)) and the arm row.

## Root cause

The model's node list is its last snapshot (`u.stats`), not the store. A
snapshot is taken only by the command a `ChangedMsg` returns (`snapshotCmd`,
`pkg/model/uimodel.go`), which the program runs on another goroutine and
applies as a `snapshotMsg`. The session's notifier sends at most one
`ChangedMsg` per `changeInterval` (100ms, `cmd/kube-node-viewer/main.go`), so
the change that brings the last nodes can still be waiting, or its snapshot
still running, when `connectAzure`'s poll of the store sees five nodes and
quits the program. The model then keeps the last snapshot it applied (the one
from `Init`, taken while the informers were still listing) or none, since
`Connect` clears it: 0 nodes either way, as in the failing runs.

`connectAzure` then calls `u.Update(model.ChangedMsg{…})` to catch up, but
drops the command it returns, so no snapshot is taken: nothing afterwards
asks for one (the test drives only window sizes and `tab`, and the stopped
program no longer receives the session's changes). `connect`, used by
`TestKindCluster`, runs that command, which is why that test does not flake.

The fix runs the returned command through `drive`, as every other message of
the test is run, and checks the header against the store's node count, so a
missed snapshot fails at once with its own message instead of a minute later
on a symptom. `drive` waits at most a second for the snapshot; one slower than
that is dropped, and the header check then fails with the screen rather than
flaking a minute later.
