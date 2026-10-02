# How hopto works on macOS

hopto is an overlay: no Dock icon, hidden until a global shortcut, centred on a display, on top of everything, gone when it loses the keyboard. Wails v2 gives a window and a web page; the rest is the pieces below. The package map and the design decisions are in [ARCHITECTURE.md](../ARCHITECTURE.md).

## 1. The window and the Dock

`main.go`:

| Option | Why |
|---|---|
| `Width: 760`, `Height: 520`, `DisableResize: true` | A fixed panel, sized for a grid of app cards |
| `Frameless: true` | No title bar; the page paints the panel and its edge |
| `AlwaysOnTop: true` | Above the app that was in front |
| `StartHidden: true` | Opening the `.app` shows nothing; the shortcuts are the way in |
| `HideWindowOnClose: true` | ⌘W hides instead of quitting |
| `BackgroundColour` with alpha 0, `Mac.WebviewIsTransparent: true` | The window is fully transparent; the page paints the rounded panel |
| `Mac.WindowIsTranslucent: false` | On purpose: its vibrancy view is rectangular and the corners came out square |
| No `Mac.Appearance` | The page follows the system's light or dark mode with `prefers-color-scheme`; a fixed appearance pinned it |
| `SingleInstanceLock` with id `com.drolosoft.hopto` | A second launch quits at once and shows the first copy's panel |

`LSUIElement` is `true` in `build/darwin/Info.plist`, but it is not enough: Wails v2 sets the "regular" activation policy at start, over the plist, and the app showed in the Dock and in ⌘⇥. `becomeAccessory()` in `hotkey_darwin.go` sets `NSApplicationActivationPolicyAccessory` on the main queue from `startup`, and that is what removes it. (`Mac.ActivationPolicy` appears in the Wails docs but is commented out in v2.16 and does not compile.)

## 2. Global shortcuts with Carbon

`hotkey_darwin.go` registers both shortcuts with Carbon's `RegisterEventHotKey`, which is global, needs no Accessibility permission, and works while the window is hidden.

