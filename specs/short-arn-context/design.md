# Design: short-arn-context

> Implements [requirements](requirements.md).

## Current state

`(*UIModel).header` (`pkg/model/header.go:148-216`) reads the context once,
`context := u.info.Context` (`header.go:158`), `-` when empty, and uses that
variable in the one-line header (`header.go:166`), the compact box
(`header.go:180`) and the full panel's `Context:` row (`header.go:192`).
Nothing in `pkg/` or `cmd/` parses ARNs. The contexts popup lists the names
from the kubeconfig (`pkg/model/contexts.go:166-176`) and marks the one equal
to `u.info.Context`. The export writes `u.info.Context` (`export.go:144`) and
builds its file name from it (`export.go:243-248`). The saved-state key is
`Context + "@" + Server` (`pkg/client/client.go:85`).

## Target state

A pure function `shortContext(name string) string` in `pkg/model/header.go`
returns `<name>` for a context that matches the EKS ARN pattern of R1 and the
input unchanged otherwise. The `context :=` line becomes
`context := shortContext(u.info.Context)`. The popup, the export, the state
key and the flags keep reading `u.info.Context` and the kubeconfig names, so
they are untouched.

## Components touched

| Path | Change |
|------|--------|
| `pkg/model/header.go` | `shortContext` and its use at line 158 |
| `pkg/model/header_internal_test.go` | `TestShortContextName`, `TestHeadersShowTheShortContextName`, `TestShortNameOnlyInTheHeaders`, `TestARNRegionIsNotThePricingRegion` |
| `pkg/client/client_test.go` | `TestClusterKeyKeepsTheFullContext` |
| `README.md` | both header paragraphs (full panel and compact box) say an EKS ARN context shows as its cluster name |
| `CHANGELOG.md` | `[Unreleased]` `Changed` |
| `specs/compact-header-no-url/requirements.md` | dated note after its counter-example "SHALL NOT shorten the context name" and its ARN out-of-scope bullet: superseded by short-arn-context |
| `specs/interactive-tui/requirements.md` | dated note under R16: the context name is superseded for ARN contexts by short-arn-context |
| `TODO.md` | remove the ARN item; add GKE context names and a shorter export file name |

## Decisions

### D1: Shorten only in the three headers

**Choice**: the full panel, the compact box and the one-line header show the
cluster name of an EKS ARN context; every other place keeps the full name.

**Justification, strongest first**:
1. The headers are what screenshots capture and what the compact box has to
   fit; the popup, the export, logs and the state file are read by the user
   or by the tool itself, where the full name is the identifier.
2. One variable feeds all three headers (the `context :=` line), so the change is
   one call and the three stay consistent.
3. The state key, the `--context` flag and the export keep working on the
   kubeconfig name, so nothing saved or scripted changes.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| Two ARN contexts for clusters with the same name in different accounts look alike in the headers. Per header: the full panel still shows the endpoint (`Cluster:`) and the region; the compact box shows only the region; the one-line header shows neither, so nothing there tells them apart | the contexts popup lists the full ARNs, and `ctrl+e` brings the full panel; adding the region or server to the one-line header is out of scope |
| The full panel's `Context:` row shows a cluster name, not the kubeconfig context name the user would type in `--context` | the README says so; `c` shows the kubeconfig name |
| The full panel no longer shows the account id at all | the account is not something the tool acts on; the popup and the export keep it |
| A screenshot of the contexts popup still shows account ids | the popup is where a user picks among ARNs, which needs the full names (D3) |

**When this would be wrong**: users rely on the header to see which AWS
account they are in (several accounts with same-named clusters, or
switching profiles), and the region and endpoint are not enough.

**Reversal action**: replace `shortContext(u.info.Context)` with
`u.info.Context` on the `context :=` line and drop the function and its tests; a
few minutes.

**Alternatives discarded**: shortening everywhere `u.info.Context` is shown
(the popup could no longer tell same-named clusters apart, and a user could
not match the header to `kubectl config get-contexts`); masking only the
account (`arn:aws:eks:us-east-1:…:cluster/demo-prod`), which keeps the line
long and the region twice.

### D2: A strict pattern, anything else unchanged

**Choice**: shorten only a name matching, in full,
`^arn:aws(-[a-z]+)*:eks:[a-z]+(-[a-z]+)+-[0-9]+:[0-9]{12}:cluster/([0-9A-Za-z][0-9A-Za-z_-]{0,99})$`;
return every other name as it is.

