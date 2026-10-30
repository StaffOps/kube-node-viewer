# Tasks: rebrand

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done

- [-] T0. Owner answers the `[TO CONFIRM]` questions; spec-refuter review
  - _Covers: — · Depends on: — · Verified by: review date in the spec header_
- [ ] T1. Rename module path in `go.mod`, imports, `.golangci.yaml`
  - _Covers: R1 · Depends on: T0 · Verified by: `scripts/verify.sh`_
- [ ] T2. `git mv cmd/eks-node-viewer cmd/kube-node-viewer`; update Makefile, goreleaser, release workflow, `.gitignore`, help/version text
  - _Covers: R2, R7 · Depends on: T1 · Verified by: `scripts/build.sh && bin/kube-node-viewer -version`_
- [ ] T3. Config file fallback with warning + tests
  - _Covers: R3 · Depends on: T2 · Verified by: `go test ./cmd/...`_
- [ ] T4. Dual label prefix helper + tests
  - _Covers: R4 · Depends on: T1 · Verified by: `go test ./pkg/...`_
- [ ] T5. Remove `hack/homebrew.go`; update `hack/attribution.tmpl` wording
  - _Covers: R6 · Depends on: T2 · Verified by: grep check in the design test plan_
- [ ] T5b. Regenerate `ATTRIBUTION.md` (root + `cmd/`) with `hack/gen_licenses.sh`, as its own commit (needs `go-licenses`; do not refresh pricing in the same commit)
  - _Covers: R2 · Depends on: T2, T5 · Verified by: `grep -c eks-node-viewer ATTRIBUTION.md` is 0_
- [ ] T6. Governance files: delete `CODEOWNERS` and `CODE_OF_CONDUCT.md`, rewrite `CONTRIBUTING.md`
  - _Covers: R6 · Depends on: T0 · Verified by: review_
- [ ] T7. `NOTICE` line + modification notes on changed upstream files
  - _Covers: R8 · Depends on: T1–T5b · Verified by: `git diff af96088 -- LICENSE` empty_
- [ ] T8. New `README.md` (usage, install, credits) + `CHANGELOG.md`
  - _Covers: R5 · Depends on: T2–T7 · Verified by: review_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Homebrew tap via goreleaser `brews:` | a `staffops/tap` repository exists |
| `CODEOWNERS` | a second maintainer joins, or branch protection requires code-owner review |
| Drop `eks-node-viewer/*` label fallback | a major release, and no known user carries the old labels |
