# Changelog

All notable changes to this project are documented here. Format:
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org/).

## [Unreleased]

### Changed

- Renamed to `kube-node-viewer`: module `github.com/staffops/kube-node-viewer`,
  binary and `cmd/kube-node-viewer`, config file `~/.kube-node-viewer`,
  README, `CONTRIBUTING.md` and `CODEOWNERS`.
- `-version` reports the module version and commit from Go build info when the
  binary is built without ldflags (`go install`).
- CI runs `scripts/verify.sh` and `scripts/build.sh`; releases build from the
  committed tree without refreshing pricing or attribution.

### Removed

- AWS Homebrew tap tooling (`hack/homebrew.go`) and the Amazon code of conduct.

### Deprecated

- The `eks-node-viewer/` label prefix and `~/.eks-node-viewer` config file.
  Both still work; `kube-node-viewer/` and `~/.kube-node-viewer` take
  precedence.

### Added

- `--group-by` and `--groups-only` (summary of usage and cost per node label),
  by João Pedro Alcantara.
- `kube-node-viewer/` label prefix for the price override label and computed
  label names.
- Spec-driven development setup: `AGENTS.md` (canonical agent guide),
  `specs/` with format rules, templates, ADR-001 and the `rebrand` spec,
  shared skills for Claude Code and Kiro (`spec-new`, `change-loop`,
  `upstream-sync`), `spec-refuter` review agent, agent hooks,
  `scripts/verify.sh` and `scripts/spec-lint.sh`.
- `scripts/build.sh` for local and cross-platform builds without
  `make generate`.
- Initial import of `awslabs/eks-node-viewer` at `fe35ed41e18b`
  (Apache 2.0), see `specs/decisions/ADR-001-import-as-new-repo.md`.
