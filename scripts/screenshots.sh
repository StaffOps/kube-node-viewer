#!/usr/bin/env bash
# Licensed under the Apache License, Version 2.0 (the "License"); see LICENSE.
#
# Regenerates the README images in .static/ from a made-up cluster, so no real
# account or cluster appears in the docs. Needs Python 3, Google Chrome (or
# Chromium; set CHROME) and ffmpeg.
#
#   scripts/screenshots.sh
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
out="$root/.static"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
chrome=${CHROME:-"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
conv="$root/scripts/screenshots/ansi2html.py"

KNV_SCREENSHOTS="$work" go test "$root/pkg/model" -run TestScreenshots -count=1 >/dev/null

# shoot <name> <width px> <height px> <scale>
shoot() {
  local name=$1 w=$2 h=$3 scale=$4 caption=()
  [[ -f "$work/$name.caption" ]] && caption=("$work/$name.caption")
  python3 -I "$conv" "$work/$name.ansi" "$work/$name.html" "${caption[@]}"
  "$chrome" --headless=new --disable-gpu --hide-scrollbars \
    --force-device-scale-factor="$scale" --window-size="$w,$h" \
    --screenshot="$work/$name.png" "file://$work/$name.html" >/dev/null 2>&1
}

mkdir -p "$out"
for name in main groups-folded groups-details filters group-by columns contexts views header-compact; do
  lines=$(wc -l <"$work/$name.ansi")
  shoot "$name" 1384 $(((lines + 1) * 14 + 36)) 2
  cp "$work/$name.png" "$out/$name.png"
done

# the animation: every frame the same size, the first and last held longer
frames=("$work"/demo-*.ansi)
: >"$work/frames.txt"
for i in "${!frames[@]}"; do
  name=$(basename "${frames[$i]}" .ansi)
  shoot "$name" 1240 540 1
  hold=2.6
  [[ $i -eq 0 || $i -eq $((${#frames[@]} - 1)) ]] && hold=3.5
  printf "file '%s'\nduration %s\n" "$work/$name.png" "$hold" >>"$work/frames.txt"
done
# the concat demuxer needs the last file repeated for its duration to apply
printf "file '%s'\n" "$work/$name.png" >>"$work/frames.txt"
ffmpeg -loglevel error -y -f concat -safe 0 -i "$work/frames.txt" \
  -vf "fps=10,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=none" \
  -loop 0 "$out/demo.gif"

ls -la "$out"
