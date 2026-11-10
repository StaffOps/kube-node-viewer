# Design: content-cloud-ids

> Implements [requirements](requirements.md).

## Current state

`scripts/checks/content.sh` defines each rule as a pattern and an allowed
pattern (`aks`/`aks_ok`, `azure`/`azure_ok`, …, lines 72-96), and `match`
(lines 193-215) reports a line that still matches once the allowed values are
removed, one at a time, from a lower-cased copy; the calls are at lines
217-237. In the full-tree mode a single `git grep` alternation of every
pattern (line 169) picks the candidate lines first. The repository's tests and
specs use `gce://example-project/…`, `gce://p/…` and resource groups with
short names (one or two letters, `mc_rg`, `mc_x`) and `example-rg`.

## Target state

Two more rule pairs, `gce`/`gce_ok` and `resourcegroup`/`resourcegroup_ok`,
added to the header list, the `git grep` alternation and the `match` calls.
The calls go before `match domain`, ahead of the `scan=` swap that drops
commit messages for the e-mail rule, so `--message` and `--commits` apply them.
Fixtures with short resource group names move to example names.

## Components touched

| Path | Change |
|------|--------|
| `scripts/checks/content.sh` | the two rule pairs, the header comment (rules and known gaps), the `git grep` alternation, the `match` calls |
| `scripts/checks/test-checks.sh` | blocked and passing cases for both rules, a message case, a full-tree case; the existing "Azure example subscription" case uses an example resource group |
| `pkg/azure/pricing_test.go`, `pkg/client/controller_test.go`, `pkg/model/aks_internal_test.go`, `pkg/model/facts_test.go`, `pkg/model/nodedetails_internal_test.go` | resource group fixtures renamed to `example-…` |
| `specs/content-cloud-ids/` | this spec, written without a blocked value |
| `AGENTS.md` | the guardrails row for real-environment data and its example values |
| `specs/README.md` | index entry |
| `TODO.md` | reword the item to the forms left; add tightening `gke_ok` |

> **Corrected 2026-11-10 (code review).** Line numbers drift: the rule
> pairs sit with the other `*_ok` patterns, `match()` defines the filter, the
> `match` calls follow it, and the full-tree `git grep` alternation is in the
> `all)` branch. Azure gives resource group names no first-character rule and
> allows Unicode letters, so the trade-off below on a leading `(` or `-` is
> wrong as written: such names, and Unicode ones, pass. Starting the name at
> `[a-z0-9_]` is what keeps regular expressions (`([^/]+)`) from matching; the
> gap is accepted, listed in the script's known gaps and in `TODO.md`
> (https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules).

## Decisions

### D1: Patterns of the id's own shape, an allow list of `example…` only

**Choice**: the extended regular expressions, matched case-insensitively:

- `gce`: `gce://([a-z0-9.-]+:)?[a-z][a-z0-9-]{4,28}[a-z0-9]/`
- `gce_ok`: `gce://(example\.com:)?example[a-z0-9-]*/`
- `resourcegroup`: `resourcegroups/[a-z0-9_][a-z0-9._()-]*`
- `resourcegroup_ok`: `resourcegroups/(mc_)?example[a-z0-9._()-]*`

Fixtures with other names are renamed.

**Justification, strongest first**:
1. The project id shape is GCP's rule (short-gke-context D2), so text that is
   not a project id (`gce://…/`, `gce://p/`, `gce://<project>/`, a regular
   expression) cannot match: no allowance is needed for it.
2. An Azure resource group name has no minimum length and no fixed shape, so
   only an explicit allow list can tell an example from a real name; `example…`
   is the convention every other rule uses. `gce_ok` is stricter than
   `gke_ok`, which accepts any project after `example.com:`: here the project
   must be an example too.
3. A name must start with a letter, a digit or `_`, so a regular expression
   after `resourceGroups/` (`([^/]+)`, `(.*)`) is not taken for a name.
4. Renaming about 20 fixture lines costs less than an allowance for short names
   that would let a real two-letter name through.
5. `mc_example…` is already the `aks` rule's example form, so the two rules
   agree on what an example node resource group is.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| An `MC_…` real group is reported twice (`aks` and `resourcegroup`) | the report is sorted and the fix is the same |
