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
# Nothing from a real environment may reach this public repository.
#   scripts/checks/content.sh                  every tracked file (verify.sh, CI)
#   scripts/checks/content.sh --staged         lines a commit adds (pre-commit)
#   scripts/checks/content.sh --message FILE   a commit message (commit-msg)
#   scripts/checks/content.sh --commits RANGE  every commit in RANGE: the lines
#                                              it adds, its message, author and
#                                              committer (CI, make check)
# Exit 0: clean; 1: findings; 2: the check could not run (bad deny-list
# regex, unknown range) — treated as a failure everywhere.
#
# Rules (what passes instead is in parentheses):
#   eks        EKS API endpoints and OIDC issuers
#              (ABCDEF0123456789ABCDEF0123456789.gr7…, …/id/ABCDEF…)
#   aks        AKS API hosts (hcp, privatelink, portal), OIDC issuers, node
#              resource groups (MC_…) and node names (aks-<pool>-<8 digits>-vmss…)
#              (example-…, cluster-abc.privatelink…, MC_example…, the
#              zero GUID, aks-<pool>-12345678-vmss…)
#   server     API server URLs: `server: https://…` and https://…:6443/8443/16443
#              (localhost, 127.0.0.1, kind.local, kubernetes.default.svc,
#              kubernetes.docker.internal, a host named example…/….example;
#              EKS/AKS hosts are the rules above)
#   gke        GKE kubeconfig context names gke_<project>_<zone>_<cluster>,
#              with a legacy domain:project too, and connectgateway_ ones
#              (gke_example…, gke_example.com:…, connectgateway_example…)
#   ip         private IPv4 addresses and ip-a-b-c-d node names (the example
#              values already in the repo, listed in ip_ok; for new ones use
#              the documentation ranges 192.0.2.0/24, 198.51.100.0/24,
#              203.0.113.0/24, which are not private, and node names from
#              them, ip-192-0-2-38…)
#   instance   EC2 instance ids (i-0123456789abcdef0, i-1234567890abcdef0)
#   account    AWS account ids in ARNs, ECR hosts or "account id"
#              (111122223333, 123456789012, 000000000000)
#   azure      Azure subscription ids (00000000-0000-0000-0000-000000000000)
#   domain     internal host names: *.corp, *.intra(net), *.lan, *.internal,
#              a.b.local (ec2/compute/docker.internal, cluster.local pass)
#   email      e-mail addresses in files (example.com/org/net, GitHub noreply,
#              git@github.com); not checked in commit messages
#   credential kubeconfig credentials, passwords and bearer tokens
#   deny       the private deny list: company, customer, account and cluster
#              names. One extended regex per line (case-insensitive) in
#              .git/info/denylist, or in $KNV_DENYLIST (CI secret, lines
#              separated by newlines). The list itself is never committed.
#              Also matched against author and committer names and e-mails.
# Known gaps: public IPs and bare 12-digit numbers are not flagged (too many
# false positives); names nobody put in the deny list pass; --commits does
# not diff merge commits (lines added while resolving a conflict are only
# seen by the full-tree check).
# A line that must keep such a value says `content-check: allow` (reviewed in
# the PR like any other line). Under GitHub Actions only the place and the
# rule are printed, never the text, so the deny list does not leak into a
# public log.
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" || exit 0

# Files never scanned: this script and its test (they hold the patterns),
# third-party notices (upstream authors' e-mails), generated price tables.
skip='^(scripts/checks/content\.sh|scripts/checks/test-checks\.sh|([^:@]*/)?ATTRIBUTION\.md|NOTICE|LICENSE|go\.sum|pkg/aws/zz_generated[^:@]*)(@[0-9a-f]+)?:'

