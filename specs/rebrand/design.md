# Design: rebrand

> Implements [requirements](requirements.md).

## Current state

| Upstream name in | Where |
|------------------|-------|
| Module path | `go.mod:1`; imports in `cmd/eks-node-viewer/main.go:30-32`, `pkg/client/controller.go:31-32`, `pkg/aws/pricing.go:36-37`, `pkg/pricing/pricing.go:17`, `pkg/model/uimodel.go:35`, tests; `.golangci.yaml` `local-prefixes` |
| Binary / cmd dir | `cmd/eks-node-viewer/`, version/help text (`main.go:54`, `flag.go:65-66`), `Makefile:13,50`, `.goreleaser.yaml:1,12`, `.gitignore`, `cmd/eks-node-viewer/.gitignore:1` |
| Version without ldflags | `flag.go:32-35` defaults `version = "dev"`, empty commit |
| Embedded attribution | root `ATTRIBUTION.md` (written by `hack/gen_licenses.sh`); `cmd/eks-node-viewer/ATTRIBUTION.md` (copied by `//go:generate cp`, `main.go:35`, embedded at `main.go:36`) |
| Config file | `cmd/eks-node-viewer/flag.go:38-41` (`configPath` set in `init()` to `~/.eks-node-viewer`) |
| Node label (real) | `pkg/client/controller.go:197` (`eks-node-viewer/instance-price`; unparsable value falls through to the pricing provider, `:198`) |
| Computed label names | `pkg/model/node.go:292-303` (`ComputeLabel`: `eks-node-viewer/node-age`, `eks-node-viewer/node-(.*)-usage`), used from `uimodel.go:191,313-317` after the real-label lookup misses; README:70 shows them in config |
| CI | `.github/workflows/test.yaml` (`make build` → `make generate`: go-licenses + pricing curl); `.github/workflows/release.yaml` (`make generate` then `git diff-index --quiet HEAD`, then goreleaser) |
| Release / Homebrew | `.goreleaser.yaml`, `hack/homebrew.go` (`aws/tap`) |
| Attribution template | `hack/attribution.tmpl:4` ("The Amazon eks-node-viewer Product") |
| Governance | `CODEOWNERS`, `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, README badges (README:1-3) |
| Fork branch | `feat/sule26-group-by` (671177b, 6b86396) changes `cmd/eks-node-viewer/{flag,main}.go`, `pkg/model/{node,uimodel}.go` and adds a group-by label lookup with a `ComputeLabel` fallback |

## Decisions

### D1: Module path matches the repository

**Choice**: `github.com/staffops/kube-node-viewer`. The rewrite replaces the
exact string `github.com/awslabs/eks-node-viewer`, never `github.com/awslabs/`
(`go.mod` depends on `github.com/awslabs/operatorpkg`).

**Justification**:
1. `go install` and `go get` resolve the module from its path; any other path
   breaks installation.
2. Same rule as `otel-libs` (its ADR-010).

**Trade-offs accepted**: every import changes in one commit; open dependabot
branches conflict.

**When this would be wrong**: if the repo moves org — then the path moves with it.

**Reversal action**: one `sed` of the exact string over imports + `go.mod`; minutes.

### D2: Accept both prefixes, new one wins

**Choice**: one helper in `pkg/model` normalizes a name with the
`eks-node-viewer/` prefix to `kube-node-viewer/` for computed names (R4b);
`updatePrice` reads `kube-node-viewer/instance-price` first, then
`eks-node-viewer/instance-price`, skipping a value that does not parse (R4a).

**Justification**:
1. Labels live in users' clusters (often set by Karpenter NodePool templates)
   and computed names live in saved configs; dropping the old prefix silently
   removes prices/columns.
2. Cost is one helper and one extra map lookup.

**When this would be wrong**: if no user carries the old names — then the
fallback is dead code; drop it in a major release.

**Reversal action**: delete the fallback helper and its tests.

### D3: Legacy config file is read, never written or moved

**Choice**: fall back to `~/.eks-node-viewer` with a one-line stderr warning
before the TUI starts; never touch the file. Config path resolution takes the
home dir as a parameter so tests can inject it.

**Justification**: the upstream binary may still be installed and reading it;
moving it would break that tool.

**When this would be wrong**: if the two tools need different settings in
practice — then drop the fallback and document copying the file.

**Reversal action**: remove the fallback branch.

### D4: Remove Homebrew tap tooling instead of adapting it

**Choice**: delete `hack/homebrew.go` and the `aws/tap` README section.

**Justification**: it writes into AWS's tap repo; no tap exists for this
project.

**When this would be wrong**: once a `staffops/tap` exists.

**Reversal action**: add a goreleaser `brews:` block (do not restore
`hack/homebrew.go`, which is tied to AWS's tap layout).

### D5: Modification note format and placement

**Choice**: each upstream `.go`/`.sh` file changed by a task gets, in the same
task, one line right after its license block:
`// Modified by the kube-node-viewer authors, 2026.` (`#` for shell).
New files get the same Apache header as upstream files (the `goheader`
template in `.golangci.yaml`), with no note. Docs (README, CONTRIBUTING) are
rewritten and credit upstream in text instead.