**Justification, strongest first**:
1. A context name is free text; a loose rule (for example "the text after the
   last `/`") would rename contexts that only look like ARNs, and the header
   would show a name the user never chose.
2. The fields follow AWS's formats: partition `aws`, optionally followed by
   `-<letters>` groups (`aws-cn`, `aws-us-gov`, the `aws-iso…` family,
   `aws-eusc`); region codes made of lower-case letter groups joined by `-`
   and ending in `-<digits>` (`us-east-1`, `cn-north-1`, `us-gov-west-1`,
   `us-iso-east-1`, `eusc-de-east-1`, whose first group has four letters);
   12-digit accounts;
   EKS cluster names of 1 to 100 letters, digits, `-` and `_`, starting with
   a letter or digit.
3. Anything outside the pattern renders as today, so the worst case of a
   pattern too strict is the current behaviour.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| A future partition or region format outside the pattern keeps showing the full ARN | that is today's behaviour; the pattern can be widened with a test case |
| GKE names (`gke_<project>_…`) keep showing the project id | out of scope, a `TODO.md` item |

**When this would be wrong**: EKS ARNs show up that the pattern rejects (a new
partition naming scheme, a region code with another shape), or a non-EKS
context name matches it.

**Reversal action**: widen or narrow the regular expression and add the case
to `TestShortContextName`; minutes.

**Alternatives discarded**: parsing with `github.com/aws/aws-sdk-go-v2/aws/arn`
(the SDK may only be imported in `pkg/aws` and `cmd/*/session.go`, and the
header lives in `pkg/model`); splitting on `:` and `/` without a pattern
(accepts malformed names).

### D3: The contexts popup keeps the full names

**Choice**: `c` lists the kubeconfig context names as they are, including
ARNs.

**Justification, strongest first**:
1. The popup is where the user picks a context; two ARNs with the same
   cluster name in different accounts or regions must stay distinguishable
   there.
2. The names match `kubectl config get-contexts` and the `--context` flag, so
   what the user sees is what they can type.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| The popup is wide and shows account ids for ARN contexts | it is a transient picker, opened on purpose |

**When this would be wrong**: users screenshot the popup as often as the
main screen, or find the ARNs in the popup hard to scan.

**Reversal action**: show `shortContext(name)` in `contextsPanel`
(`contexts.go:172`) with the full name kept for the switch; an hour with a
test, and a decision on how same-named rows are told apart.

**Alternatives discarded**: showing `<name> (<region>, <account>)` in the
popup (a new format to read, and still the account id).

## Invariants

- Requests, never usage; no new network call.
- `u.info.Context`, the kubeconfig names, the state key and the export are
  unchanged; only what the three headers print differs.
- For a context that is not an EKS ARN, every header renders byte-identically
  (the golden screens use `demo-prod` and stay unchanged).

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| Malformed ARN | Shown as it is | The full name in the header, as today |
| Same cluster name in two accounts | Same header name | The `Cluster:` endpoint and the region differ |

## Test plan

- `TestShortContextName`, table: the R1 examples (aws, aws-cn, aws-us-gov,
  aws-iso-b, aws-eusc with `eusc-de-east-1`; names with `-` and `_`, 1 and 100 characters) and the R2
  examples (plain names, `gke_…`, an IAM ARN, a nodegroup ARN, 11 and 13
  digit accounts, empty name, 101 characters, `/`, `:` or a space in the
  name, leading or trailing text, empty string).
- `TestHeadersShowTheShortContextName`: with an ARN context, the full panel
  (40 rows), the compact box (`ctrl+e`) and the one-line header (10 rows)
  each show `demo-prod` and none contains `111122223333`; with `demo-prod`
  as the context the output equals today's.
- `TestShortNameOnlyInTheHeaders`: with an ARN context, the contexts popup
  rows, the "connecting to …" row, a switch error row and the export's
  `context` field hold the full ARN.
- `TestARNRegionIsNotThePricingRegion`: an ARN in `eu-west-1`, no region
  from the server, no nodes → the full panel's `Region:` row is `-`, the
  compact box has no `eu-west-1`.
- `TestClusterKeyKeepsTheFullContext` (pkg/client): the state key is the
  full ARN plus `@` and the server.
- The golden screens stay byte-identical.
- Run on a local kind cluster whose kubeconfig context is renamed to a
  made-up ARN with account 111122223333: the three headers and `c`.
