# Changelog

All notable changes to this project are documented here. Format:
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org/).

## [Unreleased]

### Added

- Filters can exclude their values: `x` in the Filters panel switches a
  filter between keeping and excluding (`capacity≠Spot`).
- Named views (`v`): save the current view under a name, apply or delete it;
  shared by every cluster, kept in the state file.
- Export (`e`): the nodes on screen to CSV and JSON in the current directory
  (or next to the state file), with groups, prices and price kinds.
- Recommendations screen (`tab`, `:rec`): idle cost, consolidation of
  managed node groups, rightsizing, newer generation / Nitro, Graviton, old
  nodes and, always last, Spot, with the math of each row and a total that
  counts each node once. Fixed rules, no AI, no new AWS permission.
- A `:` command line (`overview`, `recommendations`).
- `IDLE/H` column, sort by idle cost and idle cost in group headings.
- An instance catalog generated from the AWS Price List (`hack/catalog`).
- Config keys `recommend-headroom` and `recommend-node-age`.
- Node details (`enter` on a node): instance, price, system, resources as
  requested / allocatable / capacity, conditions, taints and labels, live.
- AKS node prices from the public Azure Retail Prices API (pay-as-you-go,
  Spot and Windows meters), no Azure login needed.

### Changed

- The cluster summary follows the filters and search: usage and cost are
  those of the nodes on screen, marked `(filtered: N of M nodes)`.
- The header keeps the whole cluster's monthly cost after the node and pod
  counts, marked `(global cluster)`, whatever the filters.
- AKS clusters read right: capacity (`OnDemand`, or `Spot` from the scale set
  priority), the agent pool as node group, the region from the nodes, and no
  zone for VMs outside availability zones. No AWS price lookups are made.
- Prices nobody knows read `prices unknown` instead of `$0.000`, and totals
  with some unknown prices are marked `>=`; exported totals leave the price
  out and count the unpriced nodes.
- Recommendations on clusters without EKS nodes say they cover EKS only.

- The static on-demand price tables, used until the first live lookup and
  when AWS can't be reached, are refreshed (from April 2025 to October 2026).
- golangci-lint v2 runs in CI (`make lint` locally); a monthly workflow
  refreshes the price tables and, with an AWS role configured, the instance
  catalog, and opens a PR.

### Fixed

- The pending pod count, and the recommendations it holds back, counted pods
  already on a node (pulling an image, waiting on a secret); only pods waiting
  for a node count now.

## [0.8.2] - 2026-11-02

### Added

- A row cursor (`↑`/`↓`, `k`/`j`) over group headings and nodes, marked in the
  left margin; `enter` folds or unfolds the selected group on its own, saved
  with the view.

### Fixed

- Errors from the Kubernetes client or pricing no longer print over the TUI:
  while it runs they go to `kube-node-viewer.log` next to the state file, and
  the header shows the latest for two minutes.

## [0.8.1] - 2026-11-02

### Fixed

- Releases publish the Homebrew cask to `StaffOps/homebrew-tap`: the tap
  token is passed as `{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}`. In 0.8.0 the
  cask step failed and only the release archives were published.

## [0.8.0] - 2026-11-02

### Added

- Interactive TUI: start with no flags and change the view from inside.
  - Filters panel (`f`): every field (capacity type, node group, zone,
    region, instance type, status) and every node label listed up front; open
    a field to tick its values, shown with node counts; each tick filters at
    once; turn a filter off and on or clear it.
  - Group by panel (`g`): tick up to three fields as nested levels, reorder
    them with `K`/`J`; every level has node, pod, usage and cost subtotals.
  - Sort panel (`s`) on creation, name, IP, price, pods, usage per resource or
    any label; Columns panel (`o`) to show, hide and reorder columns.
  - Live search (`/`), resource cycle (`r`), context switch without
    restarting (`c`), help with keys and colours (`?`).
  - `n` folds the grouped list one level per press (nodes, then each
    sub-group level) and unfolds on the next; folded, the deepest level shown
    carries usage bars and `d` lists the level below as one-line rows.
- Panels open as popups over the node list, with their keys on their last
  line.
