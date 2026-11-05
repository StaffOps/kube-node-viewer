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
# Checks on what a commit is about to record. Used by .githooks/pre-commit
# and by the agent commit guard (scripts/agent-hooks/commit-guard.sh).
#   scripts/checks/staged.sh           staged changes (git commit)
#   scripts/checks/staged.sh --all     tracked changes too (git commit -a)
# Prints one problem per line and exits 1 when any is found.
#
# - export files from the TUI (kube-node-viewer-*.csv / *.json): cluster data
# - Python bytecode (__pycache__/, *.pyc): it records local paths
# - lines added inside a released CHANGELOG section (## [x.y.z])
# - Go files not gofmt -s clean
# - new .go/.sh/.py files without the Apache license header
# - real-environment data: endpoints, private IPs, account ids, internal
#   domains, e-mails, credentials, deny-listed names (scripts/checks/content.sh)
# - secrets (gitleaks, when installed; CI always runs it)
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" || exit 0
diffargs="--cached"
[ "${1:-}" = --all ] && diffargs="HEAD"
problems=""
add() { problems="$problems$1
"; }

# shellcheck disable=SC2086 # $diffargs is one or two words on purpose
files=$(git diff $diffargs --name-only --diff-filter=ACMR 2>/dev/null)

for f in $(printf '%s\n' "$files" | grep -E '(^|/)kube-node-viewer-[^/]*\.(csv|json)$'); do
  add "export file with cluster data: $f (exports are git-ignored; remove it with git rm --cached)"
done

for f in $(printf '%s\n' "$files" | grep -E '(^|/)__pycache__/|\.py[co]$'); do
  add "Python bytecode: $f (it records the local path of the source; git-ignored; remove it with git rm --cached)"
done

if printf '%s\n' "$files" | grep -qx CHANGELOG.md; then
  if [ "$diffargs" = HEAD ]; then src=$(cat CHANGELOG.md); else src=$(git show :CHANGELOG.md 2>/dev/null); fi
  first=$(printf '%s\n' "$src" | grep -nE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' | head -1 | cut -d: -f1)
  if [ -n "$first" ]; then
    # shellcheck disable=SC2086
    late=$(git diff $diffargs -U0 -- CHANGELOG.md | awk -v first="$first" '
      /^@@/ { split($3, a, ","); n = substr(a[1], 2) + 0; next }
      /^\+\+\+/ { next }
      /^\+/ { if (n > first && $0 !~ /^\+## \[/ && $0 !~ /^\+\[[0-9.]+\]: / && $0 !~ /^\+\[Unreleased\]: / && $0 !~ /^\+> \*\*Corrected/ && $0 != "+") print n; n++; next }
    ' | head -1)
    [ -n "$late" ] && add "CHANGELOG.md line $late is inside a released section; new entries go under ## [Unreleased]"
  fi
fi

gofiles=$(printf '%s\n' "$files" | grep -E '\.go$' | grep -vE '(^|/)zz_generated' || true)
if [ -n "$gofiles" ] && command -v gofmt >/dev/null 2>&1; then
  for f in $gofiles; do
    [ -f "$f" ] || continue
    if [ "$diffargs" = HEAD ]; then out=$(gofmt -s -l "$f"); else out=$(git show ":$f" | gofmt -s -l 2>/dev/null | sed "s#<standard input>#$f#"); fi
    [ -n "$out" ] && add "not gofmt -s clean: $f (run gofmt -s -w $f)"
  done
fi

# shellcheck disable=SC2086
for f in $(git diff $diffargs --name-only --diff-filter=A 2>/dev/null | grep -E '\.(go|sh|py)$' | grep -vE '^(\.githooks|hack)/|(^|/)zz_generated'); do
  if [ "$diffargs" = HEAD ]; then body=$(head -20 "$f" 2>/dev/null); else body=$(git show ":$f" 2>/dev/null | head -20); fi
  printf '%s\n' "$body" | grep -q 'Licensed under the Apache License' || add "new file without the Apache license header: $f (make boilerplate adds it to Go files)"
done

if [ "$diffargs" = --cached ] && [ -f scripts/checks/content.sh ]; then
  c=$(sh scripts/checks/content.sh --staged) || add "$c"
fi

# Secrets: gitleaks when installed (brew install gitleaks); CI always runs it.
if command -v gitleaks >/dev/null 2>&1; then
  for scope in --staged $( [ "$diffargs" = HEAD ] && echo unstaged ); do
    [ "$scope" = unstaged ] && scope=""
    # shellcheck disable=SC2086 # $scope is empty or one flag
    g=$(gitleaks git --pre-commit $scope -v --redact --no-banner --log-level error --exit-code 1 2>&1) ||
      add "secrets found by gitleaks (rotate them: they may be in your shell history or another commit):
$(printf '%s\n' "$g" | grep -E '^(Finding|RuleID|File|Line):' | sed 's/^/  /')"
  done
fi

[ -z "$problems" ] && exit 0
printf '%s' "$problems"
exit 1
