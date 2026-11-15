# Architecture

## Packages

| Package          | Responsibility |
| ---------------- | -------------- |
| `cmd/pandora`    | Entry point. With arguments it runs `cli.Run`; without, it starts the Bubble Tea program (alternate screen, mouse cell motion) with `tui.New()`. Errors go to stderr, exit code 1. |
| `internal/cli`   | Non-interactive subcommands (`list`, `get`, `add`, `rm`, `run`, `gen`, `export`, `import`, `doctor`, `version`, `completion`). Takes its streams as arguments and returns an exit code; never calls `os.Exit`. |
| `internal/tui`   | The interface, split by Bubble Tea role (below). Talks to storage only through `store`. |
| `internal/store` | Keyring access and the metadata file. No UI dependencies. |
| `internal/gen`   | Random secrets (`crypto/rand`): charsets, preset lengths, `Generate`. Used by the form and `pandora gen`. |
| `internal/version` | Version, commit and date: set with `-ldflags -X` by the release build, falling back to `runtime/debug.ReadBuildInfo` (pseudo-versions of untagged builds report `dev`). |

### `internal/store`

| Function | Behaviour |
| -------- | --------- |
| `SaveSecret(meta, value)` | Fails with `ErrExists` if the title is recorded. Writes the keyring value, then the metadata; if the metadata write fails the keyring entry is rolled back. |
| `GetSecretValue(title)` | Reads the value from the keyring. |
| `UpdateSecret(old, meta, newValue)` | Replaces name, category, expiry and notes and, when `newValue != nil`, the value. `Created` is preserved and `Updated` is set to today. Renaming: set new keyring entry → write metadata → delete old entry (rolls back the new entry if the metadata write fails). `ErrNotFound` / `ErrExists` for a missing source / taken target. |
| `DeleteSecret(title)` | Deletes the keyring entry (already missing is fine), then the metadata entry. `ErrNotFound` if not recorded. |
| `LoadMetadata()` | Reads `metadata.json`; missing file → empty list; collapses duplicate titles from older versions (last wins). Invalid JSON is an error. |
| `MissingValues()` | Titles whose keyring value is missing (used by `pandora doctor`). |
| `RestoreSecrets(...)` | Batch import: values first, then one metadata write; rolls back newly stored values on failure. |
| `ParseDate`, `SecretMeta.DaysUntilExpiry` | `YYYY-MM-DD` handling shared by the form, list badges and CLI. |

Metadata writes go through a temp file in the same directory and `os.Rename`, so a crash cannot leave a half-written file; the previous contents are first copied to `metadata.json.bak`. Every mutating function loads metadata first and aborts on error, so a corrupt file is never overwritten.

### `internal/cli`

`cli.Run(args, stdin, stdout, stderr) int` looks the command up in one table (`registry.go`: name, summary, flags with value hints, positional kind) and calls its handler, each with its own `flag.FlagSet`. The same table builds the usage text and the bash/zsh/fish/PowerShell completion scripts (`completion.go`); a test runs `<cmd> -h` for every command and fails if the table's flags drift from the real ones. The hidden `__complete titles|categories` helper feeds completions and prints names only, never values, and stays silent on any error. Exit codes: `0` ok, `1` runtime error, `2` usage error. `export --values` encrypts with age (scrypt passphrase recipient, armored); `export` without `--values` is plain JSON without values. `run` injects secrets into the child's environment only and forwards signals. Secret values are printed only by `get` (and generated ones by `gen`, which stores nothing).

### `internal/tui`

| File          | Contents |
| ------------- | -------- |
| `model.go`    | `model` state, list `item`, `mode`, `New()`, `Init`, `reload` (category filter + sort), pane geometry, scroll clamping, orphan check command |
| `update.go`   | `Update` (scroll/reveal bookkeeping around `update`), the 1 s `tick`, key and mouse handling, form (`openForm`, `updateForm`, `saveForm`), delete confirmation, lock |
| `view.go`     | `View` and the `render*` helpers: header, panes, list crop + scrollbar, detail card, toast overlay, countdowns, form, help overlay, welcome / error state, footer |
| `delegate.go` | `rowDelegate`: roomy (3-line) and compact (1-line) list rows, selection fill, orphan / expiry markers |
| `styles.go`   | Adaptive palette, per-category colours (`categoryColor`), gradient, pane tag border |
| `icons.go`    | `glyphSet`: Unicode glyphs and the ASCII fallback (`PANDORA_ASCII=1` or a non-UTF-8 locale); `detectNoColor` / `applyColorMode` (`NO_COLOR` or `TERM=dumb`: forces the Ascii colour profile and adds a selection marker) |
| `expiry.go`   | `expiryInfo`: label and level (`far` / `soon` ≤14 days / `expired`) for a secret |

**Modes.** `model.mode` decides what the keyboard drives:

```mermaid
stateDiagram-v2
    [*] --> Browse
    Browse --> Form: n (add) / e (edit)
    Form --> Browse: esc, or successful save
    Browse --> ConfirmDelete: d
    ConfirmDelete --> Browse: y (delete) / any other key
    Browse --> Help: ?
    Help --> Browse: any key
    Browse --> Locked: idle timeout
    Form --> Locked: idle timeout
    Locked --> Browse: any key (restores the previous mode)
```

While the list filter is active, browse-mode keys go to the list unchanged. Any key press clears the status message before it is handled. Failed saves keep the form open so nothing typed is lost.

**Single source of truth.** After every mutation (add, edit, delete, `r`, `s`, category change) the model calls `reload(selectTitle)`, which re-reads `LoadMetadata()`, applies the category filter and sort, and calls `list.SetItems`; it never patches the list by index. `reload` restores the selection by title.