- Header box with context, API server, pricing region, nodes shown of total,
  pod count (and pending pods), the active view and the main keys; `ctrl+e`
  compacts it to two lines.
- Columns for capacity type (with `/Auto` for EKS Auto Mode), node group,
  zone, region, internal IP, age, status and price; default columns are
  instance, price, capacity, node group, zone and status.
- View state saved per cluster (context plus API server URL) in
  `$XDG_STATE_HOME/kube-node-viewer/state.json`.
- Reserved nodes (Karpenter reserved capacity, EKS capacity blocks) show the
  on-demand price as an equivalent (`~$`), left out of cluster and group
  totals, which say how many reserved nodes were not priced.
- Colours by meaning: usage green above 80%, yellow from 60% to 80%, red
  below; node status, capacity type and EKS managed node groups each have a
  colour; group headings form a tree.
- `--group-by` and `--groups-only` (summary of usage and cost per node label),
  by João Pedro Alcantara.
- `kube-node-viewer/` label prefix for the price override label and computed
  label names.
- Spec-driven development setup: `AGENTS.md` (canonical agent guide),
  `specs/` with format rules, templates, ADR-001 and the `rebrand` and
  `interactive-tui` specs, shared skills for Claude Code and Kiro
  (`spec-new`, `change-loop`, `upstream-sync`), `spec-refuter` review agent,
  agent hooks, `scripts/verify.sh` and `scripts/spec-lint.sh`.
- Homebrew install: releases publish a cask to `StaffOps/homebrew-tap`
  (`brew install staffops/tap/kube-node-viewer`); release archives are
  `tar.gz`, `zip` on Windows.
- `scripts/build.sh` for local and cross-platform builds without
  `make generate`.
- Initial import of `awslabs/eks-node-viewer` at `fe35ed41e18b`
  (Apache 2.0), see `specs/decisions/ADR-001-import-as-new-repo.md`.

### Changed

- `g` opens the Group by panel instead of toggling the summary; `esc` closes
  panels and clears the search instead of quitting.
- `--group-by key=a,b` values filter in memory instead of narrowing the
  server-side selector; `--node-selector` is the only server-side scope.
- Command-line view flags apply to the run they are given on and to the
  cluster it starts on; the last view chosen in the TUI is restored per
  cluster.
- The screen redraws when the cluster changes instead of every 100 ms. Render
  time no longer depends on the number of pods (1.7 ms for 500 nodes at both
  15,000 and 150,000 pods, from 2.5 ms and 29.9 ms), and the model holds pods
  in a compact form (149 MiB at 150,000 pods, from 924 MiB).
- The pricing region comes from the EKS API server URL when there is one.
- Pages are cut by lines; a page that continues a group repeats its path; the
  lowest-priority columns are dropped first on narrow windows, and no line
  runs past the window edge. Resizing the window redraws from a clean screen.
- Node usage bars are wider (36 to 60 cells). A node's bars stay joined and
  nodes are set apart by a thin gap above their first bar; each cluster
  summary bar keeps its own gap.
- Node and pod counts are shown once, in the header.
- Renamed to `kube-node-viewer`: module `github.com/staffops/kube-node-viewer`,
  binary and `cmd/kube-node-viewer`, config file `~/.kube-node-viewer`,
  README, `CONTRIBUTING.md` and `CODEOWNERS`.
- `-version` reports the module version and commit from Go build info when the
  binary is built without ldflags (`go install`).
- CI runs `scripts/verify.sh` and `scripts/build.sh`; releases build from the
  committed tree without refreshing pricing or attribution.

### Removed

- The previous Homebrew tap tooling (`hack/homebrew.go`) and the inherited code
  of conduct.

### Fixed

- A node's price is read and written under its lock; a pod seen before its
  node is counted once the node appears.

[Unreleased]: https://github.com/StaffOps/kube-node-viewer/compare/v0.8.2...HEAD
[0.8.2]: https://github.com/StaffOps/kube-node-viewer/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/StaffOps/kube-node-viewer/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/StaffOps/kube-node-viewer/releases/tag/v0.8.0
