# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.1.0] - 2026-11-15

### Added
- Edit a secret (`e`): change name, category and optionally the value; renaming moves the keyring value.
- Delete a secret (`d`) with a confirmation dialog.
- Reload the list from disk (`r`).
- Validation: name and secret are required when adding; names must be unique.
- Status messages for save, read, update, delete and reload errors.
- Responsive two-pane layout, adaptive light/dark palette, contextual footer help.
- Tests for `internal/store` and `internal/tui` using an in-memory keyring.
- Documentation: README, architecture notes, roadmap, contributing guide and security policy.
- Help overlay (`?`) and mouse support: click selects a row; the wheel scrolls the list without moving the selection.
- Per-category colours, full-row selection highlight, bordered detail card, row separators, compact mode (`t`), position indicator and scrollbar, panes that fit their content, toasts, countdown bars, gradient header with keyring backend and status chip, welcome and error screens, spinner, minimum terminal size guard, and an ASCII fallback (`PANDORA_ASCII=1`).
- Per-item reveal (`v`) with a 10 s auto-hide; copy failures are reported, `x` clears the clipboard now, and a later copy is never cleared by an earlier one.
- Sort (`s`), category filter (`1`–`9`, `0`) and category autocomplete in the form.
- Optional **Expires** and **Notes** fields, an `Updated` date, expiry badges in the list, detail card and header, and a random secret generator (`ctrl+g`, `ctrl+l`, `ctrl+t`) in the form.
- Idle lock (`PANDORA_LOCK_MINUTES`, default 5 minutes) that masks everything and clears a pending clipboard; orphan detection that flags secrets whose keyring value is missing.
- Command-line mode: `pandora list | get | add | rm | run | export | import | doctor`; `export --values` is encrypted with an age passphrase.
- `metadata.json.bak`: the previous metadata is kept before every write.
- `pandora version` / `--version`; the version, commit and build date are embedded at build time (`make build`, release builds) and shown in the help overlay.
- `pandora gen` prints random secrets (`--length`, `--charset`, `--count`); the generator now lives in `internal/gen`.
- `pandora completion bash|zsh|fish|powershell`: shell completion for commands, flags, secret titles and categories (values are never read).
- `NO_COLOR` / `TERM=dumb` support: no colour, the selected row gets a marker and expiry levels are distinguishable by glyph.
- The help overlay scrolls, so it is fully reachable on small terminals.
- Release pipeline: GoReleaser builds macOS, Linux and Windows archives (amd64, arm64) with checksums from a `v*` tag; CI dry-runs it on pull requests. QA checklist in `docs/QA.md`, and a Linux CI job that exercises the store against a real Secret Service.
- `docs/ROADMAP.md` is now a forward plan with ordered, self-contained items.

### Changed
- Project restructured into `cmd/pandora`, `internal/store` and `internal/tui`.
- UI text is now in English.
- The add form is reset each time it opens; `enter` advances to the next field.
- New and edited secrets are selected in the list after saving.
- Metadata is written atomically, and a corrupt `metadata.json` is reported instead of being overwritten.
- The Go module path is now `github.com/RafaSG13/Pandora` (was `pandora`), so `go install github.com/RafaSG13/Pandora/cmd/pandora@latest` works.
- The CLI commands come from a single table that drives help, dispatch and completions.
- Clearing the clipboard (after 15 s, with `x`, or on lock) now only happens if the clipboard still holds what Pandora copied; anything copied since is left alone.

### Fixed
- Adding a secret with an existing name no longer silently overwrites its value or duplicates the metadata entry; duplicates written by older versions are collapsed on load.
- Save and read failures are no longer silently ignored.
