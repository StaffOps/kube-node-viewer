# Requirements: short-arn-context

> Status: draft · Owner: Carlos Gomes · Reviewed: —

## Context

`aws eks update-kubeconfig` names the context after the cluster's ARN,
`arn:aws:eks:<region>:<account>:cluster/<name>`, and most EKS users keep
that default. The three headers show the context name as it is
(`pkg/model/header.go:158`): the full panel's `Context:` row, the compact
box's first line and the one-line header. So every screenshot of the tool
shares the AWS account id, and in the compact box the ARN (about 50
characters for a short cluster name: `arn:aws:eks:` 12, the region 9 to 14,
the account 12, `:cluster/` 9, then the name) takes most of the line that
[compact-header-no-url](../compact-header-no-url/requirements.md) just
freed. The region is already shown on its own in the full panel and the
compact box, so the only part of the ARN the header needs is `<name>`.

## Usage examples

With a made-up context `arn:aws:eks:us-east-1:111122223333:cluster/demo-prod`:

```text
(full panel, before)
 Context:  arn:aws:eks:us-east-1:111122223333:cluster/demo-prod
(full panel, after)
 Context:  demo-prod

(compact box, before)
 arn:aws:eks:us-east-1:111122223333:cluster/demo-prod · us-east-1 · 10 of 10 shown · 220 pods · $728.102/month …
(compact box, after)
 demo-prod · us-east-1 · 10 of 10 shown · 220 pods · $728.102/month …

(one-line header, after)
 demo-prod │ 10 of 10 shown · 220 pods · $728.102/month … │ Filters: none │ …

(contexts popup `c`, unchanged)
 arn:aws:eks:us-east-1:111122223333:cluster/demo-prod  (current)
 staging
```

## Requirements

**R1** WHEN the context name is an EKS cluster ARN, exactly
`arn:<partition>:eks:<region>:<account>:cluster/<name>` where `<partition>`
is `aws` or `aws-` followed by lower-case letters and hyphens (`aws-cn`,
`aws-us-gov`, `aws-iso-b`…), `<region>` is an AWS region code
(`us-east-1`, `cn-north-1`, `us-gov-west-1`, `us-iso-east-1`…), `<account>`
is 12 digits and `<name>` is an EKS cluster name (a letter or digit, then
letters, digits, `-` or `_`, at most 100 characters), THEN the full panel's
`Context:` row, the compact box's first line and the one-line header SHALL
show `<name>` in place of the context name.

- [ ] `arn:aws:eks:us-east-1:111122223333:cluster/demo-prod` shows
  `demo-prod` in all three headers, and no header contains `111122223333`.
- [ ] `arn:aws-cn:eks:cn-north-1:111122223333:cluster/demo` and
  `arn:aws-us-gov:eks:us-gov-west-1:111122223333:cluster/demo` show `demo`.

_Test: `// short-arn-context R1` on `TestShortContextName` (table) and
`TestHeadersShowTheShortContextName` (the three headers), pkg/model._

**R2** WHEN the context name is anything else THEN the headers SHALL show it
unchanged: non-ARN names (`demo-prod`, `kind-knv`), GKE names
(`gke_<project>_<location>_<cluster>`), ARNs of another service or resource
(`arn:aws:iam::…`, `arn:aws:eks:…:nodegroup/…`), and malformed EKS ARNs (an
account that is not 12 digits, an empty name, a name with `/`, `:` or a
space, a missing field, extra text before or after).

- [ ] Each of those examples renders exactly as it is in all three headers.
- [ ] An empty context still shows `-`.

_Test: `// short-arn-context R2` on `TestShortContextName` (table) and
`TestHeadersShowTheShortContextName`, pkg/model._

**R3** WHEN the context is an EKS ARN THEN everything other than the three
headers SHALL keep the full context name: the contexts popup (`c`) and its
"connecting to …" line, the JSON export's `context` field and the export
file name, the saved-state key (`<context>@<server>`), the `-context` /
`--context` flag and the `context` config key, log lines and error messages.

- [ ] The contexts popup lists the full ARN, marked `(current)`.
- [ ] The JSON export's `context` is the full ARN.
- [ ] The saved-state key is unchanged, so a view saved before this change
  is restored.

_Test: `// short-arn-context R3` on `TestShortNameOnlyInTheHeaders`
(contexts popup rows, export `context`), pkg/model. The state key is built
in `pkg/client` from the kubeconfig (`client.go:85`); the change touches
neither `pkg/client` nor `pkg/state`._

## Counter-examples

- The system SHALL NOT change the saved-state key: a cluster's saved view,
  keyed by `<context>@<server>`, would be lost.
- The system SHALL NOT shorten names in the contexts popup: two ARNs with the
  same cluster name in different accounts or regions must stay apart where
  the user picks one.
- The system SHALL NOT take the region from the ARN for the `Region:` row:
  that row is the pricing region, which comes from the API server URL or the
  nodes.
- The system SHALL NOT change how `--context` or the `context` config key
  match a kubeconfig context.
- The system SHALL NOT shorten a name that only looks like an EKS ARN.

## Out of scope

- GKE context names (`gke_<project>_<location>_<cluster>`) also carry an
  account-like id, the project; they are shown as they are (`TODO.md`).
- The export file name, which embeds the context (`kube-node-viewer-<context>-…`):
  a local file next to the user, unchanged.
- A flag or config key to turn the shortening off.
