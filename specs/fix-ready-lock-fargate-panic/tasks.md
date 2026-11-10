# Tasks: fix-ready-lock-fargate-panic

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

The tests cite `// fix-ready-lock-fargate-panic R<n>` only once the spec is
active (spec-trace refuses citations of a draft).

- [x] T1. `pkg/model/node.go`: unexported `isReady`; `Update` clears the NodeClaim creation time when the new object is Ready; `Ready` only reads. `pkg/model/pod.go`: `FargateCapacityProvisioned` returns after logging an unparsable annotation; tests
  - _Covers: R1, R2, R3 · Depends on: — · Verified by: `TestReadyTakesNoWriteLock`, `TestReadyClearedOnUpdate`, `TestReadyClearedThroughTheStore`, `TestFargateCapacityMalformed` (pkg/model) and `TestFargatePriceMalformed` (pkg/aws) under `go test -race`; each fails on the old code; `TestNodeReadersAreLocked`'s comment and writer updated, the other existing tests unchanged_
- [x] T2. Docs: CHANGELOG `[Unreleased]` `### Fixed`; `TODO.md`: remove the `Node.Ready` and `Pod.FargateCapacityProvisioned` items, add the NodeClaim stub that can overwrite a registered node (review finding, out of scope); `specs/README.md` index
  - _Covers: R1, R2, R3 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh`, `make lint`, `go test -race ./pkg/... ./cmd/...` green; `scripts/kind-it.sh` locally (its own kubeconfig, cluster deleted after) as the cluster run; the note says what the kind run does not reach (no NodeClaims, no Fargate)
  - _Covers: R1, R2, R3 · Depends on: T1, T2 · Verified by: command output and the kind run, recorded here as a dated note_

  > **Verified 2026-11-10.** `scripts/verify.sh` green (`go test -race`),
  > `make lint` 0 issues, `go test -race -count=1 ./pkg/... ./cmd/...` green.
  > On a copy with `node.go` and `pod.go` from e553dc8: `TestReadyTakesNoWriteLock`
  > times out after 5 s, `TestReadyClearedOnUpdate` and both
  > `TestReadyClearedThroughTheStore` subtests read the NodeClaim time,
  > `TestFargateCapacityMalformed/2vCPU` and `TestFargatePriceMalformed` panic
  > with `index out of range [1] with length 0`; spec-refuter repeated those
  > runs. `TestNodeReadersAreLocked` and `TestNodeLocksKeepTheirResults` pass
  > on both. `scripts/kind-it.sh` on a throwaway kind cluster (own kubeconfig,
  > deleted afterwards): `TestKindCluster` passes; it reaches `Ready` and
  > `Update` on registered nodes, but neither the NodeClaim path (kind has no
  > Karpenter) nor the Fargate path, which the tests above cover.
