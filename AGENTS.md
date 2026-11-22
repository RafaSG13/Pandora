# Repository Guidelines

## Project Overview

Pandora is a Go terminal secrets manager: a Bubble Tea TUI (`pandora`) and a scriptable CLI (`pandora list|get|add|rm|run|passwd|export|import|doctor|gen|completion`). Everything is protected by a **master password**. Names, categories, notes, dates and values are encrypted; values live in the OS keyring under opaque random IDs. No network code, no telemetry, single static binary (`CGO_ENABLED=0`).

Module `github.com/RafaSG13/Pandora`, Go 1.27.4 (`go.mod`; CI and release read the version from it).

## Architecture & Data Flow

```
cmd/pandora/main.go ── args? ── cli.Run(...)  ── exit code 0/1/2
                      └ none ── tea.NewProgram(tui.New())
                       both call only ▼
internal/store  (no UI deps)
   vault.json (Argon2id-wrapped data key + XChaCha20-Poly1305 records)  +  OS keyring (service "pandora-cli")
```

- **Layering is strict.** `tui` and `cli` talk to `store` only, by **title**. `store` maps title → record ID internally; `SecretMeta` has no ID, so export/import JSON and callers never see IDs.
- **Session key.** `store.Create/Unlock` hold the data key in a package-level session; every store function returns `ErrLocked` without it. `store.Lock()` zeroes it. Tests and the TUI idle lock both rely on this global.
- **Write path** (`store/keyring.go`, `chunks.go`): `loadRecords` (decrypt) → seal the value (`"pv2:"+base64(nonce‖AEAD(value, AAD=id))`) → split into keyring entries of ≤ 2000 chars (`<id>`, `<id>.1`, … because Windows allows 2560 bytes and macOS ~4 KB per entry; the record's optional `chunks` counts them) → `saveRecords` (fresh nonce, `writeFileAtomic` with `.bak`). Values are text up to `MaxValueBytes` (32 KB, `ErrValueTooLarge`). Replacing a value (`UpdateSecret`, `RestoreSecrets`) writes it under a **fresh ID** and removes the old entries only after the vault points at the new ones; a vault-write failure removes what was just stored. Delete tolerates `keyring.ErrNotFound` so half-deleted secrets can be cleaned up.
- **Unknown title** from `GetSecretValue` returns `keyring.ErrNotFound` on purpose, so `store.IsValueMissing`, `cli.secretValue` and the TUI orphan check treat "unknown" and "value missing" alike.
- **Migration** (`store/migrate.go`): an old plaintext `metadata.json` + title-named keyring entries are converted on first start; on any failure nothing legacy is touched. A `*store.CleanupError` means "vault complete, old data not fully removed" (warn, don't fail).
- **CLI** (`internal/cli`): one command table in `registry.go` drives dispatch, help text and bash/zsh/fish/PowerShell completion. Handlers are `(*app).cmdX(args) error`; `app` holds the streams plus replaceable hooks (`getenv`, `environ`, `isTTY`, `readHidden`) so tests never need a real terminal. `Run` never calls `os.Exit`.
- **TUI** (`internal/tui`): value-receiver `model`, thin `Update` wrapper around `update`, `View` dispatching on `mode` (`modeBrowse/Form/ConfirmDelete/Help/Unlock/Setup`). All time behaviour is deadline-based (`revealUntil`, `clipUntil`, `toastUntil`, `lastActive`) driven by one 1 s `tickMsg` and the injectable `model.now`; there are no per-action timers.

## Key Directories

- `cmd/pandora/` entry point.
- `internal/store/` vault, session, keyring, value chunking (`chunks.go`), migration, restore, doctor, date helpers.
- `internal/cli/` commands, registry, unlock/passwd, export/import (age), `run`, completion.
- `internal/tui/` Bubble Tea UI: `model.go` (state, `New`, layout), `update.go` (`update`, `tick`, `lock`, auth, form), `view.go` (render, footer), `secretfield.go` (secret textarea helpers), `delegate.go` (list rows), `styles.go`, `icons.go` (`glyphSet` `g`, NO_COLOR), `expiry.go`.
- `internal/secretclip/` `secretclip.Write`: concealed clipboard copy per OS (`clipboard_{darwin,windows,other}.go`); the TUI wraps it as `writeSecretClipboard` (stub it in tests) and the CLI as `app.copySecret`.
- `internal/gen/` `crypto/rand` secret generator; `internal/version/` build metadata.
- `internal/config/` `config.json` loader (`Load` → `Settings` + warnings; env > file > default, never fails). The TUI reads it once in `New` (`model.cfg`; `useGlyphs` applies `ascii`), the CLI through `app.loadConfig`. Tests that run `gen` or `config` must call `isolate(t)` so they never read the developer's real file; a new setting needs a key in `config.Keys`, a case in `set` and `Get`, and a README row.
- `docs/` `ARCHITECTURE.md` (vault format, data flows), `ROADMAP.md` (workflow + items), `QA.md` (manual checklist). `scripts/release-notes.sh`.

## Development Commands

```sh
make run            # go run ./cmd/pandora (TUI)
make build          # bin/pandora with version ldflags
make vet && make test
gofmt -l .          # must print nothing
go test -race ./...
GOOS=windows go build ./... && GOOS=linux go build ./... && CGO_ENABLED=0 go build ./cmd/pandora
go test ./internal/tui -run '^TestIdleLockHidesEverythingAndNeedsThePassword$' -count=1 -v
```

There is no linter config: `gofmt`, `go vet`, tests are the gate. Never run the real keyring on a dev machine except deliberately; the one real-keyring test is CI-only (`PANDORA_INTEGRATION=1 go test -tags integration -run TestRealKeyring ./internal/store`).

Env vars: `PANDORA_MASTER_PASSWORD` (CLI, no terminal), `PANDORA_PASSPHRASE` (age export/import), `PANDORA_LOCK_MINUTES` (default 5, `0` disables), `PANDORA_ASCII=1`, `NO_COLOR`. `run` strips both password variables from the child's environment. Other settings come from `config.json` (see Architecture); precedence is env > file > default.

## Code Conventions & Common Patterns

- **Errors:** exported sentinels in `store` (`ErrLocked`, `ErrWrongPassword`, `ErrPasswordTooShort`, `ErrInvalidText`, `ErrExists`, `ErrNotFound`, `ErrVaultExists`), tested with `errors.Is`. CLI exit codes: `0` ok, `1` runtime, `2` usage (`usageErrorf`); messages go to stderr prefixed `pandora: `.
- **Never overwrite a corrupt or tampered vault.** Every mutator loads/decrypts first and aborts on error. Keep that when adding writers.
- **Order writes so a failure cannot lose a value:** write the new thing, then remove the old.
- **Persisted format is an API.** `vault.json` is `version: 2`; KDF params are untrusted input (`checkKDF` bounds them before any derivation). New record fields must be `omitempty` with a safe default; a real format change needs a new version and a migration.
- **No plaintext titles/categories/notes on disk, in the keyring, or in logs.** `store.ValidateText` rejects control characters (escape-sequence injection) in Title/Category/Notes; `parseExport` calls it too.
- **Never log or print secret values or passwords**, including in errors and toasts.
- **Secret-in-env:** anything that accepts a password from the environment calls `a.warnEnv(name)` once per process.
- **TUI rules:**
  - Argon2 work (`Unlock`, `Create`, `MigrateLegacy`) runs in a `tea.Cmd` (`unlockCmd` → `unlockedMsg`), never on the UI thread; re-check state when the result arrives (`orphanMsg` is dropped if locked).
  - `lock()` must clear every plaintext the model holds (list, filters, form inputs, reveal, clipboard hash). A new screen needs a `mode`, a `locked()` decision and a `renderHelp` footer case.
  - View height must equal the window height and width must not exceed it (floor 60×18). Wrap or `truncate` dynamic text. Add a size test when you touch layout.
  - Glyphs go through `g`; a new literal Unicode glyph must be added to `asciiReplacer` in `view.go`. Never encode meaning in colour alone.
  - Messages that must not scroll to the selection (`tick`, spinner, orphan) are listed in the `Update` wrapper switch.
  - If list chrome changes, update `listTopRow`, `listChrome`, `itemsInView` and `rowDelegate.Height()` together (mouse hit-testing).
  - The form's secret is a `bubbles/textarea` (`model.secret`; `inputs[fieldSecret]` is an unused slot that keeps field indices stable). `enter` = newline, `tab` next field, `ctrl+s` save, `ctrl+r` show/hide; it cannot mask, so masked state is drawn by `maskedLines`. Its height `clamp(h-18+1, 1, 8)` is what keeps the form exactly window-high at 60×18: don't add form rows without re-checking that. The textarea's default 99-row / 500-column caps are lifted on purpose (they silently cut keys and tokens).
  - The form has six fields (`Tags` is last): `enter` moves on and saves from the last one, and at 60×18 it has no spare row left (the status line sits directly under the hints).
  - A revealed value is read once into `revealValue` and must be dropped via `hideReveal()` (hide, expiry, lock, delete, save), never re-fetched per frame: chunked values cost several keychain calls.
- **Naming:** unexported helpers, `cmdXxx` for commands, `renderXxx` for views, `TestXxx` that states the behaviour. Keep exported identifiers documented.
- **Commits/PRs:** short imperative subject, body explains why; update `CHANGELOG.md` under `[Unreleased]` for user-visible changes.

## Important Files

- `internal/store/vault.go` (format, KDF, session, `writeFileAtomic`), `keyring.go` (CRUD), `chunks.go` (value split across keyring entries, `MaxValueBytes`), `migrate.go`, `restore.go`.
- `internal/cli/registry.go` (command table), `unlock.go`, `transfer.go` (export/import, `parseExport`), `completion.go`.
- `internal/tui/model.go`, `update.go`, `view.go`, `secretfield.go` (secret textarea, masking, reveal helpers).
- `docs/ARCHITECTURE.md`, `docs/ROADMAP.md`, `CONTRIBUTING.md`, `SECURITY.md`, `.goreleaser.yaml`, `.github/workflows/{ci,release}.yml`.

## Runtime/Tooling Preferences

- Go toolchain only (no Bun/Node); Go modules, no vendoring. `golang.org/x/crypto` (argon2, chacha20poly1305), `filippo.io/age` (exports only), `zalando/go-keyring`, Charm libraries (`bubbletea`, `bubbles`, `lipgloss`).
- Releases are CGO-off cross-builds via GoReleaser. To cut one: rename `## [Unreleased]` in `CHANGELOG.md` to `## [X.Y.Z] - date` (the heading format matters to `scripts/release-notes.sh`), merge, then `git tag vX.Y.Z && git push origin vX.Y.Z`; the workflow publishes archives, checksums, notes and the Homebrew formula. Don't push a tag without being asked.
- Roadmap work (`docs/ROADMAP.md`): pick the first `todo` item whose dependencies are `done`, produce a plan, and stop for the item's "Ask the user" questions before coding. Done means the Acceptance list holds, the program was actually run (TUI in a pty, CLI against a temp `HOME`), docs/CHANGELOG are updated and the item's status is flipped.

## Testing & QA

- Plain `testing` (no testify), tests live in the same package and use unexported symbols. No `t.Parallel()`: the store session and TUI clipboard stubs are globals, so tests must stay serial and reset them.
- **Isolation recipe** (copied per package; use the existing helper, don't write your own): `keyring.MockInit()`, `t.Setenv("HOME"|"XDG_CONFIG_HOME"|"AppData", tempdir)`, `store.FastKDFForTests()` (tests only!), `t.Cleanup(store.Lock)`, `store.Create("correct horse battery")`.
  - store: `setup(t)`, `mustSave`, `idOf`, `setupLegacy` (plaintext legacy profile).
  - cli: `isolate(t)` and `harness{env, tty, hidden, stdin, noMaster}.run(t, args...)` → `(code, stdout, stderr)`; each run locks the store first, like a new process.
  - tui: `newTestModel(t)` / `newModelFor(t, createVault, w, h)`, `press`, `ctrl`, `withClock` + `tickAt` (never `time.Sleep`), `submitPassword`/`enterAndRun` to run the unlock `tea.Cmd`. In the add form `enter` inside **Secret** is a newline: use `tab` to leave it (`add(m, name, cat, secret)` does). `textarea` scrolls using the lines its last `View` rendered, so tests that read the editor view after typing should render between keys.
- Test behaviour, boundaries and error paths; don't pin wording or wiring. Skip POSIX-permission tests on Windows/root.
- The CLI registry drift tests (`registry_test.go`) fail if the command table and the real flag sets diverge: extend `commands()`, don't bypass the tests.
- Before a PR: `gofmt -l .` empty, `go vet ./...`, `go test -race ./...`, the three cross-builds above, and run the changed path for real. Release QA on real keychains is the manual list in `docs/QA.md`.
