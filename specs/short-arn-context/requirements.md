# Requirements: short-arn-context

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-05 (spec-refuter; 1 HIGH, 4 MEDIUM, 5 LOW, applied), 2026-11-05 (spec-refuter, code; 3 MEDIUM, 5 LOW, applied)

## Context

`aws eks update-kubeconfig` names the context after the cluster's ARN,
`arn:aws:eks:<region>:<account>:cluster/<name>`; that is the default of
`aws eks update-kubeconfig`. The three headers show the context name as it is
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
is `aws`, optionally followed by `-<letters>` groups (`aws-cn`,
`aws-us-gov`, `aws-iso-b`, `aws-eusc`…), `<region>` is lower-case letter
groups joined by `-`, ending in `-<digits>` (`us-east-1`, `cn-north-1`,
`us-gov-west-1`, `us-iso-east-1`, `eusc-de-east-1`…), `<account>`
is 12 digits and `<name>` is an EKS cluster name (a letter or digit, then
letters, digits, `-` or `_`, at most 100 characters), THEN the full panel's
`Context:` row, the compact box's first line and the one-line header SHALL
show `<name>` in place of the context name.

- [x] `arn:aws:eks:us-east-1:111122223333:cluster/demo-prod` shows
  `demo-prod` in all three headers, and no header contains `111122223333`.
- [x] `arn:aws-cn:eks:cn-north-1:111122223333:cluster/demo`,
  `arn:aws-us-gov:eks:us-gov-west-1:111122223333:cluster/demo` and
  `arn:aws-eusc:eks:eusc-de-east-1:111122223333:cluster/demo` show `demo`.
- [x] The full panel's `Context:` row then shows the cluster name, not the
  kubeconfig context name.

_Test: `// short-arn-context R1` on `TestShortContextName` (table) and
`TestHeadersShowTheShortContextName` (the three headers), pkg/model._

**R2** WHEN the context name is anything else THEN the headers SHALL show it
unchanged: non-ARN names (`demo-prod`, `kind-knv`), GKE names
(`gke_<project>_<location>_<cluster>`), ARNs of another service or resource
(`arn:aws:iam::…`, `arn:aws:eks:…:nodegroup/…`), and malformed EKS ARNs (an
account that is not 12 digits, an empty name, a name with `/`, `:` or a
space, a missing field, extra text before or after).

- [x] Each of those examples renders exactly as it is in all three headers.
- [x] An empty context still shows `-`.

_Test: `// short-arn-context R2` on `TestShortContextName` (table) and
`TestHeadersShowTheShortContextName`, pkg/model._

> **Corrected 2026-11-08.** GKE names are no longer shown unchanged: see
> [short-gke-context](../short-gke-context/requirements.md) R1.

**R3** WHEN the context is an EKS ARN THEN everything other than the three
headers SHALL keep the full context name: the contexts popup (`c`) and its
"connecting to …" line, the JSON export's `context` field and the export
file name, the saved-state key (`<context>@<server>`), the `-context` /
`--context` flag and the `context` config key, log lines and error messages.
One on-screen exception: the export notice in the footer (`export.go:385`,
shown for 5 seconds) names the file written, which embeds the full context;
it names the actual file, so a different name there would mislead.

- [x] The contexts popup lists the full ARN, marked `(current)`.
- [x] The JSON export's `context` is the full ARN.
- [x] The "connecting to …" row and a switch error row name the full ARN.
- [x] The saved-state key is unchanged, so a view saved before this change
  is restored.

> **Corrected 2026-11-08.** The export file name and the footer notice that
> names it no longer hold the full context: for an EKS ARN or GKE context they
> use the short name ([short-gke-context](../short-gke-context/requirements.md)
> R3). The JSON `context` keeps the full name.

_Test: `// short-arn-context R3` on `TestShortNameOnlyInTheHeaders`
(contexts popup rows, the switching row, the switch error row, export
`context`), pkg/model, and `TestClusterKeyKeepsTheFullContext`
(`Cluster{Context: <arn>, Server: s}.Key() == <arn>+"@"+s`), pkg/client._

_Test: manual — `--context`, the `context` config key and log lines read the
kubeconfig name and are untouched by the diff (`pkg/model/header.go` only)._

## Counter-examples

- The system SHALL NOT change the saved-state key: a cluster's saved view,
  keyed by `<context>@<server>`, would be lost.
- The system SHALL NOT shorten names in the contexts popup: two ARNs with the
  same cluster name in different accounts or regions must stay apart where
  the user picks one.
- The system SHALL NOT take the region from the ARN for the `Region:` row:
  that row is the pricing region, which comes from the API server URL or the
  nodes. With an ARN in `eu-west-1`, no region from the server and no nodes,
  the full panel's `Region:` row is `-` and the compact box has no
  `eu-west-1` (`TestARNRegionIsNotThePricingRegion`, cited
  `// short-arn-context R1`).
- The system SHALL NOT change how `--context` or the `context` config key
  match a kubeconfig context.
- The system SHALL NOT shorten a name that only looks like an EKS ARN.

## Out of scope

- GKE context names (`gke_<project>_<location>_<cluster>`) also carry an
  account-like id, the project; they are shown as they are (`TODO.md`).
  > **Corrected 2026-11-08.** Done in
  > [short-gke-context](../short-gke-context/requirements.md), which also
  > shortened the export file name below.
- The export file name, which embeds the context (`kube-node-viewer-<context>-…`),
  and the footer notice that names it: a local file next to the user,
  unchanged (`TODO.md`: consider a shorter export file name).
- Adding the region or the server to the one-line header.
- A flag or config key to turn the shortening off.
