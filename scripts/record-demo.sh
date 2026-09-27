#!/usr/bin/env bash
# record-demo.sh: records assets/demo.gif from the real app, on a demo
# library with nothing personal in it: the example of the repository,
# both scans off, and six apps of /System/Applications added by hand.
#
# In order: it refuses to run while the Mac is in use; quits any hopto
# and sets the data folder aside; writes the demo library and opens the
# build; lets the link icons arrive; records the main display while it
# drives the panel with keys, checking before each one that hopto is in
# front; crops the recording to the panel and turns it into a GIF; and
# puts everything back, opening the hopto that was running.
#
# Usage: make build && bash scripts/record-demo.sh
# Needs: ffmpeg and ffprobe (brew install ffmpeg), the network for the
# link icons, and Screen Recording allowed for the terminal.
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
bundle="$repo/build/bin/hopto.app"
source "$repo/scripts/real-app.sh"

# Seconds without a key or a click before starting, as verify-hopto.sh.
min_idle=30

# The recording covers the whole sequence below with a moment of the
# empty desktop at each end.
record_seconds=16

# 12 frames a second and 800 pixels wide keep the GIF under 3 MB for a
# panel this size; the README shows it at 800.
gif_fps=12
gif_width=800

# Points of shadow kept around the panel when cropping.
margin=24

# The apps of the demo grid, all in /System/Applications.
demo_apps=(Calculator Calendar Maps Notes Preview TextEdit)

for tool in ffmpeg ffprobe; do
    if ! command -v "$tool" > /dev/null; then
        echo "$tool is missing: brew install ffmpeg" >&2
        exit 3
    fi
done

if [ ! -d "$bundle" ]; then
    echo "no build at $bundle: run make build first" >&2
    exit 3
fi

idle="$(idle_seconds)"
if [ "$idle" -lt "$min_idle" ]; then
    echo "the Mac is in use (idle ${idle} s, needs ${min_idle}): try again when it is free" >&2
    exit 2
fi

keep="$(mktemp -d)"
work="$keep/work"
mkdir -p "$work"
previous="$(running_bundle)"

# restore runs whatever happens: this build quits, the data comes back,
# and the hopto that was running before is opened again.
restore() {
    pkill -x hopto 2>/dev/null || true
    sleep 1
    put_back "$keep"

    if [ -n "$previous" ]; then
        open "$previous"
    fi

    echo "your data is back (copy and raw recording kept in $keep)"
}
trap restore EXIT

# step stops the recording before a key that would land in another app.
step() {
    guard || exit 1
}

# icons_ready holds once the six apps and at least three links have an
# icon file.
icons_ready() {
    [ "$(find "$data/icons" -name '*.png' | wc -l)" -ge 9 ]
}

pkill -x hopto 2>/dev/null || true
sleep 1
set_aside "$keep"
write_demo_library "$repo" "${demo_apps[@]}"

log_mark
open -n "$bundle"
wait_for 10 registered 1 || exit 1

# The seed's links fetch their icons the first time the links tab shows:
# show it once, off camera, and wait for the downloads.
chord "$key_space" "command down, option down"
wait_for 3 guard || exit 1
wait_for 20 icons_ready || echo "some link icons did not arrive; recording anyway" >&2
key "$key_escape"
wait_for 3 windows_are 0 || exit 1

# The panel's place, in points, for the crop.
chord "$key_space" "command down, shift down"
wait_for 3 windows_are 1 || exit 1
geometry="$(osascript -e 'tell application "System Events" to tell process "hopto" to get {position, size} of first window' | tr -d ' ')"
step
key "$key_escape"
wait_for 3 windows_are 0 || exit 1
IFS=, read -r left top width height <<< "$geometry"

screencapture -v -V "$record_seconds" -x "$work/demo.mov" &
recorder=$!
sleep 1.5

chord "$key_space" "command down, shift down"
sleep 1.6
step; typed "cal"
sleep 1.4
step; key "$key_escape"
sleep 0.4
step; key "$key_tab"
sleep 1.2
step; typed "git"
sleep 1.4
step; key "$key_escape"
sleep 0.4
step; chord "$key_n" "command down"
sleep 0.6
step; typed "example.org"
sleep 3
step; key "$key_escape"
sleep 0.6
step; key "$key_escape"
sleep 0.4
step; key "$key_escape"

wait "$recorder"

# The recording has the display's pixels; the geometry is in points.
movie_width="$(ffprobe -v error -select_streams v:0 -show_entries stream=width -of csv=p=0 "$work/demo.mov")"
screen_width="$(osascript -l JavaScript -e 'ObjC.import("AppKit"); $.NSScreen.screens.objectAtIndex(0).frame.size.width')"
crop="$(awk -v pixels="$movie_width" -v points="$screen_width" \
    -v left="$left" -v top="$top" -v width="$width" -v height="$height" -v margin="$margin" 'BEGIN {
    factor = pixels / points
    printf "%d:%d:%d:%d", (width + 2 * margin) * factor, (height + 2 * margin) * factor, (left - margin) * factor, (top - margin) * factor
}')"

mkdir -p "$repo/assets"
ffmpeg -loglevel error -y -ss 1 -i "$work/demo.mov" \
    -vf "crop=$crop,fps=$gif_fps,scale=$gif_width:-1:flags=lanczos,split[frames][copy];[copy]palettegen=stats_mode=diff[palette];[frames][palette]paletteuse=dither=bayer:bayer_scale=4" \
    "$repo/assets/demo.gif"

size="$(stat -f %z "$repo/assets/demo.gif")"
echo "assets/demo.gif: $((size / 1024)) KiB"
