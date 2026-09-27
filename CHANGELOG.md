# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- The launcher from the original private monorepo, rebuilt as hopto: two global shortcuts, global search, categories, favourites and usage order, Edge web apps, light and dark themes.
- The library lives in `library.toml` under Application Support; the file is re-read every time the panel opens and never overwritten when it cannot be parsed.
- The apps tab lists /Applications and ~/Applications, with icons read from each bundle, next to the apps added by hand.
- Icons for links are fetched once from the site itself (or a selfh.st hint), through a client that refuses private addresses, plain http and oversized bodies.
- Typing searches apps and links together, ranked by name, host, keywords and description; the empty panel shows favourites, recent items and each category.
- ⌘C copies the URL or path, ⌘↩ opens a link in the secondary browser, ⌥↩ reveals an app in the Finder, ⌘⇧↩ opens every item of a chip, ? shows the shortcuts.
- The interface speaks English and Spanish, following the system unless the settings say otherwise.

### Fixed
- A corrupt usage.json is no longer overwritten on the first opening.
- The selection no longer disappears when moving up on a short apps grid.
