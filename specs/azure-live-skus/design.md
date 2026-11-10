# Design: azure-live-skus

> Implements [requirements](requirements.md).

## Current state

- **Catalog:** `pkg/azure/catalog.go` loads the copied autoscaler table
  (`zz_generated_instance_types.go`, 10,993 lines) once, globally
  (`LookupSize`, `SizeNames`). `make azure-catalog` (`Makefile:114`) refreshes
  it by hand.
- **Advisor:** `pkg/azure/advisor.go` answers `Spec(t)` and `Types()` from that
  table, and `OnDemand`/`SpotIn` from a `PriceLister` (the Retail Prices
  provider). `Notes()` says why rows are missing; `Generation()` follows the
  price cache.
- **When analysis runs:** only when `recKey` changes
  (`pkg/model/recommendations.go:145-166`); `maybeAnalyze` checks it on every
  message, the 1 s tick included (`pkg/model/uimodel.go:518-528`). `recKey`
  includes the advisor's `Generation()`. `recommendInput` reads `a.Types()`
  while building the input, then calls `Begin()`, then loops over the shown
  nodes (`recommendations.go:172-195`).
- **Wiring:** `cmd/kube-node-viewer/session.go:255-266` builds one Azure
  advisor per session, with pricing disabled or not.
- **Rules:** `pkg/recommend` calls `in.Spec(t)` (global, not per node) and
  `in.Types`; `in.OnDemand(n, t)` and `in.SpotIn(n, t)` are per node. Idle,
  consolidation and old node read no catalog (`recommend.go:346,384,852`);
  Spot calls `comparableSpec` (`recommend.go:885`). The only per-pool size
  floor is `systemSizeOK` (`recommend.go:323`), for system pools.
- **Node:** `recommend.Node` has cloud, region, zone, OS, mode
  (`pkg/model/recommendations.go:198-230`), not the subscription. The
  provider ID regex accepts any `[^/\s]+` subscription (`pkg/model/node.go:42`);
  hide-cloud-ids hides it on screen. Zone labels are `<region>-<n>` on zonal
  AKS nodes and a fault domain (`0`, `1`, …) otherwise
  (`pkg/model/node.go:512-514`).
- **Permission:** `Microsoft.Compute/skus/read` "Gets the list of
  Microsoft.Compute SKUs available for your Subscription" (Azure
  permissions for Compute, read 2026-11-10). Reader includes it, and so does
  the general Contributor role (`*`); the AKS roles "Azure Kubernetes
  Service Cluster User Role", "Cluster Admin Role" and "Azure Kubernetes
  Service Contributor Role" don't (built-in roles, Containers; spec-refuter
  2026-11-10).

## Target state

```
recommendInput (Recommendations shown)
  node loop ─▶ pairs {(subscription, region)} of shown Azure nodes
  Advisor.Begin(pairs)  ─▶ start a fetch for a pair not loaded and not failing,
                           or whose retry is due
  in.Types = Advisor.Types()   (read after Begin)
  rules ─▶ Spec(t) / Available(n, t) / OnDemand / SpotIn
fetch (goroutine): validate (R10) ─▶ az account list / get-access-token (token cache)
                   ─▶ GET management.azure.com …/skus (pages) ─▶ parse (R2, R3)
                   ─▶ pair state = loaded | failed(row, retryAt) ─▶ gen++
1 s tick ─▶ recKey (includes Advisor.Generation) ─▶ analysis again
retry due (checked in Generation(), on the tick, only while Recommendations is shown)
          ─▶ gen++ ─▶ analysis ─▶ Begin starts the retry
```

- `recommend.Node` gains `Subscription string`, filled from the provider ID
  in `recommendNode`; nothing renders it.
