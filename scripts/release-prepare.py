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
"""Turn CHANGELOG [Unreleased] into the next version (used by release.yaml).

Prints the version to release (x.y.z), or nothing when [Unreleased] is
empty; then the newest released version is printed instead, and the caller
releases it only if its tag is missing.

Next version, from the newest released one and the [Unreleased] headings:
  "BREAKING" anywhere           -> major (minor while 0.x)
  Added / Changed / Removed /
  Deprecated                    -> minor
  anything else (Fixed, Security) -> patch

With --write, also: renames [Unreleased] to [x.y.z] - DATE under a new
empty [Unreleased], updates the compare links, and marks specs shipped
whose tasks are all done: the first "> Status:" line of the header, when
it says active, of requirements.md (bugfix.md only, for a fix-* spec),
design.md and tasks.md, and the spec's specs/README.md entry.

  scripts/release-prepare.py [--write] [--date YYYY-MM-DD] [--root DIR]
"""
import argparse
import datetime
import glob
import os
import re
import sys

REPO_URL = "https://github.com/StaffOps/kube-node-viewer"
VERSION_HEADING = re.compile(r"^## \[(\d+)\.(\d+)\.(\d+)\]", re.M)
MINOR_HEADINGS = {"added", "changed", "removed", "deprecated"}


def unreleased_body(text):
    start = text.find("## [Unreleased]")
    if start < 0:
        sys.exit("CHANGELOG.md has no ## [Unreleased] section")
    start = text.index("\n", start) + 1
    m = re.search(r"^## \[", text[start:], re.M)
    end = start + m.start() if m else len(text)
    return start, end, text[start:end]


def next_version(latest, body):
    major, minor, patch = latest
    headings = {h.strip().lower() for h in re.findall(r"^### (.+)$", body, re.M)}
    if "BREAKING" in body:
        return (major + 1, 0, 0) if major >= 1 else (major, minor + 1, 0)
    if headings & MINOR_HEADINGS:
        return (major, minor + 1, 0)
    return (major, minor, patch + 1)


# a spec's status is its first "> Status:" line, never a quote of it in
# prose or a note (fix-spec-tooling R1)
STATUS = re.compile(r"^> Status: (\w+)", re.M)


def ship(text, version, date):
    """text with its status line shipped, or None when that line is not active.

    Only the header, before the first "## " heading, is read.
    """
    head = text.split("\n## ", 1)[0]
    m = STATUS.search(head)
    if not m or m.group(1) != "active":
        return None
    return text[:m.start(1)] + f"shipped (v{version}, {date})" + text[m.end(1):]


def ship_specs(root, version, date):
    readme_path = os.path.join(root, "specs", "README.md")
    readme = open(readme_path).read() if os.path.exists(readme_path) else None
    for tasks in sorted(glob.glob(os.path.join(root, "specs", "*", "tasks.md"))):
        spec_dir = os.path.dirname(tasks)
        name = os.path.basename(spec_dir)
        boxes = re.findall(r"^- \[(.)\] T\d+", open(tasks).read(), re.M)
        if not boxes or any(b != "x" for b in boxes):
            continue
        # a bugfix spec keeps its status in bugfix.md (fix-spec-tooling R1)
        main = "bugfix.md" if name.startswith("fix-") else "requirements.md"
        req = os.path.join(spec_dir, main)
        if not os.path.exists(req) or ship(open(req).read(), version, date) is None:
            continue
        for f in (main, "design.md", "tasks.md"):
            p = os.path.join(spec_dir, f)
            if os.path.exists(p):
                t = ship(open(p).read(), version, date)
                if t is not None:
                    open(p, "w").write(t)
        if readme is not None:
            entry = f"[`{name}/`]({name}/) — **active**."
            if entry in readme:
                readme = readme.replace(entry, f"[`{name}/`]({name}/) — **shipped** in v{version}.", 1)
            else:
                print(f"warning: {name}: no '— **active**.' entry in specs/README.md to mark shipped", file=sys.stderr)
        print(f"shipped spec: {name}", file=sys.stderr)
    if readme is not None:
        open(readme_path, "w").write(readme)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--write", action="store_true")
    ap.add_argument("--date", default=datetime.date.today().isoformat())
    ap.add_argument("--root", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    args = ap.parse_args()

    path = os.path.join(args.root, "CHANGELOG.md")
    text = open(path).read()
    m = VERSION_HEADING.search(text)
    latest = tuple(int(x) for x in m.groups()) if m else (0, 0, 0)
    start, end, body = unreleased_body(text)
    if not body.strip():
        if m:
            print("%d.%d.%d" % latest)
        return
    version = "%d.%d.%d" % next_version(latest, body)
    if args.write:
        prev = "%d.%d.%d" % latest
        text = text[:start] + f"\n## [{version}] - {args.date}\n" + text[start:]
        link = f"[Unreleased]: {REPO_URL}/compare/v{prev}...HEAD\n"
        if link in text:
            text = text.replace(link, f"[Unreleased]: {REPO_URL}/compare/v{version}...HEAD\n"
                                f"[{version}]: {REPO_URL}/compare/v{prev}...v{version}\n", 1)
        open(path, "w").write(text)
        ship_specs(args.root, version, args.date)
    print(version)


if __name__ == "__main__":
    main()
