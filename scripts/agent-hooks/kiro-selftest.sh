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
# Offline self-test of the Kiro wiring, run before a real Kiro session
# (docs/kiro-validation.md). No Kiro needed:
#   1. checks .kiro/hooks/kube-node-viewer.json and .kiro/agents/*.json
#      against the documented schema (kiro.dev/docs/hooks,
#      kiro.dev/docs/custom-agents/configuration-reference): each of the
#      five hook scripts wired once, with its trigger and `kiro` mode;
#   2. runs each hook's command, exactly as the hook file states it, in a
#      throwaway repository, with the JSON Kiro documents on stdin
#      (hook_event_name, cwd, session_id, tool_name, tool_input), and checks
#      the exit code Kiro acts on: 2 blocks the tool, 0 allows it.
#   scripts/agent-hooks/kiro-selftest.sh     (also run by test-checks.sh)
# Prints PASS / FAIL / SKIP per case; exits 1 when a case fails.
set -u
# a hook's GIT_DIR (or a worktree's) would point git at the real repository
# (fix-hook-tests-git-env); keep the throwaway repository to itself
# shellcheck disable=SC2046 # the names are words, split on purpose
unset $(git rev-parse --local-env-vars 2>/dev/null) GIT_NAMESPACE GIT_CEILING_DIRECTORIES \
  GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL \
  GIT_TEMPLATE_DIR CLAUDE_PROJECT_DIR
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
src=$(cd "$(dirname "$0")/../.." && pwd)
hooks=$src/.kiro/hooks/kube-node-viewer.json
pass=0 fail=0 skip=0
ok()   { pass=$((pass + 1)); printf 'PASS  %s\n' "$1"; }
bad()  { fail=$((fail + 1)); printf 'FAIL  %s\n' "$1"; [ -n "${2:-}" ] && printf '%s\n' "$2" | sed 's/^/      /'; return 0; }
skipped() { skip=$((skip + 1)); printf 'SKIP  %s (%s)\n' "$1" "$2"; }
# none NAME LIST: passes when LIST (the offending items) is empty
none() { if [ -z "$2" ]; then ok "$1"; else bad "$1" "$2"; fi; }

if ! command -v jq >/dev/null 2>&1; then
  bad "jq is installed" "the hooks read Kiro's JSON with jq and let every command through without it; install it (brew install jq, apt-get install jq)"
  printf '\n%d passed, %d failed, %d skipped\n' "$pass" "$fail" "$skip"
  exit 1
fi
ok "jq is installed ($(jq --version))"

# --- 1. configuration -------------------------------------------------------
if jq -e '.version == "v1" and (.hooks | type == "array" and length > 0)' "$hooks" >/dev/null 2>&1; then
  ok "hook file: version v1 with a hooks array"
else
  bad "hook file: version v1 with a hooks array" "$hooks"
fi
# documented PascalCase triggers (kiro.dev/docs/hooks/types)
none "hook file: every trigger is a documented PascalCase trigger" \
  "$(jq -r '.hooks[] | select(.trigger | IN("SessionStart","UserPromptSubmit","PreToolUse","PostToolUse","Stop","SessionEnd","PostFileCreate","PostFileSave","PostFileDelete","PreTaskExecution","PostTaskExecution") | not) | .name' "$hooks")"
none "hook file: every hook has a name and a command action" \
  "$(jq -r '.hooks[] | select((.name | type) != "string" or .action.type != "command" or (.action.command | type) != "string") | .name // "(no name)"' "$hooks")"
none "hook file: tool hooks match the shell tag" \
  "$(jq -r '.hooks[] | select(.trigger == "PreToolUse" and .matcher != "shell") | .name' "$hooks")"
