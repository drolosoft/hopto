# hopto build and check targets. `wails build` runs npm install and npm
# run build itself (see wails.json), and it is what writes frontend/wailsjs
# and frontend/dist, which go:embed needs — running npm separately first
# would fail on a fresh clone before wailsjs exists.
.PHONY: build test e2e lint hooks clean dist

# What `make build` stamps into the binary for the help panel: what git
# describes (the tag, or the commit, plus -dirty for local changes), the
# full commit and the build time in UTC. `wails build` on its own leaves
# them empty and the panel says "dev".
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null)
BUILT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.builtAt=$(BUILT)

# wails build -clean empties frontend/dist, the tracked gitkeep included;
# the touch puts that empty file back so the tree stays clean.
build:
	wails build -clean -ldflags "$(LDFLAGS)"
	touch frontend/dist/gitkeep

test:
	cd frontend && npm test
	go vet ./...
	go test -race -count=1 -cover ./...

# Browser tests against the built page; needs frontend/dist (make build).
e2e:
	cd frontend && npm run test:e2e

# The release artefact, in build/dist: a universal .app (Apple silicon and
# Intel) zipped the way the Finder does it, and its SHA-256. -trimpath
# keeps the folders of the machine that builds it, its user name
# included, out of the binary, and the check after the build refuses a
# binary that still carries $HOME. A tag builds
# hopto-v0.1.0-macos-universal.zip.
DIST := build/dist
ZIP := $(DIST)/hopto-$(VERSION)-macos-universal.zip

dist:
	wails build -clean -trimpath -platform darwin/universal -ldflags "$(LDFLAGS)"
	touch frontend/dist/gitkeep
	test -z "$$(strings build/bin/hopto.app/Contents/MacOS/hopto | grep -F "$$HOME")"
	mkdir -p $(DIST)
	rm -f $(ZIP) $(ZIP).sha256
	ditto -c -k --keepParent build/bin/hopto.app $(ZIP)
	cd $(DIST) && shasum -a 256 $(notdir $(ZIP)) > $(notdir $(ZIP)).sha256

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
