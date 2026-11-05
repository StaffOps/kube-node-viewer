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
# Tests the guardrails (AGENTS.md "Guardrails") in a throwaway repository:
# the git hooks in .githooks, scripts/checks and the agent hooks. Each case
# makes a commit (or push, or check run) that must be blocked or must pass;
# a blocked case also names the rule that must fire.
#   scripts/checks/test-checks.sh    (CI and make check run it with dash,
#                                     the /bin/sh of Ubuntu)
# Cases needing gofmt, gitleaks, jq or python3 are skipped when missing.
set -u
# git hooks run with GIT_DIR (and in a worktree GIT_COMMON_DIR…) set to the
# repository being pushed; left set, every git command below, `git init`
# included, would act on that repository instead of the throwaway ones
# (fix-hook-tests-git-env). Drop every variable git itself treats as
# repository-local, plus the ones that redirect identity, config and
# templates, so the tests see only their own repositories.
# shellcheck disable=SC2046 # the names are words, split on purpose
unset $(git rev-parse --local-env-vars) GIT_NAMESPACE GIT_CEILING_DIRECTORIES \
  GIT_DISCOVERY_ACROSS_FILESYSTEM GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL \
  GIT_AUTHOR_DATE GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL GIT_COMMITTER_DATE \
  GIT_TEMPLATE_DIR GIT_CONFIG_KEY_0 GIT_CONFIG_VALUE_0
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
src=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
repo=$tmp/repo
pass=0 fail=0 skip=0

git init -q -b main "$repo"
cd "$repo" || exit 1
mkdir -p scripts/agent-hooks
cp -R "$src/scripts/checks" scripts/
cp "$src/scripts/agent-hooks/commit-guard.sh" "$src/scripts/agent-hooks/pr-guard.sh" scripts/agent-hooks/
cp -R "$src/.githooks" .
[ -f "$src/.gitleaks.toml" ] && cp "$src/.gitleaks.toml" .
git config user.name "Example Dev"
git config user.email "dev@example.com"
git config core.hooksPath .githooks
git config commit.gpgsign false
# a made-up company name stands in for the private deny list
printf '# test deny list\nacme[ _-]?corp\n' > .git/info/denylist
printf '# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n### Added\n\n- First.\n' > CHANGELOG.md
git add -A
git commit -q --no-verify -m "chore: base"
base=$(git rev-parse HEAD)
unset KNV_DENYLIST GITHUB_ACTIONS

reset() { git reset -q --hard "$base"; git clean -qfdx -e .git; }
ok()   { pass=$((pass + 1)); printf 'ok    %s\n' "$1"; }
bad()  { fail=$((fail + 1)); printf 'FAIL  %s\n' "$1"; printf '%s\n' "$2" | sed 's/^/      /'; }
skipped() { skip=$((skip + 1)); printf 'skip  %s (%s missing)\n' "$1" "$2"; }

# verdict NAME WANT RC OUTPUT. WANT: passes | blocked | blocked:RULE (the
# output must name RULE; a rule is printed as "  RULE" after the place).
verdict() {
  if [ "$3" -eq 0 ]; then got=passes; else got=blocked; fi
  case "$2" in
    passes) [ "$got" = passes ] && { ok "$1"; return; } ;;
    blocked) [ "$got" = blocked ] && { ok "$1"; return; } ;;
    blocked:*)
      rule=${2#blocked:}
      if [ "$got" = blocked ] && printf '%s\n' "$4" | grep -q -- "  $rule\( \|$\)"; then ok "$1"; return; fi ;;
  esac
  bad "$1: $got, want $2" "$4"
}

