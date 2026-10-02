# Testing hopto

Everything said about hopto is checked by running it. This page covers the three test suites, the page without Wails, and driving the real app from the shell without touching the mouse.

## The suites

```bash
make test     # node --test for the pure page modules, go vet, go test -race -cover
make build    # the .app, and frontend/dist for the browser tests
make e2e      # Playwright WebKit against the built page
make lint     # gofmt and golangci-lint
```

- **Go**: table tests for validation, ids, duplicates, shortcuts and the store (a missing, empty, broken, huge, future or unreadable file), the atomic writes (a failure at every step leaves the previous file and no temporary one), usage, icons against `httptest` servers (redirect loops, huge bodies, HTML served as PNG, private addresses), `.icns` parsing, discovery over fake bundles in a temp folder, and the App with a fake window and a fake `open` (every case of the shortcuts, of opening, of the dialog, of the menu). CI runs them with `-race`.
- **Page units**: `node --test` over `state.js`, `filter.js`, `keys.js`, `draft.js`, `hotkeys.js` and `i18n.js`, which touch neither the DOM nor Wails, plus a check that no module writes HTML strings.
- **Browser**: Playwright drives the built page in WebKit, the engine of the WKWebView, with a fake Go bridge, and fails any test that raised a page error or a Content Security Policy violation.

`make e2e` needs `frontend/dist` from a build. `wails build` removes `frontend/dist/gitkeep`, which `make build` puts back.

## The page without Wails

`frontend/test/harness/serve.js` serves `frontend/dist` with `fake-bridge.js` injected before the page's own module, on a free port for Playwright, or on http://localhost:8765 when run directly (`node frontend/test/harness/serve.js`). The bridge is a `window.go.main.App` with fixed data:

