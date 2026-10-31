# Requirements: rebrand

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-10-30 (spec-refuter; 2 HIGH, 7 MEDIUM, all applied)

## Context

The code was imported verbatim from `awslabs/eks-node-viewer`
([ADR-001](../decisions/ADR-001-import-as-new-repo.md)). Module path, binary,
config file, docs, CI and governance files still carry the upstream name and
AWS-owned metadata (`CODEOWNERS` → `@awslabs/karpenter`, Homebrew `aws/tap`).
Shipping like this misattributes the project and implies AWS endorsement.

## Usage examples

```text
$ go install github.com/staffops/kube-node-viewer/cmd/kube-node-viewer@latest
$ kube-node-viewer -version
kube-node-viewer version v0.1.0      # from module build info, no ldflags
...
$ scripts/build.sh && bin/kube-node-viewer -version
kube-node-viewer version 9f184a1     # from ldflags, git describe
commit: 9f184a1...
```

```text
# A cluster whose nodes carry the upstream price label keeps working unchanged:
$ kubectl label node ip-10-0-1-5 eks-node-viewer/instance-price=0.0416
$ kube-node-viewer          # shows $0.0416/hour for that node
```

```text
# A saved upstream config keeps working:
$ cat ~/.eks-node-viewer
node-sort=eks-node-viewer/node-cpu-usage=dsc
$ kube-node-viewer
kube-node-viewer: reading legacy config ~/.eks-node-viewer; move it to ~/.kube-node-viewer   (stderr)
# TUI sorted by CPU usage, as before
```

## Requirements

**R1** WHEN the module is imported or installed THEN its path SHALL be
`github.com/staffops/kube-node-viewer` (see D1).

- [ ] `go.mod`, every import and `.golangci.yaml` `local-prefixes` use it;
  `grep -rn 'github.com/awslabs/eks-node-viewer' --include='*.go' --include=go.mod --include='*.yaml' .` is empty.
- [ ] After the first push and tag: `go install github.com/staffops/kube-node-viewer/cmd/kube-node-viewer@<tag>` produces `kube-node-viewer`.

**R2** WHEN the binary runs THEN it SHALL identify itself as
`kube-node-viewer` (`-version`, `-h` usage text, flag descriptions), and WHEN
it was built without ldflags THEN `-version` SHALL report the module version
from the Go build info instead of `dev`.

- [ ] `grep -rn eks-node-viewer cmd/ --exclude=ATTRIBUTION.md` only matches
  compatibility code (R3, R4b).
- [ ] Unit test for the build-info fallback.

**R3** WHEN `~/.kube-node-viewer` exists THEN the system SHALL read it as the
config file; WHEN it does not exist but `~/.eks-node-viewer` does THEN the
system SHALL read the legacy file and print one warning line on stderr,
before the TUI starts, naming the new path.

- [ ] Unit test (home dir injected) for: new only, legacy only (warning),
  both (new wins, no warning), neither (no warning).

**R4a** WHEN a node carries the `kube-node-viewer/instance-price` or
`eks-node-viewer/instance-price` label THEN the system SHALL use its value as
the node price; WHEN both are present THEN the new key SHALL win; WHEN the
winning value does not parse as a number THEN the system SHALL try the other
key, then the pricing provider (today's fallback behaviour).

- [ ] Tests in `pkg/client` for old key, new key, both, new unparsable.

**R4b** WHEN a user passes a computed label name (`<prefix>/node-age`,
`<prefix>/node-<resource>-usage`) to `--extra-labels`, `--node-sort`,
`--group-by` or the config file THEN the system SHALL accept both
`kube-node-viewer/` and `eks-node-viewer/` as `<prefix>` with identical
results.

- [ ] Tests in `pkg/model` against `ComputeLabel` for both prefixes.

**R5** WHEN someone reads the repository THEN `README.md` SHALL describe
`kube-node-viewer`, state it is based on `awslabs/eks-node-viewer` (Apache
2.0), credit contributors of imported fork changes, keep the upstream history
(origin at AWS, talks) as credit only, point badges at this repository, and
use no AWS/Amazon branding implying endorsement.

**R6** WHEN the repository is inspected THEN AWS governance files SHALL be
gone or replaced: `CODEOWNERS` reads `* @karlipegomes @sule26`,
`CODE_OF_CONDUCT.md` deleted, `CONTRIBUTING.md` rewritten to
point to `AGENTS.md`; Homebrew `aws/tap` instructions and `hack/homebrew.go`
removed.

**R7** WHEN a tag is pushed THEN the release workflow SHALL build with
goreleaser from the committed tree (no network refresh of generated files)
and produce `kube-node-viewer_*` artifacts; WHEN a PR is opened THEN CI SHALL
run `scripts/verify.sh` and `scripts/build.sh`.

- [ ] `goreleaser check && goreleaser build --snapshot --clean` produces
  `dist/kube-node-viewer_*`.
- [ ] Neither workflow runs `make generate`.

**R8** WHEN the code is distributed THEN `LICENSE`, the original `NOTICE`
line and every existing Apache license header SHALL still be present; `NOTICE` gains
a line for this project; every upstream file changed in this spec carries the
modification note of D5; new code is licensed Apache 2.0.

- [ ] `git diff af96088 -- LICENSE` empty; `NOTICE` first line unchanged.
- [ ] Every `.go`/`.sh` file in `git diff --name-only --diff-filter=M af96088`
  contains the D5 note.

## Counter-examples

- SHALL NOT remove or rewrite the `NOTICE` copyright line or existing Apache headers.
- SHALL NOT drop support for the `eks-node-viewer/instance-price` label or the
  `eks-node-viewer/` computed names (clusters and saved configs carry them;
  breaking them is invisible until prices or columns disappear).
- SHALL NOT change pricing behavior or remove AWS pricing.
- SHALL NOT print the legacy-config warning on stdout (it would pollute
  `-version` and `-attribution` output).

## Out of scope

- Cloud-agnostic pricing / removing `pkg/aws` (separate spec if wanted).
- Refreshing AWS price tables (`make generate`).
- Closing or rebasing open dependabot branches (they conflict after T1;
  dependabot recreates them).

## Resolved questions

Answered by the owner on 2026-10-30.

- Module path: `github.com/staffops/kube-node-viewer` (lowercase, matches
  `otel-libs`; Go module paths are case-sensitive, GitHub URLs are not).
- License for new code: Apache 2.0, one license for the whole tree.
- `CODEOWNERS`: `* @karlipegomes @sule26` (Carlos Gomes and João Pedro
  Alcantara). GitHub ignores owners without write access, so `sule26` needs
  write access to `StaffOps/kube-node-viewer` for the rule to apply.
