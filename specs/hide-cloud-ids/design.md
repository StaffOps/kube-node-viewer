# Design: hide-cloud-ids

> Implements [requirements](requirements.md).

## Current state

`shortContext` (`pkg/model/header.go:331-338`) tries `eksClusterARN` and
`gkeContext` and returns the last capture of the first match; the three headers
and `exportBase` (`pkg/model/export.go:248`) use it. The Provider ID row prints
`obj.Spec.ProviderID` (`pkg/model/nodedetails.go:125`). `Node.Name()`
(`pkg/model/node.go:265-271`) falls back to `InstanceID()` (`:288-300`), which
returns the whole provider ID for anything but the `aws:///` form. Those are the
only two places the id reaches the screen; everything else that reads
`ProviderID()` uses it as a key (`cluster.go:81,88`, `aws/region.go:158`,
`client/controller.go:139`) or takes the cloud or zone from it
(`node.go:149,190,451,457`).

## Target state

`shortContext` tries two more patterns. A new pure function
`displayProviderID(id string) string` in `pkg/model/node.go` masks the two
recognised forms; the Provider ID row calls it, and so does `InstanceID()` on
its non-AWS paths. `Node.ProviderID()` is untouched. `displayServer` in
`pkg/model/header.go` masks the project of a gateway-shaped URL for the
`Cluster:` row (`header.go:196`); `u.info.Server` and the JSON `server` stay
whole.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/header.go` | `gkeFleetContext`, `connectGatewayContext`, the project and location sub-patterns shared with `gkeContext`; `displayServer` for the `Cluster:` row |
| `pkg/client/client_test.go` | the new forms in `TestClusterKeyKeepsTheFullContext` |
| `pkg/model/node.go` | `displayProviderID`; `InstanceID` uses it |
| `pkg/model/nodedetails.go` | the Provider ID row uses it |
| `pkg/model/header_internal_test.go` | the two rows of `TestShortGKEContextName` naming the forms this spec now shortens move to `TestShortFleetContextName`; `TestHeadersHideTheFleetProject` |
| `pkg/model/export_internal_test.go` | `TestFleetNameInExportAndPopup` |
| `pkg/model/nodedetails_internal_test.go`, `pkg/model/facts_test.go` | `TestDisplayProviderID`, `TestNodeDetailsHideTheCloudIDs`, `TestNamelessNodeHidesTheCloudIDs`, `TestProviderIDStaysWholeInTheStore` |
| `README.md` | header paragraph, export section, node details |
| `CHANGELOG.md` | `[Unreleased]` `Changed` |
| `specs/short-gke-context/requirements.md` | dated note on R2, R3, R4 and Out of scope |
| `TODO.md` | remove the context item; reword the ids item to labels and node names; add `edgecontainer_`, a content rule for real `gce://` ids and resource groups, and the recursive read lock on `Node.Name` |

> **Corrected 2026-11-09 (code review).** The diff also adds a dated note to
> `specs/node-details/requirements.md` (the provider ID row) and this spec's
> entry in `specs/README.md`.

## Decisions

### D1: Match gcloud's two other context forms, strictly

**Choice**: two anchored patterns built from the same project and location
sub-patterns as `gkeContext`: `gke_(aws|azure|attached)_…` and
`connectgateway_…` with an optional, non-capturing `_ns-<namespace>`
(`shortContext` returns the last capture, so that group must not capture).

**Justification, strongest first**:
1. They are what gcloud's source writes: `gke_{kind}_{project_id}_{location}_{cluster_id}`
   (`googlecloudsdk/command_lib/container/gkemulticloud/kubeconfig.py:89`, called
   with `'aws'`, `'azure'` and `'attached'` from the three `get_credentials.py`)
   and `connectgateway_{project}_{location}_{membership}` plus `'_ns-' + namespace`
   (`googlecloudsdk/command_lib/container/fleet/gateway.py:41,168-174`). That
   template is the client-side path, behind a hidden flag; gcloud's default path
   takes the context name from the API response (`gateway.py:155-158`), so the
   default form is inferred from it, not proved.
2. The field rules come from the same SDK: a membership id is an RFC 1123 label
   of at most 63 characters (`generated_clients/apis/gkehub/v1/gkehub_v1_messages.py:3121`);
   multi-cloud cluster ids are at most 63 characters
   (`generated_clients/apis/gkemulticloud/v1/gkemulticloud_v1_messages.py:27,346,749`).
3. None of the fields can hold `_`, and the forms have different field counts,
   so a name can match only one: `attached` is a valid project id, but
   `gke_attached_<location>_<cluster>` has three fields and is
   short-gke-context's form.
