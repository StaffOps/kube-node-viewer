# Requirements: short-gke-context

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-08 (spec-refuter; 6 MEDIUM, 5 LOW, applied), 2026-11-08 (spec-refuter, code; 3 MEDIUM, 5 LOW, applied)

## Context

`gcloud container clusters get-credentials` names the context
`gke_<project>_<location>_<cluster>`. The three headers show that name as it
is, so a screenshot shares the GCP project id, as EKS ARNs shared the AWS
account id before [short-arn-context](../short-arn-context/requirements.md).

The export has the same problem for both clouds: the file name is
`kube-node-viewer-<context>-<YYYYMMDD-HHMMSS>` (`pkg/model/export.go:243-248`),
and the footer shows that name for 5 seconds after `e`
(`pkg/model/export.go:385`). So the file name, and the screen for those
seconds, carry the account id or the project id. short-arn-context left this
as an accepted exception.

No GKE cluster is available to this project; the names below are made up and
the formats come from Google's documentation (design D2).

## Usage examples

With a made-up context `gke_example-project_us-central1-a_demo-prod`:

```text
(full panel, before)
 Context:  gke_example-project_us-central1-a_demo-prod
(full panel, after)
 Context:  demo-prod

(compact box, after)
 demo-prod · 10 of 10 shown · 220 pods

(one-line header, after)
 demo-prod │ 10 of 10 shown · 220 pods │ Filters: none │ …

(contexts popup `c`, unchanged)
 gke_example-project_us-central1-a_demo-prod  (current)
```

Pressing `e` with an EKS ARN or a GKE context:

```text
(file name and footer, before)
 exported 10 nodes to ./kube-node-viewer-arn_aws_eks_us-east-1_111122223333_cluster_demo-prod-20261102-153012.csv and .json
(file name and footer, after)
 exported 10 nodes to ./kube-node-viewer-demo-prod-20261102-153012.csv and .json
```

## Requirements

**R1** WHEN the context name is a GKE context, exactly
`gke_<project>_<location>_<cluster>` where `<project>` is a GCP project id (6
to 30 lower-case letters, digits or `-`, starting with a letter and not
ending with `-`), optionally preceded by a legacy organization domain and a
colon (`example.com:my-project`), `<location>` is a GCP region or zone (lower-case
letters, `-`, lower-case letters ending in digits, optionally `-` and a
lower-case suffix: `us-central1`, `europe-west4-b`, `us-central1-ai1a`) and `<cluster>` is a GKE cluster name (at
most 40 lower-case letters, digits or `-`, starting with a letter and ending
with a letter or a digit), THEN the full panel's `Context:` row, the compact
box's first line and the one-line header SHALL show `<cluster>` in place of
the context name.

- [x] `gke_example-project_us-central1-a_demo-prod` shows `demo-prod` in all
  three headers, and no header contains `example-project`.
- [x] A region (`us-central1`), a zone (`europe-west4-b`) and an AI zone
  (`us-central1-ai1a`) all match.
- [x] A domain-scoped project (`gke_example.com:example-project_us-central1-a_demo`)
  shows `demo`, and no header contains `example.com`.

_Test: `// short-gke-context R1` on `TestShortContextName` (table) and
`TestHeadersShowTheShortGKEName` (the three headers), pkg/model._

**R2** WHEN a context name starts with `gke_` and is not exactly that form
THEN the headers SHALL show it unchanged: a project that is too short or too
long, starts with a digit or ends with `-`; a missing field; an extra `_`
field; upper-case letters; a cluster name over 40 characters, starting with
a digit or ending with `-`; a location with another shape (S3NS's
`u-france-east1` included); extra text before or after, a trailing newline,
or non-ASCII letters; a malformed domain (no dot, an empty label). Two other
gcloud forms are not this one and also stay as they are:
`connectgateway_{project}_{location}_{membership}` and the GKE multicloud
`gke_{kind}_{project}_{location}_{cluster}`.