| A short-name fixture written later is blocked | the message says to use an example value; that is the rule for every other id |
| The `gce` rule misses a project id shorter than 6 characters | GCP refuses those, so none is real |
| A real value that starts with `example` passes (`gce://example<suffix>/`, `resourceGroups/example<suffix>`, an `MC_example-rg_…` whose cluster part is real) | the same prefix allowance as `aks_ok`, `gke_ok` and `server_ok`; the deny list catches listed names |
| A real name written with a leading `(` or `-` is not matched | Azure names do not start with `(`; a leading `-` is not a valid name start either |

**When this would be wrong**: the blocked short names keep coming back in
legitimate text (Azure docs quoting a short group name), or a real project id
shape differs from GCP's documented rule.

**Reversal action**: remove the pair from `match` and the `git grep`
alternation, or widen the allowed pattern, and adjust the cases; minutes.

**Alternatives discarded**: allowing names of one or two characters (a broad
allowance a real name can hide in); flagging every `gce://` (blocks code and
specs that describe the form); only adding names to the deny list (it is
private and per clone, and does not catch a group nobody listed).

> **Corrected 2026-11-10 (gaps).** The trade-off row above on values that
> start with `example` named two such values literally; D2 blocks them now, so
> the row shows them as `example<suffix>`. D2 narrows that allowance to
> `example` alone or followed by `-` or a digit, for `aks_ok` too, so the row's
> "same prefix allowance as `aks_ok`, `gke_ok`" no longer holds for those.
> The row on a leading `(` or `-` is also superseded: D2 blocks both (a `(`
> only when it starts a whole name).

### D2: Close the gaps (R5 to R10)

**Choice**:

- An example is `example` alone or followed by `-` or a digit. In `content.sh`
  `exproj='(example\.com:)?example([-0-9][a-z0-9-]*)?'` serves `gke_ok`
  (followed by `_`), `gce_ok` and `gcp_ok` (followed by `/`); `aks_ok` and
  `resourcegroup_ok` use the same shape on their names, which must then end
  (a character that cannot continue the name, or the end of the line; the
  continuation and that end test are ASCII, so a `…` after `MC_example` ends
  it). Each allowed value is anchored to its rule's prefix.
