# Contributing

Bug reports, feature requests and pull requests are welcome on
[GitHub](https://github.com/StaffOps/kube-node-viewer).

## Reporting a bug

Include the `kube-node-viewer -version` output, the command and flags you ran,
what you expected and what you saw, and your Kubernetes version.

## Changing code

The rules for this repository live in [`AGENTS.md`](AGENTS.md); they apply to
people and coding agents alike. In short:

1. Anything listed in the "spec needed" table of `AGENTS.md` starts with a
   spec in [`specs/`](specs/README.md).
2. Run `scripts/verify.sh` (gofmt, vet, tests, spec lint) before opening a PR,
   and `scripts/build.sh` to build the binary.
3. Use [Conventional Commits](https://www.conventionalcommits.org/).
4. Keep `LICENSE`, `NOTICE` and the existing Apache license headers. When you
   change a file that came from upstream, keep its modification note.

## Releasing

1. Open a PR that turns `## [Unreleased]` in `CHANGELOG.md` into
   `## [x.y.z] - YYYY-MM-DD` (keep an empty `[Unreleased]` above it) and
   updates the compare links at the bottom.
2. Merge it. The `Binary release` workflow sees a version with no tag, tags
   the merge commit `vx.y.z`, publishes the binaries and updates the
   Homebrew cask in `StaffOps/homebrew-tap`.
3. `brew update && brew upgrade --cask staffops/tap/kube-node-viewer`.

Pushing a tag by hand still releases it, for a re-run or an exception.

## License

This project is licensed under the [Apache License 2.0](LICENSE). By
contributing, you agree that your contributions are licensed under it too.
