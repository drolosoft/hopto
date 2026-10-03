#!/usr/bin/env bash
# verify-hopto.sh: drives the built app from the shell and checks what a
# person would check by hand: both shortcuts register, each one shows the
# panel with the keyboard, the same shortcut hides it, a search and Enter
# open an app and hide the panel, Esc hides it, there is no Dock icon,
# and Quit in the menu bar item ends the process.
#
# It runs on a library of its own (config.example.toml with Calculator
# added), never on yours: your data folder is set aside first and put
# back at the end, and the hopto that was running is opened again.
#
# Usage: make build && bash scripts/verify-hopto.sh
# Exit status: 0 every check passed, 1 a check failed, 2 the Mac is in
# use or its screen is locked (try again later), 3 something it needs is
# missing.
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
bundle="$repo/build/bin/hopto.app"
source "$repo/scripts/real-app.sh"

# Seconds without a key or a click before the script starts: a person
# reading the screen between two keystrokes would otherwise get keys
# typed into their window.
min_idle=30

if [ ! -d "$bundle" ]; then
    echo "no build at $bundle: run make build first" >&2
    exit 3
fi

idle="$(idle_seconds)"
if [ "$idle" -lt "$min_idle" ]; then
    echo "the Mac is in use (idle ${idle} s, needs ${min_idle}): try again when it is free" >&2
    exit 2
fi

# A locked screen puts loginwindow in front, and no key would reach hopto.
if [ "$(front)" = "loginwindow" ]; then
    echo "the screen is locked: try again when it is unlocked" >&2
    exit 2
fi

keep="$(mktemp -d)"
previous="$(running_bundle)"
calculator_was_running=no
if pgrep -x Calculator > /dev/null; then
    calculator_was_running=yes
fi
failures=0

# check runs a condition and prints one line for it; a failure is counted.
check() {
    local name="$1"
    shift

    if "$@"; then
        echo "✓ $name"
    else
        echo "✗ $name" >&2
        failures=$((failures + 1))
    fi
}

# stop ends the run at once: after a failed guard, any further key would
# land in another app.
stop() {
    echo "✗ $1" >&2
    exit 1
}

# restore runs whatever happens: this build quits, the data comes back,
# Calculator closes if the script opened it, and the hopto that was
# running before is opened again.
restore() {
    pkill -x hopto 2>/dev/null || true
    sleep 1
    put_back "$keep"

    if [ "$calculator_was_running" = no ]; then
        osascript -e 'quit app "Calculator"' 2>/dev/null || true
    fi

    if [ -n "$previous" ]; then
        open "$previous"
    fi

    echo "your data is back (copy kept in $keep)"
}
trap restore EXIT

# shown_on holds once the page logged a shown event for the tab.
shown_on() {
    log_since | grep -q "page: shown \"$1\""
}

# launched holds once Go logged the launch of the demo's Calculator.
launched() {
    log_since | grep -q "launched calculator"
}

# calculator_running holds while a Calculator process exists.
calculator_running() {
    pgrep -x Calculator > /dev/null
}

# no_hopto holds once the process is gone.
no_hopto() {
    ! pgrep -x hopto > /dev/null
}

# background_only asks System Events whether hopto stays out of the Dock.
background_only() {
    [ "$(osascript -e 'tell application "System Events" to get background only of process "hopto"')" = "true" ]
}

pkill -x hopto 2>/dev/null || true
sleep 1
set_aside "$keep"
write_demo_library "$repo" Calculator

log_mark
open -n "$bundle"

check "the apps shortcut registered" wait_for 10 registered 1
check "the links shortcut registered" wait_for 10 registered 2
check "the panel starts hidden" windows_are 0

chord "$key_space" "command down, shift down"
check "⌘⇧Space shows the panel" wait_for 3 windows_are 1
wait_for 3 guard || stop "the panel showed without the keyboard"
check "the page was told to show apps" wait_for 3 shown_on apps

chord "$key_space" "command down, shift down"
check "⌘⇧Space again hides it" wait_for 3 windows_are 0

chord "$key_space" "command down, option down"
wait_for 3 guard || stop "⌘⌥Space did not reach hopto (macOS may keep it for the Finder search window)"
check "⌘⌥Space shows the links tab" wait_for 3 shown_on links

key "$key_escape"
check "Esc with an empty search hides it" wait_for 3 windows_are 0

chord "$key_space" "command down, shift down"
wait_for 3 guard || stop "the panel came back without the keyboard"
typed "calc"
guard || stop "hopto lost the keyboard while typing"
key "$key_return"
check "Enter opens Calculator" wait_for 5 launched
check "Calculator is running" wait_for 5 calculator_running
check "the panel hides after opening" wait_for 3 windows_are 0

check "no Dock icon" background_only

osascript -e 'tell application "System Events" to tell process "hopto" to click menu bar item 1 of menu bar 2'
sleep 0.5
osascript -e 'tell application "System Events" to tell process "hopto" to click menu item 6 of menu 1 of menu bar item 1 of menu bar 2'
check "Quit in the menu bar item ends hopto" wait_for 3 no_hopto

if [ "$failures" -gt 0 ]; then
    echo "$failures check(s) failed; what hopto logged:" >&2
    log_since >&2
    exit 1
fi

echo "every check passed"
