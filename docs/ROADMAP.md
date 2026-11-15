# Roadmap

Everything that was planned for Pandora's scope (local, keyring-backed, keyboard-driven) is implemented. This file records what shipped and what is deliberately left out.

## Implemented

**Core**
- **Delete** (`d`), **Edit** (`e`, renames move the keyring value and keep the creation date), **Reload** (`r`), duplicate-title protection, atomic metadata writes with a `metadata.json.bak` backup.
- **Notes / URL and expiry** fields in the form; `Updated` date stamped on every edit. Optional fields are backwards compatible.
- **Generate secret** (`ctrl+g` in the form) with `crypto/rand`; `ctrl+l` cycles the length (16–64) and `ctrl+t` the charset.

**Day-to-day usability**
- **Per-item reveal** (`v`): resets when the selection changes; **auto-hide** after 10 s with a countdown.
- **Copy**: failures are reported instead of showing `✓ Copied`; `x` clears the clipboard now; a single deadline replaces per-copy timers, so an earlier copy can never clear a later one; countdown bar.
- **Sort** (`s`: date added, name, category) and **category filter** (`1`–`9` / `0`), plus category autocomplete in the form (`→` accepts).
- **Expiry reminders**: `◷` badge in the list, the detail card and a header counter; yellow within 14 days, red once expired.
- **Help overlay** (`?`) and a contextual footer (only keys that apply; `x` appears while a clipboard clear is pending).
- **Mouse**: click selects a row; the wheel scrolls the list without moving the selection.
- **Minimum terminal size guard** (60×18).
- **Idle lock**: after `PANDORA_LOCK_MINUTES` (default 5, `0` disables) everything is masked and a pending clipboard is cleared; any key unlocks and restores the previous screen, including a half-typed form.
- **Orphan detection**: secrets whose keyring value is missing are marked `!` (checked in the background with a spinner).

**Visual**
- Per-category colours, full-row selection, bordered detail card (title + badge, metadata, value box, notes), row separators, compact mode (`t`), position indicator and scrollbar, panes that fit their content.
- Pane title tags, focus marker in the form, toasts, countdown bars, header with gradient logo, keyring backend name, status chip and warnings.
- Welcome screen on first run, error state for unreadable metadata, spinner while the keyring is read.
- **Icons** with an ASCII fallback (`PANDORA_ASCII=1` or a non-UTF-8 locale).

**CLI and portability**
- `pandora list | get | add | rm | run`, `export` / `import` (metadata plain; values only with `--values`, encrypted with an age passphrase), and `doctor [--fix]`. See the README.

## Not planned

- **Network features, telemetry, sync.** "No network, no telemetry" is an explicit non-goal to change.
- **Keyring entries without metadata** in `doctor`: go-keyring cannot enumerate entries portably.
- **`tab` inside the filter prompt** to pick a category: the `1`–`9` keys cover it without overloading the list's own filter input.

## Testing notes

Tests isolate the keyring with `keyring.MockInit()`, the config directory by setting `HOME` / `XDG_CONFIG_HOME` / `AppData` to a temp dir, and the clipboard by stubbing `writeClipboard`. New store or TUI features should follow the same pattern (see `internal/store/keyring_test.go`, `internal/tui/update_test.go`, `internal/tui/features_test.go`) and never touch the real keychain or clipboard.