# Patterns are matched case-insensitively unless marked; allowed values are
# written in lower case (they are removed from a lower-cased copy of the line).
eks='[0-9a-f]{32}\.[a-z0-9]{3}\.[a-z0-9-]+\.eks\.amazonaws\.com|oidc\.eks\.[a-z0-9-]+\.amazonaws\.com/id/[0-9a-f]{32}'
eks_ok='abcdef0123456789abcdef0123456789|0{32}'
aks='[a-z0-9-]+\.(hcp|portal\.hcp|privatelink)\.[a-z0-9]+\.azmk8s\.io|\.oic\.prod-aks\.azure\.com/[0-9a-f-]{36}|(^|[^a-z0-9])mc_[a-z0-9.-]+_[a-z0-9._-]+_[a-z0-9]+|aks-[a-z0-9]+-[0-9]{8}-vmss'
aks_ok='(^|[^a-z0-9-])(example[a-z0-9-]*|cluster-abc)\.(hcp|portal|privatelink)\.|0{8}-0{4}-0{4}-0{4}-0{12}|(^|[^a-z0-9])mc_example[a-z0-9._-]*|aks-[a-z0-9]+-12345678-vmss'
server='server["'"'"']?[[:space:]]*[:=][[:space:]]*["'"'"']?https?://[a-z0-9.-]+|https?://[a-z0-9.-]+:(6443|8443|16443)([^0-9]|$)'
server_ok='https?://(localhost|127\.0\.0\.1|0\.0\.0\.0|kind\.local|kubernetes\.default\.svc|kubernetes\.docker\.internal|([a-z0-9-]+\.)*example(\.(com|org|net))?|example[a-z0-9-]*(\.[a-z0-9-]+)*|[a-z0-9.-]+\.(eks\.amazonaws\.com|azmk8s\.io))(:[0-9]+)?([^a-z0-9.-]|$)'
gke='(^|[^a-z0-9])(gke|connectgateway)_[a-z0-9.:-]+_[a-z0-9-]+_[a-z0-9-]+'
gke_ok='(^|[^a-z0-9])(gke|connectgateway)_example(\.com:[a-z0-9-]+)?[a-z0-9_-]*([^a-z0-9_.:-]|[.:][a-z0-9.:-]*([^a-z0-9_.:-]|$)|$)'
o='[0-9]{1,3}'
ip="(^|[^0-9.])(10\.$o\.$o\.$o|172\.(1[6-9]|2[0-9]|3[01])\.$o\.$o|192\.168\.$o\.$o)([^0-9.]|\.[^0-9]|\.?$)|ip-(10-$o|172-(1[6-9]|2[0-9]|3[01])|192-168)-$o-$o([^0-9-]|$)"
ip_ok='(^|[^0-9.])10\.0\.(0\.1|1\.5|1\.9|2\.7|3\.1)([^0-9.]|\.[^0-9]|\.?$)|ip-10-0-(0-1|1-12|1-23|1-3|1-5|11-70|12-15|12-33|12-61|12-7|133-127|2-40|2-7|3-9)([^0-9-]|$)'
instance='(^|[^a-z0-9-])i-[0-9a-f]{17}([^a-z0-9]|$)'
instance_ok='i-(0123456789abcdef0|1234567890abcdef0)'
account='arn:aws[a-z-]*:[a-z0-9-]*:[a-z0-9-]*:[0-9]{12}|(^|[^0-9])[0-9]{12}\.dkr\.ecr\.|account[ _-]?(id)?["'"'"']?[[:space:]]*[:=][[:space:]]*["'"'"']?[0-9]{12}'
account_ok='111122223333|123456789012|000000000000'
azure='subscriptions/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'
azure_ok='subscriptions/0{8}-0{4}-0{4}-0{4}-0{12}'
# case-sensitive: host names are written in lower case, and Go code such as
# addr.Internal() must not match
domain='[a-z0-9-]+\.(corp|intra|intranet|lan|internal)([^A-Za-z0-9(-]|$)|[a-z0-9-]+\.[a-z0-9-]+\.local([^A-Za-z0-9(-]|$)'
domain_ok='(ec2|compute|docker)\.internal|cluster\.local'
email='[a-z0-9._%+-]+@[a-z0-9-]+(\.[a-z0-9-]+)*\.[a-z]{2,}'
email_ok='@(example\.(com|org|net)|([a-z0-9-]+\.)?noreply\.github\.com)|git@github\.com'
credential='(client-key-data|client-certificate-data|certificate-authority-data|id-token|refresh-token|token|password|passwd|secret)["'"'"']?[[:space:]]*[:=][[:space:]]*["'"'"']?[a-z0-9+/=_-]{24,}|bearer[[:space:]]+[a-z0-9._~+/-]{20,}'

