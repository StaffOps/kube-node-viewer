# Requirements: monthly-price-columns

> Status: active · Owner: Carlos Gomes · Reviewed: 2026-11-02 (code-review agent)

## Context

The node list prices each node per hour (`PRICE/H`, and `IDLE/H` on demand).
The bill, budgets and the Recommendations savings are per month, so reading
a node's cost means multiplying by 730 in your head. The cluster summary and
the group headings already show months; the node rows do not. Two columns,
`PRICE/M` and `IDLE/M`, put the monthly figures on every node, shown by
default and toggled like any other column with `o`.

## Usage example

```text
NAME                     PODS   INSTANCE    PRICE/H  PRICE/M  IDLE/M  CAPACITY  NODEGROUP  ZONE        STATUS
ip-10-0-1-12.ec2.int…    23/58  m6g.xlarge  $0.1540  $112.42  $41.60  OnDemand  default    us-east-1a  Ready
ip-10-0-2-40.ec2.int…    11/29  r6g.large   ~$0.1008 ~$73.58  -       Reserved  data       us-east-1b  Ready
```

## Requirements

**R1** WHEN the `PRICE/M` column is shown THEN the system SHALL show each
node's price per month: its hourly price × 730, with two decimals and
thousands separators, as the Recommendations savings are written.

- [x] A node with a real price shows `$112.42`; a $32/h node shows
  `$23,360.00`.
- [x] A reserved node's on-demand equivalent shows with `~$`, like `PRICE/H`.
- [x] A node without a price shows `-`.

**R2** WHEN the `IDLE/M` column is shown THEN the system SHALL show what the
node pays per month for capacity nothing requests: its hourly idle cost
(`IDLE/H`) × 730, with two decimals.

- [x] A node without an idle cost (no real price, reserved equivalent,
  Fargate) shows `-`, like `IDLE/H`.

**R3** WHEN a cluster has no saved view THEN the system SHALL show these
columns, in order: `INSTANCE`, `PRICE/H`, `PRICE/M`, `IDLE/M`, `CAPACITY`,
`NODEGROUP`, `ZONE`, `STATUS`.

- [x] A cluster with a saved view keeps its columns unchanged: no migration.
- [x] `--extra-labels` builds the columns from these defaults plus the
  labels, on any cluster, as it does today with the current defaults.
- [x] The Columns panel (`o`) lists `PRICE/M` and `IDLE/M` (and `IDLE/H`), so
  each can be shown or hidden on its own, and moved with `K`/`J`.
- [x] A named view or a saved cluster view that lists them shows them.

**R4** WHEN the user opens the Sort panel or reads the header THEN the system
SHALL keep the sort names `price` and `idle`, with no monthly sort fields.

- [x] Sorting by `price` or `idle` orders the monthly columns too, since
  they are the hourly values × 730.

**R5** WHEN pricing is disabled (`--disable-pricing`) THEN the system SHALL
leave out `PRICE/H`, `PRICE/M`, `IDLE/H` and `IDLE/M`.

- [x] `IDLE/H`, which today still shows values from the static price table
  while `PRICE/H` is hidden, is left out too (behavior change).

**R6** WHEN the window is too narrow for every column THEN the system SHALL
drop columns in this order of importance, kept first: `STATUS`, `PRICE/M`,
`CAPACITY`, `PRICE/H`, `IDLE/M`, `INSTANCE`, `IDLE/H`, `NODEGROUP`, `ZONE`,
`IP`, `AGE`, `REGION`, then labels.

- [x] At 120 columns (cpu only) the list keeps `PRICE/M`, `CAPACITY` and
  `STATUS`.

## Counter-examples

- MUST NOT rename `PRICE/H` or `IDLE/H`, or change their values, outside
  `--disable-pricing`.
- MUST NOT change the month factor used elsewhere (730 h: summary, group
  headings, Recommendations, export totals).

## Out of scope

- Adding a monthly price to the CSV/JSON export rows (the JSON totals already
  have `pricePerMonth`).
- A monthly figure in the node details popup.
- Migrating saved clusters or named views to the new defaults: add the
  columns with `o`.
