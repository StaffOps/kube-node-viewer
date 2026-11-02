# Changelog

All notable changes to this project are documented here. Format:
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org/).

## [Unreleased]

### Added

- Interactive TUI: start with no flags and change the view from inside.
  Filters panel (`f`) on labels, capacity type (OnDemand, Spot, Reserved,
  Fargate), node group, zone, region, instance type and status; Group by panel
  (`g`) with up to three nested levels and subtotals; Sort panel (`s`);
  Columns panel (`o`); live search (`/`); resource cycle (`r`); context
  switch (`c`) without restarting; help (`?`); compact header (`ctrl+e`).
- Header panel with context, API server, region, nodes shown of total, pods
  and the active view.
- Columns for capacity type (with `/Auto` for EKS Auto Mode), node group (EKS
  managed node groups in their own colour), zone, region, internal IP, age,
  status and price.
- View state saved per cluster in
  `$XDG_STATE_HOME/kube-node-viewer/state.json`.
- Reserved nodes show the on-demand price as an equivalent (`~$`), left out of
  cluster and group totals, which say how many reserved nodes were not priced.

### Changed

- `g` opens the Group by panel instead of toggling the summary; `esc` closes
  panels instead of quitting.
- `--group-by key=a,b` values filter in memory instead of narrowing the
  server-side selector; `--node-selector` is the only server-side scope.
- Command-line view flags apply to the run they are given on; the last view
  chosen in the TUI is restored per cluster.
- The screen redraws when the cluster changes instead of every 100 ms. Render
  time no longer depends on the number of pods (1.7 ms for 500 nodes at both
  15,000 and 150,000 pods, from 2.5 ms and 29.9 ms), and the model holds pods
  in a compact form (149 MiB at 150,000 pods, from 924 MiB).
- The pricing region comes from the EKS API server URL when there is one.
- Pages are cut by lines; a page that continues a group repeats its path, and
  the lowest-priority columns are dropped first on narrow screens.

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
