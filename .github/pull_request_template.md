## What and why

<!-- One or two sentences: what this changes and the reason. Link the issue if there is one. -->

## How it was checked

<!-- The tests you added or ran. For anything native (the window, the shortcuts, the menu bar item): what you saw in the real app and how you proved it, such as a log line, count windows or a screenshot. -->

## Checklist

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make build && make e2e` passes (when the page changed)
- [ ] A line under `## [Unreleased]` in `CHANGELOG.md` (when users will notice the change)
- [ ] New texts of the page in both languages in `frontend/src/i18n.js`
- [ ] No personal data: no links, hosts, paths or e-mail addresses of your own, and no `library.toml` or `icons/` (the pre-commit guard from `make hooks` checks the files)
