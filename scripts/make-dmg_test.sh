#!/usr/bin/env bash
# Tests for make-dmg.sh: builds a disk image from a fake bundle and
# checks what a person would see on opening it: the app and the link to
# /Applications, nothing else. macOS only (hdiutil).
# Run: bash scripts/make-dmg_test.sh
set -euo pipefail

pass=0
fail=0

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"

# The image is detached before the folder goes, in case a failed case
# left it mounted.
trap 'hdiutil detach "$work/mount" -quiet 2>/dev/null || true; rm -rf "$work"' EXIT

# check runs a command string and counts it as a pass or a failure,
# named so the output says which expectation broke.
check() {
    local name="$1"
    local condition="$2"

    if eval "$condition"; then
        echo "  ok: $name"
        pass=$((pass + 1))
    else
        echo "  ✗ $name" >&2
        fail=$((fail + 1))
    fi
}

mkdir -p "$work/hopto.app/Contents/MacOS"
echo fake > "$work/hopto.app/Contents/MacOS/hopto"

echo "▶ the image holds the app and the link, nothing else:"
bash "$here/make-dmg.sh" "$work/hopto.app" "$work/out.dmg"
check "the image exists" '[ -s "$work/out.dmg" ]'

hdiutil attach "$work/out.dmg" -mountpoint "$work/mount" -nobrowse -readonly -quiet
check "the app is inside" '[ -f "$work/mount/hopto.app/Contents/MacOS/hopto" ]'
check "the link points to /Applications" '[ "$(readlink "$work/mount/Applications")" = "/Applications" ]'
check "nothing else is inside" '[ "$(ls "$work/mount" | wc -l | tr -d " ")" = "2" ]'
hdiutil detach "$work/mount" -quiet

echo "▶ bad input and reruns:"
check "a missing bundle is refused" '! bash "$here/make-dmg.sh" "$work/nothing.app" "$work/no.dmg" 2>/dev/null'
check "an existing image is replaced" 'bash "$here/make-dmg.sh" "$work/hopto.app" "$work/out.dmg"'

echo "── $pass passed, $fail failed"
[ "$fail" -eq 0 ]
