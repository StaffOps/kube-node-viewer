#!/usr/bin/env python3
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
"""Fail when a code change ships with spec tasks still open, or under a draft spec.

A change that touches code (cmd/, pkg/, hack/, scripts/, .github/,
.githooks/) and a spec (specs/<feature>/) can
merge, and so release, only when that spec is past draft (reviewed by
spec-refuter, which scripts/spec-lint.sh checks) and every task of it is
done ([x]), including its verify task (scripts/verify.sh, a run on a
cluster noting what was checked). Spec-only changes (planning) and code
changes without a spec pass.

  scripts/spec-tasks-check.py [BASE]

BASE defaults as in scripts/changelog-check.py.
"""
import os
import re
import subprocess
import sys

OPEN = re.compile(r"^- \[( |-)\] (T\d+\..*)$", re.M)
STATUS = re.compile(r"^> Status: (\w+)", re.M)
# what ships or runs: the program, and the scripts and workflows that
# check and release it
CODE = ("cmd/", "pkg/", "hack/", "scripts/", ".github/", ".githooks/")


def git(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True)


def default_base():
    if os.environ.get("GITHUB_EVENT_NAME") == "pull_request":
        return git("rev-parse", "-q", "--verify", "HEAD^1").stdout.strip()
    return git("merge-base", "HEAD", "origin/main").stdout.strip()


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else default_base()
    if not base:
        print("spec-tasks-check: no base to compare with; skipped")
        return 0
    # committed changes plus the working tree, so it also works before a commit
    changed = set(git("diff", "--name-only", base).stdout.split())
    code = [f for f in changed if f.startswith(CODE)]
    specs = sorted({f.split("/")[1] for f in changed
                    if f.startswith("specs/") and f.count("/") >= 2 and not f.startswith(("specs/decisions/", "specs/.templates/"))})
    if not code or not specs:
        print("spec-tasks-check: ok (no code change with a spec)")
        return 0
    bad, drafts = [], []
    for name in specs:
        req = os.path.join("specs", name, "bugfix.md" if name.startswith("fix-") else "requirements.md")
        if os.path.exists(req):
            m = STATUS.search(open(req).read())
            if m and m.group(1) == "draft":
                drafts.append(req)
        path = os.path.join("specs", name, "tasks.md")
        if not os.path.exists(path):
            continue
        for mark, task in OPEN.findall(open(path).read()):
            bad.append(f"{path}: [{mark}] {task[:100]}")
    if drafts:
        print("Code under a draft spec; get the spec-refuter review, record it on the status line and set it active first (a spec-only PR can stay draft):", file=sys.stderr)
        for d in drafts:
            print("  " + d, file=sys.stderr)
        return 1
    if bad:
        print("Spec tasks still open; finish them (verify includes a run on a cluster, noting what was checked) or move them to Deferred:", file=sys.stderr)
        for b in bad:
            print("  " + b, file=sys.stderr)
        return 1
    print(f"spec-tasks-check: ok ({', '.join(specs)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
