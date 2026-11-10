# Requirements: azure-live-skus

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-10 (spec-refuter; 3 HIGH, 12 MEDIUM, 10 LOW; applied); second pass 2026-11-10 (2 HIGH, 8 MEDIUM, 9 LOW; applied)

## Context

The Recommendations screen on AKS ([aks-recommendations](../aks-recommendations/requirements.md))
reads Azure VM sizes (family, vCPU, memory, architecture) from a table copied
into the binary from the cluster autoscaler (aks-recommendations D1,
`pkg/azure/zz_generated_instance_types.go`). That table goes stale:
- **Upstream lags.** Its last change was 2026-03-17 (aks-recommendations T1
  note); families released after it get no advice.
- **The repo and the binary lag.** Nothing refreshes it automatically (the
  workflow step was deferred, aks-recommendations tasks Deferred), and an
  installed binary keeps the table it was built with.
- **It is wrong in places.** It truncates memory to whole GB
  (`Standard_DS1_v2` reads 3 GiB, `Standard_B1ls` 0) and gives constrained
  sizes their full vCPU count (aks-recommendations R2).

The owner decided on 2026-11-10: this tool is for system administrators, who
can read the subscription their cluster runs in. The size catalog SHALL come
live from the Azure Resource SKUs API with the Azure CLI sign-in already on
the machine, and when there is no sign-in or no permission the screen SHALL
say so, instead of advising from an old table. No IAM role or app
registration is created for this; only an existing sign-in is used.

This changes aks-recommendations' "no Azure credentials, no ARM at run time"
rule (its context, counter-examples and invariants carry dated notes).
aks-pricing's rule ("MUST NOT ask for Azure credentials or read `~/.azure`",
`specs/aks-pricing/requirements.md:100`) is about prices and still holds:
prices stay on the public Retail Prices API with no credential, and this
tool itself never reads `~/.azure`; `az` does.

## Usage examples

An admin signed in with `az login`, Reader on the subscription, on an AKS
cluster in `eastus` (the made-up nodes of the aks-recommendations fixture):

```text
$ kube-node-viewer          # then 6 (Recommendations)
 Recommendations
 idle         agent pool userpool (4 nodes)   …
 Azure VM sizes: loading
# a second or two later, sizes and prices arrived; the note is gone:
 rightsize    aks-legacy-12345678-vmss000001 (Standard_D4s_v3)  → Standard_D2s_v3 …
 arm          agent pool userpool (4 nodes)  Standard_D8s_v5 → Standard_D8ps_v5 …
```

`enter` on the rightsize row shows, after the `after the move` line (R8):

```text
 sizes: Azure Resource SKUs API
```

Failure examples (one line each in the notes under the rows; idle,
consolidation, old-node and Spot rows still show, R5):

```text
 Azure VM sizes: the Azure CLI (az) was not found; install it and run az login
 Azure VM sizes: not signed in to Azure; run az login
 Azure VM sizes: the signed-in account can't see the nodes' subscription; az login with an account that can
 Azure VM sizes: no permission to read VM sizes (Microsoft.Compute/skus/read on the subscription, e.g. the Reader role)
 Azure VM sizes: the nodes' subscription was not found or is disabled
 Azure VM sizes: Azure Resource Manager unavailable (503); retrying in 1m
 Azure VM sizes: the nodes' provider ID names no valid subscription
```

A user with Reader on the node resource group only (not the subscription)
gets the permission line: the SKU list is a subscription-scope read
(decided 2026-11-10: the owner wants the error shown).

