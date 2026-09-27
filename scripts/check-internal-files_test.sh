#!/usr/bin/env bash
# Tests for check-internal-files.sh. Run: bash scripts/check-internal-files_test.sh
set -u
guard_script="$(dirname "$0")/check-internal-files.sh"
pass=0; fail=0
must_block() { if bash "$guard_script" --files "$1" >/dev/null 2>&1; then echo "  ✗ NOT blocked: $1"; fail=$((fail+1)); else pass=$((pass+1)); fi; }
must_allow() { if bash "$guard_script" --files "$1" >/dev/null 2>&1; then pass=$((pass+1)); else echo "  ✗ wrongly blocked: $1"; fail=$((fail+1)); fi; }

echo "▶ every internal working file this repo could grow must be blocked:"
while IFS= read -r file; do must_block "$file"; done << 'LEAKED'
.claude/commands/release-hopto.md
.superpowers/sdd/2026-09-26-hopto-1-cimientos/plan-path
CLAUDE.md
CONTINUE.md
docs/superpowers/plans/2026-09-26-hopto-1-cimientos.md
docs/prompts/v1-hopto-launch.md
reports/daily_report_2026-09-26.md
outputs/eval-schedule.md
hopto-launch-plan.md
crex-launch-session-state.md
daily-dev-post-draft.md
changelog-2026-09-26.md
LEAKED

echo "▶ the owner's personal library and its exports must never leak in:"
while IFS= read -r file; do must_block "$file"; done << 'LEAKED_LIBRARY'
library.toml
library.toml.bak
config.local.toml
icons/github-light.png
LEAKED_LIBRARY

echo "▶ legitimate public files must pass:"
while IFS= read -r file; do must_allow "$file"; done << 'PUBLIC'
README.md
CHANGELOG.md
CONTRIBUTING.md
SECURITY.md
ARCHITECTURE.md
config.example.toml
internal/library/store.go
scripts/check-internal-files.sh
seed/seed.toml
testdata/edge-info.plist
frontend/src/assets/icons/hopto.svg
.github/workflows/ci.yml
doc/architecture.md
doc/configuration.md
doc/testing.md
assets/icon.png
assets/demo.gif
scripts/real-app.sh
scripts/verify-hopto.sh
scripts/record-demo.sh
scripts/changelog-section.sh
.github/ISSUE_TEMPLATE/bug_report.yml
.github/pull_request_template.md
.github/workflows/release.yml
PUBLIC

echo "▶ links.json is the old launcher's bookmarks file, not the public seed:"
must_block links.json

echo "▶ a root .md that matches no named pattern still falls through to the allow-list check:"
must_block NOTES.md

echo "▶ the whole tracked tree must be clean right now:"
if bash "$guard_script" --tree; then pass=$((pass+1)); else echo "  ✗ tracked tree contains internal files"; fail=$((fail+1)); fi

echo "── $pass passed, $fail failed"
[ $fail -eq 0 ]
