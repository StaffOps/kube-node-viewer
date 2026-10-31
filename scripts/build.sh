#!/usr/bin/env bash
# Build kube-node-viewer locally.
#
# Unlike `make build`, this skips `make generate` (no go-licenses needed, no
# pricing tables re-downloaded), so the working tree stays clean.
#
# Usage: scripts/build.sh [-o OUTPUT] [-i] [-a] [-h]
#   -o OUTPUT  binary path (default: bin/kube-node-viewer)
#   -i         also install to $GOBIN (or $(go env GOPATH)/bin)
#   -a         cross-compile for all release targets into bin/
#   -h         show this help
set -euo pipefail

NAME="kube-node-viewer"
TARGETS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() { sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; }

output="bin/$NAME"
install=false
all=false
while getopts ":o:iah" opt; do
  case "$opt" in
    o) output="$OPTARG" ;;
    i) install=true ;;
    a) all=true ;;
    h) usage; exit 0 ;;
    :) echo "error: -$OPTARG requires an argument" >&2; usage >&2; exit 2 ;;
    *) echo "error: unknown option -$OPTARG" >&2; usage >&2; exit 2 ;;
  esac
done

command -v go >/dev/null || { echo "error: go not found in PATH" >&2; exit 1; }

# The cmd/ directory is still named after upstream until the rebrand; pick
# whichever single main package lives there.
cmd_dirs=(cmd/*/)
if [ "${#cmd_dirs[@]}" -ne 1 ]; then
  echo "error: expected exactly one directory under cmd/, found: ${cmd_dirs[*]}" >&2
  exit 1
fi
pkg="./${cmd_dirs[0]%/}"

version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
commit="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ldflags="-s -w -X main.version=$version -X main.commit=$commit -X main.date=$date -X main.builtBy=scripts/build.sh"

export CGO_ENABLED=0

build() { # build <goos> <goarch> <output>
  echo "==> $1/$2 -> $3"
  GOOS="$1" GOARCH="$2" go build -trimpath -ldflags="$ldflags" -o "$3" "$pkg"
}

echo "version=$version commit=$commit"

if $all; then
  for t in $TARGETS; do
    goos="${t%/*}"; goarch="${t#*/}"
    ext=""; [ "$goos" = windows ] && ext=".exe"
    build "$goos" "$goarch" "bin/${NAME}_${goos}_${goarch}${ext}"
  done
else
  build "$(go env GOOS)" "$(go env GOARCH)" "$output"
  if $install; then
    dest="$(go env GOBIN)"
    [ -n "$dest" ] || dest="$(go env GOPATH)/bin"
    mkdir -p "$dest"
    cp "$output" "$dest/$NAME"
    echo "==> installed $dest/$NAME"
    case ":$PATH:" in *":$dest:"*) ;; *) echo "note: $dest is not in your PATH" ;; esac
  fi
fi
