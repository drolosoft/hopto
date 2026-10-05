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
check "a missing bundle is refused" '! bash "$here/make-dmg.sh" "$work/nothing.app" "$work/no.dmg" 2> "$work/refusal"'
check "the refusal says why" 'grep -q "no bundle" "$work/refusal"'
check "a refusal leaves no image behind" '[ ! -e "$work/no.dmg" ]'
check "an existing image is replaced" 'bash "$here/make-dmg.sh" "$work/hopto.app" "$work/out.dmg"'

echo "▶ awkward paths:"
mkdir -p "$work/a folder"
cp -R "$work/hopto.app" "$work/a folder/hopto.app"
check "a space in the path and a trailing slash" 'bash "$here/make-dmg.sh" "$work/a folder/hopto.app/" "$work/a folder/spaced.dmg"'
hdiutil attach "$work/a folder/spaced.dmg" -mountpoint "$work/mount" -nobrowse -readonly -quiet
check "the app keeps its name inside" '[ -f "$work/mount/hopto.app/Contents/MacOS/hopto" ]'
hdiutil detach "$work/mount" -quiet

echo "▶ hdiutil create fails now and then:"
# A fake hdiutil first on the PATH: it fails the way a busy runner does
# for as many tries as FAKE_FAILURES says, counting them in a file, and
# then hands over to the real one. A fake sleep keeps the test from
# waiting out the pauses between tries.
real_hdiutil="$(command -v hdiutil)"
mkdir -p "$work/fakes"
cat > "$work/fakes/hdiutil" <<FAKE
#!/usr/bin/env bash
tries="\$(cat "\$FAKE_TRIES" 2>/dev/null || echo 0)"
tries=\$((tries + 1))
echo "\$tries" > "\$FAKE_TRIES"
if [ "\$tries" -le "\$FAKE_FAILURES" ]; then
    echo "hdiutil: create failed - Resource busy" >&2
    exit 1
fi
exec "$real_hdiutil" "\$@"
FAKE
printf '#!/usr/bin/env bash\n' > "$work/fakes/sleep"
chmod +x "$work/fakes/hdiutil" "$work/fakes/sleep"

rm -f "$work/tries"
check "one busy try is retried" 'PATH="$work/fakes:$PATH" FAKE_TRIES="$work/tries" FAKE_FAILURES=1 bash "$here/make-dmg.sh" "$work/hopto.app" "$work/retried.dmg" 2> "$work/retry-log"'
check "the image is made on the second try" '[ -s "$work/retried.dmg" ] && [ "$(cat "$work/tries")" = 2 ]'
hdiutil attach "$work/retried.dmg" -mountpoint "$work/mount" -nobrowse -readonly -quiet
check "the retried image holds the app" '[ -f "$work/mount/hopto.app/Contents/MacOS/hopto" ]'
hdiutil detach "$work/mount" -quiet

rm -f "$work/tries"
check "an hdiutil that always fails fails the script" '! PATH="$work/fakes:$PATH" FAKE_TRIES="$work/tries" FAKE_FAILURES=99 bash "$here/make-dmg.sh" "$work/hopto.app" "$work/never.dmg" 2> "$work/retry-log"'
check "it gives up after three tries" '[ "$(cat "$work/tries")" = 3 ]'
check "the log shows the message hdiutil gave" 'grep -q "Resource busy" "$work/retry-log"'
check "no image is left behind" '[ ! -e "$work/never.dmg" ]'

echo "── $pass passed, $fail failed"
[ "$fail" -eq 0 ]
