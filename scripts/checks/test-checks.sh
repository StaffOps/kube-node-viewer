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
  mkdir -p "$sg/scripts" "$sg/specs/demo" "$sg/pkg/model" "$sg/cmd/kube-node-viewer"
  cp "$src/scripts/spec-lint.sh" "$src/scripts/spec-trace.py" "$src/scripts/drift-check.py" "$src/scripts/spec-tasks-check.py" "$sg/scripts/"
  # shellcheck disable=SC2016 # backticks are markdown
  printf '# Specs\n\n1. [`demo/`](demo/)\n' > "$sg/specs/README.md"
  printf '# Requirements: demo\n\n> Status: active · Owner: x · Reviewed: 2026-01-01 (spec-refuter)\n\n**R1** WHEN a THEN the system SHALL b.\n\n**R2** WHEN c THEN the system SHALL d.\n\n_Test: manual — docs._\n' > "$sg/specs/demo/requirements.md"
  printf '# Design: demo\n' > "$sg/specs/demo/design.md"
  printf '# Tasks: demo\n\n- [x] T1. Do it\n  _Covers: R1, R2_\n' > "$sg/specs/demo/tasks.md"
  printf 'package model\n\n// demo R1: covered\n' > "$sg/pkg/model/a_test.go"
  printf 'package main\n\nfunc f() {\n\tflagSet.BoolVar(&flags.X, "x", false, "x")\n}\n' > "$sg/cmd/kube-node-viewer/flag.go"
  printf 'package model\n\nvar helpKeys = [][2]string{\n\t{"q", "quit"},\n}\n' > "$sg/pkg/model/panels.go"
  printf 'package model\n\nconst (\n\tFieldZone Field = "zone"\n)\n' > "$sg/pkg/model/view.go"
  printf '# demo\n' > "$sg/README.md"
  (
    cd "$sg" || exit 1
    git config user.name "Example Dev"; git config user.email "dev@example.com"; git config commit.gpgsign false
    git add -A && git commit -q -m "chore: base"
  )
  sgbase=$(git -C "$sg" rev-parse HEAD)
  # spec CASE WANT CMD...: run CMD in the spec repo, then reset it
  spec() {
    name=$1 want=$2; shift 2
    out=$(cd "$sg" && "$@" 2>&1)
    verdict "$name" "$want" $? "$out"
    git -C "$sg" reset -q --hard "$sgbase"; git -C "$sg" clean -qfdx

  }
  spec "spec trace: cited or marked"            passes  python3 -B scripts/spec-trace.py
  spec "spec trace: requirement without a test" blocked sh -c 'printf "package model\n" > pkg/model/a_test.go && python3 -B scripts/spec-trace.py'
  spec "spec trace: a range names each one"     passes  sh -c 'sed -i.b "/Test: manual/d" specs/demo/requirements.md && printf "package model\n\n// demo R1-R2\n" > pkg/model/a_test.go && python3 -B scripts/spec-trace.py'
  spec "spec lint: reviewed by spec-refuter"    passes  bash scripts/spec-lint.sh
  spec "spec lint: active without spec-refuter" blocked sh -c 'sed -i.b "s/(spec-refuter)/(owner)/" specs/demo/requirements.md && rm specs/demo/*.b && bash scripts/spec-lint.sh'
  spec "spec tasks: code under a draft spec"    blocked sh -c 'sed -i.b "s/Status: active/Status: draft/" specs/demo/requirements.md && printf "\n" >> pkg/model/view.go && python3 -B scripts/spec-tasks-check.py '"$sgbase"
  spec "spec tasks: code under an active spec"  passes  sh -c 'printf "x\n" >> specs/demo/design.md && printf "\n" >> pkg/model/view.go && python3 -B scripts/spec-tasks-check.py '"$sgbase"
  spec "drift: flag added, no docs"             blocked sh -c 'sed -i.b "s/^}/\tflagSet.BoolVar(\&flags.Y, \"y\", false, \"y\")\n}/" cmd/kube-node-viewer/flag.go && python3 -B scripts/drift-check.py '"$sgbase"
  spec "drift: help key changed, no docs"       blocked sh -c 'sed -i.b "s/\"quit\"/\"leave\"/" pkg/model/panels.go && python3 -B scripts/drift-check.py '"$sgbase"
  spec "drift: field added, README changed"     passes  sh -c 'sed -i.b "s/^)/\tFieldAge Field = \"age\"\n)/" pkg/model/view.go && printf "age\n" >> README.md && python3 -B scripts/drift-check.py '"$sgbase"
  spec "drift: refactor, same sets"             passes  sh -c 'printf "\n// moved\n" >> pkg/model/panels.go && python3 -B scripts/drift-check.py '"$sgbase"
  spec "drift: skipped by a trailer"            passes  sh -c 'sed -i.b "s/\"quit\"/\"leave\"/" pkg/model/panels.go && rm pkg/model/*.b && git add -A && git commit -q -m "fix: wording" -m "Drift-check: skip — typo only" && python3 -B scripts/drift-check.py '"$sgbase"
else skipped "spec gates" python3; fi

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

printf '\n%d passed, %d failed, %d skipped\n' "$pass" "$fail" "$skip"
[ "$fail" -eq 0 ]