- `recommendInput` gathers the pairs in its node loop, calls
  `Begin(pairs)`, then sets `in.Types` (the order changes from today's).
- `Generation()` is the sum of the price generation and the SKU state
  generation; the SKU generation moves on an answer, a failure, and when a
  failed pair's retry falls due. `Generation()` is only read through
  `recKey`, which runs only while Recommendations is shown, so retries never
  happen on other screens (R6).
- **Per-analysis snapshot.** `Begin(pairs)` takes a snapshot of the pair
  states and the loaded answers; `Types`, `Spec`, `Available` and `Notes`
  of that analysis read only the snapshot, so they agree even though they
  run in the analysis goroutine (`pkg/recommend/recommend.go:214-215`) while
  fetches complete. Advisor tests run under `-race`.
- `Spec(t)` answers from the union of the snapshot's loaded pairs (a size's
  spec doesn't depend on the region). Availability (R3) is a new
  `Input.Available func(n Node, t string) bool`, checked by the rules for
  candidates and targets and for the node's own size before a Spot row; on
  Azure it reads only the node's own pair. A nil `Input.Available` means
  "available", and `awsAdvisor` gives none, so AWS behaviour is unchanged.
- **Seams in `pkg/model`:** the `beginner` interface becomes
  `Begin(pairs []recommend.Pair)` (`pkg/model/recommendations.go:124`);
  `AdvisorsByCloud.Begin` passes the pairs to the Azure advisor
  (`recommendations.go:101-107`); `AdvisorsByCloud.Available` routes by the
  node's cloud through `forNode`, and answers true for AWS.
- **Spot without sizes.** On Azure, while the node's pair is loading or
  failed, the Spot rule skips `comparableSpec` and needs only the node's
  instance type and the two prices; the Fargate, Auto Mode, virtual-node and
  no-type guards stay (`recommend.go:885`). With the pair loaded, the node's
  own size must be in the answer and available (R3).
- `Types()` lists the snapshot's loaded sizes; a rule still filters
  candidates by `Available(n, t)`.

## Components touched

| Path | Change |
|------|--------|
| `pkg/azure/skus.go` (new) | Resource SKUs client: URL from `url.Values`, host and `nextLink` checks, paging, body cap, HTTP timeout, parse (R2, R3, R9), error type `armError` with status and code only (R5) |
| `pkg/azure/azcli.go`, `azcli_unix.go`, `azcli_windows.go` (new) | `az` runner: account list, token, cache, one in-flight request per subscription, process handling (`Setpgid` and group kill in `azcli_unix.go` only) (D1, R4, R5) |
| `pkg/azure/advisor.go` | pair states, `Begin(pairs)`, `Available`, notes, generation, retry (R1, R3, R5–R7) |
| `pkg/azure/catalog.go`, `zz_generated_instance_types.go`, `catalog_test.go` | removed; `NormalizeFamily` moves to `skus.go`; R2 citations move to `skus_test.go` (R12) |
| `pkg/azure/testdata/skus-eastus.json` (new) | recorded made-up SKU answer (D5) |
| `pkg/recommend/recommend.go` | `Node.Subscription`; `Input.Available`; availability in rightsize, swap and Spot; user-pool minimum (R11); temp-disk check (R9); `sizes:` math line (R8) |
| `pkg/recommend/testdata/azure-sizes.tsv` (new) | frozen extract of the autoscaler table with its credit, for the coverage tests (D5) |
| `pkg/recommend/tables_azure_ext_test.go`, AKS fixtures | on the extract and the recorded answer (R12) |
| `pkg/model/recommendations.go` | `Subscription` from the provider ID; pairs into `Begin`; `Types()` after it |
| `pkg/model/testdata/screens/recommendations-aks*.txt` | the `sizes:` line in open math popups (R8) |
| `cmd/kube-node-viewer/session.go` | advisor gets the SKU reader; none when pricing is disabled (R7); a Go option to inject the `az` runner and the Resource Manager base URL, used by `TestKindCluster` (D6) |
| `cmd/kube-node-viewer/kind_integration_test.go`, `testdata/kind/` | Azure Nodes created through the API, the fake `az` and Resource Manager injected (D6, T6) |
| `Makefile` | `azure-catalog` and `AUTOSCALER_AZURE` removed |
| `AGENTS.md` | `pkg/azure/` row: Retail Prices with no credentials; VM sizes from Resource Manager with the `az` sign-in |
| `README.md` | Recommendations on AKS: source, Reader, the R5 lines; Troubleshooting entry for them; autoscaler credit kept for the test extract only |
| `CHANGELOG.md` | the `[Unreleased]` AKS entry from #75 edited; if v0.16.0 has released before this merges (Monday 2026-11-16, or earlier on demand), a `### Changed` entry instead, since a released section is never edited |
| `TODO.md` | the refresh-data.yaml item removed |
| `specs/aks-recommendations/*` | dated notes (context, R2, R12, R16, counter-examples, D1, invariants, T1, Deferred) |

## Decisions

### D1: The token comes from the Azure CLI, run as a command

**Choice**: run, with stdin not attached and stderr captured:
1. `az account list --all --output json`, to classify (R5 rows 2–4) from
   structured output;
2. `az account get-access-token --resource https://management.azure.com/
   --subscription <id> --output json`, to get `accessToken` and
   `expires_on` (POSIX; `expiresOn`, local time, when `expires_on` is
   absent, as in `az` before 2.54.0 — azidentity reads it the same way,
   `azure_cli_credential.go:136-143`).

Row 2's text match uses the az 2.88.0 wording: its AAD error handler
raises `AuthenticationError(error_description, recommendation="Run the
command below to authenticate interactively; …")` (`azure/cli/core/auth/util.py:40-61`),
so the match is `AADSTS`, `authenticate interactively`, or `az login` (az's
"Please run 'az login' to setup account."); "interaction required" is never
printed, and an MFA or Conditional Access error
(`AADSTS50076`) says nothing about expiry. The fake `az` fixtures copy that
wording. Account `state` `Disabled` or `Deleted` is row 4; `Warned` and
`PastDue` subscriptions stay readable (`azure/cli/command_modules/profile/custom.py:65-70`).
Entries whose `cloudName` is not `AzureCloud` are ignored (`--all` lists
every cloud the CLI knows).

The cached token is dropped on a `401` or `403` answer and when the
subscription's `az account list` entry changes (`user.name`, `tenantId`;
compared in memory). One `get-access-token` per subscription is in flight
at a time; fetches of other regions wait for it.

