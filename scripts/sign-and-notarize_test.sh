#!/usr/bin/env bash
# Tests for sign-and-notarize.sh: runs it against fake codesign, xcrun
# and ditto, to check the order of the calls and that it stops, loudly,
# when something is missing or Apple says no. Nothing is signed for
# real here; the release workflow's rehearsal covers that.
# Run: bash scripts/sign-and-notarize_test.sh
set -euo pipefail

pass=0
fail=0

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# The id every fake submission gets, so the cases can look for it in
# the calls.
submission_id="1b2c3d4e-0000-4000-8000-000000000000"

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

mkdir -p "$work/bin" "$work/hopto.app/Contents/MacOS"
echo fake > "$work/hopto.app/Contents/MacOS/hopto"
echo fake > "$work/hopto.dmg"
echo text > "$work/notes.txt"

# Every fake appends its name and arguments to the file FAKE_CALLS names,
# so the order of the calls can be read back afterwards.
calls="$work/calls"

cat > "$work/bin/codesign" <<'FAKE'
#!/usr/bin/env bash
echo "codesign $*" >> "$FAKE_CALLS"
FAKE

# The real ditto writes the zip named by its last argument.
cat > "$work/bin/ditto" <<'FAKE'
#!/usr/bin/env bash
echo "ditto $*" >> "$FAKE_CALLS"
touch "${@: -1}"
FAKE

# The notarytool answers copy what the real one prints with --wait: the
# id shows up more than once, indented, and the last line is the status.
# FAKE_NOTARY picks the outcome: Accepted, Invalid, or Crash for a
# submission that fails by itself (no network, bad credentials).
cat > "$work/bin/xcrun" <<'FAKE'
#!/usr/bin/env bash
echo "xcrun $*" >> "$FAKE_CALLS"
case "$1 $2" in
    "notarytool submit")
        if [ "${FAKE_NOTARY:-Accepted}" = "Crash" ]; then
            echo "Error: could not connect to the notary service" >&2
            exit 1
        fi
        echo "Conducting pre-submission checks and initiating connection to the Apple notary service..."
        echo "Successfully uploaded file"
        echo "  id: $FAKE_ID"
        echo "  path: /tmp/upload.zip"
        echo "Waiting for processing to complete."
        echo "Current status: In Progress.....Processing complete"
        echo "  id: $FAKE_ID"
        echo "  status: ${FAKE_NOTARY:-Accepted}"
        ;;
    "notarytool log")
        echo "fake log: the reason"
        ;;
esac
FAKE
chmod +x "$work/bin/"*

# run_script runs the script on $target with the fakes first on the PATH
# and a complete, made-up environment. Extra arguments are further
# NAME=value pairs for env, to override or add to it.
run_script() {
    env "PATH=$work/bin:$PATH" "FAKE_CALLS=$calls" "FAKE_ID=$submission_id" \
        SIGN_IDENTITY=fake-identity APPLE_ID=someone@example.org \
        TEAM_ID=ABCDE12345 NOTARIZATION_PASSWORD=fake-password \
        "$@" \
        bash "$here/sign-and-notarize.sh" "$target" > "$work/out" 2>&1
}

echo "▶ a bundle is signed, notarised and stapled:"
target="$work/hopto.app"
: > "$calls"
check "the run succeeds" 'run_script'
check "the bundle is signed with the hardened runtime" 'grep -q "^codesign .*--options runtime.*--timestamp" "$calls"'
check "the signature is verified" 'grep -q "^codesign --verify" "$calls"'
check "signing comes before the submission" '[ "$(grep -n "^codesign" "$calls" | head -1 | cut -d: -f1)" -lt "$(grep -n "notarytool submit" "$calls" | cut -d: -f1)" ]'
check "the bundle is zipped for the upload" 'grep -q "^ditto -c -k --keepParent $work/hopto.app " "$calls"'
check "stapling comes last" 'tail -1 "$calls" | grep -q "^xcrun stapler staple $work/hopto.app"'
check "the password is not printed" '! grep -q fake-password "$work/out"'

echo "▶ a disk image is signed, notarised and stapled:"
target="$work/hopto.dmg"
: > "$calls"
check "the run succeeds" 'run_script'
check "it is signed without the runtime flag" 'grep -q "^codesign " "$calls" && ! grep -q -- "--options runtime" "$calls"'
check "it is submitted as it is" 'grep -q "notarytool submit $work/hopto.dmg" "$calls"'
check "it is not zipped" '! grep -q "^ditto" "$calls"'
check "stapling comes last" 'tail -1 "$calls" | grep -q "^xcrun stapler staple $work/hopto.dmg"'

echo "▶ Apple says no:"
target="$work/hopto.app"
: > "$calls"
check "a rejection fails" '! run_script FAKE_NOTARY=Invalid'
check "the log is asked for with the one id, however often it was printed" 'grep -q "^xcrun notarytool log $submission_id --apple-id" "$calls"'
check "the log of the submission is shown" 'grep -q "fake log: the reason" "$work/out"'
check "the output says it was not accepted" 'grep -q "did not accept" "$work/out"'
check "a rejected bundle is not stapled" '! grep -q "stapler staple" "$calls"'
check "the password is not printed" '! grep -q fake-password "$work/out"'

echo "▶ the submission itself fails:"
: > "$calls"
check "a failed submission fails the script" '! run_script FAKE_NOTARY=Crash'
check "the output says the submission failed" 'grep -q "submission failed" "$work/out"'
check "a failed submission is not stapled" '! grep -q "stapler staple" "$calls"'

echo "▶ bad input:"
: > "$calls"
check "missing environment fails" '! env -i "PATH=$work/bin:$PATH" "FAKE_CALLS=$calls" SIGN_IDENTITY=fake-identity bash "$here/sign-and-notarize.sh" "$target" > "$work/out" 2>&1'
check "missing environment names the variable" 'grep -q APPLE_ID "$work/out"'
check "missing environment signs nothing" '[ ! -s "$calls" ]'

target="$work/notes.txt"
check "another kind of file is refused" '! run_script'
check "a refused file signs nothing" '[ ! -s "$calls" ]'

echo "── $pass passed, $fail failed"
[ "$fail" -eq 0 ]
