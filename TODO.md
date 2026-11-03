# TODO

Backlog for kube-node-viewer: what the owner has to do, follow-ups found
while working, ideas, and deferred work. Rules in
[`AGENTS.md`](AGENTS.md#backlog-todomd): add what you find out of scope,
remove an item in the commit that does it, and a spec comes first when the
change needs one. Deferred tables in `specs/*/tasks.md` stay the record for
their feature; the rows below link them.

## Owner actions

- [ ] Optional: an IAM role with `pricing:GetProducts` trusted by GitHub
      OIDC, its ARN in the repo variable `AWS_CATALOG_ROLE_ARN`, so the
      monthly `refresh-data.yaml` also refreshes the instance catalog (today
      it refreshes only the static price tables).

## Guardrails

The layers in `AGENTS.md` ("Guardrails") stop honest mistakes; these close
what they cannot. Listed by impact.

- [ ] **Protect `main` with a repository ruleset**: require a PR and the
      `verify`, `lint` and `build` checks; block force-pushes and deletion;
      protect `v*` tags; let only the release workflow (GitHub Actions)
      bypass, for its version commit and tag.
      _Why:_ today anyone with write access can push to `main` with
      `--no-verify` and skip CI, the spec-task gate and the cluster-run
      rule; the `pre-push` hook only protects people who don't try. A
      force-push to `main` also forces every clone and open branch to be
      reset.
- [ ] **Turn on secret scanning and push protection; add gitleaks** to
      `pre-commit` and CI, with rules for cluster data (kubeconfig
      contents, bearer tokens, private IPs in data files).
      _Why:_ both GitHub features are free for public repos and refuse a
      push that carries a credential. Nothing today inspects file
      contents: the export-file check matches names only, so cluster data
      under another name would still be committed.
- [ ] **Hook tests in CI**: run the scenarios used when the guardrails were
      built (export staged, released CHANGELOG edited, unformatted Go,
      missing header, bad commit subject, AI trailer, push to `main`, PR
      with an open spec task, draft PR) plus `shellcheck`, as a CI job.
      _Why:_ the guardrails are shell scripts that nothing tests; a
      regex or path change could disable one silently, and the first sign
      would be a bad commit on `main`.
- [ ] **Dependency checks**: `govulncheck` in `make check` and CI;
      `go-licenses` (the `make licenses` allowlist) in CI; Dependabot
      security updates on, with its patch PRs merged by the automerge job
      when CI passes.
      _Why:_ the binary ships client-go and the AWS SDK; a known
      vulnerability or a dependency under a disallowed license would
      reach a release today, since no automated step looks. Dependabot is
      excluded from automerge now (not a repo writer), so its fixes wait.
- [ ] **Release smoke test**: after GoReleaser, download the published
      linux/amd64 archive, check that `kube-node-viewer -version` matches
      the tag and the checksum file, and run `brew audit` on the cask.
      _Why:_ a green release run proves the upload, not the artefact; a
      broken binary or a cask with a wrong URL or checksum would only show
      when someone installs it.
- [ ] **Confirm the Kiro hooks in a real session**: the Kiro config
      follows the Kiro docs but has not run in Kiro yet; a `git commit`
      with a non-conventional subject should be refused by both the git
      hook and the Kiro `commit-guard`.
      _Why:_ if Kiro ignores a field, its agent loses the early warnings
      without any error; the git hooks and CI still hold.

## Spec-driven sync

Keep specs and code telling the same story, checked by CI rather than by
memory. Listed by impact.

- [ ] **Spec review as a CI gate**: a spec may be **active** only with a
      `Reviewed:` header naming `spec-refuter` and a date; a PR may not add
      code under a spec that is still **draft**.
      _Why:_ `AGENTS.md` makes the independent review mandatory, but 4 of
      9 specs skipped it (`aks-pricing`, `node-details`: owner only;
      `monthly-price-columns`, `sort-invert`: code-review agent). A rule
      nothing checks erodes the first time someone is in a hurry.
- [ ] **Requirement → test traceability in CI**: every `**R<n>**` of an
      active or shipped spec must be named by a test (`// <spec> R<n>`);
      a requirement without one fails the PR.
      _Why:_ only 9 test files cite requirements today, so nothing shows
      which requirements are untested. This is what keeps a spec from
      describing behaviour the code no longer has.
- [ ] **Drift detector**: when key bindings (`helpKeys`), flags
      (`flag.go`) or the column list change without a change to a spec
      or to `README.md`, CI flags the PR.
      _Why:_ behaviour changes made "while there" are the usual way
      specs and docs go stale; the spec-task gate only fires when a spec
      is touched.
- [ ] **PR template** (`.github/pull_request_template.md`): spec link,
      requirements covered, the cluster it ran on, the version the merge
      releases (`python3 scripts/release-prepare.py`).
      _Why:_ the same four facts are asked of every PR; a template makes
      people and agents fill them in without being told, and reviewers
      see them in one place.
- [ ] **ADRs for the cross-cutting decisions**: automerge for writers,
      release from CHANGELOG headings, guardrail layers (git hooks + agent
      hooks + CI), history rewrite policy.
      _Why:_ only ADR-001 exists; these decisions change how everyone
      works, and without a record of the alternatives they will be
      reargued or undone by accident.

## Security

Beyond the Guardrails section above.

- [ ] **`SECURITY.md`**: how to report a vulnerability privately (GitHub
      private advisories), supported versions.
      _Why:_ the repo is public and ships a binary through Homebrew;
      without a policy, a finder's only channel is a public issue.
- [ ] **Least-privilege workflows and actions pinned by SHA**, audited by
      `zizmor` in CI.
      _Why:_ actions are referenced by tag, and a moved tag runs someone
      else's code with this repo's `contents: write` token, which can push
      to `main` and publish releases. Pinning is the standard defence.
- [ ] **Signed releases**: cosign signatures or SLSA provenance from
      GoReleaser, and an SBOM attached to each release.
      _Why:_ users installing through `brew` cannot tell today whether an
      archive came from this repo's workflow; signatures and an SBOM make
      that checkable and list what the binary contains.
- [ ] **OpenSSF Scorecard** workflow and badge.
      _Why:_ an outside, repeatable score of the supply-chain posture
      that points at what is still missing as the project grows.

## Evolution

- [ ] **Integration tests on a `kind` cluster in CI**: start kind, apply
      fixture nodes and pods, run the TUI headless, assert on the screen.
      _Why:_ the "ran on a cluster" rule depends on someone having one;
      on 2026-11-03 none was available. A kind run makes the basic
      cluster check automatic, leaving real clusters for cloud-specific
      behaviour (prices, node pools).
- [ ] **Golden-file screen snapshots in PRs**: render the main screens
      from the made-up cluster used for screenshots and show the text
      diff in the PR.
      _Why:_ UI changes are reviewed today by reading code or running the
      binary; a diff of the screen makes layout regressions visible in
      review.

## Follow-ups

- [ ] AWS pricing region when the API server is not an EKS URL (private
      endpoint, proxy): prices use the SDK region while the header shows the
      node label `topology.kubernetes.io/region`. Use the node label for
      pricing too.

- [ ] config-views error polish (spec-refuter 2026-11-03): `- label: zone`
  (space after the colon) shows a Go `map[...]`; padded values in
  `sort`/`resources`/`group-by` load silently; `sort: price/m` suggests
  `label:` instead of `sort: price`. See `specs/config-views/`.

## Ideas

- [ ] Recommendations for AKS: Azure VM catalog and family tables, so the
      Recommendations screen works on Azure nodes
      ([aks-pricing deferred](specs/aks-pricing/tasks.md#deferred),
      [recommendations deferred](specs/recommendations/tasks.md#deferred)).
- [ ] GKE: node pool labels and prices (stage 2, multi-cloud;
      [interactive-tui deferred](specs/interactive-tui/tasks.md#deferred)).
- [ ] GPU resource and columns (stage 3;
      [interactive-tui deferred](specs/interactive-tui/tasks.md#deferred)).

## Deferred (with promotion trigger)

| Item | Trigger | Source |
|------|---------|--------|
| Typed filter syntax in `/` (`cap:spot pool:gpu`) | panels feel slow for frequent filters | [interactive-tui](specs/interactive-tui/tasks.md#deferred), [views-and-export](specs/views-and-export/tasks.md#deferred) |
| OR across filter fields | owner asks | [interactive-tui](specs/interactive-tui/tasks.md#deferred) |
| Choosing the export directory | users run it from read-only directories | [views-and-export](specs/views-and-export/tasks.md#deferred) |
| Pod-level rightsizing (requests too high) | owner asks | [recommendations](specs/recommendations/tasks.md#deferred) |
| Commitments: Savings Plans, RIs, Azure reservations | someone needs effective rather than list cost | [recommendations](specs/recommendations/tasks.md#deferred), [aks-pricing](specs/aks-pricing/tasks.md#deferred) |
| A second Recommendations total including Spot | owner asks | [recommendations](specs/recommendations/tasks.md#deferred) |
| Drop the `eks-node-viewer/*` label prefix fallback | a major release, and no known user carries the old names | [rebrand](specs/rebrand/tasks.md#deferred) |