# commit NAME WANT FILE CONTENT [MESSAGE [git -c options...]]
commit() {
  name=$1 want=$2 file=$3 content=$4 msg=${5:-"chore: test"}
  if [ $# -ge 5 ]; then shift 5; else shift $#; fi
  reset
  mkdir -p "$(dirname "$file")"
  printf '%s\n' "$content" > "$file"
  git add -A
  out=$(git "$@" commit -q -m "$msg" 2>&1)
  verdict "$name" "$want" $? "$out"
}

# tree NAME WANT FILE CONTENT [env...]: full-tree mode (verify.sh, CI) on a
# committed file
tree() {
  name=$1 want=$2 file=$3 content=$4
  shift 4
  reset
  mkdir -p "$(dirname "$file")"
  printf '%s\n' "$content" > "$file"
  git add -A && git commit -q --no-verify -m "chore: t"
  out=$(env "$@" sh scripts/checks/content.sh 2>&1)
  verdict "$name" "$want" $? "$out"
}

hdr='# Licensed under the Apache License, Version 2.0 (the "License");'

echo "== files a commit adds (pre-commit)"
commit "export file"                 blocked kube-node-viewer-20260101.csv "node,cpu"
commit "python bytecode"              blocked scripts/__pycache__/x.cpython-312.pyc "bytecode"
commit "license header on new .sh"   blocked tool.sh "echo hi"
commit "new .sh with header"         passes  tool.sh "$hdr"
commit "EKS endpoint"                blocked:eks docs.md "https://0123456789ABCDEF0123456789ABCDEF.gr7.us-east-1.eks.amazonaws.com"
commit "EKS endpoint, upper case"    blocked:eks docs.md "0123456789ABCDEF0123456789ABCDEF.GR7.US-EAST-1.EKS.AMAZONAWS.COM"
commit "EKS example endpoint"        passes  docs.md "https://ABCDEF0123456789ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com"
commit "EKS OIDC issuer"             blocked:eks docs.md "https://oidc.eks.us-east-1.amazonaws.com/id/0123456789ABCDEF0123456789ABCDEF"
commit "EKS example OIDC issuer"     passes  docs.md "https://oidc.eks.us-east-1.amazonaws.com/id/ABCDEF0123456789ABCDEF0123456789"
commit "AKS host"                    blocked:aks docs.md "prod-dns-1a2b.hcp.eastus.azmk8s.io"
commit "AKS host, upper case"        blocked:aks docs.md "PROD-DNS-1A2B.HCP.EASTUS.AZMK8S.IO"
commit "AKS private host"            blocked:aks docs.md "prod-1a2b.privatelink.eastus.azmk8s.io"
commit "AKS example host"            passes  docs.md "example-dns-1a2b.hcp.eastus.azmk8s.io"
commit "AKS OIDC issuer"             blocked:aks docs.md "https://eastus.oic.prod-aks.azure.com/11111111-2222-3333-4444-555555555555/66666666-7777-8888-9999-000000000000/"
commit "AKS node resource group"     blocked:aks docs.md "MC_prod-rg_prod-aks_eastus"
commit "AKS example resource group"  passes  docs.md "MC_example-rg_example-aks_eastus"
commit "AKS node name"               blocked:aks docs.md "aks-system-37104428-vmss000002"
commit "AKS example node name"       passes  docs.md "aks-userpool-12345678-vmss000003"
commit "Azure subscription"          blocked:azure docs.md "azure:///subscriptions/11111111-2222-3333-4444-555555555555/resourceGroups/mc_x"
commit "Azure example subscription"  passes  docs.md "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/x"
commit "API server URL"              blocked:server kubeconfig "    server: https://k8s.prod.acme.dev"
commit "API server port 6443"        blocked:server docs.md "curl https://rancher.prod.io:6443/version"
commit "example API servers"         passes  kubeconfig "$(printf '    server: https://kind.local:6443\n    server: https://127.0.0.1:1\n    server: https://a.example\n    server: https://kubernetes.default.svc\n    server: https://kubernetes.docker.internal:6443')"
commit "API server, example substring" blocked:server kubeconfig "    server: https://prod-api.notexample.acme.io"
commit "API server, localhost prefix"  blocked:server kubeconfig "    server: https://localhost.acme.io:6443"
commit "GKE context"                 blocked:gke docs.md "gke_acme-prod-123_us-central1_main"
commit "GKE example context"         passes  docs.md "gke_example-project_us-central1_demo"
commit "private IP"                  blocked:ip docs.md "node at 10.20.30.40"
commit "private IP 172.16/12"        blocked:ip docs.md "pod 172.31.5.6 on it"
commit "private IP 192.168/16"       blocked:ip docs.md "192.168.1.10"
commit "private IP in 10.0/16"       blocked:ip docs.md "node 10.0.12.34"
commit "example IPs already in use"  passes  docs.md "10.0.1.5, 10.0.2.7 and 10.0.3.1."
commit "documentation range"         passes  docs.md "node at 192.0.2.10 and 203.0.113.7"
commit "version number not an IP"    passes  docs.md "v1.10.20.30.40"
commit "node name"                   blocked:ip docs.md "ip-172-31-5-6.ec2.internal"
commit "node name in 10.0/16"        blocked:ip docs.md "ip-10-0-77-8.ec2.internal"
commit "example node name"           passes  docs.md "ip-10-0-1-23.us-west-2.compute.internal"
commit "EC2 instance id"             blocked:instance docs.md "aws:///us-east-1a/i-0a1b2c3d4e5f60718"
commit "example instance id"         passes  docs.md "aws:///us-east-1a/i-0123456789abcdef0"
commit "account id in ARN"           blocked:account docs.md "arn:aws:eks:us-east-1:987654321098:cluster/prod"
commit "account id in ECR host"      blocked:account docs.md "987654321098.dkr.ecr.us-east-1.amazonaws.com/app"
commit "account id field"            blocked:account cfg.yaml "accountId: \"987654321098\""
commit "example account id"          passes  docs.md "arn:aws:eks:us-east-1:111122223333:cluster/prod"
commit "internal domain"             blocked:domain docs.md "https://grafana.build01.corp/d/x"
commit "internal .lan host"          blocked:domain docs.md "nas.lan"
commit "AD .local domain"            blocked:domain docs.md "dc01.acme-ad.local"
commit "Go method named Internal"    passes  x.md "if addr.Internal() { x := cfg.Lan; }"
commit "allowed internal names"      passes  docs.md "ip-10-0-1-5.ec2.internal svc.cluster.local kind.local ~/.local/state"
commit "e-mail"                      blocked:email docs.md "ask jane.doe@acme.io"
commit "e-mail, upper case"          blocked:email docs.md "JANE@ACME.IO"
commit "example e-mail"              passes  docs.md "dev@example.com, DEV@EXAMPLE.COM or 123+dev@users.noreply.github.com"
commit "kubeconfig token"            blocked:credential kubeconfig "    token: eyJhbGciOiJSUzI1NiIsImtpZCI6IkxYZ2Vxd"
commit "kubeconfig token, upper"     blocked:credential kubeconfig "    Token: abcdefghijklmnopqrstuvwxyz0123"
commit "kubeconfig CA data"          blocked:credential kubeconfig "    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t"
commit "password with ="             blocked:credential app.env "password=SuperSecretValue1234567890abc"
commit "bearer token"                blocked:credential docs.md "Authorization: Bearer abcdefghijklmnopqrstuvwxyz012345"
commit "short test token"            passes  cfg.yaml "  user: {token: t}"
commit "token read from code"        passes  code.txt "token = cfg.BearerTokenFileFromKubeconfig; client_secret = os.environ.get_secret_value"
commit "deny-listed name"            blocked:deny docs.md "runs at ACME Corp"
case "$out" in *ACME*) bad "deny output hides the text" "$out" ;; *) ok "deny output hides the text" ;; esac
commit "deny-listed name, variant"   blocked:deny docs.md "acmecorp-prod"
commit "non-ASCII path"              blocked:ip "café.md" "node 10.20.30.40"
commit "line starting with ++"       blocked:ip tricky.md "$(printf 'x\n++ ATTRIBUTION.md\nnode 10.20.30.40')"
commit "allow pragma"                passes  docs.md "10.20.30.40 <!-- content-check: allow -->"
commit "clean file"                  passes  docs.md "nothing to see"
commit "released CHANGELOG section"  blocked CHANGELOG.md "$(printf '# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n### Added\n\n- First.\n- Late.')"
commit "CHANGELOG [Unreleased]"      passes  CHANGELOG.md "$(printf '# Changelog\n\n## [Unreleased]\n\n### Fixed\n\n- New.\n\n## [0.1.0] - 2026-01-01\n\n### Added\n\n- First.')"
if command -v gofmt >/dev/null 2>&1; then
  commit "Go file not gofmt -s clean" blocked x.go "$(printf '// Licensed under the Apache License\npackage x\nfunc  F() {}')"
