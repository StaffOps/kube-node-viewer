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
# Checks a published release the way a user gets it, not the files the
# release run had on disk:
#   scripts/release-check.sh [--strict] [--cask] vX.Y.Z
# - every asset is in the checksum file;
# - the checksum file's Sigstore bundle verifies, issued to release.yaml of
#   this repository (needs cosign), and every archive has an SBOM;
#   --strict fails when either is missing (releases since 0.13.1 have
#   them), otherwise that is only reported;
# - the archive for this machine has a SLSA build provenance attestation
#   whose certificate names release.yaml of this repository run from main
#   or from this release's tag (gh attestation verify); --strict
#   fails without it from 0.14.0, the first release that has one; older
#   releases only get a note;
# - the archive for this machine matches the checksum file, and its binary
#   reports the release version;
# - --cask: the Homebrew cask in StaffOps/homebrew-tap is at this version
#   and its sha256 values are the release's.
# Needs gh (authenticated) and network. Exit 1 on the first failed check.
set -eu

REPO=${KNV_REPO:-StaffOps/kube-node-viewer}
TAP=${KNV_TAP:-StaffOps/homebrew-tap}
strict=false
cask=false
while [ $# -gt 1 ]; do
	case $1 in
	--strict) strict=true ;;
	--cask) cask=true ;;
	*) echo "unknown option: $1" >&2; exit 2 ;;
	esac
	shift
done
[ $# -eq 1 ] || { echo "usage: $0 [--strict] [--cask] vX.Y.Z" >&2; exit 2; }
tag=$1
v=${tag#v}

fail() { echo "release-check: $*" >&2; exit 1; }
ok() { echo "ok: $*"; }

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
cd "$dir"

sums="kube-node-viewer_${v}_sha256_checksums.txt"
gh release view "$tag" -R "$REPO" --json assets --jq '.assets[].name' > names.txt ||
	fail "no release $tag"
grep -qxF "$sums" names.txt || fail "$sums is not a release asset"

case "$(uname -s)/$(uname -m)" in
Linux/x86_64) archive=kube-node-viewer_Linux_x86_64.tar.gz ;;
Linux/aarch64 | Linux/arm64) archive=kube-node-viewer_Linux_arm64.tar.gz ;;
Darwin/*) archive=kube-node-viewer_Darwin_all.tar.gz ;;
*) fail "no archive for $(uname -s)/$(uname -m)" ;;
esac

gh release download "$tag" -R "$REPO" -p "$sums" -p "$archive"

# every asset but the checksum file and its signature is in the checksum file
grep -vxF -e "$sums" -e "$sums.sigstore.json" names.txt > listed.txt || :
while read -r a; do
	awk -v f="$a" '$2 == f { found = 1 } END { exit !found }' "$sums" ||
		fail "$a is not in $sums"
done < listed.txt
ok "every asset is in $sums"

missing=0
grep -E '\.(tar\.gz|zip)$' names.txt > archives.txt || :
while read -r a; do
	grep -qxF "$a.sbom.json" names.txt || missing=$((missing + 1))
done < archives.txt
if [ "$missing" -eq 0 ]; then
	ok "every archive has an SBOM"
elif $strict; then
	fail "$missing archives have no SBOM"
else
	echo "note: $missing archives have no SBOM"
fi

if grep -qxF "$sums.sigstore.json" names.txt; then
	command -v cosign >/dev/null || fail "cosign is not installed"
	gh release download "$tag" -R "$REPO" -p "$sums.sigstore.json"
	# issued to release.yaml of this repository, run from main or a v* tag
	cosign verify-blob --bundle "$sums.sigstore.json" \
		--certificate-oidc-issuer https://token.actions.githubusercontent.com \
		--certificate-identity-regexp "^https://github\.com/$REPO/\.github/workflows/release\.yaml@refs/(heads/main|tags/v[0-9][^/]*)\$" \
		"$sums" >/dev/null || fail "the signature of $sums does not verify"
	ok "signature of $sums"
elif $strict; then
	fail "$sums.sigstore.json is not a release asset"
else
	echo "note: $tag is not signed"
fi

# provenance exists from this version on (release-provenance D4)
FIRST_ATTESTED=0.14.0
# at_least A B: version A >= B (X.Y.Z, prerelease suffix ignored)
at_least() {
	printf '%s\n%s\n' "${1%%-*}" "$2" | awk -F. '
		NR == 1 { a1 = $1; a2 = $2; a3 = $3 }
		NR == 2 { b1 = $1; b2 = $2; b3 = $3 }
		END {
			if (a1 != b1) exit !(a1 > b1)
			if (a2 != b2) exit !(a2 > b2)
			exit !(a3 >= b3)
		}'
}
# the same identities as the signature above: release.yaml run from main
# (schedule, workflow_dispatch) or from this tag (the tag-push run); gh
# matches --cert-identity exactly, so each is tried
attested=false
for ref in refs/heads/main "refs/tags/$tag"; do
	if gh attestation verify "$archive" -R "$REPO" \
		--cert-identity "https://github.com/$REPO/.github/workflows/release.yaml@$ref" \
		--deny-self-hosted-runners >/dev/null 2>attest.err; then
		attested=true
		break
	fi
done
if $attested; then
	ok "provenance of $archive"
elif ! at_least "$v" "$FIRST_ATTESTED"; then
	echo "note: $tag predates build provenance ($FIRST_ATTESTED)"
elif $strict; then
	cat attest.err >&2
	fail "$archive has no build provenance from release.yaml"
else
	echo "note: $archive has no build provenance from release.yaml ($(sed -n '/./{p;q;}' attest.err))"
fi

if command -v sha256sum >/dev/null; then
	sha256sum --check --ignore-missing --quiet "$sums" || fail "$archive does not match $sums"
else
	shasum -a 256 --check --ignore-missing --quiet "$sums" || fail "$archive does not match $sums"
fi
tar xzf "$archive" kube-node-viewer
got=$(./kube-node-viewer -version | head -n 1)
[ "$got" = "kube-node-viewer version $v" ] || fail "$archive reports \"$got\""
ok "$archive: checksum and \"$got\""

if $cask; then
	gh api "repos/$TAP/contents/Casks/kube-node-viewer.rb" \
		-H 'Accept: application/vnd.github.raw' > cask.rb || fail "cannot read the cask"
	grep -qE "^[[:space:]]*version \"$v\"\$" cask.rb || fail "the cask is not at $v"
	n=0
	sed -nE 's/^[[:space:]]*sha256 "([0-9a-f]{64})"$/\1/p' cask.rb > cask-sums.txt
	while read -r s; do
		awk -v s="$s" '$1 == s { found = 1 } END { exit !found }' "$sums" ||
			fail "cask sha256 $s is not in $sums"
		n=$((n + 1))
	done < cask-sums.txt
	[ "$n" -gt 0 ] || fail "no sha256 in the cask"
	ok "cask at $v, $n sha256 values match"
fi
