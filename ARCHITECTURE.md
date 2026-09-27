# Architecture: hopto

hopto is a Go program with a Wails v2 window. Go owns the library, the scans, the icons and everything native; the panel is a small web page in a WKWebView that paints what Go hands it and asks Go to act, by id.

> Internal documentation for contributors and anyone who wants to understand or extend the project. How each macOS piece works in detail (window options, Carbon, Spaces, rounded corners) is in [doc/architecture.md](doc/architecture.md).

## How It Works

```
 Carbon hotkeys           menu bar item            a second launch
 (hotkey_darwin.go)       (statusbar_darwin.go)    (reopen, single instance)
        │                        │                        │
        └───────────┬────────────┴───────────┬────────────┘
                    ▼                        ▼
          ┌──────────────────────────────────────────┐
          │ App (package main)                       │
          │ show and hide, open, edit, menu, about   │
          └────┬──────────────────▲─────────────┬────┘
   events      │                  │ bound       │ /usr/bin/open
   shown,      ▼                  │ methods,    ▼
   help,   ┌──────────────────────┴──┐ ids   apps, default browser,
   icons   │ page in a WKWebView     │ only  secondary browser
           │ frontend/src/*.js       │
           └─────────────────────────┘

 App uses  internal/library    library.toml, validation, ids, the store
           internal/usage      usage.json: counts, last opened, favourites
           internal/icons      icons/<id>.png, .icns, the /user-icons/ handler
           internal/discover   /Applications, Edge web apps, Info.plist
           internal/inspect    a page's name, description and icons
           internal/safehttp   the one HTTP client
           internal/atomicfile temp file, sync, rename, .bak
```

## Flows

### ⌨️ A shortcut

```
Carbon handler (main thread) → go onHotkey(tab)
  → App.toggle(tab), under the App's lock
    → a file dialog is open        → nothing
    → shown on this tab            → remember the display → Hide
    → hidden                       → showLocked(tab): place on its display
                                     → Show → Activate → event shown(tab)
    → shown on the other tab       → event shown(other tab)
  → page: fresh state for the tab → Items, Categories, Usage, Settings,
    LibraryStatus → search box focused
```

The menu bar item's Open and a launch of the running app (Alfred, `open -a hopto`, a double click) end in the same `showLocked`, but never hide a panel that is already up.

### 🙈 Hiding

Esc with an empty search, the same shortcut again, a successful open, or the window losing the keyboard (150 ms later, and only if `document.hasFocus()` still says so, the editor is closed and no dialog is up) → `Hide` → the display is saved in `window.json` for the next show.

### ↩ Opening

```
Enter on a row → Launch(id) or OpenLink(id)
  → Go finds the path, bundle id or URL of that id (library or scan)
  → open <path> | open -b <bundle id> | open <url>
  → count it in usage.json → Hide
```

### ＋ Adding a link

```
typed text with no match → "＋ Add" row → Enter → editor (draft.js)
  → InspectURL: GET through safehttp, 5 s, 1 MiB → name, description, icon
  → Enter → AddLink(input) → Store.Apply:
      read the file again if its bytes changed → id, duplicate and field
      checks on the current file, under the store's lock → Validate →
      write a temp file, keep the old one as .bak, rename
  → icon fetched in the background → event icons → repaint
```

### ✏️ A hand edit to library.toml

The next `shown` compares the file's bytes with the last ones read or written. Changed: parse and validate. Good: it becomes the library. Broken: the last good library stays in memory, `LibraryStatus` reports the error and its line, the panel shows a notice with a button that opens the file, and every write is refused until the file parses again.

## Package Structure

```
main.go               window options, single-instance lock, asset server with the icons handler
app.go                App: the window interface, toggle, showLocked, Hide, startup
app_views.go          Items, Categories, Settings, LibraryStatus: what the page gets (never nil)
app_open.go           Launch, OpenLink, OpenLinkWith, CopyTarget, RevealInFinder
app_edit.go           links, apps, hidden apps and categories; InspectURL; icon fetches
app_pick.go           PickApp: the native open panel; twins of an app
app_menu.go           the menu bar item's entries and what each one does
app_welcome.go        the first-run welcome and the state of each shortcut
app_reopen.go         a launch of the running app shows the panel
about.go              version, commit and build date stamped by make build
screen.go             which display the panel appears on; window.json
launchagent.go        Open at login: ~/Library/LaunchAgents/com.drolosoft.hopto.plist
hotkeyspec.go         "cmd+shift+space" to a Carbon key code and modifiers
hotkeystatus.go       what RegisterEventHotKey answered, for the welcome
symbolichotkeys.go    macOS's own shortcuts, to spot the Finder's ⌘⌥Space
hotkey_darwin.go      cgo: hotkeys, accessory policy, placing, Spaces, round corners, reopen
statusbar_darwin.go   cgo: NSStatusItem and its menu
*_other.go            stubs, so go vet and the pure tests run off macOS
language*.go          the system language (AppleLanguages)
logfile.go, paths.go  ~/Library/Logs/hopto.log; the data folder
internal/
  atomicfile/         write to a temp file, sync, rename; keep a .bak
  library/            types, validation, ids, duplicates, the store
  usage/              open counts, last opened, favourites
  discover/           /Applications, ~/Applications, /System/Applications, Edge web apps
  icons/              .icns reading, fetching, normalising to PNG, /user-icons/
  inspect/            a page's title, site name, description and icons
  safehttp/           https, time and size limits, no private addresses
seed/                 the first library, in English and Spanish
frontend/src/         the page:
  main.js             glue: the state, calls to Go, wiring
  state.js             the state and the texts derived from it
  filter.js            search, ranking, sections
  keys.js               key presses to actions
  draft.js               the editor's draft
  hotkeys.js              "cmd+shift+space" as "⇧⌘Space"
  i18n.js                English and Spanish
  tabs.js                what differs between the two tabs
  render.js, cards.js    the list, chips, footer and help
  editor.js              the editor panel
  editing.js             the editor's flow: open, inspect, save, close
  keyboard.js            routes key presses
  welcome.js             the first-run welcome
frontend/test/        node --test for the pure modules, Playwright specs, the fake-bridge harness
scripts/              internal-files guard, real-app helpers, demo recording, release notes, icon drawing
```

