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
[ADR-001](specs/decisions/ADR-001-import-as-new-repo.md). Until the rebrand
([`specs/rebrand/`](specs/rebrand/)) ships, the module path, `cmd/` directory,
binary name and config file still use the upstream name `eks-node-viewer`.

## Layout

| Path | What lives there |
|------|------------------|
| `cmd/eks-node-viewer/` | `main.go` (wiring), `flag.go` (flag > `~/.eks-node-viewer` config file; only `-kubeconfig` also reads the `KUBECONFIG` env var, between the two) |
| `pkg/client/` | kubeconfig loading; informers for pods, nodes and Karpenter `nodeclaims` feeding the model |
| `pkg/model/` | `Cluster`/`Node`/`Pod` state, stats, and the Bubble Tea `UIModel` (rendering, keys) |
| `pkg/pricing/` | `Provider` interface (`NodePrice`, `OnUpdate`) |
| `pkg/aws/` | AWS pricing provider; `zz_generated_*.go` are static price tables copied from Karpenter by `make generate` — never hand-edit |
| `pkg/text/` | color-aware tabwriter |
| `hack/` | upstream helper programs (boilerplate, homebrew, licenses); several `main` packages share the dir, so `go build ./...` fails there — build `./cmd/... ./pkg/...` |
| `scripts/` | `build.sh`, `verify.sh`, `spec-lint.sh`, `agent-hooks/` |
| `specs/` | planning artefacts — see [`specs/README.md`](specs/README.md) |

## Commands

```bash
scripts/build.sh          # local binary in bin/ (-i install, -a all platforms)
scripts/verify.sh         # gofmt check, go vet, go test, spec lint — run before every commit
go test ./pkg/... ./cmd/...
```

- Do **not** run `make build` / `make generate` casually: it needs
  `go-licenses`, re-downloads AWS price tables from Karpenter `main` and
  dirties `pkg/aws/zz_generated_*.go`. Run it only as a deliberate,
  separately committed "refresh pricing" change.
- Running the binary needs a reachable cluster in the current kubeconfig
  context; never point it at a cluster the owner did not name.

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

- Go: `gofmt -s`, `go vet` clean, table-driven tests in `pkg/model`
  (`*_test.go` next to the code). Errors wrapped with `%w`.
- UI: rendering stays in `pkg/model`; Kubernetes I/O stays in `pkg/client`;
  pricing behind `pricing.Provider` — AWS SDK only in `pkg/aws` and its wiring in
  `cmd/*/main.go`.
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

| Concept | Source of truth | Claude Code | Kiro |
|---------|-----------------|-------------|------|
| Rules | `AGENTS.md` | `CLAUDE.md` → `@AGENTS.md` | read natively + `.kiro/steering/` |
| Specs | `specs/` | via this file | `.kiro/specs` → `specs/` (symlink) |
| Skills | `.kiro/skills/` | `.claude/skills` → `.kiro/skills` (symlink) | native |
| Reviewer agent | `.claude/agents/spec-refuter.md` | native | `.kiro/agents/spec-refuter.json` (prompt via `file://`) |
| Hooks | `scripts/agent-hooks/` | `.claude/settings.json` | `.kiro/agents/kube-node-viewer.json` — kiro-cli only (`kiro-cli chat --agent kube-node-viewer`); the Kiro IDE does not run them |
