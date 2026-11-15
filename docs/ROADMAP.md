# Roadmap

Features that fit Pandora's scope (local, keyring-backed, keyboard-driven) and close the gaps in the current tool. Sections below describe what is **not yet implemented**.

## Implemented

- **Delete** (`d`): confirmation dialog; removes the keyring value and the metadata entry. A value already missing from the keyring does not block cleanup.
- **Edit** (`e`): the add form prefilled; empty secret keeps the current value; renaming moves the value to the new keyring account before removing the old one, and preserves the creation date.
- **Reload** (`r`): re-reads `metadata.json`, keeping the selection when the secret still exists.
- **Duplicate titles**: adding or renaming onto an existing name is rejected with `✗ "<name>" already exists` and the form stays open. Entries duplicated by older versions are collapsed on load.
- **Safer persistence**: metadata is written atomically; a corrupt `metadata.json` is reported instead of being overwritten.
- **Category colours**: each category gets a stable colour (case-insensitive hash into an 8-colour adaptive palette), used for the row bar, the category text and the detail badge. Uncategorised secrets are muted.
- **List rows**: custom delegate with a category bar, a full-width tinted background on the selected row, ellipsis truncation and filter-match underlining.
- **Detail card**: title, category badge, created date and a bordered value box (accent border while revealed).
- **Terminal size guard**: below 60×18 a `Terminal too small` notice is shown instead of a broken layout.

## Priority 2 — day-to-day usability

| Feature | Key | Notes |
| ------- | --- | ----- |
| Per-item reveal | `v` | Make reveal apply to the selected item and reset when the selection changes or after N seconds, instead of one global toggle. |
| Auto-hide revealed value | — | Timer like the clipboard one (e.g. 10 s). |
| Copy cancel / clear now | `x` | Clear the clipboard immediately instead of waiting for the 15 s tick; also cancel stale ticks so a second copy isn't cleared early by the first. |
| Clipboard error feedback | — | `clipboard.WriteAll` errors are ignored today, so `✓ Copied` can be shown when it failed (no `xclip` on Linux, etc.). |
| Sort | `s` | Cycle name / category / created date. |
| Category filter | `tab` in filter, or `1..9` | Quick-filter by existing categories; category autocomplete in the form. |
| Notes / URL field | — | Optional non-secret text in metadata (e.g. account, URL, expiry reason). Schema addition, backwards compatible. |
| Generate secret | `ctrl+g` in the form | Random password/token with selectable length and charset (`crypto/rand`). |
| Expiry / rotation reminder | — | Optional `expires` date; show a warning badge in the list and header when near or past. |
| Help overlay | `?` | Full key list; footer stays short. |

## Visual improvements to the TUI

Assessment from running the current build at 100×30: the layout is correct but visually flat. Both panes are the same size and mostly empty space (the list holds a few rows, the detail pane three lines), the list is plain text with a thin selection bar, the only colour is a single purple accent, and nothing changes visually between states. These changes keep the default look (no theming system) but make it richer.

### Structure and density
- **Fit the content, not the terminal**: cap the panes' height to their content (with a minimum) and centre or top-align the block, instead of drawing two tall empty boxes. Alternative: keep full height but use the spare space for the items below.
- **Detail card**: add an updated/expiry block and a notes block (title, category badge, created date and bordered value box are done).
- **Row separators and padding** in the list; one-line rows (`title … category`) as a compact mode toggled with `t`, three-line rows as the roomy default.
- **Scrollbar / position indicator** (`3/12`) in the list pane instead of the default paginator dots.

### Colour and typography
- **Focus indication**: bold border plus title tag (`╭─ Secrets ─╮`) on the active pane, muted on the other; focused input in the form gets an accent border/underline, unfocused inputs are muted.
- **Status as toasts**: render the status message as a bordered pill in the bottom-right corner (green/red) that disappears after 3 s, instead of a plain line that shifts the layout.
- **Header**: a logo glyph and gradient (accent → secondary) on `PANDORA`; vault status chip (`● unlocked`), and the keyring backend name (`Keychain`, `Secret Service`, `Credential Manager`).

### Feedback and state
- **Clipboard countdown**: show `Copied · clears in 12s` with a progress bar that drains, updated each second via `tea.Tick`.
- **Reveal countdown** (with auto-hide) shown the same way.
- **Icons** (Unicode, with an ASCII fallback when `PANDORA_ASCII=1` or a non-UTF-8 locale): lock/unlock for masked/revealed, a clock for expiring secrets, `!` for orphaned entries.
- **Expiry badges**: yellow `expires in 5d`, red `expired`.
- **Empty and error states** with an illustrated ASCII vault and the two or three keys that matter, instead of a single line.
- **Spinner** while the keyring is being read (some backends prompt or are slow).

### Interaction polish
- **Modals for confirmations** (delete) with a danger-coloured border and the secret's name highlighted.
- **Contextual footer**: show only the keys that apply in the current mode (e.g. no `v`/`c` when the list is empty), with `?` for the full list.
- **Welcome screen** on first run (no metadata): one-paragraph explanation of where values are stored and a `press n` call to action.
- **Mouse support**: click to select, scroll wheel in the list (`tea.WithMouseCellMotion`).

### Suggested visual order
1. ~~Per-category colours + full-row selection highlight~~ (done)
2. ~~Detail card with a bordered value box~~ (done)
3. Toast-style status and clipboard countdown bar.
4. Content-fit layout and compact mode.
5. Welcome/empty states, icons, mouse.

## Priority 3 — data portability and safety

- **Export / import** (`pandora export`, `pandora import`): metadata only by default; values only with an explicit flag, and the export file encrypted (e.g. `age` passphrase). Never write plaintext values without an explicit warning.
- **Backup of `metadata.json`** before each write (`metadata.json.bak`). Writes are already atomic (temp file + rename).
- **Orphan detection** (`pandora doctor`): metadata entries without a keyring value, and keyring entries under `pandora-cli` without metadata; offer to repair.
- **Lock after inactivity**: mask everything and clear the clipboard after N minutes idle.
- **Audit-free by design**: keep "no network, no telemetry" as an explicit non-goal to change.

## Priority 4 — CLI mode (scriptable)

Non-interactive subcommands that share `internal/store`:

```sh
pandora list [--category TOKENS]
pandora get <title>          # prints value to stdout
pandora add <title> -c TOKENS   # value read from stdin / prompt
pandora rm <title>
pandora run -s <title> -- cmd   # inject secret as env var for one command
```

`get`/`run` make Pandora useful in scripts and shells without exposing values in shell history.

## Suggested order

1. Visual improvements (see above), starting with per-category colours and the full-row selection highlight.
2. Per-item reveal and clipboard fixes.
3. Backup of `metadata.json`, `pandora doctor`.
4. CLI mode.
5. Everything else as needed.

## Testing notes

Tests isolate the keyring with `keyring.MockInit()` and the config directory by setting `HOME` / `XDG_CONFIG_HOME` / `AppData` to a temp dir. New store or TUI features should follow the same pattern (see `internal/store/keyring_test.go`, `internal/tui/update_test.go`) and never touch the real keychain.
