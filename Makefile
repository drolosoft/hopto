# hopto build and check targets. `wails build` runs npm install and npm
# run build itself (see wails.json), and it is what writes frontend/wailsjs
# and frontend/dist, which go:embed needs — running npm separately first
# would fail on a fresh clone before wailsjs exists.
.PHONY: build test e2e lint hooks clean

# What `make build` stamps into the binary for the help panel: what git
# describes (the tag, or the commit, plus -dirty for local changes), the
# full commit and the build time in UTC. `wails build` on its own leaves
# them empty and the panel says "dev".
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null)
BUILT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.builtAt=$(BUILT)

build:
	wails build -clean -ldflags "$(LDFLAGS)"

test:
	cd frontend && npm test
	go vet ./...
	go test -race -count=1 -cover ./...

# Browser tests against the built page; needs frontend/dist (make build).
e2e:
	cd frontend && npm run test:e2e

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
