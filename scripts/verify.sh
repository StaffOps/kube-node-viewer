#!/usr/bin/env bash
# Local gate before every commit: gofmt, go vet, go test, spec lint.
# `hack/` is excluded: several upstream main packages share that directory.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

PKGS="./cmd/... ./pkg/..."

echo "==> gofmt"
unformatted=$(gofmt -s -l cmd pkg)
if [ -n "$unformatted" ]; then
  echo "gofmt -s needed on:" >&2
  echo "$unformatted" >&2
  exit 1
fi

echo "==> go vet"
# shellcheck disable=SC2086
go vet $PKGS

echo "==> go test"
# shellcheck disable=SC2086
go test $PKGS

echo "==> spec lint"
scripts/spec-lint.sh

echo "verify: ok"
