# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [v0.3.0] - 2026-10-05

### Added

- A disk image for macOS, signed with a Developer ID and notarised: open it, drag hopto to Applications, no warning.
- An installer for Windows, `amd64` and `arm64`: for one user, no administrator, a Start Menu shortcut, and a newer version replaces the one installed. Not signed yet, so SmartScreen still warns; the zips stay for who would rather not install.

| System | Download |
|---|---|
| macOS 12 or later, Apple silicon and Intel | `hopto-v0.3.0-macos-universal.dmg` |
| Windows 11, x64 (most PCs) | `hopto-v0.3.0-windows-amd64.msi` |
| Windows 11 on Arm (Snapdragon) | `hopto-v0.3.0-windows-arm64.msi` |
| Windows, without installing | the `.zip` of your architecture |

### Changed

- The macOS download is the disk image; the zip of the app is gone.
- The Go code now lives in `internal/app`, `internal/platform` and `internal/native`; the root holds `main.go` alone. Nothing changes for a user.
- A cleanup with no change in behaviour: the big files split by what each part does, the page's layout and removal logic in modules of their own with tests, the Windows build linted as well, a hermetic icon test, and a `make ci` the workflow runs.

### Fixed
- On Windows the help and the footer write the Tab key as Tab instead of the macOS glyph; the add row shows its plus once.
- The help has a name for screen readers, takes the keyboard while it is open and gives it back on close; Tab stays inside the welcome; the list is announced as the selected tab's panel.

## [v0.2.0] - 2026-10-03

### Added
- hopto runs on Windows: tray icon with the same menu, Ctrl+Shift+Space and Ctrl+Alt+Space, the programs of the Start Menu with their icons, links in the default browser, the library under `%AppData%\hopto`, open at login, the system language, and `amd64` and `arm64` zips in every release.
- `scripts/verify-hopto-windows.ps1` drives the Windows build the way `verify-hopto.sh` drives the Mac one: a first run on a profile of its own, the welcome, both shortcuts, the keyboard in the page, the taskbar and the seed.

### Changed
- `cmd` in a shortcut means the Win key on Windows; the page prints Ctrl, Alt and Enter there.

## [v0.1.1] - 2026-09-28

### Added
- A pencil on every link and app, shown when the pointer or the selection is on it, opens the editor with the mouse; before, ⌘E was the only way in and it was written only in the help.

### Changed
- `actions/checkout`, `actions/setup-go` and `actions/setup-node` in the CI and release workflows now use their newest major versions.

### Fixed
- The scripts that drive the real app no longer risk your data: `put_back` only restores from a copy that `set_aside` actually finished making, so a script that dies partway through never wipes `library.toml`, `icons/` or `usage.json`.
- `record-demo.sh` no longer closes Preview windows that were already open before the recording; it now closes only the backdrop image it created itself.
- A version tag now fails to release unless the tagged commit's CI run was green, instead of publishing a build that had skipped golangci-lint, gofmt and Playwright.
- "Open at login" refuses a translocated copy of hopto.app, which would otherwise stop working the first time the Mac restarts.
- The demo GIF shows the panel alone, with transparent rounded corners, instead of a dark frame around it.

## [v0.1.0] - 2026-09-27

### Added
- The launcher from the original private monorepo, rebuilt as hopto: two global shortcuts, global search, categories, favourites and usage order, Edge web apps, light and dark themes.
- The library lives in `library.toml` under Application Support; the file is re-read every time the panel opens and never overwritten when it cannot be parsed.
- The apps tab lists /Applications and ~/Applications, with icons read from each bundle, next to the apps added by hand.
- Icons for links are fetched once from the site itself (or a selfh.st hint), through a client that refuses private addresses, plain http and oversized bodies.
- Typing searches apps and links together, ranked by name, host, keywords and description; the empty panel shows favourites, recent items and each category.
- ⌘C copies the URL or path, ⌘↩ opens a link in the secondary browser, ⌥↩ reveals an app in the Finder, ⌘⇧↩ opens every item of a chip, ? shows the shortcuts.
- The interface speaks English and Spanish, following the system unless the settings say otherwise.
- Typing something that is not there offers "＋ Add": Enter opens the editor in the panel, reads the page's name, description and icon, ⌘1-9 picks the category (or types a new one) and Enter saves.
- ⌘E edits the selected link or app; ⌘⌫ asks in the row before deleting a link or hiding a found app, and hidden apps come back from their own chip.
- ⌘⇧E renames the active category in place and ⌘⇧⌫ deletes it once it is empty.
- Apps are added by hand through the native dialog (⌘N or "Search Applications…" on the apps tab), which is also how a found app gets a category.
- Apps in /System/Applications are found by typing without filling the grid.
- A menu bar item with Open, Help, Edit library.toml, Open at login and Quit.
- The first run shows a welcome with the shortcuts, whether each one registered, and a warning when macOS keeps ⌘⌥Space for the Finder.
- The help names the version, the commit and where library.toml lives; the broken-file notice has a button that opens the file.
- CONTRIBUTING, SECURITY and ARCHITECTURE guides; doc/ pages on how hopto works on macOS, on every key of library.toml and on testing; issue and pull request templates.
- config.example.toml: the first-run library with a comment on every setting, kept equal to it by a test.
- scripts/verify-hopto.sh drives the built app from the shell on a library of its own and checks the shortcuts, opening, hiding, the Dock and Quit, then puts your data back.
- A README with a demo, the shortcuts, the configuration and troubleshooting; scripts/record-demo.sh records the demo from the real app on a library with nothing personal in it.
- `make dist` builds the release: one universal app for Apple silicon and Intel, zipped with its SHA-256; pushing a version tag publishes it as a GitHub release with the notes of this file.

### Changed
- The category id `hidden` is reserved for the chip of hidden apps; a library using it for its own category is refused.
- The app reports its own version in the Finder and asks for macOS 12 or later, the oldest macOS that Go 1.27 runs on.

### Fixed
- A corrupt usage.json is no longer overwritten on the first opening.
- The selection no longer disappears when moving up on a short apps grid.
- An Edge web app no longer swallows a link to another page of the same host.
- Editing a link keeps its icon hint, and an edit racing an add can no longer leave two links to the same page.
- An app can no longer be added twice by hand, even when two editors or a hand edit race the save.
- Help from the menu bar waits while the file dialog is open, and closes the editor instead of painting over it.
- While the editor is open, the footer says what Enter does there ("↩ Save") instead of naming the row underneath.
- The library path in the help breaks at the edge of the panel instead of splitting every word.
- The Finder shortcut warning also reads shortcut settings that macOS stored as decimal numbers.
