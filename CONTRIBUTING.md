# Contributing to hopto

Contributions are welcome: bug fixes, translations, documentation, better defaults.

## Getting Started

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run the tests: `make test`, and `make build && make e2e` when the page changed
5. Run the linters: `make lint`
6. Commit your changes
7. Open a pull request

## Development Setup

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
git clone https://github.com/YOUR-USERNAME/hopto.git
cd hopto
make hooks   # installs the pre-commit guard described below
make build
make test
open -n build/bin/hopto.app
```

Requires macOS 12 or later, Go 1.27, Node 22, the Wails CLI v2.16.0 and the Xcode Command Line Tools (cgo builds the Carbon and Cocoa parts).

Always start the app with `open -n`. A binary run from the shell (`build/bin/hopto.app/Contents/MacOS/hopto`) never gets the keyboard and dies with the shell. Only one hopto runs at a time (the same shortcuts, and a single-instance lock), so quit the installed copy from its menu bar item first.

A development build uses the same data folder as the installed app, `~/Library/Application Support/hopto/`. Copy it before trying anything that writes:

```bash
cp -Rp ~/Library/Application\ Support/hopto /tmp/hopto-backup
```

### What belongs in the repository

This is a public repository. Only the product ships here: source, tests, user documentation, release tooling. Working material does not: planning notes, specs, prompt files, drafts, reports, editor or agent configuration, and above all a personal `library.toml` or its `icons/`. `scripts/check-internal-files.sh` enforces this as a pre-commit hook (`make hooks`) and in CI, so a stray note fails the build instead of landing in every fork's history. If it flags a file that is genuinely public, add it to the allow-list in that script with a one-line reason.

### About `wails dev`

`wails dev` serves the page on http://localhost:34115 with live reload. Its websocket accepts connections from any origin (Wails v2.16.0 does not check it), and through it a page can call every Go method the panel can, writing your library included. Keep `wails dev` sessions short and do not browse other sites while one runs. The built app has no such server.

## Code Style

- Go: `gofmt`, `go vet` and `golangci-lint run` clean (`make lint`), lines up to 78 columns, a doc comment on every function, comments in British English that say why.
- Page: plain JavaScript modules in `frontend/src`, 4 spaces, single quotes, `const` and `let`, a JSDoc block on every function. No `innerHTML`, `outerHTML` or `insertAdjacentHTML` (`frontend/test/dom-rules.test.js` checks it) and no inline script in `index.html`.
- Every control that takes a click or a key gets `--wails-draggable: no-drag`. The panel is a drag region; without it a click moves the window instead of reaching the control.
- Every new piece of page state starts in `initialState`, which the `shown` event calls: the panel must never come back with last time's search, chip or editor.
- Every text of the page lives in `frontend/src/i18n.js`, in English and Spanish; a test compares the two sets of keys.
- The conventions the code follows, measured, are in the [style card](ARCHITECTURE.md#style-card). A change should read like the code around it.
- Keep commits focused, with an English message and a prefix: `fix:`, `feat:`, `docs:`, `refactor:`, `test:`, `build:`, `chore:`.

## Testing

```bash
make test     # node --test for the pure page modules, go vet, go test -race
make build    # the .app, and frontend/dist for the browser tests
make e2e      # Playwright WebKit against the built page and a fake Go bridge
make lint     # gofmt and golangci-lint
```

The browser tests run the real page in WebKit, the engine of the app's WKWebView, with `frontend/test/harness/fake-bridge.js` in place of Go: each call the page makes lands in `window.calls`, and `window.emit('links')` plays the `shown` event. See [doc/testing.md](doc/testing.md).

A change to the window, the shortcuts or anything native is done when it has been seen in the real app. `scripts/verify-hopto.sh` drives the build from the shell on a library of its own, refuses to run while you are using the Mac, and puts your data back afterwards. On Windows, `scripts/verify-hopto-windows.ps1` does the same from PowerShell; anything native to Windows (the tray, the shortcuts, the window) is done when it has been seen there. [doc/testing.md](doc/testing.md) explains how to prove each thing with the log, `count windows` or a screenshot, and why every key a script sends first checks that hopto is in front.

## Pull Request Guidelines

- Keep PRs focused on a single change
- Include tests for new behaviour: a Go test for the logic, a Playwright test for what the page does
- Update the documentation and the CHANGELOG when behaviour changes
- Fill in the checklist of the pull request template
- Reference any related issues

## Bug Reports

Use [GitHub Issues](https://github.com/drolosoft/hopto/issues) with:
- Steps to reproduce
- Expected and actual behaviour
- The hopto version (press `?` in the panel: the help shows it with the commit)
- The macOS version and the Mac (Apple silicon or Intel)
- The relevant lines of `~/Library/Logs/hopto.log`, without your own links
