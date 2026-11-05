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
"""Fail when what a user sees changes with no spec or docs change.

    scripts/drift-check.py [BASE]

Compares, between BASE and the working tree:
  - flags: the names registered in cmd/kube-node-viewer/flag.go;
  - keys: the helpKeys entries (key and text) in pkg/model/panels.go;
  - fields: the Field constants (columns, sorts, filters) in
    pkg/model/view.go.
When any of them changed, the change must also touch specs/, README.md or
docs/. A change that alters them on purpose with no doc to update (a typo in
a help text, say) says so in a commit message of the branch with a trailer:

    Drift-check: skip — <reason>

BASE defaults as in scripts/changelog-check.py. Reading the sets rather than
the diff lets a refactor that moves code around pass untouched.
"""
import os
import re
import subprocess
import sys

DOCS = ("specs/", "README.md", "docs/")
TRAILER = re.compile(r"^Drift-check: skip [—-]+ \S", re.M)


def git(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True)


def default_base():
    if os.environ.get("GITHUB_EVENT_NAME") == "pull_request":
        return git("rev-parse", "-q", "--verify", "HEAD^1").stdout.strip()
    return git("merge-base", "HEAD", "origin/main").stdout.strip()


def flags(src):
    return set(re.findall(r'flagSet\.\w+Var\(&[\w.]+,\s*"([^"]+)"', src))


def keys(src):
    m = re.search(r"^var helpKeys = \[\]\[2\]string\{\n(.*?)^\}", src, re.M | re.S)
    return set(re.findall(r'\{"((?:[^"\\]|\\.)*)",\s*"((?:[^"\\]|\\.)*)"\}', m.group(1))) if m else set()


def fields(src):
    return set(re.findall(r'^\s*Field\w*\s+Field\s*=\s*"([^"]+)"', src, re.M))


SURFACES = [
    ("flags", "cmd/kube-node-viewer/flag.go", flags),
    ("keys", "pkg/model/panels.go", keys),
    ("fields", "pkg/model/view.go", fields),
]


def show(item):
    return " ".join(item) if isinstance(item, tuple) else item


def main():
    top = git("rev-parse", "--show-toplevel").stdout.strip()
    if top:
        os.chdir(top)
    base = sys.argv[1] if len(sys.argv) > 1 else default_base()
    if not base:
        print("drift-check: no base to compare with; skipped")
        return 0
    changes = []
    for name, path, extract in SURFACES:
        old = git("show", f"{base}:{path}").stdout
        new = open(path).read() if os.path.exists(path) else ""
        a, b = extract(old), extract(new)
        for item in sorted(b - a, key=show):
            changes.append(f"{name}: added {show(item)[:90]}")
        for item in sorted(a - b, key=show):
            changes.append(f"{name}: removed {show(item)[:90]}")
    if not changes:
        print("drift-check: ok (no flag, key or field change)")
        return 0
    changed = git("diff", "--name-only", base).stdout.split()
    if any(f.startswith(DOCS) for f in changed):
        print(f"drift-check: ok ({len(changes)} change(s), with specs or docs)")
        return 0
    log = git("log", "--format=%B", f"{base}..HEAD").stdout
    if TRAILER.search(log):
        print(f"drift-check: ok ({len(changes)} change(s), skipped by a Drift-check trailer)")
        return 0
    print("Flags, keys or fields changed with no change under specs/, README.md or docs/ (drift-check). "
          "Update the spec or the README; if no doc needs it, add a commit trailer "
          "'Drift-check: skip — <reason>':", file=sys.stderr)
    for c in changes:
        print("  " + c, file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
