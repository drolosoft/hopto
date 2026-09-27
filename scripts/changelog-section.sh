#!/usr/bin/env bash
# changelog-section.sh: prints the body of one version's section of
# CHANGELOG.md, for the notes of a GitHub release.
#
# Usage: scripts/changelog-section.sh v0.1.0 [CHANGELOG.md]
# The section runs from the heading "## [v0.1.0]" (a date may follow it)
# to the next "## [" heading; the headings and the blank lines around the
# body are left out. A missing or empty section is an error, so a release
# never goes out with empty notes.
set -euo pipefail

version="${1:?usage: $0 <version> [changelog]}"
changelog="${2:-CHANGELOG.md}"

# index() compares plain text, so the brackets of the heading need no
# escaping, and "## [v0.1]" can never match "## [v0.1.0]".
body="$(awk -v heading="## [$version]" '
    index($0, heading) == 1 { inside = 1; next }
    inside && /^## \[/ { exit }
    inside { lines[++count] = $0 }
    END {
        first = 1
        while (first <= count && lines[first] == "") first++
        last = count
        while (last >= first && lines[last] == "") last--
        for (line = first; line <= last; line++) print lines[line]
    }
' "$changelog")"

if [ -z "$body" ]; then
    echo "no section, or an empty one, for $version in $changelog" >&2
    exit 1
fi

printf '%s\n' "$body"
