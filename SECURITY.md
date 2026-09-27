# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |

## Reporting a Vulnerability

If you discover a security vulnerability in hopto, please report it responsibly:

1. **Email**: forge@drolosoft.com
2. **Subject**: `[SECURITY] hopto: <brief description>`

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

**Do not** open a public GitHub issue for security vulnerabilities.

## Security Considerations

- **Local by default**: hopto reads and writes files under `~/Library/Application Support/hopto/` and writes its log to `~/Library/Logs/hopto.log`. It uses the network for two things only: fetching link icons, and reading the title, description and icon of a page whose address you type in the editor.
- **One careful HTTP client**: both go through `internal/safehttp`, which gives up after 10 seconds for an icon and 5 for a page, follows at most 3 redirects, reads at most 512 KiB of an icon and 1 MiB of a page, ignores any proxy, and refuses loopback, private, link-local and CGNAT (tailnet) addresses, checked on the address a name resolves to. Only https is used, except for reading a page you typed as `http://`. `allow_private_icon_hosts = true` lifts the address rule for your own network.
- **Images are re-encoded**: every icon, downloaded or read from an app's `.icns`, is decoded, checked for size and written again as a PNG before the page shows it.
- **Third-party icon services are opt-in**: `google` and `duckduckgo` in `icon_services` receive the host of every link you add; the default list holds only `site`, the page itself.
- **Only ids cross from the page to Go**: the page asks to open an item by id, and Go looks the URL or path up in the library. Links open only with http or https; apps added by hand only from a `.app` under `/Applications`, `/System/Applications` or `~/Applications`.
- **No credentials stored**: the library holds names, URLs, app paths and settings. The data folder is private to your user (`0700`, files `0600`), every write is atomic, and the previous good library stays in `library.toml.bak`.
- **Unsigned builds**: releases are not signed with an Apple Developer ID yet. Check the SHA-256 published next to each zip before opening it.