else skipped "Go file not gofmt -s clean" gofmt; fi
if command -v gitleaks >/dev/null 2>&1; then
  # built at run time so this file holds no token-shaped string
  tok="ghp_$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 36)"
  commit "secret (gitleaks)"         blocked app.env "GITHUB_TOKEN=$tok"
  case "$out" in *gitleaks*) ok "secret found by gitleaks" ;; *) bad "secret found by gitleaks" "$out" ;; esac
else skipped "secret (gitleaks)" gitleaks; fi

echo "== commit message and identity (commit-msg, pre-commit)"
commit "subject not conventional"    blocked docs.md "x" "update stuff"
commit "AI co-author"                blocked docs.md "x" "$(printf 'docs: x\n\nCo-Authored-By: Claude <noreply@anthropic.com>')"
commit "deny-listed name in message" blocked:deny docs.md "x" "fix: crash seen at acme corp"
commit "private IP in message"       blocked:ip docs.md "x" "fix: node 10.20.30.40 lost its price"
commit "cluster URL in message"      blocked:eks docs.md "x" "fix: 0123456789ABCDEF0123456789ABCDEF.gr7.us-east-1.eks.amazonaws.com"
commit "e-mail in message passes"    passes  docs.md "x" "$(printf 'fix: x\n\nSigned-off-by: Jane <jane@acme.io>')"
commit "deny-listed author e-mail"   blocked:deny docs.md "x" "docs: x" -c user.email=dev@acmecorp.com
commit "deny-listed author name"     blocked:deny docs.md "x" "docs: x" -c "user.name=Dev (ACME Corp)"
# git commit -v: the diff below the scissors line is not the message
reset
printf 'node 10.20.30.40\n' > a.md && git add a.md && git commit -q --no-verify -m "docs: a"
printf 'node 10.0.1.5\n' > a.md && git add a.md
out=$(GIT_EDITOR=true git commit -q -v -e -m "docs: example IP" 2>&1)
verdict "commit -v removing real data" passes $? "$out"

echo "== commits of a branch (CI secrets job, make check: --commits)"
reset
echo x > a.md && git add a.md
git -c user.email=dev@acmecorp.com commit -q --no-verify -m "docs: a"
echo y > b.md && git add b.md
git commit -q --no-verify -m "fix: node 10.20.30.40"
out=$(sh scripts/checks/content.sh --commits "$base..HEAD")
rc=$?
if [ $rc -eq 1 ] && printf '%s\n' "$out" | grep -q 'author  deny' && printf '%s\n' "$out" | grep -q 'message:1  ip'; then
  ok "--commits: author and message"
else bad "--commits: author and message (rc=$rc)" "$out"; fi
out=$(GITHUB_ACTIONS=true sh scripts/checks/content.sh --commits "$base..HEAD")
rc=$?
if [ $rc -eq 1 ] && printf '%s\n' "$out" | grep -q 'author  deny' && printf '%s\n' "$out" | grep -q '  ip$' &&
   ! printf '%s\n' "$out" | grep -qi 'acme\|10\.20'; then
  ok "--commits under CI: rules named, text hidden"
else bad "--commits under CI: rules named, text hidden (rc=$rc)" "$out"; fi
# added in one commit, removed in the next: the branch still carries it
reset
printf 'node 10.20.30.40\n' > a.md && git add a.md && git commit -q --no-verify -m "docs: a"
printf 'node\n' > a.md && git add a.md && git commit -q --no-verify -m "docs: b"
out=$(sh scripts/checks/content.sh --commits "$base..HEAD")
verdict "--commits: data added then removed" blocked:ip $? "$out"
# trailers of our own and of cherry-picked upstream commits carry e-mails
reset
echo x > a.md && git add a.md
git commit -q --no-verify -m "$(printf 'fix: x\n\nSigned-off-by: Jane Roe <jane@upstream-corp.io>\nCo-authored-by: Joe <joe@gmail.com>\n(cherry picked from commit 0123abc)')"
out=$(sh scripts/checks/content.sh --commits "$base..HEAD")
verdict "--commits: e-mail trailers pass" passes $? "$out"
reset
echo 'mail jane@acme.io' > a.md && git add a.md && git commit -q --no-verify -m "docs: a"
out=$(sh scripts/checks/content.sh --commits "$base..HEAD")
verdict "--commits: e-mail in a file" blocked:email $? "$out"
reset
echo x > a.md && git add a.md
git -c "user.name=ACME Corp committer: dev" commit -q --no-verify -m "docs: a"
out=$(GITHUB_ACTIONS=true sh scripts/checks/content.sh --commits "$base..HEAD")
case "$out" in *ACME*) bad "--commits: identity hidden in CI" "$out" ;; *) ok "--commits: identity hidden in CI" ;; esac
out=$(sh scripts/checks/content.sh --commits "deadbeef..HEAD" 2>&1)
rc=$?
if [ $rc -eq 2 ]; then ok "--commits: unknown range fails"; else bad "--commits: unknown range fails (rc=$rc)" "$out"; fi

echo "== every tracked file (verify.sh, CI verify job)"
tree "tree: private IP"              blocked:ip docs.md "node 10.20.30.40"
tree "tree: deny list from env"      blocked:deny docs.md "made at globex" KNV_DENYLIST=globex
tree "tree: clean"                   passes  docs.md "nothing"
tree "tree: path with a colon, CI"   blocked docs/notes:v2.md "runs at ACME Corp, node 10.20.30.40" GITHUB_ACTIONS=true
case "$out" in *ACME*|*10.20*) bad "tree: path with a colon hides the text" "$out" ;; *) ok "tree: path with a colon hides the text" ;; esac
tree "tree: invalid deny regex fails" blocked docs.md "node 10.20.30.40" "KNV_DENYLIST=acme("
case "$out" in *"not a valid"*) ok "tree: invalid deny regex is reported" ;; *) bad "tree: invalid deny regex is reported" "$out" ;; esac
tree "tree: deny line matching all"  blocked docs.md "nothing" "KNV_DENYLIST=acme|"
tree "tree: deny list with CRLF"     blocked:deny docs.md "made at globex" "KNV_DENYLIST=$(printf 'globex\r')"

