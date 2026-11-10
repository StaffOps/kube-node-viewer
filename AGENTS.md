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
| `scripts/` | `build.sh`, `verify.sh`, `spec-lint.sh` (spec format; active and shipped specs reviewed by spec-refuter), `spec-tasks-check.py` (code under a spec: no draft, no open task), `spec-trace.py` (every requirement named by a test), `drift-check.py` (flags, keys, fields changed with no spec or docs), `release-prepare.py` (next version from CHANGELOG), `release-check.sh` (a published release: checksums, signature, SBOMs, provenance, binary version, cask), `checks/` (`staged.sh` pre-commit checks, `content.sh` real-environment data, `dependabot-patch.sh` automerge judge, `test-checks.sh` hook tests, `tool-versions.py` CI tools Dependabot does not bump, `release-snapshot.sh` the snapshot's `dist/`, `headers.py` license headers and D5 notes), `kind-it.sh` (integration test on a throwaway kind cluster), `agent-hooks/` (session context, destructive-git guard, commit guard, commit reminder, PR guard; `kiro-selftest.sh` feeds them Kiro's hook payloads) |
| `specs/` | planning artefacts — see [`specs/README.md`](specs/README.md) |
| `TODO.md` | backlog: owner actions, open follow-ups, ideas, deferred work |
| `.github/workflows/` | `test.yaml` (verify, lint, build, secrets, guardrails, deps, workflows, release-snapshot, kind (informational) in parallel, also weekly; weekly only: tools; automerge), `scorecard.yaml` (OpenSSF Scorecard, informational); `zizmor.yml` (workflow audit config); `actions/gitleaks/` (pinned gitleaks install), `release.yaml` (weekly, on-demand or a pushed `v*` tag; jobs deps: vuln and licenses, read only; prepare: version, tag; publish: release, signature, provenance; check), `refresh-data.yaml` (monthly price-data PR) |

## Commands

```bash
scripts/build.sh          # local binary in bin/ (-i install, -a all platforms)
scripts/verify.sh         # gofmt check, go vet, go test, spec lint — run before every commit
scripts/screenshots.sh    # regenerate README images in .static/ from a made-up cluster (Python 3, Chrome, ffmpeg)
scripts/kind-it.sh        # integration test on a throwaway kind cluster (Docker, kind); own kubeconfig, deleted after
go test ./pkg/model -run TestGoldenScreens -update   # rewrite the golden screens after a layout change made on purpose
make lint                 # golangci-lint at the version pinned in the Makefile (same as CI)
make vuln licenses        # govulncheck and go-licenses, pinned in the Makefile (CI deps job)
make check                # before a push: main merged in, verify, branch commits, hook tests, vuln, licenses, lint, build, and the next version
make kiro-selftest        # offline check of the Kiro hooks and agents (before docs/kiro-validation.md)
go test ./pkg/... ./cmd/...
```

- Do **not** run `make build` / `make generate` / `make prices` /
  `make catalog` casually: they re-download price data and dirty
  `pkg/aws/`. The monthly `refresh-data.yaml` workflow runs `make prices`
  as a PR (in a read-only job; a second job only opens the PR); `make
  catalog` needs AWS credentials and is run by hand only. By hand, either
  is a deliberate, separately committed "refresh pricing" change.
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
   on a cluster (agents: one the owner allows), or say it was not run.
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

Flow: branch → PR → CI → automatic approval and merge → weekly (or on-demand) release.
Why: [ADR-002](specs/decisions/ADR-002-merge-green-prs-from-writers.md)
(automerge), [ADR-003](specs/decisions/ADR-003-release-from-changelog-headings.md)
(release from the CHANGELOG).

