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

## Pull requests and releases

1. Branch from `main`, add your change and a line under `## [Unreleased]` in
   `CHANGELOG.md` (`Added`, `Changed`, `Fixed`, `Removed`).
2. Run `make check` (it merges nothing; it stops if `main` moved, then runs
   what CI runs and prints the version your merge will release). For a UI
   or behaviour change, also run the binary against a cluster and record
   it in the spec's verify task.
3. Open a PR. CI runs `verify`, `lint` and `build`. If you have write access
   to this repository and the PR is not a draft, it is approved and merged
   automatically when the three pass; add the `hold` label to stop that.
   Other PRs are reviewed and merged by a maintainer.
4. On merge, `[Unreleased]` becomes the next version: only `Fixed` → patch,
   anything added, changed or removed → minor. The release workflow writes
   the version into `CHANGELOG.md`, tags it, publishes the binaries and
   updates the Homebrew cask in `StaffOps/homebrew-tap`.
5. `brew update && brew upgrade --cask staffops/tap/kube-node-viewer`.

A PR with nothing under `[Unreleased]` (docs, CI) merges without a release.
Pushing a tag by hand still releases it, for a re-run or an exception.

## License

This project is licensed under the [Apache License 2.0](LICENSE). By
contributing, you agree that your contributions are licensed under it too.