4. A miss shows the full name, which is today's behaviour.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Verified only against the gcloud source and made-up names | no such cluster is available; a miss is today's behaviour |
| Two namespaces of one membership show the same header name | they are the same cluster; `c` lists both names in full |
| A fleet location with another shape keeps the full name | `_LOCATION_RE` in the SDK allows any `[a-z0-9-]+`, but the fleet locations are `global` and GCP regions; widening is a test row |
| The SDK's text for multi-cloud ids says only `/a-z-/` and 63 characters | the label rule used here is the wider RFC 1123 one, so no valid id is refused |

**When this would be wrong**: a real context, the API-generated Connect gateway
name above all, has a shape these patterns refuse, or a hand-named context
matches one and its owner wanted the long name.

**Reversal action**: drop the pattern from the list in `shortContext` and
restore the two test rows; minutes.

**Alternatives discarded**: one looser pattern for all gcloud forms (accepts
shapes gcloud never writes); splitting on `_` (renames contexts people chose);
showing `<membership>/<namespace>` (a new display form nothing else uses).

### D2: Mask only the two forms known to hold an id

**Choice**: `gce://<project>/<zone>/<instance>` keeps the zone and the instance;
the Azure resource path keeps what follows `providers/<namespace>/`. Anything
else is shown as it is.

**Justification, strongest first**:
1. They are the two forms known here to hold an id that identifies an account:
   an EC2 id holds zone and instance.
2. What stays is what the row is useful for: which instance it is.
3. `…` marks that something was taken out, so nobody copies the row expecting
   the whole id.
4. The Azure path is the form this repo already parses and tests against
   (`pkg/model/aks_internal_test.go:47`, `pkg/azure/pricing_test.go:112`); the
   `gce://` form is by convention. Neither was read from the cloud providers'
   source, and a different shape fails open.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The row can no longer be pasted as the full id | `kubectl get node -o yaml` has it; the row is for reading |
| The `gce://` form is by convention, not observed on a cluster of ours | a different shape stays as it is, today's behaviour |
| A form of another shape keeps its id on screen (fails open) | guessing which part is an id would mangle ids; R7 and the counter-example say so |
| A nameless node's displayed name changes on GCE and Azure, and `/` no longer finds it by project | it lasts until the node registers |
| Two nameless nodes with the same masked id would share a name, and the cursor and details find a node by name (`layout.go:419`, `nodedetails.go:39-43`) | it takes one cluster with the same zone and instance in two projects, or the same `<rest>` in two resource groups; an id that differs in the part shown stays distinct |

**When this would be wrong**: people use the row to copy ids into `gcloud` or
`az` commands, or a cloud uses a form that holds an id outside these two.

**Reversal action**: call `obj.Spec.ProviderID` in the row and return
`providerID` from `InstanceID`; minutes.

**Alternatives discarded**: hiding the row on those clouds (loses the instance);
a flag to show the full id (a setting for what `kubectl` already gives); masking
in `Node.ProviderID()` (it is the store's key and pricing's input).

## Invariants

- Requests, never usage; no new network call.
- `Node.ProviderID()`, the store's key, what pricing reads and the saved-state
  key are unchanged.
- An AWS node renders byte-identically (the golden screens stay unchanged).
- Everything short-gke-context R4 keeps whole stays whole.

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| A context of another shape | Shown as it is | The full name in the header, as today |
| A provider ID of another shape | Shown as it is | The full id in the row, as today |
| An empty provider ID | `-` | As today |

## Test plan

- `TestShortFleetContextName`, table: each kind, a region and a zone; a
  domain-scoped project; `global`; `_ns-…`; a membership starting with a digit;
  lengths 1 and 63, and 64 refused; another kind; a project too short; upper
  case; Cyrillic letters; missing and extra fields; leading and trailing text;
  newline; an empty namespace. Rows `content.sh` would flag are built by
  concatenation.
- `TestHeadersHideTheFleetProject`, `TestFleetNameInExportAndPopup`.
- `TestDisplayProviderID`, table for R5 to R7; `TestDisplayServer`, table for R10.
- `TestNodeDetailsHideTheCloudIDs`, `TestNamelessNodeHidesTheCloudIDs`,
  `TestProviderIDStaysWholeInTheStore`.
- The header and export fixtures use a server URL of R10's shape, so a project in
  the `Cluster:` row can fail them; `gke_attached_…` with three and four fields,
  and a name with and without `_ns-`.
- The golden screens stay byte-identical.
- Run on a local kind cluster with contexts renamed to made-up forms and fake
  Node objects carrying made-up `gce://` and `azure://` ids: three headers, `c`,
  `e`, and the Provider ID row. No real cloud cluster of those kinds is used.
