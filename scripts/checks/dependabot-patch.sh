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
# Is a Dependabot PR a patch-only update, judged from what it changes rather
# than from the dependencies Dependabot lists? `go get` also raises indirect
# requirements and can add modules, which Dependabot does not list.
#   scripts/checks/dependabot-patch.sh BASE_GO_MOD HEAD_GO_MOD [CHANGED_FILE...]
# Exit 0 (patch only) when:
# - the PR changes go.mod and go.sum only (when CHANGED_FILEs are given);
# - no module is added to or removed from the require lists;
# - every module whose version changes keeps vMAJOR.MINOR and raises PATCH,
#   both versions plain releases (no pre-release, no pseudo-version);
# - everything else in go.mod (go, toolchain, replace, exclude…) is equal.
# Otherwise exit 1 and print why. Used by the automerge job in test.yaml.
[ $# -ge 2 ] || { echo "usage: $0 BASE_GO_MOD HEAD_GO_MOD [CHANGED_FILE...]" >&2; exit 2; }
base=$1 head=$2
shift 2
for f in "$base" "$head"; do [ -r "$f" ] || { echo "cannot read $f" >&2; exit 2; }; done

for f in "$@"; do
  case "$f" in go.mod | go.sum) ;; *) echo "changes $f (only go.mod and go.sum are allowed)"; exit 1 ;; esac
done

# requires FILE: "module version" for each require entry, sorted
requires() {
  awk '
    /^require[ \t]*\(/ { inblock = 1; next }
    inblock && /^\)/ { inblock = 0; next }
    inblock && NF >= 2 && $1 !~ /^\/\// { print $1, $2; next }
    /^require[ \t]+[^( \t]/ { print $2, $3 }' "$1" | sort
}
# rest FILE: go.mod without its require entries
rest() {
  awk '
    /^require[ \t]*\(/ { inblock = 1; next }
    inblock && /^\)/ { inblock = 0; next }
    inblock { next }
    /^require[ \t]+[^( \t]/ { next }
    { print }' "$1"
}

if [ "$(rest "$base")" != "$(rest "$head")" ]; then
  echo "changes go.mod beyond require versions (go, toolchain, replace, exclude…)"
  exit 1
fi

requires "$base" > "${TMPDIR:-/tmp}/dp-base.$$"
requires "$head" > "${TMPDIR:-/tmp}/dp-head.$$"
trap 'rm -f "${TMPDIR:-/tmp}/dp-base.$$" "${TMPDIR:-/tmp}/dp-head.$$"' EXIT
if [ "$(cut -d' ' -f1 "${TMPDIR:-/tmp}/dp-base.$$")" != "$(cut -d' ' -f1 "${TMPDIR:-/tmp}/dp-head.$$")" ]; then
  echo "adds or removes a module:"
  diff "${TMPDIR:-/tmp}/dp-base.$$" "${TMPDIR:-/tmp}/dp-head.$$" | grep '^[<>]' | sed 's/^/  /'
  exit 1
fi

why=$(join "${TMPDIR:-/tmp}/dp-base.$$" "${TMPDIR:-/tmp}/dp-head.$$" | awk '
  $2 == $3 { next }
  {
    changed++
    if ($2 !~ /^v[0-9]+\.[0-9]+\.[0-9]+$/ || $3 !~ /^v[0-9]+\.[0-9]+\.[0-9]+$/) { print $1 " " $2 " -> " $3 ": not a plain release"; next }
    split(substr($2, 2), a, "."); split(substr($3, 2), b, ".")
    if (a[1] != b[1] || a[2] != b[2]) { print $1 " " $2 " -> " $3 ": not a patch update"; next }
    if (b[3] + 0 <= a[3] + 0) { print $1 " " $2 " -> " $3 ": not an upgrade" }
  }
  END { if (!changed) print "changes no module version" }')
if [ -n "$why" ]; then
  printf '%s\n' "$why"
  exit 1
fi
echo "patch updates only"
