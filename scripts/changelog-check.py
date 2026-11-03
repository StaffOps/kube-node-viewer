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
"""Fail when a change edits a released CHANGELOG section.

Changes go under ## [Unreleased]. A branch cut before a release merges its
[Unreleased] lines into the released section with no conflict; this catches
that. Every ## [x.y.z] section of the base must be unchanged, except for
added dated corrections ("> **Corrected YYYY-MM-DD"). New version sections
(a hand-made release) are fine.

  scripts/changelog-check.py [BASE]

BASE defaults to the first parent of HEAD in a pull_request CI run (the
merge ref, whose first parent is the base), else to the merge base with
origin/main.
"""
import os
import re
import subprocess
import sys

HEADING = re.compile(r"^## \[(\d+\.\d+\.\d+)\][^\n]*$", re.M)


def git(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True)


def default_base():
    if os.environ.get("GITHUB_EVENT_NAME") == "pull_request":
        return git("rev-parse", "-q", "--verify", "HEAD^1").stdout.strip()
    return git("merge-base", "HEAD", "origin/main").stdout.strip()


def sections(text):
    out = {}
    marks = list(HEADING.finditer(text))
    for i, m in enumerate(marks):
        end = marks[i + 1].start() if i + 1 < len(marks) else len(text)
        body = text[m.end():end]
        body = re.split(r"^\[[^\]]+\]: ", body, maxsplit=1, flags=re.M)[0]  # link block
        out[m.group(1)] = [ln for ln in body.strip().splitlines()]
    return out


def main():
    base = sys.argv[1] if len(sys.argv) > 1 else None
    if base is None:
        base = default_base()
    if not base:
        print("changelog-check: no base to compare with; skipped")
        return 0
    old = git("show", f"{base}:CHANGELOG.md")
    if old.returncode != 0:
        print("changelog-check: base has no CHANGELOG.md; skipped")
        return 0
    before = sections(old.stdout)
    after = sections(open("CHANGELOG.md").read())
    bad = []
    for version, lines in before.items():
        now = after.get(version)
        if now is None:
            bad.append(f"[{version}] was removed")
            continue
        extra = [ln for ln in now if ln not in lines and not ln.startswith("> **Corrected")]
        missing = [ln for ln in lines if ln not in now]
        if extra or missing:
            sample = (extra or missing)[0].strip()
            bad.append(f"[{version}] changed ({'+' if extra else '-'} {sample!r})")
    if bad:
        print("CHANGELOG.md: released sections must not change; put new entries under ## [Unreleased]:", file=sys.stderr)
        for b in bad:
            print("  " + b, file=sys.stderr)
        return 1
    print("changelog-check: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