`state.js`, `filter.js`, `keys.js`, `draft.js`, `hotkeys.js` and `i18n.js` touch neither the DOM nor Wails, so `node --test` covers them without a browser.

## Key Design Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| Library format | One TOML file | Readable and editable by hand, easy to diff and to keep in git. JSON has no comments for the example; a database cannot be opened in a text editor. |
| A broken file | Never overwritten | A typo in a hand edit must not cost the links. The last good library stays in memory and every write waits until the file parses. |
| Writes | Temp file, sync, rename; `.bak` | A crash never leaves half a file, and the last good version is one `cp` away. The file is read again before every change, so a hand edit made while hopto runs is kept. |
| No Dock icon | Accessory policy set from code | A launcher is called with a shortcut and goes away. `LSUIElement` alone is overridden by Wails v2 at start. |
| Global shortcuts | Carbon `RegisterEventHotKey` | Works from any app, needs no Accessibility permission, and stays registered while the window is hidden. |
| The page never gets nil | Every slice and map built empty in Go | A nil slice reaches JavaScript as `null` and breaks the first `.length`. |
| The page sends ids only | Go resolves ids to paths and URLs | A page that went wrong cannot make hopto open an arbitrary URL, scheme or file. |
| Icons are files | `icons/<id>.png`, served at `/user-icons/<id>.png?v=<mtime>` | Seventy icons as data URLs would be over a megabyte of JSON on every show; files are cached by the WebView and can be replaced by hand. |
| One HTTP client | `internal/safehttp` | The limits and the private-address check, made on the resolved address in `Dialer.Control`, live in one place. |
| Last screen | The panel returns to the display it was last on | Following the pointer made the panel jump between displays for people who keep it on one. `screen = "mouse"` keeps that rule for those who want it. |
| No framework in the page | Plain modules, Vite only to bundle | Small and quick in the WKWebView; the pure modules run under plain Node. |

## Style card

What the code does, measured, so a change can read like the code around it. Measured on commit `3108150`.

| Go | |
|---|---|
| `gofmt`, `go vet`, `golangci-lint` (errcheck, govet, ineffassign, staticcheck, unused, gocritic, misspell with the UK locale) | clean |
| Lines over 78 columns, cgo preambles aside | 197 |
| Functions without a doc comment | 37 |
| Comments at the end of a line | 2 |

| Page (JavaScript) | |
|---|---|
| `var` / `let` / `const` | 0 / 17 / 257 |
| Function declarations / arrow functions kept in a constant | 152 / 4 |
| One-letter names | 3, all `t`, the translator |
| Lines with two statements | 0 |
| JSDoc before internal functions | 64 of 64 |
| Comments above the line / at the end of a line | 150 / 0 |
| Median comment width | 61 characters |
| `innerHTML`, `outerHTML`, `insertAdjacentHTML` | 0 (`frontend/test/dom-rules.test.js`) |

The checks, from the repository root:

```bash
# Go lines over 78 columns, outside the C preamble of the cgo files
for f in $(git ls-files '*.go'); do
  awk -v file="$f" 'FILENAME ~ /_darwin\.go$/ && !seenC { if ($0 ~ /^import "C"/) seenC = 1; next }
    length > 78 { print file ":" FNR }' "$f"
done

# Go functions without a comment right above them
for f in $(git ls-files '*.go'); do
  awk -v file="$f" '/^func / && previous !~ /^\/\// {print file ":" FNR} {previous = $0}' "$f"
done

# Page functions without a JSDoc block right above them
for f in frontend/src/*.js; do
  awk -v file="$f" '/^(export )?(async )?function / && previous !~ /\*\/$/ {print file ":" FNR} {previous = $0}' "$f"
done
```

## Known Limitations

| Limitation | Reason |
|-----------|--------|
| macOS only | Carbon, Cocoa and the WKWebView. A port needs another hotkey backend behind `registerToggleHotkeys` and another window layer. |
| Not signed or notarised | There is no Developer ID yet; the first open needs the steps in the README. |
| Shortcuts are read at start | Carbon registers them once in `startup`; a change needs Quit and open. |
| The menu bar item keeps its labels after a language change | The labels are set when the item is built at start. |
| Open at login assumes the running binary is not reached through a symbolic link | The bundle path is taken from `os.Executable` as it is, without resolving links. |
| A data folder hopto cannot write to makes every launch a first run | "First run" means `library.toml` did not exist, and the seed write never lands, so the welcome shows each time. |
| A second copy reads `library.toml` before the single-instance lock sends it away | `NewApp` runs before Wails checks the lock. The second copy never writes. |
| Opening an item with a click paints the list once before the new selection | `openAt` renders, then moves the selection. |
| Comments added to `library.toml` by hand are lost at the next save | The TOML encoder writes the whole file. |
| Links are http and https only | Other schemes could reach other apps or the file system. |
