# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.3.0] - 2026-11-18

**Upgrade note.** No migration is needed: `vault.json` stays at version 2 and 0.2.0 vaults load unchanged. Going back to 0.2.0 afterwards loses data: saving with 0.2.0 drops the tags, and a value longer than 2000 characters, which 0.3.0 stores in several keyring entries, is not readable by 0.2.0 (0.2.0 has no chunking). Back up first with `pandora export --values -o backup.age` if you may need to go back.

### Added
- **Config file.** Optional `config.json` next to `vault.json` sets the idle lock, ASCII mode, clipboard and reveal timeouts, the expiry warning window, the generator defaults (`pandora gen` and the form), the initial sort and compact rows. Precedence is environment, then file, then default; an invalid file or value is reported once (a toast after unlocking, `pandora: warning:` on stderr) and never stops Pandora. `pandora config path` and `pandora config get [KEY]` show the location and the effective settings. See the README "Configuration" section.
- **Multi-line secrets** (SSH keys, certificates, JSON credentials), up to 32 KB. In the form, **Secret** is a multi-line editor: paste or press `enter` for a new line, `tab` moves on, `ctrl+s` saves, `ctrl+r` shows or hides the text (it is bullets otherwise). `pandora add <title> < file` reads any number of lines from stdin; `get`, `run`, `export` and `import` keep them intact.
- Long values are stored in several keyring entries so they fit the Windows (2560 bytes) and macOS limits. Records written by 0.2.0 load unchanged.
- The TUI notices when another process (the CLI, a second window) changes the vault and refreshes the list within a second, keeping the selection; a form or a delete confirmation is never interrupted, and the list updates once it closes.
- Copying a secret in the TUI now marks it as concealed so clipboard managers that respect the marker skip it: `org.nspasteboard.ConcealedType` on macOS; on Windows it is kept out of clipboard history (`Win+V`) and cloud clipboard sync. If the marker cannot be set the copy still happens without it. Linux has no such convention, so nothing changes there.
- **Tags.** A secret can have several tags besides its category (form field **Tags**, comma-separated, e.g. `prod, db`; lowercase, at most 10 of 24 characters, no spaces). `T` in the TUI cycles the tag filter, the detail card lists them, `pandora add --tag` and `pandora list --tag` (repeat for "all of") use them, `list --json` includes `tags`, and they round-trip through export and import. Old vaults and exports load unchanged. Going back to an older version and saving drops the tags.
- The `/` filter now also searches tags and notes (never the secret value). Matches in the name rank first; a match in a tag or note shows the row without underlining anything.

### Fixed
- Two Pandora processes writing at the same time no longer lose each other's changes: every write takes an exclusive lock on `vault.lock` (released automatically if a process dies). A writer that cannot get it within 10 seconds fails with "vault is busy" and changes nothing; reads never wait.
- Deleting the last row of the list no longer leaves nothing selected.

### Changed
- Pressing `enter` in **Notes** now moves to the new **Tags** field; `enter` there saves. `pandora list` prints a trailing `TAGS` column.
- Replacing a value (edit, import) now writes the new value first and removes the old one afterwards, so a failure can no longer leave a half-replaced value. In the detail card a long revealed value shows its first lines and how many more there are; `c` still copies all of it.
- In the form's **Secret** field `enter` inserts a newline; use `tab` to move to the next field.

## [0.2.0] - 2026-11-15

**Upgrade note.** The data format changes and cannot be downgraded. Before upgrading, back up with `pandora export --values -o backup.age` using the old version. The first start of this version asks you to choose a master password and migrates your secrets automatically; the password cannot be recovered.

### Added
- **Master password.** Names, categories, notes, expiry dates and values are encrypted (Argon2id key derivation, XChaCha20-Poly1305) in `vault.json`; the keyring holds only encrypted values under random IDs, so `security dump-keychain` no longer lists your titles. The first start asks for a password (at least 10 characters, not recoverable); the TUI and every CLI command that reads secrets ask for it.
- `pandora passwd` changes the master password without touching the stored values.
- In the password screens, `ctrl+r` shows or hides what you type, and the inputs are drawn as boxes with hints; passwords pasted with a trailing newline still match.
- `PANDORA_MASTER_PASSWORD` supplies the password to the CLI when there is no terminal (a warning is printed; `pandora run` hides it from the child). Shell completion lists titles and categories only when it is set.
- `PANDORA_PASSPHRASE` use now prints a warning, and export passphrases must be at least 10 characters.

### Changed
- **Persisted format.** `metadata.json` and keyring accounts named after the titles are replaced by `vault.json`. Older data is migrated on first start and the old files and keyring entries are deleted (not securely erased). Downgrading to an older version afterwards is not possible without restoring from an export.
- The idle lock now drops the key from memory, empties the list and asks for the master password again; an unsaved form is discarded. Any key no longer unlocks it.
- Renaming a secret no longer touches the keyring.
- `pandora export` without `--values` says that titles are not encrypted.

### Security
- Titles, categories and notes may not contain control characters, so a crafted import file cannot inject terminal escape sequences into `list` or the interface.
- Keyring values are bound to their entry (AEAD associated data), so swapped or tampered entries are rejected; a corrupt or tampered vault is reported and never overwritten.

## [0.1.1] - 2026-11-15

### Added
- Homebrew formula, published to `RafaSG13/homebrew-tap` by the release workflow.

### Fixed
- GitHub release notes are now set explicitly from this changelog (v0.1.0 was published with an empty description).

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