With `--disable-pricing`, no SKU request is made and the screen keeps its
current note ("pricing disabled: Azure rows that need a price are not
shown").

## Requirements

**R1** WHEN the Recommendations screen analyses Azure nodes THEN the system
SHALL read the VM sizes of each (subscription, region) of those nodes from
the Azure Resource SKUs API (`GET
https://management.azure.com/subscriptions/<id>/providers/Microsoft.Compute/skus`,
`api-version=2021-07-01`, `$filter=location eq '<region>'`), once per
session, without blocking the screen.
- The pairs come from the shown Azure nodes a size rule could compare: not
  an ACI virtual node, with an instance type and a valid region label.
  `recommendInput` gathers them in its node loop and passes them to
  `Begin(pairs)` before the input reads `Types()` (design D2). An Azure node
  with an instance type and no valid region label gets R5 row 14.
- No Azure node a size rule could compare (an AWS-only or empty view, or
  only virtual nodes): no `az` run and no request.
- The subscription comes from the node's provider ID
  (`azure:///subscriptions/<id>/resourceGroups/…`), the region from its
  `topology.kubernetes.io/region` label, lower-cased.
- Nothing is asked before Recommendations is shown, so a user who never
  opens it makes no Resource Manager request and sees no error.
- While the answer is awaited the notes say `Azure VM sizes: loading`, and
  the analysis runs again when the state changes: answer, failure, or a
  retry falling due (design D2: the advisor's `Generation` moves on each).

- [ ] Test (fake Resource Manager): opening Recommendations with two Azure
  nodes in one region sends one request; the loading note shows, then the
  rows that need sizes appear after the answer, on a static cluster (no
  snapshot change).
- [ ] Test: a failure replaces the loading note with its R5 line, on a
  static cluster.
- [ ] Test: no request before Recommendations is shown.
- [ ] Test: nodes in two regions → two requests; one subscription and
  region per request.
- [ ] Test: an AWS-only view, an empty view and a view of virtual nodes
  only → no `az` run and no request; an Azure node with no region label →
  row 14 and no request for it.

**R2** WHEN the system reads a SKU answer THEN it SHALL keep only
`resourceType` `virtualMachines` and take, per size:
- vCPU: `vCPUsAvailable` when present, else `vCPUs`; a size with
  `vCPUsAvailable` < `vCPUs`, or a `-` in its name, is constrained
  (`Standard_E8-4ds_v5`: 4 usable vCPU, constrained) and stays never a
  candidate or a target (aks-recommendations R2);
- memory: `MemoryGB`, fractional (`Standard_DS1_v2` is 3.5 GiB,
  `Standard_B1ls` 0.5 GiB);
- architecture: `CpuArchitectureType` `x64` → `x86_64`, `Arm64` →
  `arm64`;
- family: `family`, normalised as aks-recommendations R2 (lower case,
  without `standard` and `Family`);
- temp disk: `MaxResourceVolumeMB` (R9).

An entry missing vCPU, memory, architecture or family, or with 0 vCPU or 0
memory, counts as missing (aks-recommendations R12).

- [ ] Test (recorded made-up answer): D64ps_v5 64 vCPU / 208 GiB arm64;
  DS1_v2 3.5 GiB; B1ls 0.5 GiB; E8-4ds_v5 4 usable vCPU and constrained; a
  `disks` entry ignored; an entry without `MemoryGB` missing.

**R3** WHEN a size is not available to a node in the answer THEN it SHALL not
be a candidate or a target for that node. A size is available to a node
when it appears in the answer of the node's own (subscription, region)
(regions compared case-insensitively) and:
- it has no restriction of `type` `Location` (e.g. `reasonCode`
  `NotAvailableForSubscription`);
- for a zonal node (zone label `<region>-<n>`, `<n>` taken as the zone):
  the zone is in the size's `locationInfo[].zones` and not in a `Zone`
  restriction's `restrictionInfo.zones`. A node whose zone label is a fault
  domain (`0`, `1`, …, no `<region>-` prefix) is not zonal and gets the
  regional check only.

Per rule:
- **rightsize, generation, arm:** the candidate or target must be
  available.
- **Spot:** the target is the node's own size; when the answer is loaded and
  that size is not available to the node, the node gets no Spot row. While
  the node's pair is loading or failed, the Spot rule needs only the node's
  instance type and the two prices (decided 2026-11-10), so Spot rows stay;
  the Fargate, Auto Mode, virtual-node and no-type guards still apply
  (design D2).
- **The node's own size** is read from the answer, available or not, so the
  node can be compared.

- [ ] Test: a cheaper candidate with a Location restriction is never
  offered; a candidate offered in zones 1 and 2 only is offered to an
  `eastus-1` node and not to an `eastus-3` one; a fault-domain node gets the
  regional check; a candidate present only in another region's answer is
  not offered; a node whose own size is restricted is still rightsized and
  gets no Spot row.
- [ ] Test: with R5 row 7 holding, the fixture's Spot rows show with their
  amounts and the total is unchanged by them (Spot stays out of it).

**R4** WHEN the system needs a token THEN it SHALL use only the Azure CLI
sign-in already on the machine, never prompt, and never create anything:
- `az account get-access-token --resource https://management.azure.com/
  --subscription <id> --output json`, stdin not attached, with a timeout
  (design D1);
- the token is reused until two minutes before `expires_on` (or
  `expiresOn`, local time, from `az` before 2.54.0), then fetched again;
- the cached token is dropped on a `401` or `403` answer (R5 rows 6–8), and
  when that subscription's entry in `az account list` changes (`user.name`
  or `tenantId`, compared in memory, never stored or logged), so a new
  sign-in is used at the next retry;
- one token request per subscription is in flight at a time, shared by the
  fetches of all its regions;
- no browser, device-code or interactive flow; no credential is stored by
  this tool, and it never reads `~/.azure` itself.

- [ ] Test: a fake `az` on `PATH` returning a token → the request carries
  `Authorization: Bearer <token>`; the subscription passed to `az` is the
  node's.
- [ ] Test: one `az` run per subscription while the token is valid; an
  answer with only `expiresOn` is read.
- [ ] Test: `az` exiting 0 with warnings on stderr gives a token.
- [ ] Test: a token two minutes from expiry is fetched again.
- [ ] Test: after a `401`, the fake `az` returns a new token and the next
  request carries it; a changed account entry drops the token too.
- [ ] Test: two regions of one subscription → one `get-access-token` run.
- [ ] Test: a fake `az` that reads stdin returns at once (stdin is not
  attached).

**R5** WHEN the sizes can't be read THEN the Recommendations notes SHALL show
one line saying why and what to do, from this table, and log it once (the
header issue log, as price failures):

| # | Cause | Line (after `Azure VM sizes: `) |
|---|---|---|
| 1 | `az` not on `PATH` | `the Azure CLI (az) was not found; install it and run az login` |
| 2 | `az account list --all` empty, or `get-access-token` fails with `AADSTS`, `authenticate interactively` or `az login` in its stderr (az 2.88.0 wording, design D1) | `not signed in to Azure; run az login` |
| 3 | no account in that list has the node's subscription id | `the signed-in account can't see the nodes' subscription; az login with an account that can` |
| 4 | that account's `state` is `Disabled` or `Deleted` (`Warned` and `PastDue` stay readable) | `the nodes' subscription was not found or is disabled` |
| 5 | `az` fails otherwise, or times out | `the Azure CLI failed to give a token; run az account get-access-token to see why` |
| 6 | `401` | `Azure refused the token; run az login` |
| 7 | `403` `AuthorizationFailed` | `no permission to read VM sizes (Microsoft.Compute/skus/read on the subscription, e.g. the Reader role)` |
| 8 | `403` with another code (e.g. `RequestDisallowedByPolicy`) | `Azure refused the request (403); check the subscription's policies` |
| 9 | `404` `SubscriptionNotFound` | `the nodes' subscription was not found or is disabled` |
| 10 | `400`, `409`, other `4xx` | `Azure refused the request (<status>)` |
| 11 | `429`, `5xx`, network error, timeout | `Azure Resource Manager unavailable (<status>); retrying in <d>`, with `<status>` the code, `network error` or `timeout` |
| 12 | answer malformed, over the size cap, or a `nextLink` R10 refuses | `Azure Resource Manager gave an answer that can't be read; retrying in <d>` |
| 13 | provider ID subscription not a GUID (R10) | `the nodes' provider ID names no valid subscription` |
| 14 | region label missing or not `^[a-z0-9]+$` (R1, R10) | `a node has no valid region label` |

- No line or log carries a subscription id, tenant id, account, client or
  object id, scope or token. `az`'s output and Resource Manager's error
  `message` are never shown or logged (they can carry them); only the
  status and the error `code` are read.
- While a line shows, rightsize, generation and arm rows don't appear; idle,
  consolidation, old-node and Spot rows do (R3).
- With nodes in two subscriptions or regions and only one pair failing, the
  line names the affected agent pools and NodePools, not the subscription:
  `Azure VM sizes (agent pools legacy, system): <line>`.
- `az account list --all` lists every cloud the CLI knows; entries of
  another cloud (`cloudName` other than `AzureCloud`) are ignored, so a
  same-id entry there neither matches nor fails row 3.

- [ ] Test: a table test drives each row (fake `az`, fake Resource Manager)
  and checks the exact line, that it contains neither the fixture's
  subscription id nor the fake token nor the fake error message, and the
  row kinds shown.
- [ ] Test: the same failure on two analyses is logged once.
- [ ] Test: two pairs, one failing → the line names the failing pair's
  agent pools and NodePools.
- [ ] Test: row 2 from the az 2.88.0 `AuthenticationError` wording (an
  `AADSTS50076` description and the `authenticate interactively`
  recommendation); row 4 for `Disabled`, not for `Warned`; an `AzureChinaCloud`
  entry with the same id is ignored.

**R6** WHEN a size read failed (R5 rows 1–10, 14) THEN the system SHALL try again
at most once a minute while Recommendations is shown, so an admin can run
`az login` or get the role and come back without restarting. Rows 11 and 12
wait as the price provider does (fix-azure-retry: the answer's retry
header, at least a minute, at most 10 minutes). Row 13 is not retried (the
provider ID doesn't change); row 14 clears when the label appears.

- [ ] Test (fake clock, static cluster): after a `403`, no request for 59 s,
  one at 60 s while shown, none while another screen is shown; after the
  fake `az` starts returning a token and the fake Resource Manager answers,
  the rows appear without any snapshot change.

**R7** WHEN pricing is disabled (`--disable-pricing`) THEN the system SHALL
make no `az` run and no SKU request: every row that reads a size also needs
a price, so none could show (aks-recommendations R3). The note stays the
pricing-disabled one.

- [ ] Test: with pricing disabled, no `az` run and no request.

**R8** WHEN a rightsize, generation or arm row is shown for an Azure node
THEN its math SHALL include the line `sizes: Azure Resource SKUs API`, and
the subscription id SHALL appear nowhere on screen, in the search or in the
export (hide-cloud-ids).
- rightsize: after the `after the move` line (`pkg/recommend/recommend.go`
  rightsize math);
- generation and arm: after the line naming the target.
- AWS math is unchanged; the AKS golden screens with an open math popup
  change by that line.

- [ ] Test: the fixture's rightsize, generation and arm math hold the line;
  no AWS math does; no rendered screen of the fixture contains its
  subscription id, nor a `/` search for it matches, nor the CSV and JSON
  export contain it.

**R9** WHEN a generation or arm target is chosen for a node whose size has a
temp disk (`MaxResourceVolumeMB` > 0) THEN the target SHALL have one too,
in addition to the fixed tables of aks-recommendations D5, which stay the
policy. A target that fails the check gets no row.
`MaxResourceVolumeMB` 0 is read as "no temp disk" (not verified against a
real answer; T5 checks it when a real subscription is named).

- [ ] Test: a recorded target with `MaxResourceVolumeMB` 0 for a node size
  with 32768 is refused; with both > 0 it is offered.

**R10** WHEN a value from a Node object goes into a command or a request
THEN the system SHALL validate it first: the subscription id SHALL match
the GUID form (`^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$`) and
the region `^[a-z0-9]+$`; otherwise R5 row 13.
- The query is built with `url.Values`, never by string concatenation.
- The token is sent only to `https://management.azure.com`; a `nextLink`
  is followed only when its scheme is `https` and its host is
  `management.azure.com`, else R5 row 12.
- Redirects are refused (the client's `CheckRedirect` returns an error):
  row 12.
- The base URL is `https://management.azure.com` and can be changed only by
  a Go option for tests (`TestKindCluster`, design D6); no flag,
  environment variable or config key sets it.

- [ ] Test: provider IDs with `&`, `"`, spaces or a non-GUID → row 13 and no
  `az` run; region `east us` → row 13; a `nextLink` to another host or over
  `http` is not followed and no request reaches it with the token; a `302`
  is not followed; the default base is `https://management.azure.com` and
  the host check runs against it.

**R11** WHEN a rule recommends a size for an AKS user pool THEN the size
SHALL have at least 2 vCPU and 2 GiB of memory; system pools keep
aks-recommendations R15. AKS's page (learn.microsoft.com/azure/aks/quotas-skus-regions,
`ms.date` 2026-08-03, read 2026-11-10) says: "For user node pools, you might
not be able to use VM versions with fewer than two vCPUs and 2 GB of
memory." Its 2 GB is read as 2 GiB, the unit the rules use. `Standard_B1ls` (1 vCPU, 0.5 GiB), now read with
its real memory (R2), is refused by this rule rather than by a 0 entry.

- [ ] Test: a user-pool node with tiny requests is never rightsized to
  B1ls or to a 1-vCPU size; `TestB1lsNeverACandidate` holds on the new
  premise.

**R12** WHEN this ships THEN the copied autoscaler table, `catalog.go`, `make
azure-catalog` and the deferred workflow task SHALL be removed: the live
answer is the only Azure size source at run time. Tests SHALL use:
- a recorded made-up SKU answer (`pkg/azure/testdata/skus-eastus.json`)
  for parsing, restrictions, zones and the fixtures;
- a frozen test-only extract of the autoscaler table (size, family, vCPU,
  MiB, architecture) under `pkg/recommend/testdata/`, with its Apache-2.0
  credit, for the table-coverage tests (design D5).

- [ ] Test: `pkg/azure/zz_generated_instance_types.go` and
  `pkg/azure/catalog.go` don't exist; the aks-recommendations fixture still
  totals $322.66 and the mixed one $565.60; the coverage tests pass on the
  extract.

## Counter-examples

- The system SHALL NOT open a browser, ask for a device code, read
  `~/.azure` itself, or store any credential; `az` does its own.
- The system SHALL NOT fall back to an embedded size table when the live
  read fails (the owner's decision): it shows the R5 line.
- The system SHALL NOT make a Resource Manager request when no Azure node is
  shown, before Recommendations is shown, or with pricing disabled.
- The system SHALL NOT show or log a subscription id, tenant id, account,
  client or object id, scope or token, `az`'s output, or Resource Manager's
  error message.
- The system SHALL NOT pass a Node-object value to `az` or into a URL
  unvalidated (R10), nor send the token to any other host.
- The system SHALL NOT change anything for AWS nodes or AWS-only clusters
  (the AWS catalog stays as it is).

## Out of scope

- AWS: the EC2 catalog stays the embedded one; a live
  `DescribeInstanceTypes` is a possible later spec.
- Credentials other than the Azure CLI: managed identity, workload identity,
  service-principal environment variables, `azd` (design D1; deferred).
- Sovereign clouds: the Resource Manager host is `management.azure.com`
  only, even after `az cloud set`.
- Using the live answer for prices (prices stay on the Retail Prices API).
- Filtering Spot by `LowPriorityCapable` (a possible later check).

## Decided

- 2026-11-10, owner: the credential is the Azure CLI, called as a command;
  no new Go dependency (design D1).
- 2026-11-10, coordinator:
  1. Reader on the node resource group only gets the permission line (R5
     row 7).
  2. Spot rows stay when sizes can't be read (R3).
  3. A frozen test-only extract of the autoscaler table backs the coverage
     tests; the recorded answer covers parsing (R12, design D5).
  4. The live temp-disk check is in scope; the D5 maps stay the policy (R9).
  5. User-pool minimum 2 vCPU / 2 GiB; B1ls handled by it (R11).
  6. The kind run injects the fakes through a Go option in process, no build
     tag (`kind_integration_test.go` is itself `//go:build integration`;
     design D6, T6).
