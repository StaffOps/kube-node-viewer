# Design: fix-azure-retry

> Implements [bugfix](bugfix.md).

## Components touched

- `pkg/azure/retry.go` (new): `statusError`, `retryDelay`.
- `pkg/azure/pricing.go`: `get` returns a `*statusError`; `retryAt` and
  `failed` per region replace `failedAt`; `run` waits for a wake or the
  earliest due region; `fetchWanted` skips waiting regions and logs the
  resolution.
- `pkg/model/header.go`: `ResolvedPrefix`.
- `cmd/kube-node-viewer/errlog.go`: `Write` handles a resolution;
  `lastLine` strips the standard logger's date and time.

## D1. Read both retry headers, log what was read

On 429 or 503, `retryDelay` reads `Retry-After` and
`x-ms-ratelimit-microsoft.consumption-retry-after` (documented for the
China Retail Prices API), keeps the larger usable value, and the failure
line ends with `(retry header 2m0s)` or `(no retry header)`.

**Justification**: what the global API sends on 429 is unmeasured; the
log line makes the next real 429 show it, so the cap and floor can be
tuned from evidence. **Trade-offs**: a slightly longer log line.
**When this would be wrong**: the API sends neither header; then the
floor (1 minute) applies, as today. **Reversal action**: read only
`Retry-After`, or drop the note from the line.

## D2. delay = max(retry delay, min(header, 10 minutes))

A pure function `retryDelay(code, header, now, floor)` returns the delay;
the provider stores `retryAt[region] = now + delay`.

**Justification**: a header asking for less than the retry delay must not
make the provider ask more often than it does today; a bad or huge
header must not stall prices for hours. The 10-minute cap is unmeasured.
Keeping the floor outside the cap keeps a deliberately long retry delay
(tests use 1 hour). **Trade-offs**: if the API really needs more than 10
minutes, the provider asks too early and is refused again, once per 10
minutes. **When this would be wrong**: the API routinely asks for longer
than 10 minutes. **Reversal action**: raise or drop `maxRetryAfter`.

## D3. One retry time per region

`retryAt map[region]time.Time` and `failed map[region]bool` replace
`failedAt`. `fetchWanted` asks only regions whose time has passed and
returns the earliest waiting one; `run` sleeps until a wake or that
time. The failure path no longer wakes itself.

**Justification**: one region's failure is not another's. **Trade-offs**:
a small map per provider (one entry per failed region).
**When this would be wrong**: if the API limits per client rather than per region,
asking other regions during a 429 adds refused requests; they are logged
and wait their own time. **Reversal action**: go back to one failure
time for the provider.

## D4. A resolution is a log line with a prefix

The provider logs `resolved: updating Azure pricing for <region>` after
a success that follows a logged failure of that region. `errlog.Write`
writes every line to the log file; for a line starting with
`model.ResolvedPrefix` it clears the kept line only when that line equals
the rest or starts with the rest followed by `,` (so `eastus` never
resolves `eastus2`), and never keeps the resolution. `lastLine` also
strips the standard logger's date and time, so the kept line starts with
the text a resolution names.

**Justification**: keeps Azure HTTP in `pkg/azure` and the header in
`pkg/model`/`cmd`, with one string convention and no new API between the
provider and the UI. **Alternatives**: shorten `issueWindow` (hides every
other error sooner); an issue API keyed by source (more code for one
producer); not logging 429 (hides real outages). **Trade-offs**: a
string convention, checked by tests on both sides.
**When this would be wrong**: another producer logs lines starting with `resolved: ` that
match a kept error it did not cause. **Reversal action**: drop the prefix
handling in `errlog.Write` and the resolution log; the date stripping in
`lastLine` can stay.
