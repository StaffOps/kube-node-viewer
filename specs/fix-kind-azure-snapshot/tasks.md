# Tasks: fix-kind-azure-snapshot

> Implements [bugfix](bugfix.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [x] T1. `connectAzure` runs the command of its `ChangedMsg` with `drive` and fails when the header does not list the store's nodes
  - _Covers: R1 · Depends on: — · Verified by: `TestKindClusterAzureSizes` on a kind cluster, before and after, recorded in T3_
- [x] T2. Docs: `specs/README.md` index; no CHANGELOG entry (no change a user of the binary sees)
  - _Covers: R1 · Depends on: T1 · Verified by: review_
- [x] T3. Verify: `scripts/verify.sh`, `make lint`; `TestKindClusterAzureSizes` repeated on a local kind cluster before and after the fix; the PR's `kind` job
  - _Covers: R1 · Depends on: T1, T2 · Verified by: command output, recorded here as a dated note_

  > **Verified 2026-11-24.** On a local three-node kind cluster from
  > `scripts/kind-it.sh` (own kubeconfig, deleted afterwards). Before the fix
  > (origin/main plus a throwaway log of the `Nodes:` header right after
  > `connectAzure`): the first `scripts/kind-it.sh` run failed as in CI
  > (`Nodes: 0 of 0 shown`). Then `go test -tags integration -race -count=10
  > -run 'TestKindClusterAzureSizes$'`: 4 of the 10 runs started from a stale
  > snapshot (`2 of 2 shown`, only the made-up AKS nodes, three times; `5 of 5
  > shown · 0 pods` once), so whether the test passed depended on which
  > snapshot was last. The tenth run hit `go test`'s default 10-minute timeout,
  > since each run waits about a minute for the retry. After the fix, the same
  > command with `-timeout 30m`: 10 of 10 pass, every one past the new header
  > check. `scripts/verify.sh` ok, `make lint` 0 issues,
  > `go vet -tags integration ./cmd/kube-node-viewer` clean. The PR's `kind`
  > job runs the test on CI.
