# AGENTS.md — kube-node-viewer

Canonical guide for agents and humans working in this repo. Tool-specific
files only point here — `CLAUDE.md` (imports this file), `.kiro/steering/*`
(Kiro also loads this file natively). Edit this file, not those.

If this file conflicts with the code, treat it as a bug in one of the two and
flag it — do not silently follow either side.

## Repository

Terminal UI (Go, Bubble Tea) that shows scheduled pod resource **requests** vs
node allocatable capacity, plus node cost, for a Kubernetes cluster. It does
**not** read actual usage (metrics-server).

Derived from [`awslabs/eks-node-viewer`](https://github.com/awslabs/eks-node-viewer)
(Apache 2.0), imported verbatim at `fe35ed41e18b` —
[ADR-001](specs/decisions/ADR-001-import-as-new-repo.md), then renamed
([`specs/rebrand/`](specs/rebrand/)). It is a separate project: no behaviour
depends on the upstream name (no `eks-node-viewer/` label prefix, no
`~/.eks-node-viewer` config file). Upstream `.go`/`.sh`
files you change carry the line
`// Modified by the kube-node-viewer authors, 2026.` after the license block
(rebrand design D5); add it if missing.

## Layout

| Path | What lives there |
|------|------------------|
| `cmd/kube-node-viewer/` | `main.go` (wiring), `session.go` (one connection per context: clients, store, controller, pricing; `c` switches), `view.go` (flag table D8: defaults < config file < saved state < explicit flags), `flag.go` (flags; config file `~/.kube-node-viewer`; only `-kubeconfig` also reads `KUBECONFIG`), `version.go` |
| `pkg/client/` | kubeconfig (contexts, cluster key, EKS region from the server URL, AKS server detection); informers for pods, nodes and Karpenter `nodeclaims` with slimming transforms; change notifier |
| `pkg/model/` | store (`cluster.go`, `node.go`, `pod.go`: indexed, per-node totals, generations); `view.go` (`ViewState`: filters, group-by, sort, search, columns, folding); `uimodel.go` (Bubble Tea model, keys); `layout.go` (node list, groups, pages, columns); `header.go`; `panels*.go`, `popup.go`, `contexts.go` (popups for f g s o c ?) |
| `pkg/state/` | per-cluster view state file, debounced atomic writer |
| `pkg/pricing/` | `Provider` interface (`NodePrice`, `OnUpdate`); `ByCloud` picks the provider by the node's provider-ID cloud |
| `pkg/aws/` | AWS pricing provider and instance catalog; `zz_generated_*.go` are static price tables copied from Karpenter by `make prices` — never hand-edit |
| `pkg/azure/` | AKS prices from the public Azure Retail Prices API (no credentials) |
| `pkg/recommend/` | Recommendations rules (idle, consolidation, rightsizing, Graviton, Spot…); pure functions over a snapshot |
| `pkg/text/` | color-aware tabwriter |
| `hack/` | helper programs (boilerplate, licenses, `catalog` for the AWS instance catalog); several `main` packages share the dir, so `go build ./...` fails there — build `./cmd/... ./pkg/...` |
| `scripts/` | `build.sh`, `verify.sh`, `spec-lint.sh`, `release-prepare.py` (next version from CHANGELOG), `agent-hooks/` (session context, destructive-git guard, commit guard, commit reminder) |
| `specs/` | planning artefacts — see [`specs/README.md`](specs/README.md) |
| `TODO.md` | backlog: owner actions, open follow-ups, ideas, deferred work |
| `.github/workflows/` | `test.yaml` (verify, lint, build in parallel; automerge), `release.yaml` (version, tag and release on merge), `refresh-data.yaml` (monthly price-data PR) |

## Commands

```bash
scripts/build.sh          # local binary in bin/ (-i install, -a all platforms)
scripts/verify.sh         # gofmt check, go vet, go test, spec lint — run before every commit
scripts/screenshots.sh    # regenerate README images in .static/ from a made-up cluster (Python 3, Chrome, ffmpeg)
make lint                 # golangci-lint at the version pinned in the Makefile (same as CI)
go test ./pkg/... ./cmd/...
```

- Do **not** run `make build` / `make generate` / `make prices` /
  `make catalog` casually: they re-download price data and dirty
  `pkg/aws/`. The monthly `refresh-data.yaml` workflow does it as a PR; by
  hand, only as a deliberate, separately committed "refresh pricing" change.
- Running the binary needs a reachable cluster in the current kubeconfig
  context; never point it at a cluster the owner did not name.
- Run it from a scratch directory, not the repo root: `e` (export) writes
  CSV/JSON with node names and IPs into the working directory. Exports are
  git-ignored; never commit cluster data.

## Spec-driven workflow (mandatory)

Specs live in **`specs/` at the repo root** — never in `.kiro/specs/` (that
path is only a symlink to `specs/` so the Kiro IDE spec panel shows them).
Format rules: [`specs/README.md`](specs/README.md). Templates:
[`specs/.templates/`](specs/.templates/).

| Change | Spec needed |
|--------|-------------|
| New flag, key binding, displayed column, pricing source, behavior change | `specs/<feature>/` requirements + design + tasks **before** code |
| Decision with real alternatives or costly to revert (> 1 day) | `specs/decisions/ADR-NNN-*.md` |
| Bugfix | `specs/fix-<slug>/bugfix.md` (current / expected / unchanged) + tasks |
| Typo, dependency bump, comment, pure refactor with no behavior change | none — commit message is enough |

The loop (skill `change-loop`):

1. **Spec** — requirements (WHEN … THEN the system SHALL …), design with
   decisions, tasks with status. Skill `spec-new` drafts it.
2. **Review the spec** — independent adversarial review (agent `spec-refuter`)
   before implementing. The author is never its own reviewer.
3. **Implement** task by task; tick the task only when its acceptance
   criteria are verified.
4. **Verify** — `scripts/verify.sh` green; for UI changes, run the binary
   against a cluster the owner named, or say it was not run.
5. **Review the diff** (agent `spec-refuter` or `code-review`), fix, re-review.
6. **Docs** — `CHANGELOG.md` (`## [Unreleased]`), `README.md` usage/flags,
   spec corrections recorded as dated notes, never silent edits.
7. **Commit** per validated task.

## Backlog (`TODO.md`)

Owner actions, follow-ups found while working, ideas and deferred work live
in [`TODO.md`](TODO.md). Add an item when you find something out of scope
instead of fixing it on the side; remove it in the commit that does it.
An item that becomes work goes through the change loop (a spec first when
the table above asks for one). Deferred tables in `specs/*/tasks.md` stay
the record for their feature; `TODO.md` links them.

## Releases and pull requests

Flow: branch → PR → CI → automatic approval and merge → automatic release.

- **CI** (`test.yaml`): `verify`, `lint` and `build` run in parallel. On a
  PR from a branch of this repo whose author has write access, not a draft
  and without the `hold` label, the `automerge` job approves and merges it
  once all three pass. Forks and other authors wait for a person.
- **CHANGELOG**: changes go under `## [Unreleased]` only (the
  `commit-guard` hook blocks lines added to a released section). A
  released section (`## [x.y.z]`) is never edited, except for a dated
  correction.
- **Release** (`release.yaml`, `scripts/release-prepare.py`): after a merge,
  entries in `[Unreleased]` become the next version — `Fixed`/`Security`
  only → patch; `Added`/`Changed`/`Removed`/`Deprecated` → minor;
  `BREAKING` → major (minor while 0.x). The workflow writes `## [x.y.z]`,
  the compare links and the **shipped** status of finished specs, pushes
  that commit, tags it and publishes binaries and the Homebrew cask. A PR
  without CHANGELOG entries releases nothing. See skill `release`.
- Agents never push to `main` or push tags; they open PRs.
- **Reviewing a PR** (skill `pr-review`): check it against the current
  `main`, not its base. A branch that predates a release merges its
  CHANGELOG lines into the released section without a conflict; fix it by
  merging `main` into the branch (no force-push on someone else's branch).

## Provenance and licensing (non-negotiable)

- Keep `LICENSE`, `NOTICE` (Amazon copyright line) and every existing Apache
  license header. Files modified from upstream get a modification note; new files get the Apache 2.0 header (`hack/boilerplate.go`
  format) — [ADR-001](specs/decisions/ADR-001-import-as-new-repo.md).
- Never use "AWS", "Amazon" or "EKS" in the project name, logo or wording that
  implies endorsement. Describing compatibility ("works with EKS") is fine.
- Code from other forks or upstream: `git cherry-pick -x` so the original
  author stays the commit author; credit them in `README.md` — skill
  `upstream-sync`.
- New dependencies must be MIT / Apache-2.0 / BSD-3-Clause / ISC
  (`make licenses` allowlist).

## Conventions

- Language: everything in the repository and the product is English — TUI
  text, help, errors and logs, code and comments, specs, docs, commits, PR
  and issue text. The owner may talk to agents in Portuguese; reply in the
  language they use, but write repository content in English.
- Go: `gofmt -s`, `go vet` clean, table-driven tests in `pkg/model`
  (`*_test.go` next to the code). Errors wrapped with `%w`.
- UI: rendering stays in `pkg/model`; Kubernetes I/O stays in `pkg/client`;
  pricing behind `pricing.Provider` — AWS SDK only in `pkg/aws` and its wiring in
  `cmd/*/` (`session.go`); Azure HTTP only in `pkg/azure`.
- Lint: `make lint` clean (CI runs the same pinned version).
- Commits: Conventional Commits (`feat:`, `fix:`, `docs:`, `build:`,
  `chore:`, optional scope e.g. `feat(model):`). No AI co-author or
  "generated with" trailers.
- Push only when the owner asks. Never force-push without explicit approval.

## Remotes

| Remote | URL | Use |
|--------|-----|-----|
| `origin` | `git@github.com:StaffOps/kube-node-viewer.git` | this project |
| `upstream` | `awslabs/eks-node-viewer` | pull fixes, cherry-pick with `-x` |
| `sule26` | `git@github.com:sule26/eks-node-viewer.git` | `--group-by` / `--groups-only` source |

## Tooling map

Claude Code and Kiro follow the same rules, specs, skills, reviewer and
hooks. Each concept has one source of truth; the other tool points at it.
Change the source, never a copy.

| Concept | Source of truth | Claude Code | Kiro |
|---------|-----------------|-------------|------|
| Rules | `AGENTS.md` | `CLAUDE.md` → `@AGENTS.md` | IDE reads it natively; the CLI agent loads it as a `file://` resource; `.kiro/steering/` only pins Kiro-specific behaviour |
| Specs | `specs/` (+ `specs/.templates/`) | via this file | `.kiro/specs` → `specs/` (symlink) |
| Backlog | `TODO.md` | via this file | via this file |
| Skills | `.kiro/skills/<name>/SKILL.md` | `.claude/skills` → `.kiro/skills` (symlink) | IDE: native; CLI agent: `skill://` resource |
| Reviewer agent | `.claude/agents/spec-refuter.md` | native | `.kiro/agents/spec-refuter.json` (prompt via `file://`) |
| Hook scripts | `scripts/agent-hooks/` (one script, a `claude`/`kiro` mode where output differs) | `.claude/settings.json` | CLI: `.kiro/agents/kube-node-viewer.json` (`kiro-cli chat --agent kube-node-viewer`); IDE: `.kiro/hooks/kube-node-viewer.json` |
| Permissions | — | `.claude/settings.json` (`settings.local.json` is personal) | per agent in `.kiro/agents/*.json` |
| Model | not pinned | session choice | session choice (`/model`) |

Skills: `spec-new`, `change-loop`, `upstream-sync`, `release`, `pr-review`.
Adding a skill: `.kiro/skills/<name>/SKILL.md` with `name` and `description`
frontmatter; both tools pick it up, and `scripts/agent-hooks/session-context.sh`
lists it. Adding a hook: write the script once, then wire it in both
`.claude/settings.json` and the two Kiro files.

Hooks: `session-context` (start), `guard-destructive` (asks/blocks
history rewrites, force-pushes, hard resets), `commit-guard` (blocks
committing exports or editing released CHANGELOG sections),
`commit-reminder` (pre-commit checklist).