Exit 0 with warnings on stderr is success. Each run has a 30 s timeout
(chosen, unmeasured); on Unix the process gets its own process group
(`Setpgid`) and the group is killed on timeout, and `cmd.WaitDelay` is set,
because Homebrew's `az` is a bash script that starts Python without `exec`,
so killing bash alone leaves the pipe open. The token is cached per
subscription until two minutes (chosen) before it expires. No Azure SDK.

**Justification, strongest first**:
1. **The owner's decision (2026-11-10): call `az`, no new dependency.**
   AKS admins already have the CLI; `az aks get-credentials` and kubelogin's
   `azurecli` mode use it.
2. **`azidentity` fails the licence allowlist.** A scratch module with
   `azidentity` v1.15.0 links `github.com/pkg/browser` on linux, darwin and
   windows, BSD-2-Clause, even for `AzureCLICredential` alone
   (`go-licenses report`, 2026-11-10; confirmed by spec-refuter);
   `make licenses` allows MIT, Apache-2.0, BSD-3-Clause and ISC
   (`Makefile:47`).
3. **`--subscription` picks the tenant.** `az` resolves the subscription's
   tenant from its profile (`_profile.py:379-416`), so a guest admin in
   several tenants needs nothing more. `get-access-token` never opens a
   browser (spec-refuter, 2026-11-10).
4. **Classifying from `az account list` is stable**: it is local, JSON, and
   names `id` and `state`; only the sign-in failures of `get-access-token`
   are read from its text (`AADSTS`, `authenticate interactively`, `az
   login`, above), and anything else gets the generic row 5.

> **Corrected 2026-11-10 (diff review).** Point 4 said "expired" and
> "interaction required" were read from text; the code and the paragraph
> above match `AADSTS`, `authenticate interactively` and `az login`, each
> with a test. `Sizes` now takes the session's context: switching context or
> quitting cancels it, `Close` waits up to 3 s, and a running `az` is killed
> with its process group. A SIGKILL of the program itself can still leave a
> hung `az` behind.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| No managed identity, workload identity or service-principal env vars | the owner's case is an admin at a workstation; a jump host can `az login --identity` |
| About a second per `az` run, two runs the first time | in the background, at most once per subscription per token lifetime, plus the retries R6 allows while an error holds |
| Windows is not tested in CI (`az.cmd` goes through `cmd.exe`) | inputs are validated before they reach the command line (R10); `GOOS=windows go vet ./pkg/azure` and a windows cross-build run in T1; T5 records a manual Windows check |
| On Windows, killing `az.cmd` at the timeout can leave its Python child running | rare (a hung `az`), and it exits on its own; a job object is not worth the code yet |
| The `az account list` output names tenants and users | it is parsed in memory, never logged or shown |

**When this would be wrong**: admins report running it where `az` can't be
installed (containers, CI) — then a token from `azcore`-compatible code, or
IMDS directly.

**Reversal action**: replace `azcli.go` with a token function; the SKU client
takes one, so nothing else changes.

