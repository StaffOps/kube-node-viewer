# Tasks: label-visibility

> Implements [design](design.md). Status: `[ ]` todo · `[-]` in progress · `[x]` done
> (Kiro's task-list format — the Kiro spec panel tracks these checkboxes).

- [x] T1. `model.LabelPolicy`: compile allow/deny patterns, `*` matcher, `Shows`; Apache header
  - _Covers: R1, R2, R3, R4 · Depends on: — · Verified by: `go test ./pkg/model -run LabelPolicy`_
- [x] T2. Config loader: path from `XDG_CONFIG_HOME`, strict YAML decode, string-only list items, errors naming the file; Apache header
  - _Covers: R8, R9 · Depends on: T1 · Verified by: `go test ./cmd/kube-node-viewer -run Settings`_
- [x] T3. Flags `--labels-allow` / `--labels-deny` replacing the file lists; wire `SetLabelPolicy` in `main.go`
  - _Covers: R7 · Depends on: T2 · Verified by: `go test ./cmd/kube-node-viewer -run Settings`; flags absent from the state file_
- [x] T4. Apply the policy in `filterFieldChoices` for Filters, Group by, Sort and Columns, keeping in-use labels (snapshot at panel open) and computed labels
  - _Covers: R1, R2, R3, R5 · Depends on: T1 · Verified by: `go test ./pkg/model -run 'Panel|Choices'`_
- [x] T5. Status line in `popup.go` with the hidden count; `a` to show all until the panel closes (Sort/Columns rebuild, Filters closes a hidden open field); `?` help entry
  - _Covers: R6 · Depends on: T4 · Verified by: `go test ./pkg/model -run ShowAll`_
- [x] T6. Docs: README (config file, flags, `a`), CHANGELOG `[Unreleased]`, example `config.yaml`
  - _Covers: — · Depends on: T3, T5 · Verified by: review; README "Label visibility" section, CHANGELOG `[Unreleased]`. The YAML examples live in the README (no separate example file)_
- [-] T7. Verify: `scripts/verify.sh` green, coverage ≥ 90% on new files, manual run on a cluster the owner names
  - _Covers: R1, R5, R6 · Depends on: T6 · Verified by: command output and owner check_
  - _Note (2026-11-02): `scripts/verify.sh` green; coverage 100% on `labelpolicy.go`, `loadSettings` 90.9%, `readLabelLists` 93.8%. The manual run on a cluster the owner names is pending._
- [x] T8. Review follow-ups (2026-11-02): short windows keep the key hints before the status line; Sort and Columns refresh their list when nodes change while open; Columns keeps the cursor after `a`; flag help shows the resolved config path
  - _Covers: R6, R9 · Depends on: T5 · Verified by: `go test ./pkg/model -run 'Popup|Sort|Columns'`, `go test ./cmd/kube-node-viewer -run Settings`_
  - _Note (2026-11-02): implemented; `scripts/verify.sh` green. Corrects design ("rows take `height-5`"): the box is never taller than its height; it drops lines in this order: rows down to one, the separator, the status line, the hint, then the top border. Narrow boxes show `<n> hidden • a: all` / `all labels • a: hide`. Independent tests and review done (2026-11-02): verify green, touched functions 94–100% except `refreshFields` 80% (defensive empty-list branch)._

## Deferred

| Task | Promotion trigger |
|------|-------------------|
| Allow/deny lists per kubeconfig context | The owner needs different label sets on clusters they open in the same run |
| Move the `~/.kube-node-viewer` options into `config.yaml` | A second option needs list or nested values, or the two files start to conflict |
| Line numbers in config errors (`go.yaml.in/yaml/v3` with `KnownFields`) | Users report that field-only messages are not enough |
