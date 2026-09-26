# hopto build and check targets. `wails build` runs npm install and npm
# run build itself (see wails.json), and it is what writes frontend/wailsjs
# and frontend/dist, which go:embed needs — running npm separately first
# would fail on a fresh clone before wailsjs exists.
.PHONY: build test lint hooks clean

build:
	wails build -clean

test:
	cd frontend && npm test
	go vet ./...
	go test -race -count=1 -cover ./...

lint:
	test -z "$$(gofmt -l .)"
	golangci-lint run ./...

# Installs the pre-commit guard so internal files never reach a commit.
# git rev-parse --git-path resolves to the right hooks folder in a
# worktree too, where .git is a file pointing elsewhere, not a folder.
hooks:
	printf '#!/usr/bin/env bash\nexec bash scripts/check-internal-files.sh\n' > "$$(git rev-parse --git-path hooks)/pre-commit"
	chmod +x "$$(git rev-parse --git-path hooks)/pre-commit"

clean:
	rm -rf build/bin frontend/dist/* frontend/wailsjs
	touch frontend/dist/gitkeep