**Time.** One `tickMsg` per second (started in `Init`) drives everything time-based: reveal auto-hide, the clipboard deadline, toast expiry and the idle lock. State holds *deadlines* (`revealUntil`, `clipUntil`, `toastUntil`, `lastActive`), never per-action timers, so a second copy cannot be cleared by the first copy's timer. `model.now` is injectable for tests.

**Reveal.** Only `revealTitle` is shown, until `revealUntil`. `Update` clears it whenever the selected title differs.

**Clipboard.** `writeClipboard` and `readClipboard` (variables, stubbed in tests) are the only clipboard access. Failures are reported instead of showing `✓ Copied`. Copying stores a SHA-256 of the value (`clipHash`), never the value. Clearing (deadline, `x`, lock) reads the clipboard first and clears only when it still matches (trailing newlines ignored); a read error clears anyway, and content that no longer matches is left alone (`x` says so; the automatic paths are silent). The lock screen clears a pending clipboard the same way.

**Orphans.** `Init` and `r` run `checkOrphans` off the UI thread (some keyring backends are slow or prompt); the result (`orphanMsg`) marks rows and the detail card of secrets whose value is missing. A spinner shows in the header meanwhile.

**Layout.** Computed from the last `tea.WindowSizeMsg`: the list pane gets `max(30, (w-6)*45/100)` columns, the detail pane the rest (6 = two borders per pane plus a 2-column gap). Panes fit their content: outer height `clamp(listChrome + n*rowHeight + 4, minPaneOuter, h-4)`, top-aligned, with the status row and footer pinned to the bottom, so the view is always exactly `h` lines. Below 60×18 the view is replaced by `Terminal too small`. The list is a bubbles `list.Model` sized to hold every item on one page; `listView` crops its rendered lines to a window starting at `model.scroll` (the wheel changes only that offset, never the selection), replaces the status bar text with the position indicator, and appends the scrollbar. Toasts are composed over the bottom-right of the detail pane with `ansi.Truncate`/`TruncateLeft`.

**Mouse geometry.** Click hit-testing uses `listTopRow` (8) and the row height; `TestClickGeometryMatchesRenderedList` and `TestCompactRowsGeometry` fail if the rendered list moves.

## Data flow

### Save (add or edit)

```mermaid
sequenceDiagram
    participant U as User
    participant T as tui.saveForm
    participant S as store
    participant K as OS keyring
    participant F as metadata.json
    U->>T: enter on last field / ctrl+s
    T->>T: validate (name; secret when adding; expiry date)
    T->>S: SaveSecret / UpdateSecret
    S->>F: LoadMetadata (duplicate check)
    S->>K: Set(service, title, value)
    S->>F: backup, then atomic write
    S-->>T: nil / ErrExists / error
    T->>S: reload via LoadMetadata
    T->>T: toast "✓ Saved" / "✓ Updated", or error with form kept open
```

### Delete

`d` → confirmation dialog → `y` → `store.DeleteSecret` → `reload` → `✓ Deleted "<name>"`. Any other key cancels with no side effects.

### Copy

`enter`/`c` → `store.GetSecretValue(title)` → `writeClipboard(value)`; on success `clipUntil = now + 15 s`, `clipHash` records the value's hash and a countdown bar appears. The tick clears the clipboard when the deadline passes (see *Clipboard* above); `x` clears it immediately. A new copy simply moves the deadline and the hash.

## Persisted data

`<os.UserConfigDir()>/pandora/metadata.json` (directory `0700`, file `0600` on Unix; Windows has no POSIX modes and relies on the user profile ACLs) holds a JSON array of `store.SecretMeta`:

```json
[
  {
    "title": "githubToken",
    "category": "TOKENS",
    "created": "2026-11-15",
    "updated": "2026-11-20",
    "expires": "2027-01-31",
    "notes": "https://github.com/settings/tokens"
  }
]
```

| Field      | Type   | Meaning |
| ---------- | ------ | ------- |
| `title`    | string | Secret name, unique; also the keyring account |
| `category` | string | Free-form label |
| `created`  | string | `YYYY-MM-DD` creation date |
| `updated`  | string | Optional; set on every edit |
| `expires`  | string | Optional `YYYY-MM-DD`; drives the warning badges |
| `notes`    | string | Optional free text (e.g. a URL); never put values here |

Secret values are stored only in the OS keyring, service `pandora-cli`. The JSON field names are a persisted format: changes must be backwards compatible (a missing field must have a sensible default; the three optional fields are `omitempty`).

## Testing

Tests replace the keyring with `keyring.MockInit()`, point `HOME`, `XDG_CONFIG_HOME` and `AppData` at a temp dir, and stub `writeClipboard` / `readClipboard`, so nothing touches the real keychain, config or clipboard. `internal/tui` tests drive `Update` with key, mouse and tick messages (with an injected clock) and assert on model state, store contents and the rendered view. The single exception is `internal/store/integration_test.go` (build tag `integration`, and it also needs `PANDORA_INTEGRATION=1`): it talks to the real OS keyring and only runs in the Linux CI job, inside a throwaway D-Bus session. Release QA on real keychains and terminals is a manual checklist: [QA.md](QA.md).

## Known limitations

- A secret deleted or renamed by another Pandora instance is only noticed after `r` (reload); acting on it fails with a status message.
- Two concurrent instances can race on `metadata.json` (last writer wins).
- go-keyring cannot enumerate entries, so `doctor` finds metadata without a value but not keyring entries without metadata.
- Selecting text with the mouse needs `Shift` (`Alt` in some terminals) because Pandora captures the mouse.
