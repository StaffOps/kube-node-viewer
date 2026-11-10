# Requirements: short-edge-context

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-10 (spec-refuter; 1 HIGH, 2 MEDIUM, 5 LOW, applied)

## Context

`gcloud edge-cloud container clusters get-credentials` names the context
`edgecontainer_<project>_<location>_<cluster>`
(`googlecloudsdk/command_lib/edge_cloud/container/kubeconfig.py:64-77`, called
at `surface/edge_cloud/container/clusters/get_credentials.py:64-66` with the
cluster reference's `projectsId`, `locationsId` and `clustersId`). With
`--offline-credential` it appends `_<userId>_offline`
(`get_credentials.py:83`). The headers and the export file name show the name
whole, so a screenshot or an attached export shares the GCP project id, as GKE
contexts did before [short-gke-context](../short-gke-context/requirements.md)
and [hide-cloud-ids](../hide-cloud-ids/requirements.md), which left this form
out of scope.

The API server URL of such a context is `https://<endpoint>:<port>`
(`kubeconfig.py:220-221`, and `:162-169` on the offline path), so the
`Cluster:` row holds no project.

No Google Distributed Cloud cluster is available to this project; the names
below are made up, and the format comes from the gcloud source (design D1).

## Usage examples

With a made-up context:

```text
(full panel, before)
 Context:  edgecontainer_example-project_us-central1_demo-edge
(full panel, after)
 Context:  demo-edge

(contexts popup `c`, unchanged)
 edgecontainer_example-project_us-central1_demo-edge  (current)

(file name after `e`)
 kube-node-viewer-demo-edge-20261110-153012.csv
```

An offline credential's context,
`edgecontainer_example-project_us-central1_demo-edge_<user id>_offline`, also
shows `demo-edge`.

## Requirements

**R1** WHEN the context name is exactly
`edgecontainer_<project>_<location>_<cluster>`, `<project>` and `<location>`
as in short-gke-context R1 and `<cluster>` 1 to 63 lower-case letters, digits
or `-`, starting and ending with a letter or a digit, THEN the three headers
SHALL show `<cluster>` in place of the context name.

- [x] A region and a zone both match.
- [x] A domain-scoped project (`example.com:example-project`) matches.
- [x] No header contains the project.

_Test: `// short-edge-context R1` on `TestShortEdgeContextName` (table) and
`TestHeadersHideTheEdgeProject` (the three headers), pkg/model._

**R2** WHEN the context name is the form of R1 followed by `_`, a user id of
one or more characters with no white space, and `_offline` THEN the three
headers SHALL show `<cluster>`.

- [x] A user id holding `_`, `@` and `.` matches.
- [x] The headers hold neither the project nor the user id.

_Test: `// short-edge-context R2` on `TestShortEdgeContextName` and
`TestHeadersHideTheEdgeProject`, pkg/model._

**R3** WHEN a context name starts with `edgecontainer_` and is neither form
above THEN the headers SHALL show it unchanged: a project, location or cluster
outside its rule; a project number; a cluster of 64 characters; upper-case or
non-ASCII letters; a missing or extra field; a user id suffix without
`_offline`, or with an empty user id; extra text before or after; a trailing
newline.

- [x] Each of those renders exactly as it is.
- [x] Every name the patterns of short-arn-context, short-gke-context and
  hide-cloud-ids accept or refuse keeps its result.

_Test: `// short-edge-context R3` on `TestShortEdgeContextName`, pkg/model._

**R4** WHEN the context is a form of R1 or R2 THEN the export file name SHALL
use the short name as short-gke-context R3 says, and the contexts popup and its
"connecting to" line, the JSON export's `context`, the saved-state key, the
`--context` flag, the `context` config key, log lines and error messages SHALL
keep the full name.

- [x] The file name and the footer notice hold no project.
- [x] The popup, the "connecting to" row and the JSON `context` hold the full
  name.
- [x] The saved-state key is unchanged.

_Test: `// short-edge-context R4` on `TestEdgeNameInExportAndPopup`,
pkg/model, and `TestClusterKeyKeepsTheFullContext`, pkg/client._

_Test: manual — only the `--context` flag, the `context` config key and log and
error lines: they read the kubeconfig name and are untouched by the diff._

**R5** WHEN a commit adds a line holding an `edgecontainer_` context name that
is not an example (`edgecontainer_example…`), in either form and in any case,
THEN the real-environment data check SHALL refuse it, as it refuses a real
`gke_` or `connectgateway_` name.

- [x] A real-looking name, its offline form, its domain-scoped form, a mixed
  case name and a real name after an example on one line are blocked.
- [x] An `edgecontainer_example…` name passes, also inside an export file name.

_Test: `# short-edge-context R5` on the `test-checks.sh` cases, scripts/checks._

> **Corrected 2026-11-10 (code review).** R3's "an extra field" holds for
> names without a `_<user id>_offline` ending: a name ending in `_x_offline`
> takes everything after the cluster as the user id, so a malformed name such as
> `edgecontainer_<project>_<location>_<extra>_<cluster>_x_offline` shows its
> fourth field (design D1); it shows less, never more. R2's "no white space"
> means no ASCII space, tab, carriage return, line feed or form feed; other
> spaces count as part of the user id.

## Counter-examples

- The system SHALL NOT shorten a name in the contexts popup.
- The system SHALL NOT remove or shorten the JSON export's `context`.
- The system SHALL NOT shorten a name that only looks like the form.

## Out of scope

- A project number in place of the project id
  (`edgecontainer_<number>_…`): refused and shown whole, as for the `gke_` and
  `connectgateway_` forms; one `TODO.md` item covers all of them.
- Provider IDs and node names of Distributed Cloud nodes, whose format is not
  known here.
- A Distributed Cloud cluster's prices, node pools and node labels.
- Any other gcloud or cloud context form.