**Alternatives discarded**:
- `azidentity.NewDefaultAzureCredential` — fails `make licenses` (point 2).
- `armcompute` v7.3.0 — passes the allowlist (it pulls azcore, not
  azidentity or pkg/browser), but one GET doesn't need it.
- `azcore` with our own `az` token function — valid, and passes the
  allowlist; not taken because plain `net/http` does the one GET.
- IMDS for managed identity — no case for it yet.

### D2: One request per (subscription, region), lazily; state changes move the generation

**Choice**: `Begin(pairs)` starts one fetch per pair not yet loaded, not in
flight, and not failing with a retry still pending. Each pair is `loading`,
`loaded` or `failed(row, retryAt)`. Every state change, and a retry falling
due, increments the SKU generation that `Generation()` adds to the price
generation, so `recKey` changes and the screen analyses again on the next
tick, on a static cluster too. The answer is kept for the session; `c` to
another context builds a new session and asks again.

The request uses `$filter=location eq '<region>'` (api-version 2021-07-01,
"Only location filter is supported currently", REST reference read
2026-11-10), an HTTP timeout of 60 s per page (chosen, unmeasured), a body
cap of 64 MiB per page (chosen; the per-region answer holds every Compute
resource type and its size is unmeasured), and follows `nextLink` within
R10's checks.

**Justification, strongest first**:
1. **Analysis only runs when `recKey` changes**; without the generation
   moving on a failure or a due retry, the loading note would stay and R6
   could never fire (spec-refuter H1).
2. **Lazy:** a user who never opens Recommendations makes no request and sees
   no error (R1).
3. **Per region:** one region's answer is all its nodes need.
4. **Sizes don't change within a session.**

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Answer size unmeasured | capped and paged; T5 measures it when the owner names a real subscription |
| A size released mid-session is unseen | restart or `c` |
| One analysis a minute while an error holds | the rules are pure functions over a snapshot, already run on every change |

**When this would be wrong**: the answer takes more than a few seconds or
nears the cap — then stream-parse it.

**Reversal action**: none needed; a cache TTL could be added.

**Alternatives discarded**: fetching at session start — every AKS user would
pay a request and see sign-in errors on the Overview; a timer goroutine for
retries — `recKey` polling already gives "only while shown".

### D3: Errors are fixed lines, retried while the screen is shown, never hidden behind a stale table

**Choice**: each failure maps to one R5 row, shown in the Recommendations
notes while it holds and logged once per distinct line. The SKU client has
its own error type (status and Resource Manager error `code` only), not the
price provider's `statusError`, whose text says "retail prices API"
(`pkg/azure/retry.go:44-49`). Rows 1–10 retry once a minute while shown;
11 and 12 use `retryDelay` (`pkg/azure/retry.go:58`); 13 never.

**Justification, strongest first**:
1. **The owner's decision**: say there is no permission, don't advise from
   old data.
2. **Fixed lines carry no identifiers.** `az` output and Resource Manager
   messages can name the subscription, tenant, user, client or object id and
   scope (`AuthorizationFailed` does); only status and code are kept.
3. **The retry is user-driven:** an admin runs `az login` or gets the role,
   and the screen recovers within a minute, without restarting (the open
   TODO "`c` on the current context only closes the popup" makes restarting
   the only other way).

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| An `az account list` and a request a minute while a sign-in or permission error holds and the screen is open | local and cheap; stops when the screen is left; the token cache (R4) still avoids `get-access-token` while a token is valid |
| Unmatched failures land in generic rows (5, 10) | they name the command or status that shows the cause |

**When this would be wrong**: users find the note noisy on clusters where
they deliberately lack the role — then a config key to turn it off.

**Reversal action**: drop the retry; a failure then holds for the session.

**Alternatives discarded**: falling back to the embedded table — the owner
refused it; showing `az`'s or Resource Manager's message — leaks
identifiers.

### D4: `--disable-pricing` also skips `az` and the SKU request

**Choice**: with pricing disabled the advisor gets no SKU reader (R7).

**Justification**: every rule that reads a size (rightsize, generation, arm,
Spot) also needs a price, and none runs without one (aks-recommendations
R3); `az` runs and sign-in errors would buy nothing. No new flag.

**When this would be wrong**: a rule needs sizes without prices.

**Reversal action**: pass the reader regardless.

### D5: Tests: a recorded made-up answer for parsing, a frozen autoscaler extract for coverage