- [x] Each of those renders exactly as it is.
- [x] Names that are neither GKE nor EKS ARNs still render as they are.

_Test: `// short-gke-context R2` on `TestShortContextName` (table), pkg/model._

> **Corrected 2026-11-08.** The R1 and R2 table is `TestShortGKEContextName`,
> not `TestShortContextName`, whose old row for a GKE name staying unchanged
> was deleted rather than flipped.

**R3** WHEN `e` writes an export for a context that is an EKS ARN
([short-arn-context](../short-arn-context/requirements.md) R1) or a GKE name
(R1) THEN the file name SHALL be
`kube-node-viewer-<cluster>-<YYYYMMDD-HHMMSS>` with `<cluster>` the short
name, and the footer notice SHALL name that file. WHEN the context is any
other name THEN the file name SHALL be as today: the context reduced to
`[A-Za-z0-9._-]`, or `cluster` when empty. A name already in use SHALL still
get `-2`, `-3`… (files are created exclusively).

- [x] An EKS ARN gives `kube-node-viewer-prod-20261102-153012`, with no
  account id in it.
- [x] A GKE name gives `kube-node-viewer-demo-prod-…`, with no project id.
- [x] Two exports of same-named clusters in the same second give two pairs
  of files, the second with `-2`.
- [x] The footer notice after `e` holds neither the account id nor the
  project id.

_Test: `// short-gke-context R3` on `TestExportBaseName` (table),
`TestExportFileNameIsShort` (the written files and the notice) and
`TestSameShortNameDoesNotOverwrite`, pkg/model._

**R4** WHEN the context is an EKS ARN or a GKE name THEN everything other than
the three headers and the export file name SHALL keep the full context name:
the contexts popup (`c`) and its "connecting to …" line, the JSON export's
`context` field, the saved-state key (`<context>@<server>`), the `--context`
flag and the `context` config key, log lines and error messages.

- [x] The contexts popup lists the full GKE name, marked `(current)`.
- [x] The JSON export's `context` is the full name, for an ARN and a GKE name.
- [x] The saved-state key is unchanged.

_Test: `// short-gke-context R4` on `TestShortGKENameOnlyInTheHeaders`
(popup rows, "connecting to" row, export `context`), pkg/model, and
`TestClusterKeyKeepsTheFullContext` (the GKE case), pkg/client._

_Test: manual — only the `--context` flag, the `context` config key and log
and error lines: they read the kubeconfig name and are untouched by the diff.
The popup, the "connecting to" row, the JSON `context` and the state key have
the tests above._

## Counter-examples

- The system SHALL NOT change the saved-state key: a saved view keyed by
  `<context>@<server>` would be lost.
- The system SHALL NOT shorten names in the contexts popup, where the user
  picks among contexts.
- The system SHALL NOT remove or shorten the JSON export's `context` field:
  it is what identifies the cluster of a file whose name no longer does.
- The system SHALL NOT take the location from a GKE name for the `Region:`
  row: that row is the pricing region (`TestGKELocationIsNotThePricingRegion`,
  cited `// short-gke-context R1`).
- The system SHALL NOT overwrite an export whose short name and second match.
- The system SHALL NOT shorten a name that only looks like a GKE context.

## Out of scope

- Prices and node pool labels for GKE (`TODO.md`, Ideas).
- Adding the location or the project to any header.
- `connectgateway_…` and GKE multicloud (`gke_{kind}_…`) context names, which
  keep the project id in the headers (`TODO.md`).
- Other places that can hold a project or account id on screen: the Provider
  ID row of node details (`pkg/model/nodedetails.go:125`, `gce://<project>/…`
  by convention, unobserved here) and a node name when a cloud puts one in
  it (`TODO.md`). This spec's claim is only that the three headers and the
  export file name no longer show it.
- AKS context names, which carry no account-like id by default.
- A flag or config key to turn the shortening off.