- `resourcegroup` gains these alternatives: the separator after
  `resourceGroups` is `/`, `%2f` or `\/`; a key `resource[_-]?group(_?name)?`
  followed by `:`, `=` or `:=` and a quoted value, by `:` and an unquoted
  value ending the line (a `#` comment allowed), or by `=` and a name with no
  space that ends at a space, `;`, `&`, `|`, a quote or the end of the line
  (not `,` or `)`, a Python keyword argument); `--resource-group` or
  `--node-resource-group` followed by spaces or `=`; `-ResourceGroupName`
  (Az PowerShell) followed by spaces; `-g` after an `az` word on the same
  line, not past `|`, `;` or `&`. Name continuation includes UTF-8
  continuation bytes (`\200`-`\277`), so a `(` name or an unquoted YAML
  value with an accented letter still reaches its end. After `resourceGroups/` a name starts
  with any number of `-`, then a letter, a digit, `_` or the lead byte of a
  non-ASCII letter in UTF-8 (`printf '\303-\337\340-\341\343-\357\360-\364'`,
  so the file stays ASCII), or with `(` when the name then runs to `/`, a
  quote, a space, `,`, `;`, `&`, `%`, `\` or the end of the line.
- A new rule `gcp`:
  `projects<sep>([a-z0-9.-]+:)?([a-z][a-z0-9-]{4,28}[a-z0-9]|[0-9]{6,})<sep>(zones|regions|locations|global)([^a-z0-9_-]|$)`,
  where `<sep>` is `/`, `%2f` or `\/` (as after `resourceGroups`), allowed
  `projects<sep>(<example project>|123456789|123456789012|0+)<sep>`; `$gcp`
  joins the full-tree `git grep` alternation as is (ASCII).
- `match()` runs its `grep` and `sed` with `LC_ALL=C`, so the byte ranges
  mean the same in every locale; the full-tree `git grep` gets an ASCII
  prefilter for `resourcegroup` (`resource[_-]?group`, `[[:space:]]-g` then a
  space or `=`) instead of the pattern itself, and keeps the user's locale for
  the deny list.

**Justification, strongest first**:
1. A hyphen or digit after `example` is how every example in the repository
   is written (`example-rg`, `example-project`, `example1-demo1…`), so
   requiring it costs no fixture and stops a real name that only starts with
   `example`.
2. A quoted value, an unquoted one that ends a YAML line, or `KEY=name` is
   what JSON, YAML, Terraform, shell and `az` write; Go code assigns a group
   from an identifier (`ResourceGroup: group,`, `x := y`), which no form
   matches, and gofmt never writes `KEY=name`.
3. `-g` alone is common outside Azure (`npm -g`, `go -gcflags`); after `az`
   on the same command it is the resource group.
4. Each collection after `projects/<id>/` is a GCP resource path (compute,
   GKE, the PD CSI node id), while `projects/` alone is a common directory
   name.
5. Byte ranges in the C locale are the only portable way to say "a non-ASCII
   letter" in a POSIX ERE; the lead bytes skip `\302` (`·`, `©`, no-break
   space) and `\342` (`…`, arrows, box drawing), which the repository's text
   uses after a `/`. A `(` start must be a whole name, so a regular
   expression after `resourceGroups/` (`(mc_)?…`, `([^/]+)`, `(a|b)`) is
   not taken for one.

**Trade-offs accepted**:

| Cost | Why acceptable |
|------|----------------|
| `example<letters>` or `example_…` values written later are blocked | the message says to use an example value; none exists today |
| The forms listed out of scope in the requirements pass (table output, `az group --name`, Terraform and Bicep blocks, multi-line YAML, an unquoted value before more text, `--project`, registries, other collections, subscription ids outside `subscriptions/`) | the deny list catches listed names; `TODO.md` keeps the item |
| A name starting with a letter whose lead byte is `\342` passes; `\303` also holds `×` and `÷`, `\360`-`\364` emoji | letters in that block are rare; a group named with a symbol is blocked, which is harmless |
| The unquoted YAML form also matches a JS, TS or Bicep object property that ends a line (`resourceGroup: group`) | this repository has none; rename the identifier or use an example |
| Python and TypeScript annotations that end a line (`resource_group: str`, `resourceGroup: string`) match the unquoted YAML form | rename the parameter or write an example; a Python keyword argument from a variable (`resource_group_name=rg,`) passes |
| `gcp` matches a directory layout shaped like a GCP path (`<dir>/projects/<name>/global/…`, `…/regions/…`) | uncommon in this repository; rename with an example project |
| `match()` folds case only for ASCII | every pattern and allowed value is ASCII already; the deny list keeps its own locale |

**When this would be wrong**: legitimate text keeps quoting real-looking
resource group values (Azure docs), or a GCP resource path this repository
needs uses a collection outside the list.

**Reversal action**: drop the alternatives or the `gcp` pair and their
cases; minutes.

**Alternatives discarded**: flagging `-g` anywhere (false positives in npm and
Go flags); flagging every `projects/<id>/` (local paths, GitHub boards);
setting `LC_ALL=C` for the whole script (the deny list would lose case folding
for non-ASCII names); a leading `(` taken as a name start in any case (it
blocks the regular expressions this spec and its tests quote).

## Invariants

- Every tracked file passes the full-tree check after the change.
- No report prints the matched text under GitHub Actions.
- Existing rules and cases keep their results.
- No commit of this change holds a blocked value, in a file or in its message
  (`--commits` checks each commit alone).

## Failure modes

| Failure | Behavior | How the user notices |
|---------|----------|----------------------|
| A real id in a commit | blocked by pre-commit, `commit-guard`, CI | the report names the place and the rule |
| A fixture with a short name | blocked the same way | replace it with an example value |

## Test plan

- `test-checks.sh`: each acceptance criterion of R1 and R2 as a `commit` case
  (blocked or passes), a message case and a `tree` case (R3).
- `content.sh --staged` before each commit of this change.
- `scripts/verify.sh` on the whole tree (R4).
