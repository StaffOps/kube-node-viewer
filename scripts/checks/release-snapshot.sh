#!/bin/sh
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Checks the dist/ directory of `goreleaser release --snapshot` the way
# scripts/release-check.sh checks a published release (release-provenance R4):
#   scripts/checks/release-snapshot.sh [--no-sbom] [dist]
# - one checksum file, and the four archives of a release in it;
# - every archive has an SPDX SBOM (<archive>.sbom.json) that is in the
#   checksum file too (--no-sbom: skipped, for a local run without syft);
# - the linux archive for this machine, when there is one, holds a binary
#   that starts and reports a version.
# Exit 1 on the first failed check.
set -eu

sbom=true
if [ "${1:-}" = --no-sbom ]; then
	sbom=false
	shift
fi
dist=${1:-dist}

fail() { echo "release-snapshot: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

[ -d "$dist" ] || fail "no directory $dist"
cd "$dist"

set -- kube-node-viewer_*_sha256_checksums.txt
if [ $# -ne 1 ] || [ ! -s "$1" ]; then
	fail "expected one checksum file, found: $*"
fi
sums=$1

listed() { awk -v f="$1" '$2 == f { found = 1 } END { exit !found }' "$sums"; }

n=0
for a in kube-node-viewer_Darwin_all.tar.gz kube-node-viewer_Linux_arm64.tar.gz \
	kube-node-viewer_Linux_x86_64.tar.gz kube-node-viewer_Windows_x86_64.zip; do
	[ -s "$a" ] || fail "$a was not built"
	listed "$a" || fail "$a is not in $sums"
	if $sbom; then
		[ -s "$a.sbom.json" ] || fail "$a has no SBOM"
		listed "$a.sbom.json" || fail "$a.sbom.json is not in $sums"
		grep -q '"spdxVersion"' "$a.sbom.json" || fail "$a.sbom.json is not an SPDX document"
	fi
	n=$((n + 1))
done
if $sbom; then
	ok "$n archives and their SBOMs in $sums"
else
	ok "$n archives in $sums (SBOMs not checked)"
fi

case "$(uname -s)/$(uname -m)" in
Linux/x86_64) archive=kube-node-viewer_Linux_x86_64.tar.gz ;;
Linux/aarch64 | Linux/arm64) archive=kube-node-viewer_Linux_arm64.tar.gz ;;
Darwin/*) archive=kube-node-viewer_Darwin_all.tar.gz ;;
*) archive= ;;
esac
if [ -n "$archive" ]; then
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	tar xzf "$archive" -C "$tmp" kube-node-viewer
	got=$("$tmp/kube-node-viewer" -version | head -n 1)
	case $got in
	"kube-node-viewer version "?*) ok "$archive: \"$got\"" ;;
	*) fail "$archive reports \"$got\"" ;;
	esac
fi