**Choice**:
- `pkg/azure/testdata/skus-eastus.json`: a hand-built answer in the API's
  shape with the public sizes the fixtures and the R2, R3, R9 cases need,
  their public specs, and made-up locations, zones and restrictions.
- `pkg/recommend/testdata/azure-sizes.tsv`: size, family, vCPU, MiB and
  architecture of every size in the autoscaler table at `03e49d9cb822`,
  extracted once, frozen, never refreshed and never in the binary. Its
  header lines: `Copyright The Kubernetes Authors.`, the Apache-2.0
  notice, and a statement of change (Apache-2.0 §4(b)): "Extracted from
  kubernetes/autoscaler cluster-autoscaler/cloudprovider/azure/azure_instance_types.go
  at commit 03e49d9cb822: name, family, vCPU, memory and architecture
  only." `headers.py` doesn't read `.tsv`, so T4 checks the header itself. The table-coverage tests (every D5 key
  and target exists, every mapping keeps the temp disk family set) run on it.

**Justification**: no credential in CI or the repo; public size specs are not
environment data; coverage over 1,370 real sizes is kept without a refresh
chore (decided 2026-11-10).

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The extract ages | it only checks the fixed D5 tables; a family it lacks gets no row at run time, not wrong advice |
| The recorded answer is hand-made | T2 checks each size's numbers against the public size pages and records the date |

**When this would be wrong**: a D5 table needs a family newer than the
extract — extract again, by hand, in that PR.

**Reversal action**: delete the extract and narrow the coverage tests to the
recorded answer.

### D6: The kind run injects the fakes through a Go option

**Choice**: the session takes a Go option, the `azure` field of
`sessions` (`azureOption{run, base, transport, lister}`), that replaces the
`az` runner, the Resource Manager base URL and transport, and the candidate
prices. Only tests set it: `TestKindClusterAzureSizes`
(`cmd/kube-node-viewer/kind_integration_test.go`, itself
`//go:build integration`) and `session_test.go`; no flag, environment variable or config
key reaches it, and release builds always use `az` on `PATH` and
`https://management.azure.com`.

**Justification**: the kind test already runs the session in process
(`kind_integration_test.go:1,69`), so a build tag would add a second binary
for nothing.

**When this would be wrong**: a test needs the real binary against the fake.

**Reversal action**: add a build-tagged main for that test.

## Invariants

- No Azure HTTP and no `az` run outside `pkg/azure` (AGENTS.md layering).
- No subscription id, tenant id, account, client or object id, scope or
  token in any rendered text, log line or export.
- The token goes only to `https://management.azure.com`.
- AWS rows, AWS golden screens and the EKS fixture total ($242.94) unchanged.
- No interactive prompt, ever; `Update` never waits on `az` or Resource
  Manager.

## Failure modes

| Failure | Behaviour |
|---------|-----------|
| `az` missing, not signed in, wrong account, subscription disabled | R5 rows 1–4; idle, consolidation, old-node and Spot rows; retried each minute while shown |
| `az` hangs | process group killed at 30 s; row 5 |
| No role, policy denial, other `4xx` | rows 7, 8, 10 |
| `429`/`5xx`/network/timeout | row 11; `retryDelay` |
| Answer over the cap, malformed, foreign `nextLink` | row 12; `retryDelay` |
| Bad provider ID or region | row 13; no `az` run |
| One of two subscriptions fails | its agent pools named in the line; the other's nodes are advised |

## Test plan

- **Unit, `pkg/azure`:** a fake `az` script on `PATH` (account list shapes,
  token with `expires_on` or only `expiresOn`, warnings, each failure, a hang
  for the timeout); an `httptest` TLS Resource Manager (pages, foreign
  `nextLink`, 400/401/403×2/404/409/429/503, oversize, malformed); parse of
  the recorded answer (R2, R3, R9); token reuse; R10 validation; retry clock
  (R6).
- **Unit, `pkg/recommend` / `pkg/model`:** fixtures on the recorded answer
  ($322.66, $565.60, $242.94); availability per rule; user-pool minimum;
  temp-disk check; R5 row kinds; R8 line and no subscription in any golden
  screen; static-cluster recovery through `recKey`.
- **Run:** `TestKindCluster` runs in process (`kind_integration_test.go:1,69`),
  so the session option injects a fake `az` runner and the fake Resource
  Manager's URL there; release builds take no such input. A real
  subscription only when the owner names one; Windows `az.cmd` checked by
  hand; otherwise recorded as not run.