# Deny list: every line must be a valid regex, or the whole check would
# silently match nothing.
deny=""
bad=""
add_deny() { # SOURCE, lines on stdin
  n=0
  while IFS= read -r l; do
    n=$((n + 1))
    printf '%s\n' "$l" | grep -q '^[[:space:]]*\(#\|$\)' && continue
    printf x | grep -E -- "$l" >/dev/null 2>&1
    if [ $? -eq 2 ]; then bad="$bad  $1 line $n is not a valid extended regex
"; continue; fi
    if printf '\n' | grep -qiE -- "$l"; then bad="$bad  $1 line $n matches an empty line (empty alternative?)
"; continue; fi
    deny="$deny${deny:+|}$l"
  done
}
f="$(git rev-parse --git-common-dir 2>/dev/null)/info/denylist"
# here-documents, not pipes: add_deny must run in this shell to set $deny.
# tr: a list saved with CRLF line ends would otherwise match nothing.
if [ -f "$f" ]; then add_deny .git/info/denylist <<EOF
$(tr -d '\r' < "$f")
EOF
fi
if [ -n "${KNV_DENYLIST:-}" ]; then add_deny KNV_DENYLIST <<EOF
$(printf '%s\n' "$KNV_DENYLIST" | tr -d '\r')
EOF
fi
if [ -n "$bad" ]; then
  printf 'content check cannot run: fix the deny list\n%s' "$bad" >&2
  exit 2
fi
if [ -z "$deny" ] && [ -n "${GITHUB_ACTIONS:-}" ]; then
  echo "::notice::content check: no deny list (KNV_DENYLIST is not available to this run); company names are not checked"
fi

# diff_lines PREFIX_FORMAT: "file:line: text" for each added line of a
# unified diff on stdin; with --commits, "file@sha:line: text".
diff_lines() {
  awk '
    /^\001/ { c = "@" substr($0, 2); next }
    /^diff --git / { h = 1; next }
    h && /^\+\+\+ / { f = substr($0, 5); sub(/^"/, "", f); sub(/"$/, "", f); sub(/^b\//, "", f); next }
    /^@@/ { h = 0; split($3, a, ","); n = substr(a[1], 2) + 0; next }
    h { next }
    /^\+/ { print f c ":" n ": " substr($0, 2); n++ }'
}

mode=${1:-all}
idents=""
case "$mode" in
  --staged)
    text=$(git -c core.quotePath=false diff --cached -U0 --no-color --no-ext-diff --diff-filter=ACMR | diff_lines)
    idents="author: $(git var GIT_AUTHOR_IDENT 2>/dev/null | sed 's/ [0-9]* [-+][0-9]*$//')
committer: $(git var GIT_COMMITTER_IDENT 2>/dev/null | sed 's/ [0-9]* [-+][0-9]*$//')" ;;
  --message)
    [ -f "${2:-}" ] || { echo "usage: $0 --message FILE" >&2; exit 2; }
    # stop at the scissors line of `git commit -v` (the diff below it is
    # checked as the commit's content, not as its message)
    text=$(sed '/^# -\{24\} >8 -\{24\}$/,$d' "$2" | grep -v '^#' | awk '{ print "commit message:" NR ": " $0 }') ;;
  --commits)
    [ -n "${2:-}" ] || { echo "usage: $0 --commits RANGE" >&2; exit 2; }
    git rev-list "$2" -- >/dev/null 2>&1 || { echo "content check: unknown commit range: $2" >&2; exit 2; }
    text=$(git log --format='%x01%h%n%B' "$2" | awk '
      /^\001/ { c = substr($0, 2); n = 0; next }
      { n++; print "commit " c " message:" n ": " $0 }' | grep -v '^commit [0-9a-f]* message:[0-9]*: $')
    diffs=$(git -c core.quotePath=false log -p -U0 --no-color --no-ext-diff --no-renames --diff-filter=ACMR --format='%x01%h' "$2" | diff_lines)
    text="$text
$diffs"
    idents=$(git log --format='commit %h: author: %an <%ae>%ncommit %h: committer: %cn <%ce>' "$2") ;;
  all)
    text=$(git -c core.quotePath=false grep -n -I -E -i "$eks|$aks|$server|$gke|$ip|$instance|$account|$azure|$domain|$email|$credential${deny:+|$deny}" -- . 2>/dev/null)
    rc=$?
    [ "$rc" -gt 1 ] && { echo "content check: git grep failed (rc=$rc)" >&2; exit 2; } ;;
  *) echo "usage: $0 [--staged | --message FILE | --commits RANGE]" >&2; exit 2 ;;
