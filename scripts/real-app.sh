#!/usr/bin/env bash
# real-app.sh: shell helpers to drive the real hopto.app, sourced by
# verify-hopto.sh and record-demo.sh. doc/testing.md explains each one.
#
# Keys go through System Events and land in whatever app has the
# keyboard. Every key after a global shortcut goes through `guard`
# first, so a script stops instead of typing into your window when you
# click elsewhere while it runs.

# hopto's data folder and log, the same paths the app uses.
data="$HOME/Library/Application Support/hopto"
logfile="$HOME/Library/Logs/hopto.log"

# The key codes the scripts send (Carbon's kVK_ values).
key_space=49
key_return=36
key_escape=53
key_tab=48
key_n=45

# front prints the name of the app that has the keyboard. System Events'
# "frontmost" is always false for an accessory app like hopto, so this
# asks NSWorkspace instead.
front() {
    osascript -l JavaScript -e 'ObjC.import("AppKit"); $.NSWorkspace.sharedWorkspace.frontmostApplication.localizedName.js'
}

# guard fails, and says why, unless hopto has the keyboard.
guard() {
    local app
    app="$(front)"

    if [ "$app" != "hopto" ]; then
        echo "STOP: $app is in front, not hopto" >&2
        return 1
    fi
}

# windows prints how many windows hopto has on screen: 1 shown, 0 hidden.
windows() {
    osascript -e 'tell application "System Events" to tell process "hopto" to count windows'
}

# windows_are holds when hopto has that many windows on screen.
windows_are() {
    [ "$(windows)" = "$1" ]
}

# key sends one key; chord sends it with modifiers, such as
# "command down, shift down". Two functions rather than one with an
# optional argument, which zsh expands differently when pasted there.
key() {
    osascript -e "tell application \"System Events\" to key code $1"
}

chord() {
    osascript -e "tell application \"System Events\" to key code $1 using {$2}"
}

# typed types text one character at a time, at about the pace of a
# person, so a recording shows the search narrowing.
typed() {
    local text="$1"
    local index

    for ((index = 0; index < ${#text}; index++)); do
        osascript -e "tell application \"System Events\" to keystroke \"${text:index:1}\""
        sleep 0.08
    done
}

# idle_seconds prints how long the keyboard and the mouse have been
# untouched. awk keeps reading to the end of ioreg's output instead of
# exiting on the first match: under pipefail, an early exit here sends
# ioreg a SIGPIPE, which the callers' set -e would otherwise abort on.
idle_seconds() {
    ioreg -c IOHIDSystem | awk '/HIDIdleTime/ && !seen {print int($NF / 1000000000); seen = 1}'
}

# wait_for runs a command every quarter of a second until it succeeds or
# the timeout, in seconds, runs out; it returns the command's last answer.
wait_for() {
    local timeout="$1"
    shift
    local tries=$((timeout * 4))

    while [ "$tries" -gt 0 ]; do
        if "$@"; then
            return 0
        fi

        sleep 0.25
        tries=$((tries - 1))
    done

    "$@"
}

# log_mark remembers where the log ends now; log_since prints what hopto
# logged after that.
log_mark() {
    log_start=$(wc -l < "$logfile" 2>/dev/null || echo 0)
}

log_since() {
    tail -n +$((log_start + 1)) "$logfile"
}

# registered holds once the log says the shortcut with that id (1 apps,
# 2 links) registered since the mark.
registered() {
    log_since | grep -q "hotkey $1 registered"
}

# running_bundle prints the .app of the hopto running now, or nothing, so
# a script can open the same copy again when it ends.
running_bundle() {
    local pid binary
    pid="$(pgrep -x hopto | head -1 || true)"

    if [ -n "$pid" ]; then
        binary="$(ps -o comm= -p "$pid")"
        echo "${binary%/Contents/MacOS/hopto}"
    fi
}

# set_aside copies the data folder into $1/data, checks the copy, and
# only then removes the original, so hopto can start on a library of the
# script's own. It marks the job done only once that is true, so a
# script that dies partway through (an osascript call, ffmpeg, a failed
# wait_for) never leaves put_back thinking there is a safe copy to fall
# back on. put_back restores it exactly as it was.
set_aside() {
    local keep="$1"

    mkdir -p "$keep"

    if [ -d "$data" ]; then
        cp -Rp "$data" "$keep/data"
        diff -rq "$data" "$keep/data" > /dev/null
        rm -rf "$data"
    fi

    # Proves the steps above ran to completion (or there was nothing to
    # save in the first place); put_back refuses to touch $data without
    # this marker.
    touch "$keep/aside"
}

put_back() {
    local keep="$1"

    # Without the marker, set_aside either never ran or died partway
    # through, so $keep/data cannot be trusted: removing $data now would
    # either destroy the only copy or replace it with a partial one.
    if [ ! -f "$keep/aside" ]; then
        return 0
    fi

    rm -rf "$data"

    if [ -d "$keep/data" ]; then
        mkdir -p "$data"
        rsync -a --delete "$keep/data/" "$data/"
    fi
}

# write_demo_library writes a library.toml for a script: the example of
# the repository with both scans off (so nothing of this Mac shows), the
# English texts, the main display, and the apps of /System/Applications
# named in the arguments, each with the icon of its bundle.
write_demo_library() {
    local repo="$1"
    shift
    local name id

    mkdir -p "$data/icons"
    chmod 700 "$data"

    sed -e 's/^language = .*/language = "en"/' \
        -e 's/^screen = .*/screen = "main"/' \
        -e 's/^scan_applications = .*/scan_applications = false/' \
        -e 's/^discover_edge_apps = .*/discover_edge_apps = false/' \
        "$repo/config.example.toml" > "$data/library.toml"

    for name in "$@"; do
        id="$(echo "$name" | tr 'A-Z ' 'a-z-')"
        printf '\n[[apps]]\nid = "%s"\nname = "%s"\npath = "/System/Applications/%s.app"\ncategory = "tools"\n' \
            "$id" "$name" "$name" >> "$data/library.toml"
        sips -s format png -Z 256 \
            "/System/Applications/$name.app/Contents/Resources/AppIcon.icns" \
            --out "$data/icons/$id.png" > /dev/null
    done

    chmod 600 "$data/library.toml"
}