- Registration runs on the main thread: the C code wraps it in `dispatch_async(dispatch_get_main_queue(), …)`. Wails owns that thread, so the block runs once its loop starts. `startup` calls it.
- The Carbon handler calls the exported Go function `launcherHotkeyPressed`, which runs on the main thread. It only starts a goroutine (`go onHotkey(tab)`); calling the Wails runtime from there would block the thread the runtime needs.
- Both shortcuts share the handler. The id given at registration (1 apps, 2 links) comes back in the event and becomes the tab.
- `RegisterEventHotKey`'s answer comes back to Go (`launcherHotkeyRegistered`) and goes to the log (`hotkey 1 registered`, or `hotkey 2: RegisterEventHotKey failed with status -9878` when another app has it) and to the welcome panel.
- The shortcuts come from `library.toml` (`hotkey_apps`, `hotkey_links`), parsed by `hotkeyspec.go` into a key code and Carbon modifier masks. See [configuration.md](configuration.md#shortcuts).

⌘⌥Space, the default for links, is also macOS's "Show Finder search window". Carbon accepts the registration, but the system keeps the press and opens "Searching This Mac". The welcome panel spots it (it reads `~/Library/Preferences/com.apple.symbolichotkeys.plist`, entry 65, and the combination it is bound to) and has a button to the right settings page. To turn it off: System Settings, Keyboard, Keyboard Shortcuts, Spotlight, and untick "Show Finder search window". From the shell:

```bash
defaults write com.apple.symbolichotkeys AppleSymbolicHotKeys -dict-add 65 \
  '<dict><key>enabled</key><false/><key>value</key><dict><key>parameters</key><array><integer>65535</integer><integer>49</integer><integer>1572864</integer></array><key>type</key><string>standard</string></dict></dict>'
/System/Library/PrivateFrameworks/SystemAdministration.framework/Resources/activateSettings -u
```

The same with `<true/>` turns it back on.

## 3. Showing with the keyboard, on the right display

An accessory app does not come to the front on its own: `WindowShow` makes the window visible without the keyboard. `activateApp()` calls `[NSApp activateIgnoringOtherApps:YES]` on the main queue after every show.

`centerOnScreen` places the window before it shows: centred on the chosen display, 8 % of its height above the middle so it reads as an overlay. The display follows `screen`: `last` is the one the panel was on when it last hid (saved in `window.json`; a display that is gone falls back to the main one), `mouse` the one under the pointer, `main` the one with the menu bar. The same function sets the overlay behaviour of the window: `CanJoinAllSpaces`, `FullScreenAuxiliary` and `Stationary`. Without `CanJoinAllSpaces`, with "Displays have separate Spaces" on, the window belonged to one Space and macOS switched Spaces to show it, so the panel seemed to jump to the other display.

`centerOnScreen` uses `dispatch_sync`, so the window is in place before `Show`. It must only be called from a goroutine: from the main thread it would wait for itself forever. Every way in (the hotkey handler's goroutine, bound methods called by the page, the menu's goroutine) respects that.

The order, in `showLocked`: place → `Show` → `Activate` → the `shown` event with the tab. The page resets everything on `shown` and reloads the lists. The App's state (`visible`, `tab`, `dialogOpen`) is under a mutex, because calls arrive from the shortcuts, the menu, the page and a second launch at once.

## 4. Round corners

A transparent window still showed a faint rectangle around the panel: macOS treated it as opaque (a rectangular shadow) and the WKWebView painted outside the panel. `centerOnScreen` sets `opaque = NO`, a clear background and a shadow, clips the content view to a layer with a radius of 20 (the panel's own radius, `masksToBounds`), and sets `drawsBackground = NO` on the WKWebView. After each show, `invalidateShadow` makes the shadow follow the painted shape.

## 5. Light and dark

`style.css` defines the colours as variables on `:root` and redefines them under `@media (prefers-color-scheme: light)`. It works because the window has no fixed appearance and the WKWebView follows the system.

## 6. Hiding

The panel hides on Esc with an empty search, on the same shortcut again, after opening something, and when the window loses the keyboard. The last one waits 150 ms and checks `document.hasFocus()` first: the first show after start fires a `blur` while the window is being activated, and without the wait the panel hid itself at the first press. It never hides on `blur` while the editor is open (a click elsewhere to copy a URL must not lose the draft) or while the native file dialog is up. `Hide` saves the display the window is on for the next show.

## 7. Apps

`internal/discover` lists, every time the panel opens:

- The `.app` bundles one level under `/Applications` and `~/Applications`, their `Utilities` folder and any `*.localized` folder, from each bundle's `Info.plist`: the name, `CFBundleIdentifier` and `CFBundleIconFile`. The same bundle reached twice (two roots, a symbolic link, the same bundle id) is listed once. A scan is cached until the folders change.
- The apps of `/System/Applications` (Mail, Notes, Calculator) in the same pass, marked search only: they show when you type, never in the grid or the counts.
- The web apps installed from Microsoft Edge in `~/Applications/Edge Apps.localized`, with the host of their page (`CrAppModeShortcutURL` in the bundle's `Info.plist`) as the description. ⌘↩ on one opens the page in the browser instead.

Their icons are read from the bundle's `.icns`: the container is a list of `type (4 bytes) + length (4, big endian, header included) + data`, and the `ic08`, `ic07`, `ic09`, `ic13` and `ic12` entries are plain PNG. hopto takes the first it finds in that order, decodes it, checks its size and serves it through the icons handler, cached by the bundle's modification time. Found apps get ids like `app-mail` and `edge-github`, so usage, favourites and hiding work for them as for anything else.

`Launch(id)` opens an app with `open -b <bundle id>` when it has one (it survives a move) and `open <path>` otherwise, the same as a double click in the Finder, then hides the panel.

## 8. Links

`OpenLink(id)` opens the link's URL with `open`, which hands it to the default browser, and `OpenLinkWith(id)` with `open -b <secondary_browser> <url>`. Only ids reach Go from the page, and only `http` and `https` URLs are accepted in the library, so no other scheme can reach `open`. How the library is read and written is in [configuration.md](configuration.md#how-hopto-reads-the-file).

## 9. Usage and favourites

`internal/usage` keeps, in `usage.json`, how many times each item was opened, when it was last opened, and the favourites, under keys `<tab>:<id>` (`links:mdn`, `apps:app-mail`) so an app and a link with the same id never share a count. Every change is written at once, atomically. A file that does not parse is logged and left alone: hopto starts with empty counts and does not overwrite it.

The page puts favourites first, then the five items opened most recently, then each category; inside a group the items opened most come first. ⌘F and the star of each row mark a favourite.

## 10. Icons

`internal/icons` fetches a link's icon once, in the background, the first time the link has none: the `icon` hint first (`sh:<name>` is an icon of selfh.st, from jsDelivr pinned to one commit, so a renamed icon upstream never changes what a hint means), then the icons the page declares (`apple-touch-icon`, `<link rel=icon>`) when `site` is in `icon_services`, then the other services in their order. Every candidate goes through `internal/safehttp`: https only, 10 seconds each and 30 for the whole chain, 3 redirects, 512 KiB, no private addresses unless `allow_private_icon_hosts`. The first image that decodes (at most 1024 by 1024 pixels) is scaled and written as `icons/<id>.png`; the page gets `/user-icons/<id>.png?v=<mtime>`, served by the asset handler from that folder only (the id is checked against the id pattern). When it arrives, an `icons` event tells the page to repaint.

## 11. The page

`frontend/src` is plain JavaScript modules bundled by Vite (listed in [ARCHITECTURE.md](../ARCHITECTURE.md#package-structure)). The search box always has the keyboard, so typing filters; the keys the page uses are taken before the box sees them.

- The whole panel is a drag region (`--wails-draggable: drag`) except the controls. **Every control that takes a click or a key needs `--wails-draggable: no-drag`**, or a click moves the window instead of reaching it.
- The `shown` event is where everything the user left half done is reset: tab, chip, search, selection, editor, pending questions. A piece of state that is not reset there comes back the next time.
- The page builds every element with `createElement`; no HTML strings. The build adds a Content Security Policy to `index.html` (`vite.config.js`: scripts only from the page itself, images from the page and `data:`), and a blocked request goes to the log.
- `Debug(message)` writes a line from the page to `~/Library/Logs/hopto.log` (`page: …`): the way to follow keys and focus without a web inspector.

## 12. The menu bar item, a second launch and login

- `statusbar_darwin.go` puts an `NSStatusItem` in the menu bar with Open hopto, Help, Edit library.toml, Open at login and Quit hopto. A click calls Go on the main thread, which only starts a goroutine, as for the shortcuts.
- A launch of the running app (Alfred, `open -a hopto`, a double click in the Finder) reaches Go through the reopen method hopto adds to Wails' application delegate; a second process (`open -n`) is stopped by the single-instance lock, which calls back into the first. Both show the panel like the apps shortcut, never hide it, and do nothing while the file dialog is up.
- Open at login writes `~/Library/LaunchAgents/com.drolosoft.hopto.plist`, which runs `/usr/bin/open <the .app>` at login, and removes it when unticked. It is not loaded with `launchctl`, so ticking it does not start a second copy now.

## 13. Limits

The known limits and their reasons are in [ARCHITECTURE.md](../ARCHITECTURE.md#known-limitations).

## 14. Windows

The engine is the same binary logic; what differs sits in files that end in `_windows.go`. One goroutine locked to its thread (`native_windows.go`) owns a hidden window, the tray icon, the two shortcuts (`RegisterHotKey`) and the one message loop their events arrive in; every entry point the engine calls posts work to that thread and waits, so user32 only ever sees its owner. The tray menu is built from the current items at each click, so its ticks are always the latest. No library does this: the two Go tray libraries pump messages from a thread that does not own their window, and the hotkey library polls every 10 ms.

The window is Wails' own, found by the class name `hoptoWindow`; it gets the tool-window style so it has no taskbar button, is centred on a monitor with `SetWindowPos`, and is brought to the front from the native thread, which holds the right to take the foreground after a hotkey press. Monitors are remembered by a hash of their device name.

Apps are the shortcuts of the two Start Menus, read by a parser of the Shell Link format (`internal/discover/shelllink.go`); the shortcut itself is what opens, through `ShellExecute`, as every other open on Windows does. Icons come from the shell's 256 px image list. "Open at login" is a value in the user's `Run` key.
