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
# Real-environment identifiers must not reach this public repository.
#   scripts/checks/content.sh            every tracked file (verify.sh, CI)
#   scripts/checks/content.sh --staged   lines a commit adds (pre-commit)
# Flags:
# - real EKS API endpoints (<32 hex>.<xx>.<region>.eks.amazonaws.com);
#   use ABCDEF0123456789ABCDEF0123456789 in tests and docs;
# - real AKS API hosts (<name>.hcp.<region>.azmk8s.io); use example-… names;
# - anything in the private deny list: one extended regex per line
#   (case-insensitive) in .git/info/denylist, or in $KNV_DENYLIST (CI
#   secret, lines separated by newlines). The list itself is never committed.
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" || exit 0

eks='[0-9A-Fa-f]{32}\.[a-z0-9]{3}\.[a-z0-9-]+\.eks\.amazonaws\.com'
eks_ok='(ABCDEF0123456789ABCDEF0123456789|0{32})\.'
aks='[a-z0-9-]+\.hcp\.[a-z0-9]+\.azmk8s\.io'
aks_ok='(^|[^a-z0-9-])example[a-z0-9-]*\.hcp\.'

deny=""
f="$(git rev-parse --git-common-dir 2>/dev/null)/info/denylist"
[ -f "$f" ] && deny=$(grep -v '^[[:space:]]*\(#\|$\)' "$f")
[ -n "${KNV_DENYLIST:-}" ] && deny=$(printf '%s\n%s' "$deny" "$KNV_DENYLIST" | grep -v '^$')

if [ "${1:-}" = --staged ]; then
  # file:line text of added lines
  text=$(git diff --cached -U0 --diff-filter=ACMR -- . ':!scripts/checks/content.sh' | awk '
    /^\+\+\+ b\// { file = substr($0, 7); next }
    /^@@/ { split($3, a, ","); n = substr(a[1], 2) + 0; next }
    /^\+/ { print file ":" n ": " substr($0, 2); n++ }')
else
  text=$(git grep -n -I -E -i "$eks|$aks${deny:+|$(printf '%s' "$deny" | paste -sd'|' -)}" -- . ':!scripts/checks/content.sh' 2>/dev/null)
fi

hits=$(printf '%s\n' "$text" | grep -E "$eks" | grep -vE "$eks_ok")
hits="$hits
$(printf '%s\n' "$text" | grep -E "$aks" | grep -vE "$aks_ok")"
if [ -n "$deny" ]; then
  hits="$hits
$(printf '%s\n' "$text" | grep -i -E "$(printf '%s' "$deny" | paste -sd'|' -)")"
fi
hits=$(printf '%s\n' "$hits" | grep -v '^$' | sort -u)
[ -z "$hits" ] && exit 0
echo "real-environment identifiers (cluster endpoints or private deny list); replace them with example values:"
printf '%s\n' "$hits" | cut -c1-160 | sed 's/^/  /'
exit 1