> **Corrected 2026-10-30.** New files first got a `Copyright … authors` line;
> dropped because the `goheader` template matches the header exactly and the
> note sits outside the license block for the same reason.

**Justification**:
1. Apache 2.0 §4(b) requires prominent notices in modified files.
2. Adding it inside each task avoids a final pass that misses files changed
   late (the README, T8).
3. `hack/boilerplate.go:63` only checks for the license URL, so the extra line
   does not break it.

**When this would be wrong**: if a per-change log is wanted instead — then
move the notes to a `MODIFICATIONS.md` referenced from `NOTICE`.

**Reversal action**: remove the line with one `sed`.

### D6: CI and release stop refreshing generated files

**Choice**: `test.yaml` runs `scripts/verify.sh` and `scripts/build.sh`.
`release.yaml` drops `make generate` and the `git diff-index` check and runs
only goreleaser on the tagged tree. Price tables and attribution are refreshed
by hand in their own commits.

**Justification**:
1. `make generate` curls Karpenter `main`; any upstream price change makes the
   release job fail at `git diff-index`, so R7 could never pass reliably.
2. It needs `go-licenses` and network on every PR for no test value.

**Trade-offs accepted**: price tables can go stale between manual refreshes.

**When this would be wrong**: if stale prices become a user complaint — then
add a scheduled workflow that opens a PR with refreshed tables.

**Reversal action**: restore the two steps in `release.yaml`.

### D7: Build the rebrand on top of `feat/sule26-group-by`

**Choice**: the rebrand branch starts from `feat/sule26-group-by` (commits
kept with author and `-x` trailers), so the rename and the R4b prefix helper
cover `--group-by` in one pass.

**Justification**:
1. The group-by code adds a new `ComputeLabel` caller; renaming main first and
   rebasing later would rename twice and resolve conflicts in the same lines.
2. The fork changes are already reviewed and pushed.

**Trade-offs accepted**: the rebrand PR also carries the group-by feature.

**When this would be wrong**: if the owner decides not to ship group-by —
then base the branch on `main` and drop R4b's `--group-by` item.

**Reversal action**: rebase the rebrand commits onto `main` with
`git rebase --onto main feat/sule26-group-by`.

## Invariants

- Displays requests vs allocatable, never actual usage.
- Apache 2.0 obligations from ADR-001 (R8).
- Flags and their defaults unchanged apart from names in help text.

## Test plan

- `scripts/verify.sh` green after every task.
- New unit tests for R2 (build info), R3, R4a, R4b.
- `scripts/build.sh && bin/kube-node-viewer -version`.
- `goreleaser check && goreleaser build --snapshot --clean` (R7).
- Name sweep, with an explicit allowlist:
  `grep -rIln 'eks-node-viewer' --exclude-dir=.git --exclude-dir=specs --exclude-dir=.kiro --exclude-dir=.claude --exclude-dir=bin . | grep -vE '^(\./)?(NOTICE|README\.md|AGENTS\.md|CHANGELOG\.md)$'`
  lists only the files holding the D2/D3 compatibility code
  (`cmd/kube-node-viewer/flag.go`, `flag_test.go`, `pkg/model/node.go`).

> **Corrected 2026-10-30.** The allowlist patterns assumed a `./` prefix, which BSD
> grep omits, and the sweep scanned `bin/`; both fixed after running it in T9.
