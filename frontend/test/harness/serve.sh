#!/usr/bin/env bash
# Serves the built page (frontend/dist) with the fake bridge injected, on
# http://localhost:8765, so the DOM logic can be tried in any browser or
# driven with Playwright without Wails. Build first (`wails build`).
# Stop it with Ctrl-C or `pkill -f 'http.server 8765'`.
set -euo pipefail

cd "$(dirname "$0")/../.."

harness=$(mktemp -d)
cp -R dist/. "$harness"
cp test/harness/fake-bridge.js "$harness/fake-bridge.js"

# The bridge has to exist before the page's module runs.
sed -i '' 's|<script type="module"|<script src="./fake-bridge.js"></script><script type="module"|' "$harness/index.html"

echo "Harness at http://localhost:8765 (from $harness)"
cd "$harness" && python3 -m http.server 8765
