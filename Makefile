# hopto build and check targets. `wails build` runs npm install and npm
# run build itself (see wails.json), and it is what writes frontend/wailsjs
# and frontend/dist, which go:embed needs — running npm separately first
# would fail on a fresh clone before wailsjs exists.
.PHONY: build build-windows build-windows-dist test e2e fmt lint ci hooks clean dist

# What `make build` stamps into the binary for the help panel: what git
# describes (the tag, or the commit, plus -dirty for local changes), the
# full commit and the build time in UTC. `wails build` on its own leaves
# them empty and the panel says "dev".
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null)
BUILT ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# The package that holds the stamps, the App's help panel.
STAMP := github.com/drolosoft/hopto/internal/app
LDFLAGS := -X $(STAMP).version=$(VERSION) -X $(STAMP).commit=$(COMMIT) -X $(STAMP).builtAt=$(BUILT)

# wails build -clean empties frontend/dist, the tracked gitkeep included;
# every recipe that builds or cleans runs this afterwards to put that
# empty file back and keep the tree clean. A variable and not a target of
# its own so it stays one line of the recipe that needs it, in the place
# where the build leaves the folder empty.
RESTORE_GITKEEP := touch frontend/dist/gitkeep

build:
	wails build -clean -ldflags "$(LDFLAGS)"
	$(RESTORE_GITKEEP)

# The Windows exes, both architectures, cross-compiled from here: Wails
# is pure Go on Windows, so none of their toolchain is needed. The first
# build cleans build/bin and the frontend; the second keeps the frontend
# the first one just built.
build-windows:
	wails build -clean -trimpath -platform windows/arm64 -ldflags "$(LDFLAGS)" -o hopto-windows-arm64.exe
	wails build -trimpath -platform windows/amd64 -ldflags "$(LDFLAGS)" -o hopto-windows-amd64.exe
	$(RESTORE_GITKEEP)

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

# The Windows zips: the exe and the licence at the root, one per
# architecture, with the same $HOME check as the Mac binary. dist depends
# on this one, and not the other way round, because build-windows cleans
# build/bin: the exes have to be built before the universal .app, or the
# .app would be wiped out. The loop runs under set -e: make only sees the
# exit status of the last command, so a zip that failed for amd64 would
# otherwise be hidden by a good arm64.
build-windows-dist: build-windows
	mkdir -p $(DIST)
	set -e; for arch in amd64 arm64; do \
	  test -z "$$(strings build/bin/hopto-windows-$$arch.exe | grep -F "$$HOME")" || exit 1; \
	  rm -f $(DIST)/hopto-$(VERSION)-windows-$$arch.zip $(DIST)/hopto-$(VERSION)-windows-$$arch.zip.sha256; \
	  zip -j -q $(DIST)/hopto-$(VERSION)-windows-$$arch.zip build/bin/hopto-windows-$$arch.exe LICENSE; \
	  (cd $(DIST) && shasum -a 256 hopto-$(VERSION)-windows-$$arch.zip > hopto-$(VERSION)-windows-$$arch.zip.sha256); \
	done

# The whole release: the Windows zips first, then the universal .app.
dist: build-windows-dist
	wails build -clean -trimpath -platform darwin/universal -ldflags "$(LDFLAGS)"
	$(RESTORE_GITKEEP)
	test -z "$$(strings build/bin/hopto.app/Contents/MacOS/hopto | grep -F "$$HOME")"
	mkdir -p $(DIST)
	rm -f $(ZIP) $(ZIP).sha256
	ditto -c -k --keepParent build/bin/hopto.app $(ZIP)
	cd $(DIST) && shasum -a 256 $(notdir $(ZIP)) > $(notdir $(ZIP)).sha256

# The formatting check on its own: it needs nothing installed, so the CI
# test job can run it without golangci-lint, which has a job of its own.
fmt:
	test -z "$$(gofmt -l .)"

lint: fmt
	golangci-lint run ./...

# What the CI test job runs, in its order: the build writes the bindings
# and the page that test (go vet) and e2e need. The workflow calls this
# target so the two cannot drift apart.
ci: fmt build test e2e

# Installs the pre-commit guard so internal files never reach a commit.
# git rev-parse --git-path resolves to the right hooks folder in a
# worktree too, where .git is a file pointing elsewhere, not a folder.
hooks:
	printf '#!/usr/bin/env bash\nexec bash scripts/check-internal-files.sh\n' > "$$(git rev-parse --git-path hooks)/pre-commit"
	chmod +x "$$(git rev-parse --git-path hooks)/pre-commit"

clean:
	rm -rf build/bin frontend/dist/* frontend/wailsjs
	$(RESTORE_GITKEEP)
