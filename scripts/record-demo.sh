#!/usr/bin/env bash
# record-demo.sh: records assets/demo.gif from the real app, on a demo
# library with nothing personal in it: the example of the repository,
# both scans off, and six apps of /System/Applications added by hand.
#
# In order: it refuses to run while the Mac is in use; covers the whole
# screen with a flat backdrop, since the panel is translucent and would
# otherwise show whatever is behind it; quits any hopto and sets the
# data folder aside; writes the demo library and opens the build; lets
# the link icons arrive; records the main display while it drives the
# panel with keys, checking before each one that hopto is in front;
# crops the recording to the panel and turns it into a GIF; and puts
# everything back, closing the backdrop and opening the hopto that was
# running.
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

# The panel is translucent (vibrancy), so the recording always shows
# whatever sits behind it. This flat, neutral colour reads well behind
# both the light and the dark theme and keeps the user's own desktop
# out of the GIF.
backdrop_color="#3b4048"

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

# Whether Preview was already open, recorded before this script ever
# touches it, so restore quits it only when this script is the one that
# opened it. Written as an if, not a && chain, so a "not running" result
# (a normal, expected failure of pgrep) never trips set -e.
preview_was_running=0
if pgrep -x Preview > /dev/null; then
    preview_was_running=1
fi

keep="$(mktemp -d)"
work="$keep/work"
mkdir -p "$work"
previous="$(running_bundle)"

# restore runs whatever happens: the backdrop closes, this build quits,
# the data comes back, and the hopto that was running before is opened
# again.
restore() {
    # Only the backdrop window, by name: "every window" would also close
    # whatever Preview windows the user already had open, unsaved work
    # included, when preview_was_running is 1.
    osascript -e 'tell application "Preview" to close (every window whose name is "backdrop.png") saving no' 2>/dev/null || true

    if [ "$preview_was_running" -eq 0 ]; then
        osascript -e 'tell application "Preview" to quit' 2>/dev/null || true
    fi

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

# preview_ready holds once Preview has a window open for the backdrop.
preview_ready() {
    [ "$(osascript -e 'tell application "System Events" to tell process "Preview" to count windows')" -ge 1 ]
}

# The main display's size in points, and its backing scale (2 on a
# Retina display, 1 otherwise): together they give the backdrop the
# exact pixel size the recording captures, so no edge of the screen is
# left uncovered. The crop later reuses this same screen_width.
screen_width="$(osascript -l JavaScript -e 'ObjC.import("AppKit"); $.NSScreen.screens.objectAtIndex(0).frame.size.width')"
screen_height="$(osascript -l JavaScript -e 'ObjC.import("AppKit"); $.NSScreen.screens.objectAtIndex(0).frame.size.height')"
backing_scale="$(osascript -l JavaScript -e 'ObjC.import("AppKit"); $.NSScreen.screens.objectAtIndex(0).backingScaleFactor')"
backdrop_width="$(awk -v points="$screen_width" -v scale="$backing_scale" 'BEGIN {printf "%d", points * scale}')"
backdrop_height="$(awk -v points="$screen_height" -v scale="$backing_scale" 'BEGIN {printf "%d", points * scale}')"

# The panel is translucent, so the recording would otherwise show
# whatever sits behind it, including the user's own desktop. A flat
# image the size of the screen, opened in Preview and put behind the
# panel, is the only way to keep that out of the GIF.
ffmpeg -loglevel error -y -f lavfi -i "color=c=${backdrop_color}:s=${backdrop_width}x${backdrop_height}" \
    -frames:v 1 "$work/backdrop.png"
open -a Preview "$work/backdrop.png"
wait_for 5 preview_ready || exit 1
osascript -e "tell application \"Preview\" to set bounds of front window to {0, 0, $screen_width, $screen_height}"
osascript -e 'tell application "Preview" to activate'

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
step; key "$key_escape"
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
# screen_width was already read before the backdrop was drawn.
movie_width="$(ffprobe -v error -select_streams v:0 -show_entries stream=width -of csv=p=0 "$work/demo.mov")"
crop="$(awk -v pixels="$movie_width" -v points="$screen_width" \
    -v left="$left" -v top="$top" -v width="$width" -v height="$height" -v margin="$margin" 'BEGIN {
    factor = pixels / points
    printf "%d:%d:%d:%d", (width + 2 * margin) * factor, (height + 2 * margin) * factor, (left - margin) * factor, (top - margin) * factor
}')"

mkdir -p "$repo/assets"
# -ss 2, not 1: with the backdrop in place the first second is a still
# frame of flat colour rather than bare desktop, and skipping a little
# further in leaves the GIF starting once the panel is already moving.
ffmpeg -loglevel error -y -ss 2 -i "$work/demo.mov" \
    -vf "crop=$crop,fps=$gif_fps,scale=$gif_width:-1:flags=lanczos,split[frames][copy];[copy]palettegen=stats_mode=diff[palette];[frames][palette]paletteuse=dither=bayer:bayer_scale=4" \
    "$repo/assets/demo.gif"

size="$(stat -f %z "$repo/assets/demo.gif")"
echo "assets/demo.gif: $((size / 1024)) KiB"
