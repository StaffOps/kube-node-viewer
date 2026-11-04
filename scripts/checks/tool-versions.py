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
"""Report CI tools pinned by version that Dependabot does not update.

    scripts/checks/tool-versions.py [--min-age DAYS]

For each pin below, reads the version in the file and every release newer
than it (Go module proxy, or PyPI for zizmor; prereleases and yanked ones
skipped). A tool is behind when one of those releases is at least
--min-age days old (default 14: a release that young is not adopted yet),
so a tool releasing every week is still caught. Also fails when the
golangci-lint versions of the Makefile and test.yaml differ. Exit 1 when
anything is behind, 2 when a lookup fails; the weekly CI run fails, which
mails the owner. Needs network; no token. The Go proxy's time is the
release commit's, close to the publication for these tools.
"""

import argparse
import datetime
import json
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

# name, file, regex with one group (the version), where releases are listed;
# "after>regex" reads the first match after a line matching after
PINS = [
    ("golangci-lint", "Makefile", r"^GOLANGCI_LINT_VERSION=v?(\S+)", "go:github.com/golangci/golangci-lint/v2"),
    ("golangci-lint (CI)", ".github/workflows/test.yaml", r"golangci-lint-action@>^\s+version: v?(\S+)", "go:github.com/golangci/golangci-lint/v2"),
    ("govulncheck", "Makefile", r"^GOVULNCHECK_VERSION=v?(\S+)", "go:golang.org/x/vuln"),
    ("go-licenses", "Makefile", r"^GO_LICENSES_VERSION=v?(\S+)", "go:github.com/google/go-licenses/v2"),
    ("actionlint", ".github/workflows/test.yaml", r"actionlint/cmd/actionlint@v?(\S+)", "go:github.com/rhysd/actionlint"),
    ("zizmor", ".github/workflows/test.yaml", r"zizmor==(\S+)", "pypi:zizmor"),
    ("gitleaks", ".github/actions/gitleaks/action.yml", r"GITLEAKS_VERSION: v?(\S+)", "go:github.com/zricethezav/gitleaks/v8"),
    ("cosign", ".github/workflows/release.yaml", r"cosign-release: v?(\S+)", "go:github.com/sigstore/cosign/v3"),
    ("syft", ".github/workflows/release.yaml", r"syft-version: v?(\S+)", "go:github.com/anchore/syft"),
]


HEADERS = {"User-Agent": "kube-node-viewer-tool-versions"}


def get_json(url):
    req = urllib.request.Request(url, headers=HEADERS)
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.load(r)


def parse_time(s):
    return datetime.datetime.fromisoformat(s.replace("Z", "+00:00"))


STABLE = re.compile(r"^\d+(\.\d+)*$")


def releases(source):
    """Stable releases as {version without v: release time}, fetched lazily."""
    kind, name = source.split(":", 1)
    if kind == "go":
        req = urllib.request.Request(f"https://proxy.golang.org/{name}/@v/list", headers=HEADERS)
        with urllib.request.urlopen(req, timeout=30) as r:
            versions = [v.strip().lstrip("v") for v in r.read().decode().split()]
        return {v: (lambda v=v: parse_time(get_json(f"https://proxy.golang.org/{name}/@v/v{v}.info")["Time"]))
                for v in versions if STABLE.match(v)}
    info = get_json(f"https://pypi.org/pypi/{name}/json")
    out = {}
    for v, files in info["releases"].items():
        files = [f for f in files if not f.get("yanked")]
        if STABLE.match(v) and files:
            when = min(parse_time(f["upload_time_iso_8601"]) for f in files)
            out[v] = lambda when=when: when
    return out


def key(version):
    return tuple(int(p) for p in version.split("."))


def pinned(file, pattern):
    after, _, pattern = pattern.rpartition(">")
    seen = not after
    for line in (ROOT / file).read_text().splitlines():
        seen = seen or bool(re.search(after, line))
        m = seen and re.search(pattern, line)
        if m:
            return m.group(1)
    raise SystemExit(f"{file}: no line matches {pattern}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--min-age", type=int, default=14, metavar="DAYS")
    args = ap.parse_args()
    now = datetime.datetime.now(datetime.timezone.utc)
    behind = 0
    seen = {}
    for name, file, pattern, source in PINS:
        have = pinned(file, pattern).lstrip("v")
        try:
            if source not in seen:
                seen[source] = releases(source)
            newer = sorted((v for v in seen[source] if key(v) > key(have)), key=key, reverse=True)
            # the newest release old enough to adopt, if any
            ripe = next(((v, (now - seen[source][v]()).days) for v in newer
                         if (now - seen[source][v]()).days >= args.min_age), None)
        except (urllib.error.URLError, OSError, ValueError, KeyError) as e:
            print(f"ERROR   {name}: looking up {source}: {e}", file=sys.stderr)
            return 2
        if ripe:
            behind += 1
            print(f"BEHIND  {name}: {have} in {file}; {ripe[0]} released {ripe[1]} days ago")
        elif newer:
            print(f"ok      {name}: {have} (newer: {', '.join(newer)}, all under {args.min_age} days)")
        else:
            print(f"ok      {name}: {have}")
    lint = {pinned(f, p) for n, f, p, _ in PINS if n.startswith("golangci-lint")}
    if len(lint) > 1:
        behind += 1
        print(f"DIFFER  golangci-lint: Makefile and test.yaml pin {sorted(lint)}")
    return 1 if behind else 0


if __name__ == "__main__":
    sys.exit(main())