echo "== Dependabot patch-only judge (automerge)"
gomod() { # FILE x/text-version aws-version [extra line]
  printf 'module example.com/m\n\ngo 1.26.10\n\nrequire (\n\tgolang.org/x/text %s\n\tgithub.com/aws/aws-sdk-go-v2 %s // indirect\n%s)\n' "$2" "$3" "${4:-}" > "$1"
}
judge() { # NAME WANT HEAD_TEXT HEAD_AWS [EXTRA [FILES...]]
  name=$1 want=$2
  gomod "$tmp/base.mod" v0.42.0 v1.30.1
  gomod "$tmp/head.mod" "$3" "$4" "${5:-}"
  if [ $# -ge 5 ]; then shift 5; else shift $#; fi
  out=$(sh scripts/checks/dependabot-patch.sh "$tmp/base.mod" "$tmp/head.mod" "$@" 2>&1)
  verdict "$name" "$want" $? "$out"
}
judge "patch updates, indirect too"   passes  v0.42.1 v1.30.4 "" go.mod go.sum
judge "minor update"                  blocked v0.43.0 v1.30.1 ""
judge "unlisted indirect minor"       blocked v0.42.1 v1.31.0 ""
judge "module added"                  blocked v0.42.1 v1.30.1 "$(printf '\tgithub.com/spf13/cobra v1.9.0\n')"
judge "pseudo-version"                blocked v0.42.1-0.20260101000000-abcdef123456 v1.30.1 ""
judge "downgrade"                     blocked v0.42.0 v1.30.0 ""
judge "other file changed"            blocked v0.42.1 v1.30.1 "" go.mod .github/workflows/test.yaml
judge "no version changed"            blocked v0.42.0 v1.30.1 ""
gomod "$tmp/base.mod" v0.42.0 v1.30.1
gomod "$tmp/head.mod" v0.42.1 v1.30.1
sed 's/^go 1.26.10/go 1.27.0/' "$tmp/head.mod" > "$tmp/head2.mod"
out=$(sh scripts/checks/dependabot-patch.sh "$tmp/base.mod" "$tmp/head2.mod" 2>&1)
verdict "go directive changed" blocked $? "$out"
# an indented ")" still closes the block: the replace after it is compared
printf 'module m\n\nrequire (\n\ta v1.0.0\n  )\n\nreplace a => ./good\n' > "$tmp/base.mod"
printf 'module m\n\nrequire (\n\ta v1.0.1\n  )\n\nreplace a => ./evil\n' > "$tmp/head.mod"
out=$(sh scripts/checks/dependabot-patch.sh "$tmp/base.mod" "$tmp/head.mod" 2>&1)
verdict "replace after an indented )" blocked $? "$out"

echo "== spec gates"
if command -v python3 >/dev/null 2>&1; then
  sg=$tmp/specs-repo
  git init -q -b main "$sg"
  mkdir -p "$sg/scripts" "$sg/specs/demo" "$sg/specs/rough" "$sg/pkg/model" "$sg/cmd/kube-node-viewer"
  cp "$src/scripts/spec-lint.sh" "$src/scripts/spec-trace.py" "$src/scripts/drift-check.py" "$src/scripts/spec-tasks-check.py" "$sg/scripts/"
  # shellcheck disable=SC2016 # backticks are markdown
  printf '# Specs\n\n1. [`demo/`](demo/)\n2. [`rough/`](rough/)\n' > "$sg/specs/README.md"
  printf '# Requirements: demo\n\n> Status: active · Owner: x · Reviewed: 2026-01-01 (spec-refuter)\n\n**R1** WHEN a THEN the system SHALL b.\n\n**R2** WHEN c THEN the system SHALL d.\n\n_Test: manual — docs._\n' > "$sg/specs/demo/requirements.md"
  printf '# Requirements: rough\n\n> Status: draft · Owner: x · Reviewed: —\n\n**R1** WHEN a THEN the system SHALL b.\n' > "$sg/specs/rough/requirements.md"
  for d in demo rough; do
    printf '# Design: %s\n' "$d" > "$sg/specs/$d/design.md"
    printf '# Tasks: %s\n\n- [x] T1. Do it\n  _Covers: R1, R2_\n' "$d" > "$sg/specs/$d/tasks.md"
  done
  printf 'package model\n\n// demo R1: covered\nfunc TestA(t *testing.T) {}\n' > "$sg/pkg/model/a_test.go"
  printf 'package main\n\nfunc f() {\n\tflagSet.BoolVar(&flags.X, "xflag", cfg.getBoolValue("xflag", false), "x")\n}\n' > "$sg/cmd/kube-node-viewer/flag.go"
  printf 'package model\n\nvar helpKeys = [][2]string{\n\t{"q", "quit"},\n}\n' > "$sg/pkg/model/panels.go"
  printf 'package model\n\nfunc k(s string) {\n\tswitch s {\n\tcase "q":\n\t}\n}\n' > "$sg/pkg/model/uimodel.go"
  printf 'package model\n\nconst (\n\tFieldZone Field = "zone"\n)\n' > "$sg/pkg/model/view.go"
  printf '# demo\n\n-xflag, q\n' > "$sg/README.md"
  (
    cd "$sg" || exit 1
    git config user.name "Example Dev"; git config user.email "dev@example.com"; git config commit.gpgsign false
    git add -A && git commit -q -m "chore: base"
  )
  sgbase=$(git -C "$sg" rev-parse HEAD)
  # spec CASE WANT SCRIPT: run SCRIPT with sh in the spec repo ($B is the
  # base commit), then reset it
  spec() {
    out=$(cd "$sg" && B=$sgbase PYTHONDONTWRITEBYTECODE=1 sh -c "$3" 2>&1)
    verdict "$1" "$2" $? "$out"
    git -C "$sg" checkout -q main; git -C "$sg" reset -q --hard "$sgbase"; git -C "$sg" clean -qfdx
  }
  trace='python3 scripts/spec-trace.py'
  spec "spec trace: cited or marked"                passes  "$trace"
  spec "spec trace: requirement without a test"     blocked "printf 'package model\n' > pkg/model/a_test.go && $trace"
  spec "spec trace: a file header does not count"   blocked "printf 'package model\n\n// demo R1: covered\n\nfunc TestA(t *testing.T) {}\n' > pkg/model/a_test.go && $trace"
  spec "spec trace: a range names each one"         passes  "sed -i.b '/Test: manual/d' specs/demo/requirements.md && printf 'package model\n\n// demo R1-R2\nfunc TestA(t *testing.T) {}\n' > pkg/model/a_test.go && $trace"
  spec "spec trace: R10 is not R1"                  blocked "printf 'package model\n\n// demo R10\nfunc TestA(t *testing.T) {}\n' > pkg/model/a_test.go && $trace"
  spec "spec trace: unknown spec cited"             blocked "printf 'package model\n\n// demo R1, no-such-spec R2\nfunc TestA(t *testing.T) {}\n' > pkg/model/a_test.go && $trace"
  spec "spec trace: draft spec cited"               blocked "printf 'package model\n\n// demo R1, rough R1\nfunc TestA(t *testing.T) {}\n' > pkg/model/a_test.go && $trace"
  spec "spec trace: withdrawn marker"               passes  "sed -i.b 's/Test: manual — docs/Test: withdrawn — a dated note/' specs/demo/requirements.md && $trace"
  spec "spec trace: marker naming its requirement"  passes  "sed -i.b 's/Test: manual — docs/Test: ci (R2) — the guardrails job/' specs/demo/requirements.md && $trace"
  spec "spec lint: reviewed by spec-refuter"        passes  "bash scripts/spec-lint.sh"
  spec "spec lint: active without spec-refuter"     blocked "sed -i.b 's/(spec-refuter)/(owner)/' specs/demo/requirements.md && rm specs/demo/*.b && bash scripts/spec-lint.sh"
  spec "spec lint: spec-refuter not run"            blocked "sed -i.b 's/(spec-refuter)/(code-review; spec-refuter not run)/' specs/demo/requirements.md && rm specs/demo/*.b && bash scripts/spec-lint.sh"
  spec "spec lint: unknown status"                  blocked "sed -i.b 's/Status: draft/Status: wip/' specs/rough/requirements.md && rm specs/rough/*.b && bash scripts/spec-lint.sh"
  spec "spec lint: reviewed but still UNREVIEWED"   blocked "sed -i.b 's/^UNREVIEWED=.*/UNREVIEWED=\"demo\"/' scripts/spec-lint.sh && rm scripts/*.b && bash scripts/spec-lint.sh"
  # fix-spec-tooling R2: '## D1.', '### D1:' and a bare '### D1' are decisions
  # the failure must be the marker count, not some other lint error
  norule="bash scripts/spec-lint.sh 2>&1 | grep -q \"D1 (design.md:[0-9]*) has no 'When this would be wrong'\""
  spec "spec lint: decision ## D1. without markers" passes "printf '\\n## D1. A choice\\n\\nText.\\n' >> specs/demo/design.md && $norule"
  spec "spec lint: decision ### D1: without markers" passes "printf '\\n### D1: A choice\\n\\nText.\\n' >> specs/demo/design.md && $norule"
  spec "spec lint: decision ### D1 bare, no markers" passes "printf '\\n### D1\\n\\nText.\\n' >> specs/demo/design.md && $norule"
  # fix-spec-tooling R2: both forms with the markers pass; '## Decisions' is not one
  spec "spec lint: decision, both forms with markers" passes "printf '\\n## D1. A\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n\\n### D2: B\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && bash scripts/spec-lint.sh"
  spec "spec lint: decision, ## Decisions not counted" passes "printf '\\n## Decisions\\n\\nNone yet.\\n' >> specs/demo/design.md && bash scripts/spec-lint.sh"
  # fix-spec-lint-markers R1: each decision carries both markers in its own section
  lint='bash scripts/spec-lint.sh'
  # fix-spec-lint-markers R1
  spec "spec lint: markers under one decision only"     passes  "printf '\\n## D1. A\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n**When this would be wrong**: x. **Reversal action**: y.\\n\\n## D2. B\\n\\nNone.\\n' >> specs/demo/design.md && ! $lint 2>/dev/null && $lint 2>&1 | grep -q 'demo: D2 (design.md:[0-9]*) has no'"
  # fix-spec-lint-markers R1
  spec "spec lint: markers in each decision"            passes  "printf '\\n## D1. A\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n\\n## D2. B\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1
  spec "spec lint: a ## decision ends at a ### decision" passes "printf '\\n## D1. A\\n\\nNone.\\n\\n### D2: B\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && ! $lint 2>/dev/null && $lint 2>&1 | grep -q 'demo: D1 (design.md:[0-9]*) has no'"
  # fix-spec-lint-markers R1
  spec "spec lint: markers in a correction note"        passes  "printf '\\n## D1. A\\n\\nText.\\n\\n> **Corrected 2026-11-05.** **When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1
  spec "spec lint: only the reversal is missing"        passes  "printf '\\n## D1. A\\n\\n**When this would be wrong**: x.\\n' >> specs/demo/design.md && ! $lint 2>/dev/null && $lint 2>&1 | grep -q \"D1 (design.md:[0-9]*) has no 'Reversal action'\" && ! $lint 2>&1 | grep -q 'would be wrong'"
  # fix-spec-lint-markers R1
  spec "spec lint: marker in a code block does not count" blocked "printf '\\n## D1. A\\n\\n\\140\\140\\140\\n**When this would be wrong**: x. **Reversal action**: y.\\n\\140\\140\\140\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1
  spec "spec lint: marker in a ~~~ block does not count" blocked "printf '\\n## D1. A\\n\\n~~~\\n**When this would be wrong**: x. **Reversal action**: y.\\n~~~\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1
  spec "spec lint: heading inside a fence ends nothing" passes  "printf '\\n## D1. A\\n\\n\\140\\140\\140\\n## not a heading\\n~~~\\n\\140\\140\\140\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1
  spec "spec lint: #### heading stays inside its decision" passes "printf '\\n### D1: A\\n\\n#### Detail\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1
  spec "spec lint: ### decisions end at the next ## heading" blocked "printf '\\n## Decisions\\n\\n### D1: A\\n\\nText.\\n\\n## Invariants\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1
  spec "spec lint: a ## decision ends at the next ## heading" passes "printf '\\n## D1. A\\n\\nText.\\n\\n## Invariants\\n\\n**When this would be wrong**: x. **Reversal action**: y.\\n' >> specs/demo/design.md && ! $lint 2>/dev/null && $lint 2>&1 | grep -q 'demo: D1 (design.md:[0-9]*) has no'"
  # fix-spec-lint-markers R1: a shorter fence does not close a longer one
  spec "spec lint: a shorter fence does not close"   blocked "printf '\\n## D1. A\\n\\n\\140\\140\\140\\140\\n\\140\\140\\140\\n**When this would be wrong**: x. **Reversal action**: y.\\n\\140\\140\\140\\140\\n' >> specs/demo/design.md && $lint"
  # fix-spec-lint-markers R1: an indented heading (up to three spaces) is a decision
  spec "spec lint: an indented decision heading counts" passes "printf '\\n   ## D1. A\\n\\nText.\\n' >> specs/demo/design.md && ! $lint 2>/dev/null && $lint 2>&1 | grep -q 'demo: D1 (design.md:[0-9]*) has no'"
  # fix-spec-lint-markers R1: a failing awk fails the lint
  spec "spec lint: a failing awk fails the lint"     passes  "mkdir -p fakebin && printf '#!/bin/sh\\nexit 2\\n' > fakebin/awk && chmod +x fakebin/awk && ! PATH=\$PWD/fakebin:\$PATH bash scripts/spec-lint.sh 2>/dev/null && PATH=\$PWD/fakebin:\$PATH bash scripts/spec-lint.sh 2>&1 | grep -q 'marker check failed (awk)'"
  # fix-spec-tooling R3: the index entry's status matches the spec's
  spec "spec lint: index says draft, spec active"   blocked "sed -i.b 's|(demo/)\$|(demo/) — **draft**.|' specs/README.md && rm specs/*.b && grep -q 'demo/) — \*\*draft' specs/README.md && bash scripts/spec-lint.sh"
  spec "spec lint: index matches the spec"          passes  "sed -i.b 's|(demo/)\$|(demo/) — **active**.|' specs/README.md && rm specs/*.b && grep -q 'demo/) — \*\*active' specs/README.md && bash scripts/spec-lint.sh"
  spec "spec tasks: code under a draft spec"        blocked "printf 'x\n' >> specs/rough/design.md && printf '\n' >> pkg/model/view.go && python3 scripts/spec-tasks-check.py \$B"
  spec "spec tasks: a script under a draft spec"    blocked "printf 'x\n' >> specs/rough/design.md && printf '\n' >> scripts/spec-lint.sh && python3 scripts/spec-tasks-check.py \$B"
  spec "spec tasks: code under an active spec"      passes  "printf 'x\n' >> specs/demo/design.md && printf '\n' >> pkg/model/view.go && python3 scripts/spec-tasks-check.py \$B"
  # shellcheck disable=SC2016 # $B expands in the case's shell
  drift='python3 scripts/drift-check.py $B'
  addflag="sed -i.b 's/^}/\tflagSet.BoolVar(\&flags.Y, \"yflag\", false, \"y\")\n}/' cmd/kube-node-viewer/flag.go && rm cmd/kube-node-viewer/*.b"
  spec "drift: new flag not in README"              blocked "$addflag && printf 'x\n' >> specs/demo/design.md && $drift"
  spec "drift: new flag named in README"            passes  "$addflag && printf -- '-yflag\n' >> README.md && $drift"
  spec "drift: help text changed, no docs"          blocked "sed -i.b 's/\"quit\"/\"leave\"/' pkg/model/panels.go && $drift"
  spec "drift: specs/README.md is not a spec"       blocked "sed -i.b 's/\"quit\"/\"leave\"/' pkg/model/panels.go && printf 'x\n' >> specs/README.md && $drift"
  spec "drift: field added, README changed"         passes  "sed -i.b 's/^)/\tFieldAge Field = \"age\"\n)/' pkg/model/view.go && printf 'age\n' >> README.md && $drift"
  spec "drift: refactor, same sets"                 passes  "printf '\n// moved\n' >> pkg/model/panels.go && $drift"
  spec "drift: key handled, not in help"            blocked "sed -i.b 's/case \"q\":/case \"q\", \"x\":/' pkg/model/uimodel.go && printf 'x\n' >> README.md && $drift"
  spec "drift: nothing found means the code moved"  blocked "printf 'package model\n' > pkg/model/panels.go && $drift"
  spec "drift: skipped by a trailer"                passes  "sed -i.b 's/\"quit\"/\"leave\"/' pkg/model/panels.go && rm pkg/model/*.b && git add -A && git commit -q -m 'fix: wording' -m 'Drift-check: skip — typo only' && $drift"
  spec "drift: trailer on another commit"           blocked "printf 'x\n' > other.txt && git add -A && git commit -q -m 'chore: x' -m 'Drift-check: skip — not this' && sed -i.b 's/\"quit\"/\"leave\"/' pkg/model/panels.go && rm pkg/model/*.b && git add -A && git commit -q -m 'fix: wording' && $drift"
  spec "drift: trailer before the base"             blocked "git commit -q --allow-empty -m 'chore: x' -m 'Drift-check: skip — old' && nb=\$(git rev-parse HEAD) && sed -i.b 's/\"quit\"/\"leave\"/' pkg/model/panels.go && rm pkg/model/*.b && git add -A && git commit -q -m 'fix: wording' && python3 scripts/drift-check.py \$nb"
  spec "drift: pull_request base is HEAD^1"         blocked "git checkout -q -b pr && sed -i.b 's/\"quit\"/\"leave\"/' pkg/model/panels.go && rm pkg/model/*.b && git add -A && git commit -q -m 'fix: wording' && git checkout -q main && git merge -q --no-ff -m merge pr && GITHUB_EVENT_NAME=pull_request python3 scripts/drift-check.py"
else skipped "spec gates" python3; fi

echo "== release prepare"
if command -v python3 >/dev/null 2>&1; then
  rp=$tmp/release-repo
  # rp CASE CHECK: a fresh tree with a feature spec, a done bugfix spec, an
  # open bugfix spec, a draft whose prose quotes the status, a shipped spec
  # whose note quotes it; release-prepare --write, then CHECK in that tree
  rp() {
    rm -rf "$rp"; mkdir -p "$rp/specs"
    for d in feat fix-done fix-open rough old; do mkdir -p "$rp/specs/$d"; done
    # shellcheck disable=SC2016 # backticks are markdown
    printf '# Specs\n\n1. [`feat/`](feat/) — **active**. F.\n2. [`fix-done/`](fix-done/) — **active**. B.\n3. [`fix-open/`](fix-open/) — **active**. O.\n4. [`rough/`](rough/) — **draft**. R.\n5. [`old/`](old/) — **shipped** in v0.1.0. S.\n' > "$rp/specs/README.md"
    printf '# R\n\n> Status: active · Owner: x\n' > "$rp/specs/feat/requirements.md"
    printf '# D\n\n> Implements [requirements](requirements.md).\n' > "$rp/specs/feat/design.md"
    printf '# B\n\n> Status: active · Reported: x\n\nThe text "Status: active" in prose.\n' > "$rp/specs/fix-done/bugfix.md"
    printf '# B\n\n> Status: active · Reported: x\n' > "$rp/specs/fix-open/bugfix.md"
    printf '# R\n\n> Status: draft · Owner: x\n\nWas it "Status: active"? No.\n> Status: active is quoted here.\n' > "$rp/specs/rough/requirements.md"
    printf '# R\n\n> Status: shipped (v0.1.0, 2026-01-01) · Owner: x\n\n> **Corrected 2026-01-02.** It said Status: active by mistake.\n' > "$rp/specs/old/requirements.md"
    for d in feat fix-done rough old; do printf '# T\n\n- [x] T1. Do it\n' > "$rp/specs/$d/tasks.md"; done
    printf '# T\n\n- [x] T1. Do it\n- [-] T2. Doing it\n' > "$rp/specs/fix-open/tasks.md"
    printf '# Changelog\n\n## [Unreleased]\n\n### Fixed\n\n- A fix.\n\n## [0.1.0] - 2026-01-01\n\n### Added\n\n- First.\n\n[Unreleased]: https://github.com/StaffOps/kube-node-viewer/compare/v0.1.0...HEAD\n[0.1.0]: https://github.com/StaffOps/kube-node-viewer/releases/tag/v0.1.0\n' > "$rp/CHANGELOG.md"
    [ "${RP_EMPTY:-}" ] && printf '# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n- First.\n' > "$rp/CHANGELOG.md"
    (cd "$rp" && find specs -type f -exec cksum {} + | sort > "$tmp/rp.before")
    out=$(cd "$rp" && python3 -B "$src/scripts/release-prepare.py" --write --date 2026-02-02 --root . > "$tmp/rp.out" 2>&1 && RP_OUT=$tmp/rp.out sh -c "$2" 2>&1)
    verdict "$1" passes $? "$out"
  }
  # fix-spec-tooling R1: a bugfix spec with every task done ships, files and index
  rp "release prepare: a done bugfix spec ships" "grep -q '^> Status: shipped (v0.1.1, 2026-02-02) · Reported: x' specs/fix-done/bugfix.md && grep -q 'fix-done/) — \*\*shipped\*\* in v0.1.1' specs/README.md && grep -q 'The text \"Status: active\" in prose' specs/fix-done/bugfix.md"
  # fix-spec-tooling R1: an open task keeps a bugfix spec active
  rp "release prepare: an open bugfix spec stays" "grep -q '^> Status: active' specs/fix-open/bugfix.md && grep -q 'fix-open/) — \*\*active\*\*' specs/README.md"
  # fix-spec-tooling R1: prose quoting the status does not ship a draft
  rp "release prepare: a draft quoting it stays" "grep -q '^> Status: draft' specs/rough/requirements.md && grep -q 'Was it \"Status: active\"? No.' specs/rough/requirements.md && grep -q '^> Status: active is quoted here' specs/rough/requirements.md && ! grep -q 'shipped spec: rough' \$RP_OUT"
  # fix-spec-tooling R1: a shipped spec's note quoting the status is unchanged
  rp "release prepare: a shipped spec is unchanged" "grep -q 'It said Status: active by mistake' specs/old/requirements.md && grep -q '^> Status: shipped (v0.1.0' specs/old/requirements.md"
  # unchanged: a feature spec still ships, files and index
  rp "release prepare: a feature spec ships" "grep -q '^> Status: shipped (v0.1.1, 2026-02-02) · Owner: x' specs/feat/requirements.md && grep -q 'feat/) — \*\*shipped\*\* in v0.1.1' specs/README.md"
  # unchanged: the CHANGELOG and compare links as before
  rp "release prepare: the CHANGELOG as before" "printf '# Changelog\n\n## [Unreleased]\n\n## [0.1.1] - 2026-02-02\n\n### Fixed\n\n- A fix.\n\n## [0.1.0] - 2026-01-01\n\n### Added\n\n- First.\n\n[Unreleased]: https://github.com/StaffOps/kube-node-viewer/compare/v0.1.1...HEAD\n[0.1.1]: https://github.com/StaffOps/kube-node-viewer/compare/v0.1.0...v0.1.1\n[0.1.0]: https://github.com/StaffOps/kube-node-viewer/releases/tag/v0.1.0\n' | cmp - CHANGELOG.md"
  # unchanged: no [Unreleased] entries, nothing marked
  RP_EMPTY=1 rp "release prepare: nothing to release" "find specs -type f -exec cksum {} + | sort | cmp - $tmp/rp.before"
else skipped "release prepare" python3; fi

echo "== license headers"
# shellcheck disable=SC2059 # $hdr is a printf format on purpose (its \n)
if command -v python3 >/dev/null 2>&1; then
  hd=$tmp/headers-repo
  git init -q -b main "$hd"
  mkdir -p "$hd/scripts/checks" "$hd/old" "$hd/.githooks"
  cp "$src/scripts/checks/headers.py" "$hd/scripts/checks/"
  hdr='/*\nLicensed under the Apache License, Version 2.0 (the "License");\n    http://www.apache.org/licenses/LICENSE-2.0\nlimitations under the License.\n*/\n'
  printf 'Apache License\n' > "$hd/LICENSE"
  printf 'upstream notice\n' > "$hd/NOTICE"
  printf "$hdr\npackage x\n" > "$hd/up.go"
  printf "${hdr}\npackage y\n\nfunc a() {}\nfunc b() {}\nfunc c() {}\n" > "$hd/old/moved.go"
  printf 'all:\n\techo\n' > "$hd/Makefile"
  printf 'data\n' > "$hd/data.json"
  printf '# Readme\n' > "$hd/README.md"
  (cd "$hd" && git add -A && git -c user.name=x -c user.email=x@example.com commit -q --no-verify -m import)
  himp=$(git -C "$hd" rev-parse HEAD)
  n='// Modified by the kube-node-viewer authors, 2026.'
  headers() {
    name=$1 want=$2 cmd=$3
    git -C "$hd" reset -q --hard "$himp"; git -C "$hd" clean -qfdx
    out=$(cd "$hd" && sh -c "$cmd" 2>&1 && git add -A && KNV_IMPORT_COMMIT=$himp python3 -B scripts/checks/headers.py 2>&1)
    verdict "$name" "$want" $? "$out"
  }
  headers "headers: untouched import"                  passes  "true"
  headers "headers: upstream file changed, no note"    blocked:"upstream file changed without the modification note" "printf '// x\n' >> up.go"
  headers "headers: note after the license block"      passes  "printf \"$hdr\n$n\n\npackage x\n// x\n\" > up.go"
  headers "headers: note far from the license block"   blocked:"modification note not right after the license block" "printf \"$hdr\npackage x\n\n$n\n\" > up.go"
  headers "headers: note is the package doc"           blocked:"no blank line after the modification note (it would become the package doc)" "printf \"$hdr\n$n\npackage x\n// x\n\" > up.go"
  headers "headers: upstream file moved, no note"      blocked:"upstream file changed without the modification note" "mkdir -p new && git mv old/moved.go new/moved.go && printf '// x\n' >> new/moved.go"
  headers "headers: upstream file moved, with note"    passes  "mkdir -p new && git mv old/moved.go new/moved.go && printf \"$hdr\n$n\n\npackage y\n\nfunc a() {}\nfunc b() {}\nfunc c() {}\n\" > new/moved.go"
  headers "headers: Makefile changed, no note"         blocked:"upstream file changed without the modification note in its first 5 lines" "printf 'x:\n' >> Makefile"
  headers "headers: Makefile changed, note on top"     passes  "printf '# Modified by the kube-node-viewer authors, 2026.\nall:\n\techo\nx:\n' > Makefile"
  headers "headers: file type without a rule"          blocked:"upstream file changed, no rule for its type: add a note or an EXEMPT entry" "printf 'other\n' > data.json"
  headers "headers: README is exempt"                  passes  "printf 'more\n' >> README.md"
  headers "headers: new script without a header"       blocked:"no Apache license header" "printf '#!/bin/sh\necho\n' > new.sh"
  headers "headers: git hook without a header"         blocked:"no Apache license header" "mkdir -p .githooks && printf '#!/bin/sh\necho\n' > .githooks/pre-commit"
  headers "headers: python file without a header"      blocked:"no Apache license header" "printf 'print(1)\n' > new.py"
  headers "headers: short header is not enough"        blocked:"no Apache license header" "printf '#!/bin/sh\n# Licensed under the Apache License, Version 2.0; see LICENSE.\n' > new.sh"
  headers "headers: LICENSE changed"                   blocked:"LICENSE differs from the imported one" "printf 'x\n' >> LICENSE"
  headers "headers: NOTICE first line changed"         blocked:"NOTICE first line changed" "printf 'other\n' > NOTICE"
  out=$(cd "$hd" && KNV_IMPORT_COMMIT=0000000000000000000000000000000000000000 python3 -B scripts/checks/headers.py 2>&1); rc=$?
  if [ "$rc" -eq 2 ] && printf '%s' "$out" | grep -q 'is not in this clone'; then ok "headers: import commit missing (exit 2)"; else bad "headers: import commit missing (exit 2)" "rc=$rc $out"; fi
else skipped "license headers" python3; fi

echo "== push (pre-push)"

reset
git init -q --bare "$tmp/remote.git"
git remote add origin "$tmp/remote.git" 2>/dev/null
out=$(git push -q origin HEAD:main 2>&1)
verdict "push to main" blocked $? "$out"
git tag v0.0.1
out=$(git push -q origin v0.0.1 2>&1)
verdict "push of a tag" blocked $? "$out"

echo "== agent hooks"
if command -v jq >/dev/null 2>&1; then
  reset
  echo "node,cpu" > kube-node-viewer-1.csv && git add -A
  hook() { printf '{"tool_input":{"command":"%s"}}' "$2" | CLAUDE_PROJECT_DIR=$repo sh "scripts/agent-hooks/$1" claude; }
  if hook commit-guard.sh 'git commit -m x' | grep -q '"deny"'; then ok "commit guard denies"; else bad "commit guard denies" "passed"; fi
  if [ -z "$(hook commit-guard.sh 'git status')" ]; then ok "commit guard ignores other commands"; else bad "commit guard ignores other commands" "denied"; fi
  if command -v python3 >/dev/null 2>&1; then
    # stands in for scripts/spec-tasks-check.py finding an open task
    printf 'import sys\nprint("specs/x/tasks.md: 1 open task")\nsys.exit(1)\n' > scripts/spec-tasks-check.py
    if hook pr-guard.sh 'gh pr create --fill' | grep -q '"deny"'; then ok "PR guard denies a PR with an open task"; else bad "PR guard denies a PR with an open task" "passed"; fi
    if [ -z "$(hook pr-guard.sh 'gh pr create --draft --fill')" ]; then ok "PR guard lets a draft through"; else bad "PR guard lets a draft through" "denied"; fi
    if hook pr-guard.sh 'gh pr ready 12' | grep -q '"deny"'; then ok "PR guard denies gh pr ready"; else bad "PR guard denies gh pr ready" "passed"; fi
  else skipped "agent PR guard" python3; fi
else skipped "agent hooks" jq; fi

echo "== kiro hooks"
# the Kiro payloads and config checks live in one place, kiro-selftest.sh,
# which a Kiro user also runs before a real session (docs/kiro-validation.md)
if command -v jq >/dev/null 2>&1; then
  out=$(sh "$src/scripts/agent-hooks/kiro-selftest.sh" 2>&1); rc=$?
  if [ "$rc" -eq 0 ]; then ok "kiro selftest: $(printf '%s\n' "$out" | tail -1)"
  else bad "kiro selftest" "$(printf '%s\n' "$out" | grep -E '^(FAIL|      )')"; fi
else skipped "kiro selftest" jq; fi

printf '\n%d passed, %d failed, %d skipped\n' "$pass" "$fail" "$skip"
[ "$fail" -eq 0 ]