# wired SCRIPT TRIGGER ARG: the script appears in exactly one hook, with that
# trigger and that argument (the kiro mode prints what Kiro reads)
wired() {
  n=$(jq --arg s "scripts/agent-hooks/$1" '[.hooks[] | select(.action.command | contains($s))] | length' "$hooks")
  want="sh scripts/agent-hooks/$1${3:+ $3}"
  got=$(jq -r --arg s "scripts/agent-hooks/$1" '.hooks[] | select(.action.command | contains($s)) | "\(.trigger) \(.action.command)"' "$hooks")
  if [ "$n" -eq 1 ] && [ "$got" = "$2 $want" ]; then ok "hook file: $1 wired once ($2, \"$want\")"
  else bad "hook file: $1 wired once ($2, \"$want\")" "found $n: ${got:-none}"; fi
}
wired session-context.sh SessionStart ""
wired guard-destructive.sh PreToolUse kiro
wired commit-guard.sh PreToolUse kiro
wired commit-reminder.sh PreToolUse kiro
wired pr-guard.sh PreToolUse kiro
# one source of truth: the hooks live in the hook file, not in the profile
if jq -e 'has("hooks") | not' "$src/.kiro/agents/kube-node-viewer.json" >/dev/null; then
  ok "agent profile: no inline hooks (they live in the hook file)"
else
  bad "agent profile: no inline hooks (they live in the hook file)" "remove .hooks from .kiro/agents/kube-node-viewer.json"
