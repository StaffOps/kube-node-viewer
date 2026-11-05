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
# PreToolUse hook: on `git commit`, remind the docs/spec checklist.
#   commit-reminder.sh claude  -> JSON additionalContext (Claude Code)
#   commit-reminder.sh kiro    -> plain text on stdout (Kiro)
# Reads the hook payload (JSON with .tool_input.command) on stdin.
mode=${1:-claude}
command -v jq >/dev/null 2>&1 || exit 0
# shellcheck disable=SC1003 # '\n\\' is a tr set (newline, backslash), not an escaped quote
cmd=$(jq -r '.tool_input.command // .tool_input.cmd // ""' 2>/dev/null | tr '\n\\' '  ' | sed -E "s/'[^']*'|\"[^\"]*\"//g")
printf '%s\n' "$cmd" | grep -Eq '(^|[;&|(/[:space:]])git([[:space:]]+(-C|-c)[[:space:]]+[^[:space:]]+|[[:space:]]+-[^[:space:]]+)*[[:space:]]+commit([[:space:]]|$)' || exit 0
msg="Pre-commit checklist (kube-node-viewer; the git pre-commit hook and commit-guard check exports, CHANGELOG, gofmt, headers, real-environment data and secrets): make check green (verify, branch commits, hook tests, vuln, licenses, lint, build; main merged in); task status updated in specs/<feature>/tasks.md; CHANGELOG.md under [Unreleased] only; README usage if flags/keys changed; Apache headers on new files; no export files (kube-node-viewer-*.csv/json) or other cluster data staged, no real names, IPs, account ids or company names in files or in the message; TODO.md updated if this closes or finds an item; Conventional Commit message with no AI trailers."
if [ "$mode" = kiro ]; then
  printf '%s\n' "$msg"
  exit 0
fi
jq -n --arg m "$msg" '{hookSpecificOutput: {hookEventName: "PreToolUse", additionalContext: $m}}'
