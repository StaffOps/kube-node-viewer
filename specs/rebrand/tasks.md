# Tasks: rebrand

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
>
> Every task that edits an upstream `.go`/`.sh` file adds the D5 note in the same commit.

- [x] T0. Owner answers the open questions; spec-refuter review applied
  - _Covers: — · Depends on: — · Verified by: review date in the spec header_
- [ ] T1. Create `feat/rebrand` from `feat/sule26-group-by` (D7); rename module path (exact string, D1) in `go.mod`, imports, `.golangci.yaml`
  - _Covers: R1 · Depends on: T0 · Verified by: `scripts/verify.sh`; R1 grep empty_
- [ ] T2. `git mv cmd/eks-node-viewer cmd/kube-node-viewer`; update `Makefile`, `.goreleaser.yaml`, root and `cmd/` `.gitignore`, help/version text; build-info fallback for `-version` + test
  - _Covers: R2 · Depends on: T1 · Verified by: `scripts/build.sh && bin/kube-node-viewer -version`; `go test ./cmd/...`_
- [ ] T3. Config file fallback with stderr warning; inject home dir; tests
  - _Covers: R3 · Depends on: T2 · Verified by: `go test ./cmd/...`_
- [ ] T4. Price label lookup with both keys (R4a) + computed-name prefix helper used by `--extra-labels`, `--node-sort`, `--group-by` (R4b); tests
  - _Covers: R4a, R4b · Depends on: T1 · Verified by: `go test ./pkg/...`_
- [ ] T5. Remove `hack/homebrew.go`; update `hack/attribution.tmpl` wording
  - _Covers: R6 · Depends on: T2 · Verified by: `grep -rn 'aws/tap' .` empty_
- [ ] T5b. Regenerate attribution in its own commit: `hack/gen_licenses.sh && go generate ./cmd/...` (needs `go-licenses`; no pricing refresh)
  - _Covers: R2 · Depends on: T2, T5 · Verified by: `grep -c eks-node-viewer ATTRIBUTION.md cmd/kube-node-viewer/ATTRIBUTION.md` is 0 for both_
- [ ] T6. Governance files: `CODEOWNERS` = `* @karlipegomes @sule26`, delete `CODE_OF_CONDUCT.md`, rewrite `CONTRIBUTING.md`
  - _Covers: R6 · Depends on: T0 · Verified by: GitHub shows no CODEOWNERS errors on the file page_
- [ ] T7. CI: `test.yaml` runs `scripts/verify.sh` + `scripts/build.sh`; `release.yaml` drops `make generate` and `git diff-index` (D6)
  - _Covers: R7 · Depends on: T2 · Verified by: `goreleaser check && goreleaser build --snapshot --clean` → `dist/kube-node-viewer_*`_
- [ ] T8. New `README.md` (usage, install, credits, badges) + `CHANGELOG.md` entry
  - _Covers: R5 · Depends on: T2–T7 · Verified by: review_
- [ ] T9. `NOTICE` line; check D5 notes on every modified upstream file; run the design name sweep
  - _Covers: R8 · Depends on: T1–T8 · Verified by: R8 checks; name sweep allowlist_

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Homebrew tap via goreleaser `brews:` | a `staffops/tap` repository exists |
| Drop `eks-node-viewer/*` prefix fallback | a major release, and no known user carries the old names |
| `golangci-lint run` in `scripts/verify.sh` | `golangci-lint` is installed locally and in CI |
| Scheduled price-table refresh PR | stale prices are reported |