fi
# CLI V3 tool selectors: category tags, tool ids, @server, * (CLI 2.x names
# such as grep or glob are not V3 tags)
for a in "$src"/.kiro/agents/*.json; do
  none "agent $(basename "$a"): tools are V3 selectors" \
    "$(jq -r '(.tools // [])[] | select(IN("*","read","write","shell","web","subagent","spec","context","knowledge","@builtin","@mcp") or startswith("@") | not)' "$a")"
  none "agent $(basename "$a"): permission rules use documented capabilities and effects" \
    "$(jq -r '(.permissions.rules // [])[] | select((.capability | IN("fs_read","fs_write","shell","web_fetch","web_search","mcp","subagent","skill","power","context","diagnostics","sandbox_network","all","builtin","filesystem") | not) or (.effect | IN("allow","ask","deny") | not)) | tojson' "$a")"
  case $(jq -r '.prompt // ""' "$a") in
    file://*)
      p=$(jq -r '.prompt' "$a"); p=${p#file://}
      if [ -f "$(dirname "$a")/$p" ]; then ok "agent $(basename "$a"): prompt file exists"; else bad "agent $(basename "$a"): prompt file exists" "$p"; fi ;;
  esac
done

# --- 2. the hooks' commands with Kiro's payloads ----------------------------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
repo=$tmp/repo
git init -q -b main "$repo"
cd "$repo" || exit 1
mkdir -p scripts
cp -R "$src/scripts/agent-hooks" "$src/scripts/checks" scripts/
cp "$src/scripts/changelog-check.py" scripts/
cp "$src/.gitignore" .
git config user.name "Example Dev"
git config user.email "dev@example.com"
git config commit.gpgsign false
printf '# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n### Added\n\n- First.\n\n[0.1.0]: https://example.com/compare\n' > CHANGELOG.md
git add -A && git commit -q --no-verify -m "chore: base"
git update-ref refs/remotes/origin/main HEAD # changelog-check compares with it

# payload EVENT [COMMAND]: the hook event JSON Kiro sends on stdin
payload() {
  if [ "$1" = PreToolUse ]; then
    jq -cn --arg cwd "$repo" --arg c "$2" '{hook_event_name: "PreToolUse", cwd: $cwd, session_id: "kiro-selftest", tool_name: "execute_bash", tool_input: {command: $c}}'
  else
    jq -cn --arg e "$1" --arg cwd "$repo" '{hook_event_name: $e, cwd: $cwd, session_id: "kiro-selftest"}'
  fi
}
# run HOOK-SCRIPT EVENT [COMMAND]: runs that hook's command from the hook
# file, in the repository root as Kiro does; sets rc, out, err
run() {
  hc=$(jq -r --arg s "scripts/agent-hooks/$1" '.hooks[] | select(.action.command | contains($s)) | .action.command' "$hooks" | head -1)
  [ -n "$hc" ] || hc="false # $1 is not in the hook file"
  payload "$2" "${3:-}" | sh -c "$hc" >"$tmp/out" 2>"$tmp/err"
  rc=$?; out=$(cat "$tmp/out"); err=$(cat "$tmp/err")
}
# expect NAME blocks|allows|tells: 2 with a reason on stderr; 0 and quiet;
# 0 with text on stdout
expect() {
  case $2 in
    blocks) [ "$rc" -eq 2 ] && [ -n "$err" ] && { ok "$1"; return; } ;;
    allows) [ "$rc" -eq 0 ] && [ -z "$err$out" ] && { ok "$1"; return; } ;;
    tells)  [ "$rc" -eq 0 ] && [ -n "$out" ] && { ok "$1"; return; } ;;
  esac
  bad "$1 (want: $2)" "exit $rc; stdout: ${out:-<empty>}; stderr: ${err:-<empty>}"
}

run session-context.sh SessionStart
if [ "$rc" -eq 0 ] && printf '%s' "$out" | grep -q "kube-node-viewer session context"; then
  ok "session context: SessionStart prints the context (exit 0)"
else
  bad "session context: SessionStart prints the context (exit 0)" "exit $rc; $out$err"
fi

# the checklist (docs/kiro-validation.md) uses echo-prefixed forms; both reach the guard
run guard-destructive.sh PreToolUse 'echo git push --force origin kiro-validation'
expect "guard: blocks echo git push --force (checklist test 3)" blocks
run guard-destructive.sh PreToolUse 'git push --force origin feature'
expect "guard: blocks git push --force" blocks
run guard-destructive.sh PreToolUse 'echo git reset --hard HEAD'
expect "guard: blocks echo git reset --hard (checklist test 4)" blocks
run guard-destructive.sh PreToolUse 'git tag -d v1.0.0'
expect "guard: blocks a tag delete" blocks
run guard-destructive.sh PreToolUse 'git commit -m "docs: mention git push --force"'
expect "guard: allows a commit quoting a force push" allows
run guard-destructive.sh PreToolUse 'ls'
expect "guard: allows ls (checklist test 10)" allows

# exports are git-ignored, so the checklist stages one with -f
echo "node,cpu" > kube-node-viewer-validation.csv && git add -f kube-node-viewer-validation.csv
run commit-guard.sh PreToolUse 'git commit -m "test: kiro validation"'
expect "commit guard: blocks a commit with an export file staged (checklist test 5)" blocks
git rm -q --cached kube-node-viewer-validation.csv && rm -f kube-node-viewer-validation.csv
run commit-guard.sh PreToolUse 'git commit -m "feat: add"'
expect "commit guard: allows a clean commit" allows
run commit-guard.sh PreToolUse 'git status'
expect "commit guard: allows other commands" allows

run commit-reminder.sh PreToolUse 'git commit --allow-empty -m "bad subject"'
expect "commit checklist: printed on git commit (checklist test 7)" tells
run commit-reminder.sh PreToolUse 'go test ./...'
expect "commit checklist: quiet on other commands" allows

if command -v python3 >/dev/null 2>&1; then
  run pr-guard.sh PreToolUse 'echo gh pr create --fill'
  expect "PR guard: allows a PR when nothing is wrong (checklist test 8)" allows
  # the command checklist test 9 gives: a line under the newest released heading
  python3 -c "p='CHANGELOG.md';t=open(p).read();i=t.index(chr(10),t.index(chr(10)+'## [0.')+1);open(p,'w').write(t[:i]+chr(10)+'- test'+t[i:])"
  run pr-guard.sh PreToolUse 'echo gh pr create --fill'
  expect "PR guard: blocks a PR that edits a released CHANGELOG section (checklist test 9)" blocks
  run pr-guard.sh PreToolUse 'echo gh pr create --draft --fill'
  expect "PR guard: allows a draft PR" allows
  git checkout -q CHANGELOG.md
else
  skipped "PR guard cases" "python3 missing; the PR guard lets everything through without it"
fi

printf '\n%d passed, %d failed, %d skipped\n' "$pass" "$fail" "$skip"
[ "$fail" -eq 0 ]
