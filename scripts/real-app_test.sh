#!/usr/bin/env bash
# Tests for the set_aside/put_back safety net in real-app.sh: put_back
# must never remove the data folder unless set_aside actually finished
# backing it up first. Run: bash scripts/real-app_test.sh
#
# Every case runs against a private $HOME under /tmp, never the real
# one, and checks that $data lands under it before touching anything.
set -u

pass=0
fail=0

# real_app is where the helpers under test live, resolved once so both
# cases can source it after pointing HOME at their own sandbox.
real_app="$(cd "$(dirname "$0")" && pwd)/real-app.sh"

# refuse_unless_sandboxed exits the whole run rather than risking a real
# folder: a bug in the setup below must never fall through into $data
# meaning the user's actual Application Support folder.
refuse_unless_sandboxed() {
    local home="$1"

    if [[ "$data" != "$home"/* ]]; then
        echo "  ✗ refusing to run: \$data is not under the sandbox HOME" >&2
        exit 1
    fi
}

# case_survives_without_marker calls put_back as if the script had died
# before set_aside ever ran (no "$keep/aside" marker): the folder must
# come out exactly as it went in.
case_survives_without_marker() {
    local home keep
    home="$(mktemp -d)"
    export HOME="$home"

    # Sourced here, not at file scope, so $data is computed against this
    # case's own sandbox HOME rather than a HOME from an earlier case.
    # shellcheck source=real-app.sh
    source "$real_app"
    refuse_unless_sandboxed "$home"

    mkdir -p "$data"
    echo "original" > "$data/library.toml"
    keep="$(mktemp -d)"

    put_back "$keep"

    if [ -f "$data/library.toml" ] && [ "$(cat "$data/library.toml")" = "original" ]; then
        pass=$((pass + 1))
    else
        echo "  ✗ put_back touched data although set_aside never ran" >&2
        fail=$((fail + 1))
    fi

    rm -rf "$home" "$keep"
}

# case_restores_after_set_aside runs the pair as intended: set_aside
# copies the folder away and marks the job done, put_back brings it back
# byte for byte.
case_restores_after_set_aside() {
    local home keep
    home="$(mktemp -d)"
    export HOME="$home"

    # shellcheck source=real-app.sh
    source "$real_app"
    refuse_unless_sandboxed "$home"

    mkdir -p "$data/icons"
    echo "original" > "$data/library.toml"
    echo "icon" > "$data/icons/example.png"
    keep="$(mktemp -d)"

    set_aside "$keep"

    if [ -d "$data" ]; then
        echo "  ✗ set_aside left the folder in place" >&2
        fail=$((fail + 1))
        rm -rf "$home" "$keep"
        return
    fi

    put_back "$keep"

    if diff -rq "$keep/data" "$data" > /dev/null 2>&1; then
        pass=$((pass + 1))
    else
        echo "  ✗ put_back did not restore byte-identical data" >&2
        fail=$((fail + 1))
    fi

    rm -rf "$home" "$keep"
}

# case_survives_partial_set_aside simulates a set_aside that died midway
# (the diff never ran, so no marker exists): the original must still be
# there, untouched, because put_back has nothing proven to restore from.
case_survives_partial_set_aside() {
    local home keep
    home="$(mktemp -d)"
    export HOME="$home"

    # shellcheck source=real-app.sh
    source "$real_app"
    refuse_unless_sandboxed "$home"

    mkdir -p "$data"
    echo "original" > "$data/library.toml"
    keep="$(mktemp -d)"

    # A half-written copy, as cp -Rp failing partway would leave one,
    # with no "$keep/aside" marker beside it.
    mkdir -p "$keep/data"
    echo "partial" > "$keep/data/library.toml"

    put_back "$keep"

    if [ -f "$data/library.toml" ] && [ "$(cat "$data/library.toml")" = "original" ]; then
        pass=$((pass + 1))
    else
        echo "  ✗ put_back restored a partial copy over the original" >&2
        fail=$((fail + 1))
    fi

    rm -rf "$home" "$keep"
}

echo "▶ put_back refuses to touch data when set_aside never ran:"
case_survives_without_marker

echo "▶ set_aside then put_back restores the folder byte for byte:"
case_restores_after_set_aside

echo "▶ a half-finished set_aside (no marker) never overwrites the original:"
case_survives_partial_set_aside

echo "── $pass passed, $fail failed"
[ "$fail" -eq 0 ]
