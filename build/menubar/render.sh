#!/bin/bash
# Renders the menu bar icons: every SVG here at 1x (18 px) and 2x
# (36 px) for the Retina menu bar, plus the frog in colour from the app
# icon's source. The PNGs land in internal/app/menubar, where the app
# embeds them. Needs rsvg-convert (brew install librsvg) and sips.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
out="$here/../../internal/app/menubar"

for svg in "$here"/*.svg; do
	name=$(basename "$svg" .svg)
	rsvg-convert -w 18 -h 18 "$svg" -o "$out/$name.png"
	rsvg-convert -w 36 -h 36 "$svg" -o "$out/$name@2x.png"
done

# The frog in colour is the repository's own icon scaled down: no tile,
# like assets/icon.png.
sips -z 18 18 "$here/../../assets/icon.png" --out "$out/colour.png" >/dev/null
sips -z 36 36 "$here/../../assets/icon.png" --out "$out/colour@2x.png" >/dev/null
