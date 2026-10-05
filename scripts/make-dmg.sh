#!/usr/bin/env bash
# make-dmg.sh: packs an .app into a compressed disk image with a link to
# /Applications beside it, so installing is one drag. Plain hdiutil: no
# background picture and no third-party tool to install on the runner.
#
# Usage: bash scripts/make-dmg.sh <path/to/hopto.app> <path/to/out.dmg>
set -euo pipefail

app="${1:?usage: make-dmg.sh <app> <dmg>}"
dmg="${2:?usage: make-dmg.sh <app> <dmg>}"

if [ ! -d "$app" ]; then
    echo "make-dmg: no bundle at $app" >&2
    exit 1
fi

# The folder hdiutil turns into the image: the bundle and the link.
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT

# ditto keeps the signature's extended attributes and the stapled
# ticket, which a plain cp -R can drop.
ditto "$app" "$stage/$(basename "$app")"
ln -s /Applications "$stage/Applications"

mkdir -p "$(dirname "$dmg")"
rm -f "$dmg"
hdiutil create -volname hopto -srcfolder "$stage" -fs HFS+ -format UDZO -quiet "$dmg"
