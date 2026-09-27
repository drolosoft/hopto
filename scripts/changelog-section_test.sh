#!/usr/bin/env bash
# Tests for changelog-section.sh. Run: bash scripts/changelog-section_test.sh
set -u
script="$(dirname "$0")/changelog-section.sh"
fixture="$(mktemp)"
trap 'rm -f "$fixture"' EXIT
pass=0
fail=0

cat > "$fixture" << 'CHANGELOG'
# Changelog

---

## [Unreleased]

### Added
- Something not released yet.

## [v0.2.0] - 2026-10-10

### Fixed
- A fix.

## [v0.1.0] - 2026-09-28

### Added
- The first release.

### Fixed
- An early fix.

## [v0.0.1] - 2026-09-01

CHANGELOG

# expect compares what the script prints for a version with the text
# given, and counts the result.
expect() {
    local version="$1"
    local want="$2"
    local got
    got="$(bash "$script" "$version" "$fixture" 2>/dev/null)"

    if [ "$got" = "$want" ]; then
        pass=$((pass + 1))
    else
        echo "  ✗ $version: got <<$got>>"
        fail=$((fail + 1))
    fi
}

# must_fail checks that the script refuses a version.
must_fail() {
    if bash "$script" "$1" "$fixture" > /dev/null 2>&1; then
        echo "  ✗ $1: should have failed"
        fail=$((fail + 1))
    else
        pass=$((pass + 1))
    fi
}

echo "▶ a section comes out without its heading or the blank lines around it:"
expect v0.1.0 "$(printf '### Added\n- The first release.\n\n### Fixed\n- An early fix.')"
expect v0.2.0 "$(printf '### Fixed\n- A fix.')"
expect Unreleased "$(printf '### Added\n- Something not released yet.')"

echo "▶ a version that is only the start of another one does not match it:"
must_fail v0.1

echo "▶ a missing version and an empty section are refused:"
must_fail v9.9.9
must_fail v0.0.1

echo "── $pass passed, $fail failed"
[ $fail -eq 0 ]
