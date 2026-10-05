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

# hdiutil create now and then fails on GitHub's hosted macOS runners
# with "Resource busy", when something else on the machine still holds
# the new volume for a moment; the same command a few seconds later
# works. Three tries with a pause between them, and after the third its
# own message is what the log shows. A failed try may leave a partial
# image, which hdiutil would refuse to overwrite, so each try starts
# without one.
attempts=3
pause_seconds=5
attempt=1
until rm -f "$dmg" && hdiutil create -volname hopto -srcfolder "$stage" -fs HFS+ -format UDZO -quiet "$dmg"; do
    if [ "$attempt" -ge "$attempts" ]; then
        echo "make-dmg: hdiutil failed $attempts times" >&2
        exit 1
    fi
    echo "make-dmg: hdiutil failed, trying again in $pause_seconds seconds" >&2
    attempt=$((attempt + 1))
    sleep "$pause_seconds"
done
