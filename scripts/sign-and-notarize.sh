#!/usr/bin/env bash
# sign-and-notarize.sh: signs an .app or a .dmg with a Developer ID,
# sends it to Apple's notary service, waits for the answer and staples
# the ticket to it, so a Mac accepts it even with no network: macOS
# still asks once before opening something downloaded, but no longer
# refuses it.
# (The file says "notarize" because Apple's tool is spelled notarytool.)
#
# Usage: bash scripts/sign-and-notarize.sh <hopto.app | file.dmg>
#
# Environment, all of it required and none of it ever printed:
#   SIGN_IDENTITY          the Developer ID Application identity, by name
#                          or by hash, in a keychain on the search list
#   APPLE_ID               the Apple account that submits
#   TEAM_ID                its developer team
#   NOTARIZATION_PASSWORD  an app-specific password of that account
#
# Never add `set -x` here: it would print the password with the rest of
# the command line.
set -euo pipefail

target="${1:?usage: sign-and-notarize.sh <hopto.app | file.dmg>}"

# Checked before anything is signed, so a missing secret costs nothing.
# ${!name} reads the variable whose name is in $name.
for name in SIGN_IDENTITY APPLE_ID TEAM_ID NOTARIZATION_PASSWORD; do
    if [ -z "${!name:-}" ]; then
        echo "sign-and-notarize: $name is not set" >&2
        exit 1
    fi
done

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# What goes to Apple: a disk image travels as it is, a bundle has to be
# zipped first, and ditto is the zip the notary service documents.
case "$target" in
    *.app)
        # The hardened runtime is a condition of notarisation; the
        # secure timestamp keeps the signature valid after the
        # certificate expires.
        codesign --force --options runtime --timestamp --sign "$SIGN_IDENTITY" "$target"
        codesign --verify --deep --strict "$target"
        upload="$work/upload.zip"
        ditto -c -k --keepParent "$target" "$upload"
        ;;
    *.dmg)
        codesign --force --timestamp --sign "$SIGN_IDENTITY" "$target"
        codesign --verify --strict "$target"
        upload="$target"
        ;;
    *)
        echo "sign-and-notarize: $target is neither an .app nor a .dmg" >&2
        exit 1
        ;;
esac

# --wait returns once Apple has decided. A rejection may or may not come
# with a non-zero exit status, depending on the Xcode version, so the
# status line of the answer is what counts. The exit status is kept
# apart: pipefail makes the pipeline fail when notarytool does, even
# though tee succeeds, and it tells a submission that never got an answer
# (no network, bad credentials) from one that was answered.
# --timeout 30m: --wait has no limit of its own, and a stuck queue at
# Apple would otherwise hold the CI job for hours.
answer="$work/answer"
submit_status=0
xcrun notarytool submit "$upload" \
    --apple-id "$APPLE_ID" --team-id "$TEAM_ID" --password "$NOTARIZATION_PASSWORD" \
    --wait --timeout 30m | tee "$answer" || submit_status=$?

# The answer is indented and repeats the id; only a line that is a
# status on its own counts, not "Current status: ..." progress lines.
if ! grep -Eq '^ *status: ' "$answer"; then
    echo "sign-and-notarize: the submission failed (exit status $submit_status) and Apple gave no answer" >&2
    exit 1
fi

if ! grep -Eq '^ *status: Accepted *$' "$answer"; then
    # The log says which file and which rule; without it a rejection is
    # a guess. The id is printed more than once, so only the first is used.
    submission="$(awk '/^ *id: /{ print $2; exit }' "$answer")"
    xcrun notarytool log "$submission" \
        --apple-id "$APPLE_ID" --team-id "$TEAM_ID" --password "$NOTARIZATION_PASSWORD" || true
    echo "sign-and-notarize: Apple did not accept $target" >&2
    exit 1
fi

xcrun stapler staple "$target"
