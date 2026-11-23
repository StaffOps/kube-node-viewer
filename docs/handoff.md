# Handoff

Last updated: 2026-11-23 by Backstage docs-standard pilot · branch `main`

## State

Latest release is **v0.17.0** (2026-11-18), published by `release.yaml` with binaries and the
Homebrew cask; `[Unreleased]` in the
[CHANGELOG](https://github.com/StaffOps/kube-node-viewer/blob/main/CHANGELOG.md) is empty. The tool
is a Go (Bubble Tea) terminal UI showing pod requests against node allocatable capacity and node
cost, derived from `awslabs/eks-node-viewer`; see the
[README](https://github.com/StaffOps/kube-node-viewer/blob/main/README.md). Flow, per
[AGENTS.md](https://github.com/StaffOps/kube-node-viewer/blob/main/AGENTS.md): branch -> PR -> CI ->
automerge -> weekly release on Mondays when `[Unreleased]` has entries. CI: the last push run on
`main` (v0.17.0) passed; the weekly scheduled run of 2026-11-23 failed only in the `tools` job,
which reports pinned CI tools behind upstream: zizmor 1.31.1 (1.32.0), syft 1.55.0 (1.57.1),
goreleaser 2.19.0 (2.20.0). OpenSSF Scorecard passes.

## Done

- v0.17.0: Clusters screen (`:clusters`), `contexts.allow` / `contexts.deny`, GKE nodes no longer priced with AWS prices (`b4fadcf`, PRs [#83](https://github.com/StaffOps/kube-node-viewer/pull/83), [#85](https://github.com/StaffOps/kube-node-viewer/pull/85))
- Backstage `catalog-info.yaml` and TechDocs (PRs [#81](https://github.com/StaffOps/kube-node-viewer/pull/81), [#82](https://github.com/StaffOps/kube-node-viewer/pull/82))
- v0.16.0 (2026-11-11): Recommendations on AKS nodes from live Azure Resource SKUs
- Rebuild the pinned vuln and license tools when Go changes (PR [#78](https://github.com/StaffOps/kube-node-viewer/pull/78))

## In progress

- Open Dependabot PRs not merged automatically:
  [#86](https://github.com/StaffOps/kube-node-viewer/pull/86) `golang.org/x/text` 0.43.0 -> 0.44.0,
  [#80](https://github.com/StaffOps/kube-node-viewer/pull/80) AWS dependencies,
  [#79](https://github.com/StaffOps/kube-node-viewer/pull/79) actions group (its CI run failed on 2026-11-20).
- No open issues. Several feature branches remain on the remote (for example `feat/config-views`, `feat/interactive-tui`); whether any is still active is not stated (inferred stale).

## Next steps

1. Bump zizmor, syft and goreleaser in the workflows so the `tools` job is green (suggested).
2. Review and merge the Dependabot PRs above (suggested).
3. Owner items in [TODO.md](https://github.com/StaffOps/kube-node-viewer/blob/main/TODO.md): release push with a GitHub App instead of the deploy key; decide whether the `kind` job counts as the cluster run; run [Kiro validation](kiro-validation.md) in a real Kiro session.

## Decisions

ADRs live in `specs/decisions/`:

- [ADR-001](https://github.com/StaffOps/kube-node-viewer/blob/main/specs/decisions/ADR-001-import-as-new-repo.md) import as a new repo
- [ADR-002](https://github.com/StaffOps/kube-node-viewer/blob/main/specs/decisions/ADR-002-merge-green-prs-from-writers.md) merge green PRs from writers (automerge)
- [ADR-003](https://github.com/StaffOps/kube-node-viewer/blob/main/specs/decisions/ADR-003-release-from-changelog-headings.md) release from CHANGELOG headings
- [ADR-004](https://github.com/StaffOps/kube-node-viewer/blob/main/specs/decisions/ADR-004-three-guardrail-layers.md) three guardrail layers
- [ADR-005](https://github.com/StaffOps/kube-node-viewer/blob/main/specs/decisions/ADR-005-never-rewrite-published-history.md) never rewrite published history

## Risks and open questions

- `release.yaml` pushes with a long-lived deploy key that bypasses the `main` and `v*` rulesets (TODO.md, Guardrails).
- Monday schedules started 9 to 10 hours late on 2026-11-09; the fallback is `gh workflow run release.yaml` (TODO.md).
- Follow-ups in TODO.md include `scripts/kind-it.sh` failing to reach the kind API server when a VPN route covers the Docker default networks, and `NOTICE` missing from release archives.

## How to verify

```bash
gh run list -R StaffOps/kube-node-viewer -L 5
gh release view -R StaffOps/kube-node-viewer
scripts/verify.sh   # gofmt, go vet, go test, spec lint
make lint
```
