# Changelog

All notable changes to this project are documented here. Format:
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[SemVer](https://semver.org/).

## [Unreleased]

### Fixed

- A `config.yaml` error from the YAML parser, such as a key written twice,
  is one line with its line number, without the library's prefix.
- `config.yaml` views no longer load names that can never match: a label
  key or resource name with leading or trailing whitespace or a comma
  (`"zone "`, `zone,region`), a resource written `usage:cpu`, and padded filter values or `labels.allow`/`deny`
  patterns are errors with their place in the file. `- label: zone` (a
  space after the colon) says to write `label:zone`; `sort: price/m` says
  `sort: price`, `sort: age` says `sort: creation`. Values in errors read
  as written (`{label: zone}`, `[a, b]`, `1000000`), and "quote it" is only
  suggested for a value YAML read as a number, boolean or null.
- Clusters reached through an API server URL that names no EKS region (a
  private endpoint behind your own DNS name, `kubectl proxy`, a
  self-managed cluster) price each AWS node in its own region, from its
  region label or zone, instead of the AWS SDK's region, which could be a
  different one than the header showed. Spot nodes no longer lose their
  price when the SDK has no region configured.

### Changed

- `config.yaml` views refuse a built-in name or column title in another
  case instead of reading it as a node label: `columns: [INSTANCE]`,
  `[instance]` or `group-by: [Zone]` showed an empty column or group that
  looked like the real one, and now say to write `instance-type` or
  `zone`; `columns: [PRICE/M]` says to write `price/m`. The same goes for
  a computed label without its prefix or in another case (`node-age`:
  write `kube-node-viewer/node-age`) and for `Label:zone`. A config that
  names a real node label such as `Zone` or `Status` bare must now write
  `label:Zone`.
- The `/` search takes a regular expression: `search|batch` keeps the nodes
  matching either word, `^node-1$` only that name. Text without regex
  characters (`.` alone does not count, so IPs and instance types stay as
  they were) matches as before. A pattern that does not compile yet is
  matched as plain text, marked `invalid regex: as text` in the footer and
  `(as text)` in the header.

## [0.13.1] - 2026-11-04

### Security

- Releases are signed: the checksum file has a keyless Sigstore signature
  (`kube-node-viewer_X.Y.Z_sha256_checksums.txt.sigstore.json`) issued to
  the release workflow, and each archive has an SPDX SBOM listed in it.
  README "Verifying a release" shows how to check them.

## [0.13.0] - 2026-11-03

### Added

- Pods tab in the node details popup (`tab` inside it): every pod the node's
  totals count, with its CPU and memory requests, their percent of
  allocatable, its share of the node (the larger of the two percents) and
  what that share costs. DaemonSet pods are listed with the others, marked
  `ds`. `d` switches to a DaemonSets view: a fixed `Total (<n> pods, <m>
  sets)` line from the summed requests, then one row per DaemonSet. `s`
  sorts by cost, CPU, memory or name and `S` flips the direction; pods with
  no CPU or memory request sort last.
- On the node list, with the cursor on a node, a line above the keys shows
  what its DaemonSets request, absolute and as a share of the node, with no
  cost: `ip-10-0-1-23 · DaemonSets: 8% of the node (cpu 620m 8% · mem 1.1Gi
  4%)`. The list is one line shorter for it; a window too short to keep
  three list lines besides it leaves the line out.
- The Details tab of the node details popup has a DaemonSets section: the
  DaemonSet pods and sets on the node and their CPU and memory requests,
  absolute and in percent of allocatable.

### Changed

- The Instance line of the Details tab shows the monthly price after the
  hourly one (`$0.1287/h · $93.95/mo`), from the `PRICE/M` cell.
- The Details tab shows CPU in millicores: `7516m (95%) / 7910m / 8000m`,
  not `/ 8`, and a resource whose allocatable equals its capacity shows
  requested / allocatable only. Pods moved out of Resources into their own
  section: `Pods  on the node / max`, `23 (40%) / 58`.
- The informer keeps the name of a pod's controller, not only its kind
  (about +2.6 MiB of heap at 150,000 pods).

## [0.12.0] - 2026-11-03

### Added

- Config views: `views` in `config.yaml` defines views by hand (columns,
  group-by, sort and order, resources, filters, groups-only), listed first
  in the Views panel (`v`) and marked `(config)`; `d` and saving under their
  name are refused. `default-view` opens each cluster with one of them the
  first time the run connects to it; `start-with: last-view` keeps each
  cluster's saved view and uses the default view only for a cluster with
  none.
  How to write them: [docs/views.md](docs/views.md).

### Changed

- `config.yaml` is read strictly: errors name the place of the problem
  (`views[1].filters[0]`), keys compare exactly (`Labels:` or `Allow:`,
  accepted before, now stop the start), and `yes`, `123` or `~` where a
  string is expected are errors.

### Fixed

- The Columns panel offers the `IP` column, as the README said.

## [0.11.0] - 2026-11-03

### Added

- `PRICE/M` and `IDLE/M` columns: each node's price and idle cost per month
  (× 730 h), shown by default next to `PRICE/H`. Clusters with saved
  columns keep theirs; add the new ones with `o`.

### Changed

- With `--disable-pricing`, `IDLE/H` is left out like `PRICE/H`; it used to
  show idle costs from the static price table.
- Narrow windows drop columns in a new order, kept first: `STATUS`,
  `PRICE/M`, `CAPACITY`, `PRICE/H`, `IDLE/M`, `INSTANCE`, `IDLE/H`,
  `NODEGROUP`, `ZONE`. The default columns need about 172 columns of
  terminal width to all show.
- The footer says "1 column hidden", not "1 columns hidden".

## [0.10.0] - 2026-11-03

### Added

- Label visibility: `labels.allow` and `labels.deny` in
  `~/.config/kube-node-viewer/config.yaml` (or under an absolute
  `$XDG_CONFIG_HOME`) pick the node labels the Filters, Group by, Sort and
  Columns panels offer. Patterns match the whole key, `*` matches anything,
  deny wins. Labels the view uses stay listed; details, search and export
  are unchanged. A status line counts the hidden labels and `a` shows them
  until the panel closes. `--labels-allow` and `--labels-deny` replace a
  list for one run. An invalid file stops the start with the file and the
  reason.

## [0.9.1] - 2026-11-03

### Added

- `I` (shift+i) on the main screen flips the sort between ascending and
  descending without opening the Sort panel.

## [0.9.0] - 2026-11-02

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
- A node without the zone label takes the zone its EC2 provider ID names.

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

[Unreleased]: https://github.com/StaffOps/kube-node-viewer/compare/v0.13.1...HEAD
[0.13.1]: https://github.com/StaffOps/kube-node-viewer/compare/v0.13.0...v0.13.1
[0.13.0]: https://github.com/StaffOps/kube-node-viewer/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/StaffOps/kube-node-viewer/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/StaffOps/kube-node-viewer/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/StaffOps/kube-node-viewer/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/StaffOps/kube-node-viewer/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/StaffOps/kube-node-viewer/compare/v0.8.2...v0.9.0
[0.8.2]: https://github.com/StaffOps/kube-node-viewer/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/StaffOps/kube-node-viewer/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/StaffOps/kube-node-viewer/releases/tag/v0.8.0
