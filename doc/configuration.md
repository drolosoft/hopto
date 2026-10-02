# Configuration

hopto keeps everything it knows about your apps and links in one file, `library.toml`, next to a few files it manages itself. This page lists every key the file accepts. [config.example.toml](../config.example.toml) is the library a first run writes, with a comment on each setting.

## Where the files are

| Path | What it holds |
|---|---|
| `~/Library/Application Support/hopto/library.toml` | Settings, categories, links, apps added by hand, hidden apps |
| `~/Library/Application Support/hopto/library.toml.bak` | The previous good version, kept at every save |
| `~/Library/Application Support/hopto/icons/<id>.png` | The icon of each link and of each app added by hand |
| `~/Library/Application Support/hopto/usage.json` | How often and when each item was opened, and the favourites |
| `~/Library/Application Support/hopto/window.json` | The display the panel was last shown on |
| `~/Library/Logs/hopto.log` | What hopto did: shortcuts registered, items opened, problems |

On Windows the same files sit in these folders:

| | macOS | Windows |
|---|---|---|
| Data folder | `~/Library/Application Support/hopto/` | `%UserProfile%\AppData\Roaming\hopto\` |
| Log | `~/Library/Logs/hopto.log` | `%UserProfile%\AppData\Local\hopto\hopto.log` |

The Windows folder is derived from the profile folder, so a redirected `APPDATA` is not followed.

The folder is private to your user (`0700`, files `0600`). **Edit library.toml** in the menu bar item opens the file in your text editor, and the help panel (`?`) shows its path.

## How hopto reads the file

- The first run writes a small library, three categories and six links, in English or Spanish after the language of the system.
- hopto reads the file again every time the panel opens and parses it only when its bytes changed. A hand edit shows up the next time you press a shortcut.
- A change made from the panel is written to a temporary file and renamed over `library.toml`, so a crash never leaves half a file. The version it replaces goes to `library.toml.bak`.
- A file that does not parse, or breaks one of the rules on this page, is never overwritten. The panel keeps the last good library, shows the problem and its line at the top with a button that opens the file, and refuses to add, edit or delete until the file is fixed. The same goes for a file over 1 MiB, one with more than 2000 links and apps or more than 64 categories, and one written by a newer hopto.
- Keys hopto does not know are left alone when reading, and dropped at the next save.
- hopto writes the whole file when it saves, so comments added by hand are lost at the next change made from the panel. Keep notes in `description` or `keywords`.
- The two shortcuts are read when hopto starts. After changing them, quit hopto from the menu bar item and open it again. Everything else takes effect the next time the panel opens.

## `version`

| Key | Type | Default |
|---|---|---|
| `version` | integer | `1` |

The schema of the file. Leave it as it is: hopto refuses to rewrite a file with a higher number, which only a newer hopto writes.

## `[settings]`

| Key | Type | Default | What it does |
|---|---|---|---|
| `language` | `"auto"`, `"en"`, `"es"` | `"auto"` | Language of the panel and the menu bar item. `auto` follows macOS: Spanish when it is your first language, English otherwise. |
| `hotkey_apps` | shortcut | `"cmd+shift+space"` | Shows the apps tab; pressed again, hides the panel. |
| `hotkey_links` | shortcut | `"cmd+option+space"` | Shows the links tab; pressed again, hides the panel. |
| `screen` | `"last"`, `"mouse"`, `"main"` | `"last"` | Where the panel appears: the display it was last shown on (else the main one), the display under the pointer, or always the main one. |
| `scan_applications` | boolean | `true` | Lists the apps of `/Applications` and `~/Applications` (their `Utilities` and `*.localized` folders too), and finds those of `/System/Applications` while typing. |
| `discover_edge_apps` | boolean | `true` | Lists the web apps installed from Microsoft Edge (`~/Applications/Edge Apps.localized`). |
| `icon_services` | list of `"site"`, `"duckduckgo"`, `"google"` | `["site"]` | Where link icons come from after the link's own `icon` hint, in this order. `site` reads the icons the page declares. The two services receive the host of every link you add, so they are off unless listed. |
| `secondary_browser` | bundle id, or path | none | The browser ⌘↩ opens links in, such as `"com.apple.Safari"` or `"org.mozilla.firefox"`. On Windows it is the path of a browser's exe. Without it, ⌘↩ on a link says there is none. |
| `allow_private_icon_hosts` | boolean | `false` | Lets icon downloads and page reading reach loopback, private, link-local and CGNAT (tailnet) addresses, for links to a router or a home server. |

### Shortcuts

A shortcut is modifiers joined by `+`, then one key, in any order and any case: `cmd+shift+space`, `ctrl+option+k`.

- Modifiers: `cmd` (or `command`), `shift`, `option` (or `opt`, `alt`), `ctrl` (or `control`). At least one of `cmd`, `option` or `ctrl`: `shift` alone would steal ordinary typing.
- Keys: `a` to `z`, `0` to `9`, `space`, `return`, `tab`, `escape`, `f1` to `f12`. Letters and digits are the physical keys of an ANSI keyboard, whatever your layout prints on them.
- `cmd+space` (Spotlight) and `cmd+tab` are refused.
- A shortcut that does not parse falls back to its default, with a line in the log. If both are the same, the links one goes back to its default.
- What the modifiers mean depends on the system. On macOS `cmd` is ⌘, `option` (or `alt`) is ⌥ and `ctrl` is ⌃. On Windows `cmd` is the Win key, `ctrl` is Ctrl, and `alt` or `option` is Alt.
- The seeds differ: `cmd+shift+space` and `cmd+option+space` on macOS, `ctrl+shift+space` and `ctrl+alt+space` on Windows.
- The list of refused shortcuts changes with the system. On Windows it holds Win+L, Win+Tab, Win+Space and Win+Shift+Space.
- `cmd+option+space` belongs to the Finder ("Show Finder search window") until you turn that off in System Settings, Keyboard, Keyboard Shortcuts, Spotlight. The welcome panel warns about it.

## `[[categories]]`

The chips above the list, in the order of the file.

| Key | Type | Rules |
|---|---|---|
| `id` | text | Lowercase letters, digits and dashes, up to 64, starting with a letter or a digit. Unique among the categories. Not `favorites`, `favoritos`, `applications`, `edge` or `hidden`, which are the page's own chips. |
| `name` | text | Up to 80 characters. |
| `tab` | `"links"` or `"apps"` | The tab the chip belongs to. |

A category that still has items cannot be deleted from the panel (⌘⇧⌫ asks only once it is empty).

## `[[links]]`

| Key | Type | Rules |
|---|---|---|
| `id` | text | The shape of a category id. Unique across links and apps. Not starting with `app-` or `edge-`, which belong to found apps. It also names `icons/<id>.png` and the usage counts. |
| `name` | text | Required, up to 80 characters. |
| `description` | text | Optional, up to 200 characters. Shown under the name and searched. |
| `url` | text | Required. `http` or `https`, with a host, no user name, no spaces or invisible characters, up to 2048 bytes. |
| `category` | category id | A category of the `links` tab. |
| `keywords` | list of text | Optional. Extra words to search by, each up to 80 characters. |
| `icon` | text | Optional. `"sh:<name>"` for an icon of [selfh.st/icons](https://selfh.st/icons) (for a dark panel, the `-light` variants read better), or an https URL of an image. |

hopto fetches a link's icon once, the first time it has none, and keeps it in `icons/<id>.png`. It tries the hint, then each of `icon_services`, and uses the first image that decodes. A link with no icon shows its initial. Replace the file by hand to use your own picture (a square PNG, 256 pixels is plenty).

## `[[apps]]`

Apps added by hand. Found apps are not stored here; they are listed afresh every time the panel opens.

| Key | Type | Rules |
|---|---|---|
| `id` | text | As for links. |
| `name` | text | Required, up to 80 characters. |
| `description` | text | Optional, up to 200 characters. |
| `path` | text | An absolute path ending in `.app`, under `/Applications`, `/System/Applications` or `~/Applications`. |
| `bundle_id` | text | Such as `com.apple.calculator`. |
| `category` | category id | A category of the `apps` tab. |

On Windows `path` is a `.exe` or a `.lnk` under Program Files, the Start Menu or `AppData\Local\Programs`; `bundle_id` has no meaning there.

An app needs a `path`, a `bundle_id` or both. With a bundle id hopto opens it with `open -b`, which still works after the app moves; with only a path, with `open <path>`. Adding a found app by hand (⌘E on it) gives it a category, and it then shows once, as your entry.

## `[[hidden]]`

| Key | Type | Rules |
|---|---|---|
| `id` | text | The id of a found app: `app-…` from the scan, `edge-…` for an Edge web app. |

⌘⌫ on a found app writes this row and the app leaves the list; it comes back from the "Hidden" chip, where ⌘⌫ shows it again.

## When the file is refused

The notice at the top of the panel names the line (for a parse error) and the rule. The rule names, as the log writes them:

| Problem | Meaning |
|---|---|
| `id.invalid`, `id.reserved`, `id.duplicate` | An id with other characters, one that starts with `app-` or `edge-`, or one used twice |
| `name.required`, `name.long`, `name.control` | A missing name, one over 80 characters, or one with control characters |
| `url.invalid` | Not an http or https address with a host |
| `icon.invalid` | An icon hint that is neither `sh:<name>` nor an https URL |
| `category.unknown`, `category.tab` | A category that does not exist, or one of the other tab |
| `app.target`, `app.path`, `app.bundle` | An app with neither path nor bundle id, a path outside the three folders, or a bundle id with the wrong shape |
| `hidden.prefix` | A hidden row that is not a found app |
| `settings.language`, `settings.screen`, `settings.icons`, `settings.hotkey`, `settings.browser` | A setting with a value hopto does not know |

Fix the line, save, and press the shortcut again. The last good version is in `library.toml.bak` if you need it.
