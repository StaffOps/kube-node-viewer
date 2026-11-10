# Tasks: fix-node-name-lock

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

The tests cite `// fix-node-name-lock R<n>` only once the spec is active
(spec-trace refuses citations of a draft).

- [x] T1. Unexported lock-free helpers in `pkg/model/node.go`; `Name`, `InstanceID`, `IsOnDemand`, `IsSpot`, `IsFargate`, `IsAuto`, `Labels`, `Pods`, `PodsCopy`, `InstanceType` and `NotReadyTime` take the lock once and call only helpers; `NodePrice` in `pkg/aws/pricing.go` reads the pods once; tests
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: `TestNodeLocksDoNotDeadlock`, `TestNodeReadersAreLocked`, `TestNodeLocksKeepTheirResults` (pkg/model) and `TestFargatePriceReadsThePodsOnce` (pkg/aws) under `go test -race`; each fails with its method reverted alone on a copy; the existing tests unchanged_
- [x] T2. Docs: CHANGELOG `[Unreleased]` `### Fixed`; `TODO.md`: remove the `Node.Name` read lock item, add `Ready` taking the write lock on every render of a Ready node and `Pod.FargateCapacityProvisioned` indexing a failed match; `specs/README.md` index
  - _Covers: R1, R2, R3, R4 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh`, `make lint`, `go test -race ./pkg/... ./cmd/...` green; `scripts/kind-it.sh` locally (its own kubeconfig, cluster deleted after) as the cluster run, the change not being cloud-specific; the note says the kind run hits neither the NodeClaim nor the Fargate path, which the tests cover
  - _Covers: R1, R2, R3, R4 · Depends on: T1, T2 · Verified by: command output and the kind run, recorded here as a dated note_

  > **Verified 2026-11-10.** `scripts/verify.sh` green (`go test -race`),
  > `make lint` 0 issues, `go test -race -count=1 ./pkg/... ./cmd/...` green.
  > Mutation check on a copy, each fix reverted alone, all caught: `Name`,
  > `PodsCopy` and `InstanceType` calling exported methods under the lock
  > (`TestNodeLocksDoNotDeadlock` times out); `IsOnDemand`, `IsSpot`,
  > `IsFargate`, `IsAuto`, `Labels` and `Pods` without the lock (`DATA RACE`
  > in their `TestNodeReadersAreLocked` subtest); `NotReadyTime` restored to
  > its old body (`DATA RACE`, three runs out of three); `NodePrice` reading
  > the pods twice (`TestFargatePriceReadsThePodsOnce` panics with an index out
  > of range). `TestNodeLocksKeepTheirResults` passes on the old code too, as
  > an unchanged-behaviour test should. `scripts/kind-it.sh` on a throwaway
  > three-node kind cluster (own kubeconfig, deleted afterwards): `TestKindCluster`
  > passes. That run hits neither the NodeClaim path (kind has no Karpenter)
  > nor the Fargate path; the tests above cover them.
  >
  > After the code review, on 2026-11-10: a `Created` subtest added (removing
  > its lock now gives `DATA RACE`), `Labels` checked to return the node's own
  > map, a nameless Azure row, and `NodePrice` lists the pods only for a Fargate
  > node. `go test -race ./pkg/model/ ./pkg/aws/` green.
