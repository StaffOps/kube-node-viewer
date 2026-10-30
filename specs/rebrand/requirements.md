# Requirements: rebrand

> Status: draft · Owner: Carlos Gomes · Reviewed: — (spec-refuter pending)

## Context

The code was imported verbatim from `awslabs/eks-node-viewer`
([ADR-001](../decisions/ADR-001-import-as-new-repo.md)). Module path, binary,
config file, docs and governance files still carry the upstream name and
AWS-owned metadata (`CODEOWNERS` → `@awslabs/karpenter`, Homebrew `aws/tap`).
Shipping like this misattributes the project and implies AWS endorsement.

## Usage examples

```text
$ go install github.com/staffops/kube-node-viewer/cmd/kube-node-viewer@latest
$ kube-node-viewer -version
kube-node-viewer version v0.1.0
commit: 0e1e53d
...
```

```text
# A cluster whose nodes carry the upstream label keeps working unchanged:
$ kubectl label node ip-10-0-1-5 eks-node-viewer/instance-price=0.0416
$ kube-node-viewer          # shows $0.0416/hour for that node
```

## Requirements

**R1** WHEN the module is imported or installed THEN its path SHALL be the
repository path (see D1).

- [ ] `go.mod`, every import, `.golangci.yaml` `local-prefixes` use it.
- [ ] `go install <path>/cmd/kube-node-viewer@<ref>` produces `kube-node-viewer`.

**R2** WHEN the binary runs THEN it SHALL identify itself as
`kube-node-viewer` (`-version`, `-h` usage text, flag descriptions).

- [ ] `grep -rn --exclude=ATTRIBUTION.md 'eks-node-viewer' cmd/` only matches
  compatibility code (R3, R4); `ATTRIBUTION.md` regenerated (T5b).

**R3** WHEN `~/.kube-node-viewer` exists THEN the system SHALL read it as the
config file; WHEN it does not exist but `~/.eks-node-viewer` does THEN the
system SHALL read the legacy file and print one warning naming the new path.

- [ ] Unit test for: new only, legacy only (warning), both (new wins, no warning), neither.

**R4** WHEN a node carries `eks-node-viewer/instance-price`,
`eks-node-viewer/node-age` or `eks-node-viewer/node-<resource>-usage`
THEN the system SHALL keep honoring them, and SHALL also honor the same keys
under the `kube-node-viewer/` prefix, the new prefix winning on conflict.

- [ ] Tests in `pkg/model` / `pkg/client` for old key, new key, both.

**R5** WHEN someone reads the repository THEN `README.md` SHALL describe
`kube-node-viewer`, state it is based on `awslabs/eks-node-viewer` (Apache
2.0), credit contributors of imported fork changes, and use no AWS/Amazon
branding implying endorsement.

**R6** WHEN the repository is inspected THEN AWS governance files SHALL be
gone or replaced: `CODEOWNERS` deleted (single maintainer, no team to route
reviews to), `CODE_OF_CONDUCT.md` deleted, `CONTRIBUTING.md` rewritten to
point to `AGENTS.md`; Homebrew `aws/tap` instructions and `hack/homebrew.go`
removed.

**R7** WHEN a tag is pushed THEN the release workflow and `.goreleaser.yaml`
SHALL produce `kube-node-viewer_*` artifacts.

**R8** WHEN the code is distributed THEN `LICENSE`, the original `NOTICE`
line and every existing Apache license header SHALL still be present; `NOTICE` gains
a line for this project; files changed from upstream carry a modification
note; new code is licensed Apache 2.0.

- [ ] `git diff af96088 -- LICENSE` empty; `NOTICE` first line unchanged.

## Counter-examples

- SHALL NOT remove or rewrite the `NOTICE` copyright line or existing Apache headers.
- SHALL NOT drop support for `eks-node-viewer/*` node labels (users' clusters
  carry them; breaking them is invisible until prices disappear).
- SHALL NOT change pricing behavior or remove AWS pricing.

## Out of scope

- Cloud-agnostic pricing / removing `pkg/aws` (separate spec if wanted).
- Refreshing AWS price tables (`make generate`).
- Merging `feat/sule26-group-by` (it will need the same import-path rename
  when rebased).

## Resolved questions

Answered by the owner on 2026-10-30.

- Module path: `github.com/staffops/kube-node-viewer` (lowercase, matches
  `otel-libs`; Go module paths are case-sensitive, GitHub URLs are not).
- License for new code: Apache 2.0, one license for the whole tree.
- `CODEOWNERS`: not needed. Single maintainer; it only adds value when
  reviews must be routed to a team or required by branch protection.
  Re-add it when a second maintainer joins.
