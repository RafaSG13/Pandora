# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Process hardening: core dumps are disabled on Unix, and on Linux the process is non-dumpable (`/proc/<pid>/{mem,environ}` and same-user ptrace are refused).
- Wayland copies are marked sensitive through `wl-copy --sensitive` (clipboard managers such as Klipper skip them); without it, copying falls back to the plain write.
- **Documentation site** (MkDocs Material, in `docs/`, published to GitHub Pages): getting started, how Pandora works, threat model, cryptography, vault format, sharing, hardening, guides and a full reference. `docs/reference/cli-flags.md` is generated from the command table by `go run ./scripts/docsgen`, and a test fails when it is stale or when `configuration.md` misses a setting or key action. The README is shorter and links to it.
- **Sharing secrets between machines, without a network.** `pandora id` prints this vault's share key (a post-quantum age key plus an Ed25519 signing key, stored encrypted in `vault.json`). `pandora peer add|list|rm` manages trusted peers after you compare fingerprints. `pandora export --values --to PEER [-s TITLE]` encrypts only the chosen secrets for that peer and signs them; `pandora import FILE --values` opens it with no passphrase and refuses files not signed by a trusted peer or not made for this vault. Moving the file (AirDrop, scp, USB) is up to you. An older Pandora that rewrites the vault drops the identity, so peers must be added again.
- Interface, lock: `L` locks at once (rebindable as `lock`): everything is masked and the master password is asked for, without waiting for the idle timeout.
- Interface, form: pressing `esc` (or `ctrl+c`) on an add/edit form with unsaved changes asks first; the same key again discards them.
- **Username.** Secrets have an optional user name (login or e-mail): a **Username** field in the form, shown in the detail card, copied with `u` (rebindable as `copy_username`; plain clipboard, not auto-cleared, not audited), searched by `/`, and carried by `export` / `import` and `list --json`. `pandora add -u` / `--username` sets it. It is a new optional field in `vault.json` (still version 2): an older Pandora ignores it and drops it on its next write. The Bitwarden import now puts a login's user name there instead of in the notes.
- Interface, viewer: `V` (rebindable as `view`) opens the whole value of the selected secret in a scrollable viewer; it closes `reveal_seconds` after the last key.
- Interface, filters: `f` (rebindable as `filters`) opens a picker of every category and tag with counts, so categories past the ninth are reachable. The `/` filter understands `tag:x`, `cat:x` and `user:x` qualifiers alongside free text.
- Interface, mouse: double-clicking a row copies that secret (like `c`), and clicking the detail card's value box reveals or hides the value (like `v`).
- Interface, keys: the list-screen keys can be rebound with a `keys` object in `config.json` (see the README, "Rebinding keys"). The help overlay, the footer and the in-card hints show the effective keys. `pandora config get keys` prints them.
- `config.json` key `background` (`auto`, `light`, `dark`) fixes the colour scheme without asking the terminal.

### Fixed
- Interface, layout: the screen is now exactly as tall as the terminal in every mode and at every size from 60×18 up. Before, the header scrolled off the screen on the first-run welcome and the load-error card, on the detail card when a category, expiry or history hint was too wide for it, and on the list while a filter was being typed or applied. Long category names, search terms and form errors are cut with an ellipsis.
- Interface, resizing: shrinking the terminal while editing no longer leaves the cursor outside the Secret, Name or other field (a blank row until the next key). Fields you are not in show their beginning.
- Interface, filter: after editing, adding or deleting a secret, pressing `r`, or a change made by another process, an active filter no longer empties the list.
- Interface, keys: `pgup` / `pgdn` page through the list as the README says, and `ctrl+c` quits from the add/edit form like everywhere else.

### Changed
- Interface, start-up: the terminal's background colour is asked for before the interface takes over the keyboard, not during its first frame, where the reply could not be read. Set `background` to `light` or `dark` to skip the question.
- Interface: the footer shows `enter apply · esc cancel` while a filter is typed and `esc clear filter` once it is applied; the detail pane says when nothing matches; at 60×18 the toast is one line so it does not cover the value box; the detail card drops its notes and then its metadata rows, never the value box, when the terminal is short.

## [0.6.0] - 2026-11-23

