# Design: azure-live-skus

> Implements [requirements](requirements.md).

## Current state

- **Catalog:** `pkg/azure/catalog.go` loads the copied autoscaler table
  (`zz_generated_instance_types.go`, 10,993 lines) once, globally
  (`LookupSize`, `SizeNames`). `make azure-catalog` (`Makefile:114`) refreshes
  it by hand.
- **Advisor:** `pkg/azure/advisor.go` answers `Spec(t)` and `Types()` from that
  table, and `OnDemand`/`SpotIn` from a `PriceLister` (the Retail Prices
  provider). `Notes()` says why rows are missing; `Generation()` bumps the
  analysis key when prices arrive (`pkg/model/recommendations.go:145-166`).
- **Wiring:** `cmd/kube-node-viewer/session.go:255-266` builds one Azure
  advisor per session, with pricing disabled or not.
- **Rules:** `pkg/recommend` calls `in.Spec(t)` (global, not per node) and
  `in.Types`; `in.OnDemand(n, t)` is per node. Idle, consolidation and old
  node use no catalog (`recommend.go:346,384,852`).
- **Node:** `recommend.Node` has cloud, region, zone, OS, mode
  (`pkg/model/recommendations.go:198-230`); not the subscription. The
  subscription is in `obj.Spec.ProviderID`; hide-cloud-ids hides it on screen
  (`pkg/model/node.go:42`).
- **Permission:** `Microsoft.Compute/skus/read` "Gets the list of
  Microsoft.Compute SKUs available for your Subscription"
  (learn.microsoft.com, Azure permissions for Compute, read 2026-11-10).
  The AKS "Cluster User" role grants none of it (built-in roles, Containers,
  read 2026-11-10), so cluster access alone doesn't give it; Reader on the
  subscription does.

## Target state

```
Recommendations shown ─▶ model.recommendInput ─▶ Advisor.Begin()
                                               ├─ Spec(t) / Types()  ◀── skuCache (per subscription+region)
                                               │                         ▲ fetch in background:
                                               │                         │  az account get-access-token (token cache)
                                               │                         │  GET management.azure.com …/skus
                                               ├─ OnDemand(n,t): restricted for n? → no price
                                               └─ Notes(): loading / R5 line / waiting for prices
skuCache answer ─▶ Generation()++ ─▶ recKey changes ─▶ analysis again
```

- `recommend.Node` gains `Subscription string`, filled from the provider ID
  in `recommendNode`; nothing renders it.
- `Advisor.Begin()` also records the (subscription, region) pairs of the
  shown Azure nodes; the advisor starts a fetch for a pair it has not got or
  whose retry time passed (R1, R6). `Types()` and `Spec(t)` answer from the
  union of the loaded pairs (a size's spec doesn't depend on the region);
  availability is per pair and per zone, applied in `OnDemand`/`SpotIn`,
  which already take the node (R3).

## Components touched

| Path | Change |
|------|--------|
| `pkg/azure/skus.go` (new) | Resource SKUs client: request, paging (`nextLink`), body cap, parse (R2, R3), error classes (R5) |
| `pkg/azure/token.go` (new) | `az account get-access-token` runner, token cache, error classes (R4, R5) |
| `pkg/azure/advisor.go` | catalog from the SKU cache; pairs from `Begin`; restrictions; notes; retry (R1, R3, R5–R7) |
| `pkg/azure/catalog.go`, `zz_generated_instance_types.go`, `catalog_test.go` | removed; `NormalizeFamily` moves to `skus.go` (R9) |
| `pkg/azure/testdata/skus-eastus.json` (new) | recorded made-up SKU answer for the tests (D5) |
| `pkg/recommend/recommend.go` | `Node.Subscription`; math size line names the source (R8) |
| `pkg/recommend/tables_azure_ext_test.go`, AKS fixtures | read the recorded answer instead of the table (R9) |
| `pkg/model/recommendations.go` | `Subscription` from the provider ID; pairs passed through `Begin` |
| `cmd/kube-node-viewer/session.go` | advisor gets the SKU reader; nothing when pricing is disabled (R7) |
| `Makefile` | `azure-catalog` target and `AUTOSCALER_AZURE` removed |
| `README.md`, `CHANGELOG.md`, `TODO.md` | source, permissions, errors; Deferred T6 row and TODO item removed |
| `specs/aks-recommendations/*` | dated notes (D1, R2, R12, R16, T1, T6) |

