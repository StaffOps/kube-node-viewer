# Design: short-edge-context

> Implements [requirements](requirements.md).

## Current state

`shortContext` (`pkg/model/header.go`) tries `eksClusterARN`, `gkeContext`,
`gkeFleetContext` and `connectGatewayContext` and returns the last capture of
the first match; the three headers (`header.go:161`) and `exportBase`
(`export.go:248`) use it. The patterns share `gkeProject`, `gkeLocation` and
`dnsLabel`. `scripts/checks/content.sh`'s `gke` rule catches `gke_` and
`connectgateway_` names, with `gke_ok` letting `…_example…` through.

## Target state

A fifth pattern, `edgeContainerContext`, in the list `shortContext` tries,
built from the same sub-patterns, with an optional non-capturing offline
suffix. The `gke` and `gke_ok` rules of `content.sh` take `edgecontainer` as a
third prefix.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/header.go` | `edgeContainerContext`, the list in `shortContext` |
| `pkg/model/header_internal_test.go` | `TestShortEdgeContextName`, `TestHeadersHideTheEdgeProject` |
| `pkg/model/export_internal_test.go` | `TestEdgeNameInExportAndPopup` |
| `pkg/client/client_test.go` | an `edgecontainer_` case in `TestClusterKeyKeepsTheFullContext` |
| `scripts/checks/content.sh` | `edgecontainer` in the `gke` rule and `gke_ok`, and in the rule's comment (`edgecontainer_example…`) |
| `scripts/checks/test-checks.sh` | the R5 cases |
| `README.md` | the header paragraph and the export section |
| `CHANGELOG.md` | `[Unreleased]` `Changed` |
| `specs/hide-cloud-ids/requirements.md` | a dated note on Out of scope |
| `specs/README.md` | the index entry |
| `TODO.md` | remove the `edgecontainer_` item; add one for project numbers in gcloud context names |

> **Corrected 2026-11-10 (code review).** D1 accepts one more cost: a
> malformed name ending in `_x_offline` with an extra field before it shortens
> to its fourth field, not the cluster; it shows less, not more. The README's
> export section needed no change (it already names gcloud contexts
> generically); only the header paragraph changed, as T2 says.

## Decisions

### D1: A strict pattern from gcloud's template, offline suffix included

**Choice**: shorten only a name matching, in full,
`^edgecontainer_<gkeProject>_<gkeLocation>_(<dnsLabel>)(?:_\S+_offline)?$`.

**Justification, strongest first**:
1. gcloud writes exactly `edgecontainer_{project_id}_{location}_{cluster_id}`
   (`googlecloudsdk/command_lib/edge_cloud/container/kubeconfig.py:75`); the one
   caller passes the cluster reference's ids (`get_credentials.py:64-66`) and,
   with `--offline-credential`, appends `'_' + userId + '_offline'`
   (`get_credentials.py:83`).
2. The cluster's location is a region: the cluster resource takes the location
   attribute (`command_lib/edge_cloud/container/resources.yaml:16-20,34-43`;
   zones are a separate resource), `get_credentials` reads the cluster from the
   API first (`get_credentials.py:62-63`), and `locations.py:33-35` rewrites a
   zone to its region. So `gkeLocation` fits.
3. The project, location and cluster cannot hold `_`, and the prefix differs
   from every other pattern's, so a name matches one pattern at most. The user
   id has no character rule (`edgecontainer_v1_messages.py:903`) and may hold
   `_`; it comes after the cluster, whose label cannot, so the cluster is still
   the field after the third `_`. The suffix group does not capture, because
   `shortContext` returns the last capture.
4. A miss shows the full name, today's behaviour.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The SDK states no rule for the cluster id; the RFC 1123 label (1 to 63) is assumed, as for fleet memberships and multi-cloud clusters | a wider label only refuses fewer names; a refused name shows whole, as today |
| A user id with white space is refused | a refused name shows whole; ids with white space are not expected |
| Verified only against the gcloud source and made-up names | no Distributed Cloud cluster is available |

**When this would be wrong**: a real context has a shape this pattern refuses,
or gcloud changes the template or the offline suffix.

**Reversal action**: drop `edgeContainerContext` from the list in
`shortContext` and the test rows; minutes.

**Alternatives discarded**: `[a-z0-9-]+` for the location (accepts shapes no
location has); refusing the offline form (it holds the project too).

### D2: The content check catches the form too

**Choice**: `edgecontainer` joins `gke` and `connectgateway` in `content.sh`'s
`gke` rule and in `gke_ok`.

**Justification, strongest first**:
1. The screen no longer shows the project, so a commit is the next place it
   can leak; the other two gcloud prefixes are already caught.
2. One rule keeps the allowed example form the same for the three prefixes.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Tests and docs must build real-looking names by concatenation or use `edgecontainer_example…` | the same as for `gke_` today |

**When this would be wrong**: the rule flags text in the repo that is not a
context name.

**Reversal action**: remove `edgecontainer` from the two alternations; minutes.

**Alternatives discarded**: a separate rule (same regex, one more name in the
output).

## Invariants

- Requests, never usage; no new network call.
- The kubeconfig names, the state key and the JSON `context` are unchanged.
- Every other context renders byte-identically (the golden screens unchanged).

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| A context of another shape | Shown as it is | The full name, as today |

## Test plan

- `TestShortEdgeContextName`, table: a region and a zone, a domain-scoped
  project, cluster lengths 1 and 63, a cluster starting with a digit; offline
  forms with `_`, `@` and `.` in the user id; R3's refusals (project number,
  missing `_offline`, empty user id, white space in it, …); one row of each
  other pattern, unchanged results.
- `TestHeadersHideTheEdgeProject`, `TestEdgeNameInExportAndPopup`, the client
  state key case.
- `test-checks.sh`: R5's cases.
- Mutation check of the pattern on a copy.
- The golden screens unchanged.
- Run on a local kind cluster with the context renamed to made-up
  `edgecontainer_` names (plain and offline): three headers, `c`, `e`.