esac
text=$(printf '%s\n' "$text" | grep -Ev "$skip" | grep -v 'content-check: allow')
scan=$text # what match() reads

# place LINE: "file:line" (or "file@sha:line"); never the text, even when
# the path itself holds a colon
place() {
  p=$(printf '%s\n' "$1" | sed -nE 's/^((commit [0-9a-f]+ message|[^:]*):[0-9]+):.*/\1/p')
  printf '%s\n' "${p:-(a path with a colon)}"
}
report() { # RULE LINE
  if [ -n "${GITHUB_ACTIONS:-}" ] || [ "$1" = deny ]; then
    printf '%s  %s\n' "$(place "$2")" "$1"
  else
    printf '%s  %s  %s\n' "$(place "$2")" "$1" "$(printf '%s\n' "$2" | sed -E 's/^(commit [0-9a-f]+ message|[^:]*):[0-9]+: ?//' | cut -c1-120)"
  fi
}

# match RULE PATTERN [ALLOWED] [cs]: lines with PATTERN left once ALLOWED
# values are removed. cs: case-sensitive.
match() {
  i=-i
  [ "${4:-}" = cs ] && i=
  # shellcheck disable=SC2086 # $i is empty or -i
  printf '%s\n' "$scan" | grep -E $i -- "$2" | while IFS= read -r line; do
    rest=$line
    # shellcheck disable=SC2018,SC2019 # ASCII only on purpose: the patterns are ASCII
    [ -z "$i" ] || rest=$(printf '%s\n' "$line" | LC_ALL=C tr 'A-Z' 'a-z')
    # allowed values go one at a time, leaving a space: a pattern may consume
    # the delimiter the next value needs, and joining the text around a value
    # must not hide what follows it
    if [ -n "${3:-}" ]; then
      prev=
      while [ "$rest" != "$prev" ]; do
        prev=$rest
        rest=$(printf '%s\n' "$rest" | sed -E "s#$3# #g")
      done
    fi
    # shellcheck disable=SC2086
    printf '%s\n' "$rest" | grep -Eq $i -- "$2" || continue
    report "$1" "$line"
  done
}

hits=$(
  match eks "$eks" "$eks_ok"
  match aks "$aks" "$aks_ok"
  match server "$server" "$server_ok"
  match gke "$gke" "$gke_ok"
  match ip "$ip" "$ip_ok"
  match instance "$instance" "$instance_ok"
  match account "$account" "$account_ok"
  match azure "$azure" "$azure_ok"
  match domain "$domain" "$domain_ok" cs
  # e-mails only in content: commit trailers (Signed-off-by, Co-authored-by,
  # cherry-picked upstream commits) carry them by design
  scan=$(printf '%s\n' "$text" | grep -Ev '^commit ([0-9a-f]+ )?message:')
  match email "$email" "$email_ok"
  scan=$text
  match credential "$credential"
  if [ -n "$deny" ]; then
    printf '%s\n' "$text" | grep -i -E -- "$deny" | while IFS= read -r line; do report deny "$line"; done
    [ -n "$idents" ] && printf '%s\n' "$idents" | grep -i -E -- "$deny" | sed -E 's/^((commit [0-9a-f]+: )?(author|committer)):.*/\1  deny (name or e-mail)/'
  fi
)
hits=$(printf '%s\n' "$hits" | grep -v '^$' | sort -u)
[ -z "$hits" ] && exit 0
echo "real-environment data (rules in scripts/checks/content.sh); replace it with example values:"
printf '%s\n' "$hits" | sed 's/^/  /'
exit 1