## Decisions

### D1: The token comes from the Azure CLI, run as `az account get-access-token`

**Choice**: run `az account get-access-token --resource-type arm
--subscription <id> --output json` (30 s timeout), read `accessToken` and
`expires_on`, cache it per subscription until two minutes before it
expires. No Azure SDK.

**Justification, strongest first**:
1. **The owner's words: "the same identity as `az login`".** AKS admins
   already have the CLI; `az aks get-credentials` and kubelogin's
   `azurecli` mode use it.
2. **`azidentity` fails the licence allowlist.** A scratch module with
   `azidentity` v1.15.0 links `github.com/pkg/browser` on linux, darwin
   and windows, which is BSD-2-Clause (`go-licenses report`, 2026-11-10);
   `make licenses` allows MIT, Apache-2.0, BSD-3-Clause and ISC only
   (`Makefile:47`). Its other modules are MIT, Apache-2.0 or BSD-3-Clause
   (azcore, MSAL, golang-jwt, godebug, google/uuid, x/crypto, x/net,
   x/sync, x/text).
3. **No new dependency at all**, and `azidentity`'s `AzureCLICredential`
   runs the same command anyway.
4. **`--subscription` picks the right tenant.** `az` resolves the
   subscription's tenant from its account list (`az account
   get-access-token -h`: "Name or ID of subscription"), so a guest admin in
   several tenants needs nothing more.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| No managed identity, workload identity or service-principal env vars | the owner's case is an admin at a workstation; a jump host with a managed identity can `az login --identity` |
| About a second per `az` run | once per subscription per hour at most; in the background |
| Classifying `az` failures from its stderr text | the classes are coarse (R5), and an unmatched failure still gets an actionable line (row 4) |

**When this would be wrong**: admins report running it where `az` can't be
installed (containers, CI) — add `azidentity` with the licence question
answered, or IMDS directly.

**Reversal action**: replace `token.go` with a `TokenCredential`; the SKU
client takes a token function, so nothing else changes.

**Alternatives discarded**:
- `azidentity.NewDefaultAzureCredential` — fails `make licenses` (point 2);
  allowing BSD-2-Clause is the owner's call and not needed for `az`.
- Calling IMDS for managed identity ourselves — no case for it yet.
- `armcompute` SDK — the same licence chain plus a large module for one GET.

### D2: One Resource SKUs request per (subscription, region), lazily, cached for the session

**Choice**: the first analysis that shows Azure nodes starts one request per
(subscription, region) pair, with `$filter=location eq '<region>'`
(api-version 2021-07-01, "Only location filter is supported currently",
REST reference read 2026-11-10). The answer is kept for the session; `c` to
another context builds a new session and so asks again.

**Justification, strongest first**:
1. **Lazy:** a user who never opens Recommendations makes no request and sees
   no error (R1).
2. **Per region:** a region's answer is all one node needs; the unfiltered
   list covers every region.
3. **Sizes don't change within a session.**

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Answer size unmeasured: the answer holds every Compute resource type of the region (disks, snapshots, …) | read with a 64 MiB cap and `nextLink` paging; T5 measures it on a real subscription when the owner names one |
| A size released mid-session is unseen | restart or `c` |

**When this would be wrong**: the per-region answer is so large that it
takes more than a few seconds — then stream-parse it.

**Reversal action**: none needed; a cache TTL could be added.

**Alternatives discarded**: fetching at session start — every AKS user would
pay a request and see sign-in errors on the Overview.

### D3: Errors are shown, retried while the screen is shown, never hidden behind a stale table

**Choice**: each failure maps to one fixed line (R5) shown in the
Recommendations notes while it holds and logged once; sign-in, permission
and subscription failures are retried at most once a minute while
Recommendations is shown (R6); `429`/`5xx`/network follow
`retryDelay` (`pkg/azure/retry.go:58`).

**Justification, strongest first**:
1. **The owner's decision**: say there is no permission, don't advise from
   old data.
2. **Fixed lines carry no identifiers.** `az` stderr and ARM error bodies can
   name the subscription, tenant or user; they are classified, never shown.
3. **The retry is user-driven:** an admin runs `az login` in another terminal
   and the screen recovers within a minute, without restarting (the open
   TODO "`c` on the current context only closes the popup" makes restarting
   the only other way).

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| An `az` run a minute while a permission error holds and the screen is open | cheap, local; stops when the screen is left |
| Some `az` failures land in the generic row | it tells the admin the command that shows the cause |

**When this would be wrong**: users find the note noisy on clusters where
they deliberately lack the role — then a config key to turn the catalog off.

**Reversal action**: drop the retry; a failure then holds for the session.

**Alternatives discarded**: falling back to the embedded table — the owner
refused it; showing `az`'s raw message — leaks identifiers.

### D4: `--disable-pricing` also skips the SKU request

**Choice**: with pricing disabled the advisor gets no SKU reader (R7).

**Justification**: every rule that reads a size (rightsize, generation, arm,
Spot) also needs a price, and none runs without one (aks-recommendations
R3); a request and its sign-in errors would buy nothing. No new flag.

**When this would be wrong**: a rule needs sizes without prices.

**Reversal action**: pass the reader regardless.

### D5: Tests use a recorded made-up SKU answer

**Choice**: `pkg/azure/testdata/skus-eastus.json` holds a hand-built answer
in the API's shape, with the public sizes the aks-recommendations fixtures,
the D5 tables (generation, Arm, temp disk) and the R2/R3 cases need, public
specs from the Azure size pages, and made-up locations, zones and
restrictions. The table-coverage tests check the D5 tables against it.

**Justification**: no credential in CI or in the repo; public size specs are
not environment data.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Coverage tests now check the tables against the fixture, not against all 1,370 real sizes | a table target missing from a real region shows at run time as no row, not as wrong advice |
| The fixture is hand-made | T2 checks each size's numbers against the public size pages and records the date |

**When this would be wrong**: a table entry goes wrong unnoticed — keep a
test-only copy of the autoscaler table, outside the binary, for coverage.

**Reversal action**: add that copy under `testdata/`.

**Alternatives discarded**: keeping the autoscaler file for tests only —
possible (D5 "When this would be wrong"), but it keeps the refresh chore the
owner wants gone.

## Invariants

- No Azure HTTP outside `pkg/azure` (AGENTS.md); `os/exec` of `az` lives
  there too.
- No subscription id, tenant id, account or token in any rendered text, log
  line or export.
- AWS rows, AWS golden screens and the EKS fixture total ($242.94) unchanged.
- No interactive prompt, ever.

## Failure modes

| Failure | Behaviour |
|---------|-----------|
| `az` missing, not signed in, wrong account, no role, subscription gone | R5 line; idle, consolidation, old-node rows only; retried each minute while shown (R6) |
| `az` hangs | killed at 30 s; R5 row 4 |
| ARM `429`/`5xx`/network | R5 row 8; `retryDelay` |
| Answer over 64 MiB, or malformed | treated as row 8 with status `bad answer`; retried |
| One of two subscriptions fails | its nodes get the R5 line's rows only; the other's nodes are advised |

## Test plan

- **Unit, `pkg/azure`:** fake `az` script on `PATH` (token, each failure);
  `httptest` Resource Manager (pages, 401/403/404/429/503, oversize); parse
  of the recorded answer (R2, R3); token reuse; retry clock (R6).
- **Unit, `pkg/recommend` / `pkg/model`:** fixtures on the recorded answer
  ($322.66, $565.60, $242.94); R5 row kinds; R8 source line and no
  subscription in any golden screen.
- **Run:** kind with made-up `azure://` nodes and the binary built with a test
  build tag that points the Resource Manager host at a local fake and puts a
  fake `az` first on `PATH` (T5); release builds have no such hook. A real
  subscription only when the owner names one; otherwise recorded as not run.