- every Go call the page makes is appended to `window.calls` (`OpenLink:mdn`, `AddLink:{…}`);
- `window.emit('links')` plays the `shown` event, `window.listeners.help(true)` the menu bar's Help;
- a test can arm answers before acting: `window.nextSave` (the next save's result), `window.nextPick` and `window.pickDelay` (the file dialog), `window.inspectDraft` and `window.inspectDelay` (reading a page), `window.libraryStatus` (a broken file), `window.welcome`, `window.language`.

It is the way to test the page's logic (order, chips, stars, keys, the editor) when the real app cannot be driven. It does not replace the real app for anything native: the window, focus, the shortcuts, the dialog.

## The real app from the shell

### Start it with `open`

```bash
make build
pkill -x hopto; open -n build/bin/hopto.app
```

Always with `open`. A binary run directly (`build/bin/hopto.app/Contents/MacOS/hopto`) is not activated as an app, never gets the keyboard, and dies with the shell. Only one hopto runs at a time: quit the installed copy first (menu bar item, Quit hopto, or `pkill -x hopto`), and open it again at the end.

A build uses your real data folder. Copy it before a test that writes, and put it back after:

```bash
cp -Rp ~/Library/Application\ Support/hopto /tmp/hopto-backup
# ... the test ...
rsync -a --delete /tmp/hopto-backup/ ~/Library/Application\ Support/hopto/
```

### Keys, with a guard

`scripts/real-app.sh` has the helpers (source it into a bash shell). Keys go through System Events:

```bash
osascript -e 'tell application "System Events" to key code 49 using {command down, shift down}'  # ⌘⇧Space
osascript -e 'tell application "System Events" to key code 36'                                   # Return
osascript -e 'tell application "System Events" to key code 53'                                   # Esc
osascript -e 'tell application "System Events" to keystroke "calc"'
```

A global shortcut reaches hopto whatever app is in front. Every other key goes to the app that has the keyboard, so **check which app is in front right before each key**, or the key lands in your editor or your browser. System Events' `frontmost` does not work for hopto: it is always false for an accessory app. Ask NSWorkspace instead:

```bash
osascript -l JavaScript -e 'ObjC.import("AppKit"); $.NSWorkspace.sharedWorkspace.frontmostApplication.localizedName.js'
```

It prints `hopto` while hopto has the keyboard. `guard` in `real-app.sh` wraps it and fails otherwise.

### Clicks: cliclick, never System Events

```bash
cliclick c:660,516
```

System Events' `click at {x, y}` fails silently or with error -25208 most of the time, and its failures look like "the app ignores the click". `cliclick` (`brew install cliclick`) clicks for real. Move the window to the main display first: coordinates on a secondary display can be negative.

### Proof that something happened

- **Windows**: `osascript -e 'tell application "System Events" to tell process "hopto" to count windows'` is 1 with the panel shown and 0 hidden. `get {position, size} of first window` gives its place in points.
- **The log**, `~/Library/Logs/hopto.log`, better than reading text through accessibility (it changes while the page loads). Mark where it ends before acting (`wc -l`) and read what came after. Lines worth knowing: `hotkey 1 registered`, `hotkey 2: RegisterEventHotKey failed with status …`, `toggle apps: visible=false tab=apps`, `page: shown "links"`, `launched <id>: …`, `opened link <id>: <url>`, `page: window blur`, `toggle ignored: a dialog is open`, `reopen: hopto launched again`, `menu: quit`.
- **Processes**: `pgrep -x hopto` (still alive?), `pgrep -x Calculator` (did Enter open it?).
- **Screenshots**: `screencapture -x -R<x>,<y>,<w>,<h> shot.png` with the window's place; compare two with `md5 -q`, or look at them. On a Retina display the image has two pixels per point.
- **A link**: Enter on a link logs `opened link …`; the page in the browser is the second proof (`osascript -e 'tell application "Safari" to get URL of every tab of every window'`, every window, since the browser may open a new one).
- **No Dock icon**: `osascript -e 'tell application "System Events" to get background only of process "hopto"'` is `true`.

### When you are using the Mac

A test from the shell shares the keyboard and the focus with you. Click in another window while it runs and the panel hides (the log says `page: window blur` with no Esc before it), and every key sent after that goes to your window. Check the app in front before each key, and when the result does not match the log, run the sequence again before looking for a bug in the code. `idle_seconds` in `real-app.sh` says how long the Mac has been untouched; the scripts refuse to start under 30 seconds.

### `scripts/verify-hopto.sh`

The whole sequence as one script: it sets your data folder aside, starts the build on a library of its own (the example library with Calculator added), and checks that both shortcuts register, that each one shows the panel with the keyboard, that the same shortcut hides it, that `calc` and Enter open Calculator and hide the panel, that Esc hides it, that there is no Dock icon, and that Quit in the menu bar item ends the process. Then it puts your data back and opens the hopto you had running.

```bash
make build && bash scripts/verify-hopto.sh
```

Exit status: 0 every check passed, 1 a check failed (the log of the run follows), 2 the Mac is in use, 3 there is no build.

## The real app on Windows

Windows has no `osascript`; the checks use Win32 from PowerShell. The exe cross-compiles on the Mac (`make build-windows`), so the only thing a Windows machine (a VM does) is needed for is running it.

- **The window**: `FindWindow('hoptoWindow', $null)` finds it by the class name `main.go` sets; `IsWindowVisible` is the "count windows" of Windows, 1 shown and 0 hidden. `GetWindowLongPtr(h, -20)` with bit `0x80` set means no taskbar button.
- **Keys**: `keybd_event` sends a chord; `RegisterHotKey` sees synthetic keys. They only reach the desktop they are sent from, so the script runs inside the session, never over SSH.
- **The log**: `%LocalAppData%\hopto\hopto.log`; the lines are the same as on macOS, plus `native: tray icon added`.
- **The tray menu**: by hand, right-click the icon → Quit; `Get-Process hopto` empty and `menu 5 picked` in the log.
- **Open at login**: `Get-ItemProperty HKCU:\Software\Microsoft\Windows\CurrentVersion\Run | Select-Object hopto`.

`scripts/verify-hopto-windows.ps1 -Exe <path>` runs the shortcut, focus, taskbar and seed checks on a temporary profile and prints one line per check, like `verify-hopto.sh`.

## Before saying "done"

1. `make lint`, `make test`, and `make build && make e2e` when the page changed.
2. The change seen in the real app, with its proof in hand: a log line, `count windows`, a process, a screenshot.
3. For the window, the shortcuts or the menu: `scripts/verify-hopto.sh` passes.
4. Your data folder back as it was, and your own copy of hopto running again.
