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

## Follow-ups

- [ ] AWS pricing region when the API server is not an EKS URL (private
      endpoint, proxy): prices use the SDK region while the header shows the
      node label `topology.kubernetes.io/region`. Use the node label for
      pricing too.

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