This release carries everything planned as 0.4.0 (scripting and import), 0.5.0 (vault recovery and value history) and 0.6.0 (audit log): no 0.4.x or 0.5.x tags were cut.

**Upgrade note.** Previous values are secrets too: they stay in the OS keyring (encrypted under the same key as the current value) until they are pruned or the secret is deleted, so history adds exposure; set `history_depth` to `0` to switch it off. Going back to a version without this feature drops the history the next time it saves the vault and leaves those keyring entries behind, unreferenced (encrypted, but never cleaned up). Back up first with `pandora export --values -o backup.age` if you may need to go back.

### Added
- `pandora env -s TITLE[=ENV_NAME] [-s ...]` prints secrets as dotenv `KEY=value` lines (values with anything beyond `A-Za-z0-9_./:@%+-` are double-quoted and escaped, so multi-line values round-trip). It refuses to print to a terminal unless `--force` is given.
- `pandora get -q` / `--newline=false` omits the trailing newline.
- `pandora get --copy` copies the value to the clipboard (concealed, as in the TUI) instead of printing it. The clipboard is not cleared automatically: only the TUI does that.
- `pandora expiring [--within 14d] [--json]` lists expired and soon-to-expire secrets (never values) and exits `3` when there are any, `0` when none, so cron and CI can react. The default window is `expiry_warn_days` from the config file, like the TUI; `--within` overrides it. `3` is a new exit code used only by this command.
- `pandora import --format dotenv|bitwarden-json FILE` imports from a dotenv file (the inverse of `pandora env`) or an unencrypted Bitwarden JSON export. Logins become secrets whose value is the password (user name and first URL go into the notes, the folder becomes the category); secure notes become secrets holding the note. Cards, identities, TOTP seeds and custom fields are not imported, and everything skipped is listed with its reason. `import --dry-run` (any format) lists what would be imported, replaced or skipped and writes nothing. `--category` sets the category for dotenv imports. The source file is plain text: delete it once you have checked the import.
- `pandora doctor` now also checks `vault.json` and `vault.json.bak` against your master password and says whether each opens and how many secrets it holds. If `vault.json` is damaged or missing and the backup opens, `pandora doctor --fix` restores it (asking first unless `-y`); the damaged file is kept as `vault.json.corrupt`. A wrong password never restores anything, a healthy or newer-version vault is never replaced, and nothing is touched when the backup is unusable too. The backup is the state before the last change, so that change is lost; if it replaced a value, `doctor` lists the affected entries but only removes them on a second `--fix`. Other commands that meet a damaged vault now point at `pandora doctor`.
- **Value history.** Changing a secret's value (in the form, or `import --overwrite`) keeps the old value as a previous version: the last `history_depth` (new `config.json` key, default 3, `0` turns it off, at most 20). `H` in the interface lists them (`r` reveals one, `enter` makes it current again); `pandora history TITLE [--json]` lists them (dates only), `pandora get TITLE --version -1` reads one and `pandora restore TITLE [--version -1]` brings one back (the value it replaces becomes `-1`, so a restore can be undone). Saving a value it already has creates no version; deleting a secret deletes all its versions; `export` never includes them. `doctor` checks them and `doctor --fix` forgets missing ones, but keeps an entry whose current value is gone while it still has previous values, so you can restore one. History is stored as an optional `history` field in the encrypted record: `vault.json` stays at version 2 and older vaults load unchanged.
- **Audit log** (opt-in). With `"audit_log": true` in `config.json`, every `reveal`, `copy`, `get`, `run`, `env`, `export`, `import`, `add`, `edit`, `rm` and `restore`, from the interface or the command line, is recorded once with its time and the secret's title, never a value. Each line of `audit.log` is encrypted under the vault's key, so titles and times are not readable on disk; at 1 MiB it rotates once to `audit.log.1`. `pandora log [--since 7d|12h|YYYY-MM-DD] [--json]` shows it. Failed unlock attempts are not recorded. A failure to write the log never stops the action: you get a warning.

### Changed
- Every command that opens the vault now reads `config.json` (it needs `history_depth` and `audit_log`), so a problem in the file is reported as a `pandora: warning:` on stderr by all of them, not only by `gen`, `config`, `expiring` and `import`. It still never changes the exit code.

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