- **Tested before it ships**: a change to code (`cmd/`, `pkg/`, `hack/`,
  `scripts/`, `.github/`, `.githooks/`) that touches a spec merges only
  when that spec is past draft and every task of it is `[x]`, its verify task
  included (`scripts/verify.sh`, `make lint`, a run on any cluster,
  recorded as a dated note of what was checked, never the cluster's name (this repo is public)). `scripts/spec-tasks-check.py` fails CI
  otherwise; open the PR as a draft until then.
- **CI** (`test.yaml`): `verify`, `lint`, `build`, `secrets`,
  `guardrails`, `deps`, `workflows`, `release-snapshot` and `kind` (informational) run in parallel (and weekly on `main`, so a new
  advisory for a shipped dependency shows up). On a PR from a branch of this repo whose
  author has write access, not a draft and without the `hold` label, the
  `automerge` job approves and merges it once all of them but `kind` pass. Dependabot
  PRs that only raise patch versions merge the same way, judged from the
  go.mod diff, not from what Dependabot lists; any other Dependabot PR,
  GitHub Actions updates included, waits for a person. Forks and other authors wait for a person.
- **CHANGELOG**: changes go under `## [Unreleased]` only (the
  `commit-guard` hook blocks lines added to a released section, and
  `scripts/changelog-check.py` in `verify.sh` fails CI on it). A
  released section (`## [x.y.z]`) is never edited, except for a dated
  correction.
- **Release** (`release.yaml`, `scripts/release-prepare.py`): merges only
  add to `[Unreleased]`; releases go out in batches — every Monday 12:00
  UTC when `[Unreleased]` has entries, on demand with
  `gh workflow run release.yaml`, or right after the merge of a PR
  labelled `release`. The entries decide the version: `Fixed`/`Security`
  only → patch; any `Added`/`Changed`/`Removed`/`Deprecated` → minor;
  `BREAKING` → major (minor while 0.x). The workflow writes `## [x.y.z]`,
  the compare links and the **shipped** status of finished specs, pushes
  that commit, tags it and publishes binaries and the Homebrew cask. See
  skill `release`.
- **Rulesets** (repository settings): `main` takes changes only through a
  PR whose `verify`, `lint`, `build`, `secrets`, `guardrails`, `deps` and
  `workflows` checks passed (no force-push, no deletion); `v*` tags
  cannot be created, moved or deleted. Admins and the release deploy key
  bypass both, on purpose: admins so the owner can fix a release or a tag
  by hand (on `main` only through a PR), the deploy key because
  `release.yaml` pushes its commit and tag with `RELEASE_DEPLOY_KEY`, and
  GitHub does not let `GITHUB_TOKEN` bypass a ruleset.
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
  `cmd/*/session.go` (`hack/` tools such as `hack/catalog` are exempt;
  `pkg/model/sdkboundary_test.go` checks it); Azure HTTP only in `pkg/azure`.
- Lint: `make lint` clean (CI runs the same pinned version).
- Commits: Conventional Commits (`feat:`, `fix:`, `docs:`, `build:`,
  `chore:`, optional scope e.g. `feat(model):`). No AI co-author or
  "generated with" trailers.
- Push only when the owner asks. Never force-push without explicit approval;
  published history is never rewritten
  ([ADR-005](specs/decisions/ADR-005-never-rewrite-published-history.md)).

## Remotes

| Remote | URL | Use |
|--------|-----|-----|
| `origin` | `git@github.com:StaffOps/kube-node-viewer.git` | this project |
| `upstream` | `awslabs/eks-node-viewer` | pull fixes, cherry-pick with `-x` |
| `sule26` | `git@github.com:sule26/eks-node-viewer.git` | `--group-by` / `--groups-only` source |

## Guardrails

Three layers check the same rules
([ADR-004](specs/decisions/ADR-004-three-guardrail-layers.md)). Git hooks run for everyone (people,
Kiro, Claude Code); agent hooks warn the agent before it acts and say why;
CI is the gate nothing merges or releases without. `make hooks` installs
the git hooks; the session-context hook does it on the first agent session.

| When | What is checked | Git hook | Agent hook (Kiro + Claude) | CI | Fix |
|------|-----------------|----------|----------------------------|----|-----|
| Session start | branch, open spec tasks, flow; installs git hooks | — | `session-context` | — | — |
| Any shell command | history rewrite, force-push, hard reset, branch/tag delete | — | `guard-destructive` (asks / blocks) | — | owner runs it |
| `git commit` | export files (cluster data) staged | `pre-commit` | `commit-guard` | — | `git rm --cached` |
| `git commit` | lines added to a released CHANGELOG section | `pre-commit` | `commit-guard` | `changelog-check.py` | move them under `[Unreleased]` |
| `git commit` | real-environment data in added lines: cluster URLs and IDs (EKS endpoints and OIDC issuers; AKS hcp/privatelink hosts, OIDC issuers, `MC_` resource groups, `aks-…-vmss` nodes, subscription ids; GKE contexts; GCE provider-ID projects and GCP project ids or numbers in `projects/<id>/zones…` paths; Azure resource groups in resource ids (also `%2F`/`\/`-escaped), `--resource-group`, `az … -g`, `resourceGroup`/`resource_group` keys; any `server:` or `:6443` API URL), private IPs and node names, EC2 instance ids, AWS account ids (ARN, ECR, `accountId`), internal domains (`.corp`, `.lan`, `.internal`, `a.b.local`…), e-mails, credentials, the private deny list; the deny list also against author and committer | `pre-commit` | `commit-guard` | `verify.sh` (every file), `secrets` job (every line each PR commit adds) — `scripts/checks/content.sh` | example values: the ones listed in `content.sh`, new IPs from 192.0.2.0/24 or 203.0.113.0/24 (and node names such as `ip-192-0-2-38…`), `111122223333`, `example.com`, `example-…`, `gce://example-project/` or `gce://example.com:example-…/`, `projects/example-project/` or `projects/123456789/`, `resourceGroups/example-rg` or `resourceGroups/MC_example…` (an example is `example` alone or followed by `-` or a digit), the zero GUID; a reviewed exception says `content-check: allow` |
| `git commit` | secrets (gitleaks: cloud keys, tokens, private keys) | `pre-commit` (when gitleaks is installed) | `commit-guard` | `secrets` job (every commit of the PR) | remove it **and rotate it**; false positive: `gitleaks:allow` or `.gitleaks.toml` |
| `git commit` | Go files not `gofmt -s` clean | `pre-commit` | `commit-guard` | `verify.sh` | `gofmt -s -w` |
| PR, `verify.sh` | the AWS SDK used outside `pkg/aws` and `cmd/*/session.go` (imported directly, or through `pkg/aws`) | — | — | `verify.sh` (`go test`: `pkg/model/sdkboundary_test.go`) | move the SDK code into `pkg/aws` |
| `git commit` | new `.go`/`.sh`/`.py` without the Apache header; in CI every `.go`/`.sh`/`.py` file and git hook, the D5 modification note right after the license block on every upstream file changed since the import (renames followed; exemptions listed in the script), `LICENSE` and the `NOTICE` first line | `pre-commit` (new files) | `commit-guard` (new files) | `verify.sh` (`scripts/checks/headers.py`) | `make boilerplate` (Go) or copy the header; `Modified by the kube-node-viewer authors, 2026.` as a comment right after the license block (in Go, a blank line after it) |
| `git commit` | Conventional Commit subject; no AI co-author / "generated with" | `commit-msg` | `commit-reminder` (checklist) | — | reword |
| `git commit` | real-environment data in the message (same rules, e-mails excepted) | `commit-msg` | — | `secrets` job (`content.sh --commits`) | reword |
| `git push` | push to `main` or a `v*` tag (`release.yaml` starts only on `v*` tags) | `pre-push` (owner override `KNV_PUSH_MAIN=1`) | — | GitHub refuses it (rulesets; only the release deploy key bypasses) | open a PR |
| `git push` | `main` merged in; verify, branch commits (messages, authors, secrets), hook tests, vulnerabilities and licenses, lint, build | `pre-push` → `make check` | — | `verify`, `secrets`, `guardrails`, `deps`, `lint`, `build` jobs | skill `pre-push` |
| PR, weekly | known vulnerabilities reachable from our code (govulncheck); dependency licenses outside MIT / Apache-2.0 / BSD-3-Clause / ISC (go-licenses) | — | — | `deps` job | upgrade the module (`go get …@fixed`), or replace the dependency |
| PR | the workflows: actionlint, and zizmor (actions pinned by commit SHA and not impostor commits, no workflow left on the default token permissions, no credential left in a checkout that does not push, no cache in the release build, injection) | — | — | `workflows` job (`.github/zizmor.yml` lists the reviewed exceptions) | pin with `@<sha> # vX.Y.Z`; add `permissions:` to the job |
| Weekly | CI tools pinned by version that Dependabot does not see (zizmor, actionlint, gitleaks, golangci-lint, govulncheck, go-licenses, cosign, syft, goreleaser, kind): a release at least 14 days old; one tool pinned at two versions (also every commit: `verify.sh` runs `--pins-only`) | — | — | `tools` job (`scripts/checks/tool-versions.py`) | raise the pin (golangci-lint in the Makefile and test.yaml together, cosign, goreleaser and syft in release.yaml and test.yaml together; gitleaks with its `GITLEAKS_SHA256`, kind with its `KIND_SHA256`) |
| PR, `verify.sh` | specs: the status is draft, active, shipped or superseded; an active or shipped spec names its review on the status line (`Reviewed: <date> (spec-refuter…` or `Reviewed: spec-refuter <date>`); code (`cmd/`, `pkg/`, `hack/`, `scripts/`, `.github/`, `.githooks/`) may not change under a draft spec | — | — | `spec-lint.sh`, `spec-tasks-check.py` (in `verify.sh`) | run the spec-refuter review, record it, set the spec active |
| PR, `verify.sh` | every `**R<n>**` of an active or shipped spec is named in the comment right above the test that checks it (`// <spec> R<n>` above `func Test…` or `t.Run`), or marked in the spec `Test: manual`, `Test: ci` or `Test: withdrawn` with the reason; a citation of an unknown or draft spec, or of a requirement the spec lacks, fails | — | — | `spec-trace.py` (in `verify.sh`) | cite it on the test that checks it, or write the test |
| PR, `verify.sh` | flags (with default and usage), config keys, help keys (`helpKeys`) or fields (`view.go`) changed with no change to a spec, `README.md` or `docs/`; a new flag, config key or help key not named in `README.md`; a key `uimodel.go` handles missing from the help | — | — | `drift-check.py` (in `verify.sh`) | update the spec or README; no doc to update: trailer `Drift-check: skip — <reason>` on the commit that changes them |
| PR | the screens: each main screen of the made-up cluster against its golden text file in `pkg/model/testdata/screens/` (`TestGoldenScreens`); a layout change shows as a diff of those files in the PR | — | — | `verify` job | if on purpose, `go test ./pkg/model -run TestGoldenScreens -update` and commit the files |
| PR | the binary's sessions against a real API server: nodes, pods per node and their requests, node group and zone, and each node's Pods tab match what a kind cluster lists (`scripts/kind-it.sh`, `TestKindCluster`) | — | — | `kind` job (informational: does not gate automerge, not a required check) | fix the regression, or the fixture in `cmd/kube-node-viewer/testdata/kind/` |
| PR | the guardrails themselves: shellcheck and `scripts/checks/test-checks.sh` (each case blocked or passed in a throwaway repo) | — | — | `guardrails` job | fix the hook or the case |
| `gh pr create` / `ready` / `merge` | released CHANGELOG sections; open tasks in a touched spec (the verify task includes a cluster run) | — | `pr-guard` (`--draft` passes) | `changelog-check.py`, `spec-tasks-check.py` | finish the tasks; draft until then |
| PR green | trusted author, or a Dependabot PR that only raises patch versions (`scripts/checks/dependabot-patch.sh` on its go.mod) → approve, merge | — | — | `automerge` job (`hold` label stops it) | — |
| Release | known vulnerabilities and licenses again, before anything is tagged | — | — | `release.yaml` `deps` job (`make vuln licenses`, read only) | upgrade the module, then release |
| PR changing `.goreleaser.yaml` or `release.yaml`; `main`; weekly | the release build without publishing: cosign, syft and goreleaser at `release.yaml`'s pins, four archives in the checksum file, an SBOM each, the binary starts | — | — | `release-snapshot` job (`scripts/checks/release-snapshot.sh`; passes at once on other PRs) | fix the release config |
| Release published | what a user downloads: every asset in the checksum file, its Sigstore signature from `release.yaml`, an SBOM per archive, build provenance from `release.yaml` (from 0.14.0), the linux binary's `-version`, the cask's version and sha256 | — | — | `release.yaml` `check` job (`scripts/release-check.sh --strict`) | fix it in a PR and release again; a failed attestation: `gh run rerun --failed` (skill `release`) |
| Monday 12:00 UTC, `gh workflow run release.yaml`, or merge of a PR labelled `release` | `[Unreleased]` → version, tag, binaries, cask | — | — | `release.yaml` | skill `release` |

What to run locally, and when:

| Moment | Run | Takes |
|--------|-----|-------|
| While coding | `go test ./pkg/model/...` (or the package you touch) | seconds |
| Before each commit | nothing extra: `pre-commit` runs the fast checks | < 1 s |
| UI change | `go test ./pkg/model -run TestGoldenScreens`; on purpose: `-update`, review the diff of `pkg/model/testdata/screens/`, commit it | seconds |
| Change to `pkg/client` or the session wiring | `scripts/kind-it.sh` (CI runs it too) | 2 min |
| UI or behaviour change done | `scripts/build.sh`, then the binary against any cluster (agents: one the owner allows), from a scratch dir; note what you checked (not which cluster) in the spec's verify task | minutes |
| Before a push | `git merge origin/main`, then `make check` (`pre-push` runs it too) | 1–5 min |
| After a push | `gh pr checks <n> --watch` | CI time |
| After the merge | skill `release`: confirm the version, `brew upgrade` | minutes |

Nothing from a real environment goes into this public repo: cluster
names, API endpoints, node names, IPs, account or company names — in code,
tests, specs, commit messages or PR text. Use made-up values. Names to
block go in the private deny list, one extended regex per line,
case-insensitive (`acme[ _-]?corp` catches `ACME Corp` and `acme-corp`):
`.git/info/denylist` on your clone and the `KNV_DENYLIST` secret in CI; the
list is never committed. Add your employer, customers, internal domains and
account names. CI logs print only the place and the rule, never the text.
Install gitleaks (`brew install gitleaks`) so secrets are caught before the
commit; without it the hook warns and CI catches them after the push —
too late for a secret, which must then be rotated.

`--no-verify` skips the git hooks; CI still runs every check that matters
for a merge. Use it only for a WIP push to a draft PR.

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
| Hook scripts | `scripts/agent-hooks/` (one script, a `claude`/`kiro` mode where output differs) | `.claude/settings.json` | `.kiro/hooks/kube-node-viewer.json`, loaded by the IDE 1.x and CLI V3 in any session (`kiro-cli chat --agent kube-node-viewer` included); CLI 2.x: none (the git hooks and CI still apply); offline check `scripts/agent-hooks/kiro-selftest.sh`, real-session check [`docs/kiro-validation.md`](docs/kiro-validation.md) |
| Permissions | — | `.claude/settings.json` (`settings.local.json` is personal) | per agent in `.kiro/agents/*.json` |
| Model | not pinned | session choice | session choice (`/model`) |

Skills: `spec-new`, `change-loop`, `pre-push`, `pr-review`, `release`,
`upstream-sync`.
Adding a skill: `.kiro/skills/<name>/SKILL.md` with `name` and `description`
frontmatter; both tools pick it up, and `scripts/agent-hooks/session-context.sh`
lists it. Adding a hook: write the script once, then wire it in both
`.claude/settings.json` and `.kiro/hooks/kube-node-viewer.json`, and add
its Kiro cases to `scripts/agent-hooks/kiro-selftest.sh`.
