# Changelog

All notable changes to this project are documented here. Format:
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org/).

## [Unreleased]

### Added

- Spec-driven development setup: `AGENTS.md` (canonical agent guide),
  `specs/` with format rules, templates, ADR-001 and the `rebrand` spec,
  shared skills for Claude Code and Kiro (`spec-new`, `change-loop`,
  `upstream-sync`), `spec-refuter` review agent, agent hooks,
  `scripts/verify.sh` and `scripts/spec-lint.sh`.
- `scripts/build.sh` for local and cross-platform builds without
  `make generate`.
- Initial import of `awslabs/eks-node-viewer` at `fe35ed41e18b`
  (Apache 2.0), see `specs/decisions/ADR-001-import-as-new-repo.md`.
