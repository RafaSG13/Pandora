# Roadmap

Everything planned before the first release is implemented (see the [changelog](../CHANGELOG.md)). This file is the forward plan, written to be **read by an engineer or an AI agent that then produces a detailed implementation plan** for the next item. Read [How to use this document](#how-to-use-this-document) first.

## How to use this document

### Picking the item

1. If the user named an item (`R-07`), use it. Otherwise take the **first item in the [index](#index) whose status is `todo` and whose `Depends` are all `done`**.
2. Re-verify the item's *Starting points* against the code: files and symbols drift. If something moved, trust the code and fix the roadmap entry in the same change.
3. Set the item's status to `in-progress` when implementation starts, not when planning starts.

### Producing the plan (before writing code)

Reply with a plan containing exactly these sections, then **stop and wait for answers if any `Ask the user` question is unresolved**. Questions that have a *Recommended* default may be answered by choosing it, but say so.

| Section | Content |
| --- | --- |
| Findings | What the code does today that the item touches (file, symbol, behaviour), confirmed by reading it, not assumed. |
| Decisions | Each open design decision with options, the chosen one and why. Start from the item's *Decisions*; add new ones you find. |
| Questions | Anything only the user can decide. Keep the item's `Ask the user` list; drop answered ones. |
| Steps | Ordered, each small enough to verify alone: files, functions, data-format changes. |
| Test plan | Behaviour and boundary tests to add or change (see [Testing rules](#testing-rules)) and the manual smoke run that proves the change. |
| Docs | Which of README, ARCHITECTURE, CHANGELOG, SECURITY, this roadmap change. |
| Risks | Persisted-format, security, cross-platform and migration risks, and how each is contained. |

### Finishing

An item is `done` only when its **Acceptance** list holds *and*:

- `gofmt -l .` is empty, `go vet ./...` and `go test -race ./...` pass, and `GOOS=windows go build ./...` and `GOOS=linux go build ./...` succeed;
- the changed path was exercised by running the program (TUI in a pty, CLI against a temp `HOME`), not only by tests;
- README / ARCHITECTURE / CHANGELOG (`[Unreleased]`) are updated, obsolete code is removed (clean cutover, no compatibility shims), and this file's status is flipped to `done (<PR or version>)`.

### Item fields

`Status` is one of `todo`, `in-progress`, `blocked (<reason>)`, `done (<ref>)`, `dropped (<reason>)`. `Size` is S (≤ half a day), M (1–2 days), L (a week or more, needs design first). `Depends` lists item IDs that must be `done` first; `soft` means helpful but not required.

## Constraints that apply to every item

- **No network code in the binary and no telemetry.** See [Not planned](#not-planned); lifting it needs an explicit user decision.
- **Secret values live only in the OS keyring** (service `pandora-cli`), never in metadata, logs, error messages, toasts or test output. Exports with values are encrypted.
- **`metadata.json` is a persisted format.** New fields are `omitempty` and a missing field must have a sensible default, so old files keep loading and a downgrade does not corrupt anything. Any item that cannot honour this must say so in its plan and ask the user.
- **Layering.** `internal/store` has no UI dependency; `internal/tui` reaches the keyring and files only through `store`; `internal/cli` takes its streams as arguments and returns an exit code (`0` ok, `1` runtime, `2` usage).
- **Terminal floor.** The TUI must stay usable at 60×18, have an ASCII fallback (`PANDORA_ASCII=1`), and keep the view exactly the window height.
- **Time-based behaviour uses deadlines** driven by the 1 s `tickMsg` and the injectable `model.now`, not per-action timers.

### Testing rules

Never touch the real keychain, config directory or clipboard. Use `keyring.MockInit()`, temp `HOME` / `XDG_CONFIG_HOME` / `AppData`, and the stubbed `writeClipboard` (see `internal/store/keyring_test.go`, `internal/tui/update_test.go`, `internal/tui/features_test.go`). Test behaviour, boundaries and error paths; do not pin wording or wiring.

## Index

| ID | Item | Milestone | Size | Depends | Status |
| --- | --- | --- | --- | --- | --- |
| R-01 | Version information | 0.1.0 | S | – | todo |
| R-02 | Go module path | 0.1.0 | S | – | todo |
| R-03 | Release pipeline | 0.1.0 | M | R-01, R-02 | todo |
| R-04 | Homebrew tap and install docs | 0.1.0 | S | R-03 | todo |
| R-05 | Real-environment QA | 0.1.0 | M | – | todo |
| R-06 | Security model hardening (docs) | 0.1.0 | S | – | todo |
| R-07 | Clipboard hygiene | 0.1.0 | M | – | todo |
| R-08 | Shell completions | 0.1.0 | M | – | todo |
| R-09 | `pandora gen` | 0.1.0 | S | – | todo |
| R-10 | `NO_COLOR` and small-terminal help | 0.1.0 | S | – | todo |
| R-20 | Multiline values | 0.2 | M | – | todo |
| R-21 | Tags | 0.2 | M | – | todo |
| R-22 | Search in notes and tags | 0.2 | S | R-21 (soft) | todo |
| R-23 | Config file | 0.2 | M | – | todo |
| R-24 | Value history | 0.2 | L | R-23 (soft) | todo |
| R-25 | Import from other tools | 0.2 | M | – | todo |
| R-26 | Dotenv output and CLI ergonomics | 0.2 | S | – | todo |
| R-27 | External-change detection and file lock | 0.2 | M | – | todo |
| R-28 | Keyring index and recovery | 0.2 | L | R-24 (soft) | todo |
| R-30 | `expiring` command | 0.3 | S | – | todo |
| R-31 | Unlock challenge | 0.3 | L | R-23 | todo |
| R-32 | Local audit log | 0.3 | M | R-23 | todo |
| R-40 | TOTP codes | later | L | R-20 (soft) | todo |
| R-41 | Folder-sync vault file | later | L | R-23, R-28 | todo |

Milestones are a suggested cut, not a contract: **0.1.0** is "safe and easy to install", **0.2** is data model and workflows, **0.3** is security hardening and automation.

---

## Milestone 0.1.0 — publishable

### R-01 Version information
**Status** todo · **Size** S · **Depends** –

**Why.** There is no `--version` (no `-ldflags`, no `debug.ReadBuildInfo` use anywhere), so a bug report cannot say which build it came from.

**Scope.** `pandora version` and `pandora --version` print version, commit, build date and Go version; the TUI header or help overlay shows the version. Out: update checks (network).

**Starting points.** `cmd/pandora/main.go`, `internal/cli/cli.go` (`topUsage`, the command dispatch in `run`), `Makefile` (`build`).

**Decisions.**
- Source of truth: `-ldflags -X` set by the release build, falling back to `runtime/debug.ReadBuildInfo()` so `go install` and `go build` still report something useful. *Recommended.*
- Where the variables live: a tiny `internal/version` package, so both `cli` and `tui` can read it without importing `main`.
- Output format stays one line, stable and greppable (`pandora 0.1.0 (abcdef1, 2026-11-20, go1.27.4)`).

**Ask the user.** None expected.

**Acceptance.** `pandora --version` and `pandora version` exit 0 with the same text; an unreleased build prints `dev`; the Makefile and (later) the release pipeline inject the real values; a test covers the `ReadBuildInfo` fallback formatting.

**Docs.** README (Command-line mode), CHANGELOG.

### R-02 Go module path
**Status** todo · **Size** S · **Depends** –

**Why.** `go.mod` declares `module pandora`, so `go install github.com/RafaSG13/Pandora/cmd/pandora@latest` cannot work, and the name collides with any other local module called `pandora`.

**Scope.** Rename the module to `github.com/RafaSG13/Pandora` and rewrite every `pandora/internal/...` import. Mechanical, but it touches every package, so do it in its own commit/PR before anything else is in flight.

**Starting points.** `go.mod`; `grep -rn '"pandora/' --include=*.go`; README and CONTRIBUTING mention of `go install`/`go build`.

**Decisions.**
- Use the GitHub path verbatim (case matters for the proxy). *Recommended.*
- Do it before R-03/R-04 so release artefacts and docs never reference the old path.

**Ask the user.** Confirm the canonical repository path (`github.com/RafaSG13/Pandora`) and that it will not be renamed soon.

**Acceptance.** `go build ./...`, `go vet ./...`, `go test ./...` pass; no `"pandora/` import remains; `go install github.com/RafaSG13/Pandora/cmd/pandora@<ref>` works from a clean `GOPATH` once the change is on the remote.

**Docs.** README install section, CONTRIBUTING.

### R-03 Release pipeline
**Status** todo · **Size** M · **Depends** R-01, R-02

**Why.** Only `.github/workflows/ci.yml` exists; there are no tags and no binaries. `SECURITY.md` says "no tagged releases yet".

**Scope.** On a `v*` tag, build and publish macOS, Linux and Windows archives for amd64 and arm64 with checksums, release notes from the changelog, and the version injected per R-01. Tag `v0.1.0` after R-05 passes. Out: signing infrastructure beyond what is decided below.

**Starting points.** `.github/workflows/ci.yml` (matrix, `go-version-file`), `Makefile`, `CHANGELOG.md` (`[Unreleased]` → `[0.1.0]`), `SECURITY.md` (Supported versions).

**Decisions.**
- Tooling: GoReleaser with a `.goreleaser.yaml` and a `release.yml` workflow. *Recommended* over hand-written matrix jobs (checksums, archives and Homebrew come for free).
- Archive naming `pandora_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows); `checksums.txt`.
- Signing: checksums only for 0.1.0; cosign keyless or macOS notarisation later. State the limitation (Gatekeeper warns on unsigned downloads) in the install docs.
- CGO off. Verify the keyring and clipboard libraries do not need it on the three OSes (they use `dbus`/exec helpers, not cgo; confirm by cross-building).

**Ask the user.** Versioning policy (SemVer, `v0.x` may break the CLI?) and whether to pay for Apple notarisation now.

**Acceptance.** Pushing a throwaway tag on a fork or `--snapshot` locally produces the six archives plus checksums; each archive's binary prints the injected version; CI keeps its current checks; `SECURITY.md` lists the supported version.

**Docs.** README (Install), CONTRIBUTING (how to cut a release), SECURITY, CHANGELOG.

### R-04 Homebrew tap and install docs
**Status** todo · **Size** S · **Depends** R-03

**Why.** Homebrew is the natural channel for a macOS terminal tool.

**Scope.** A `RafaSG13/homebrew-tap` repository updated by the release pipeline; an Install section in the README covering brew, `go install`, and downloaded archives.

**Decisions.** Formula via GoReleaser's `brews` section. *Recommended.* Needs a token secret with write access to the tap repo.

**Ask the user.** Create the tap repository and provide the token secret (the agent cannot create secrets). Is a `scoop` bucket for Windows wanted?

**Acceptance.** `brew install RafaSG13/tap/pandora` installs a binary that prints the released version (verified by the user or in a macOS CI job).

**Docs.** README.

### R-05 Real-environment QA
**Status** todo · **Size** M · **Depends** –

**Why.** Everything so far used the in-memory keyring and a pty. Nothing has run against the real macOS Keychain, libsecret, or Windows Credential Manager, nor in real terminals.

**Scope.** (a) A written manual checklist `docs/QA.md`; (b) an automated Linux integration test against a real Secret Service where feasible; (c) running the checklist before each tagged release.

**Checklist content.** Add / edit / rename / delete / copy / export / import / `run` against the real keyring; first-run keychain prompt; locked keychain; headless Linux without a Secret Service (the error screen and CLI message must be understandable); terminals: Terminal.app, iTerm2, Windows Terminal, GNOME Terminal, tmux, SSH; mouse click and wheel, `Shift`-select, resize, light and dark backgrounds, `PANDORA_ASCII=1`.

**Decisions.**
- CI integration test: `dbus-run-session` + `gnome-keyring-daemon --unlock` on `ubuntu-latest` behind a build tag (`//go:build integration`). *Recommended.* macOS and Windows runners stay manual.
- The checklist lives in the repo, not in a PR template, so it is versioned with the code.

**Ask the user.** Which machines and terminals the user can actually test on; findings that need code changes become new roadmap items.

**Acceptance.** `docs/QA.md` exists and has been executed once on macOS with results recorded (date, terminal, OS version); the Linux integration job is green or its absence is documented.

**Docs.** CONTRIBUTING (release checklist link), ARCHITECTURE (known limitations updated with findings).

### R-06 Security model hardening (docs)
**Status** todo · **Size** S · **Depends** –

**Why.** The README security model is accurate but silent on three real exposures.

**Scope.** Document, in README and SECURITY.md, honestly and briefly:
1. `pandora run` puts values in the child's environment: visible to the child, its children, and (on some OSes) `ps eww`/`/proc/<pid>/environ` to same-user processes.
2. The idle lock **masks the screen only**; any key unlocks, because the keyring is already open for the session. It is not authentication (see R-31).
3. Secret *names*, categories, notes and expiry dates are stored in plain text in `metadata.json` (already mentioned) and in the `.bak`; `export` without `--values` writes them too.
Also state the threat model in one paragraph: shoulder surfing, accidental screen sharing and shell-history leaks are in; same-user malware is out.

**Starting points.** `README.md` "Security model", `SECURITY.md` "Scope", `internal/cli/run.go`.

**Decisions.** Whether `run` should warn once on stderr when stdout is a TTY. *Recommended: no*, documentation only.

**Acceptance.** README and SECURITY say the three points above; no behaviour change.

### R-07 Clipboard hygiene
**Status** todo · **Size** M · **Depends** –

**Why.** `clearClipboard` and the lock path call `writeClipboard("")` unconditionally (`internal/tui/update.go`), so if the user copied something else in the meantime it is wiped, and the secret is visible to clipboard-history tools while it sits there.

**Scope.** (a) Clear only if the clipboard still holds the secret Pandora put there; (b) where the OS supports it, mark the copy as concealed/transient so clipboard managers skip it. Out: wiping process memory.

**Starting points.** `writeClipboard` / `clipboard.WriteAll` (`github.com/atotto/clipboard` v0.1.4, which also has `ReadAll`), `copySelected` and `clearClipboard` in `internal/tui/update.go`, `clipUntil` in `model.go`.

**Decisions.**
- (a) Remember a hash (not the value) of what was copied; at clear time `ReadAll`, compare, clear only on match; if the read fails, fall back to clearing. Add a `readClipboard` variable stubbed in tests next to `writeClipboard`. *Recommended.*
- (b) macOS `org.nspasteboard.ConcealedType`, Windows `ExcludeClipboardContentFromMonitorProcessing`/`CanIncludeInClipboardHistory`, Linux has no standard. atotto cannot set these; options are a small per-OS file with build tags (exec `pbcopy` cannot set types; needs `osascript`/cgo or a library) or leaving (b) as documented limitation. Evaluate cost before committing; ship (a) alone if (b) needs cgo.

**Ask the user.** Accept per-OS code (and possibly a new dependency) for (b)?

**Acceptance.** Test: copy secret → copy something else → deadline passes → the other content is untouched. Test: clipboard read failure still clears. README security text updated to match what (b) achieves.

### R-08 Shell completions
**Status** todo · **Size** M · **Depends** –

**Why.** The CLI has eight subcommands, flags, and secret titles worth completing.

**Scope.** `pandora completion bash|zsh|fish|powershell` prints a script; secret titles complete for `get`, `rm`, `run -s` by calling a hidden `pandora __complete` helper (titles only, never values). Optionally a man page.

**Starting points.** `internal/cli/cli.go` (`topUsage`, `run`), `commands.go`, `run.go`, `transfer.go` (each subcommand has its own `flag.FlagSet`).

**Decisions.**
- The CLI is hand-written on `flag`, not Cobra. Do **not** migrate to Cobra just for completions. Instead keep a single command table (name, summary, flags) that drives usage text, dispatch and completion generation, so the three cannot drift. *Recommended.*
- Man page: generate from the same table into `docs/pandora.1`, checked in, or skip for 0.1.0.

**Ask the user.** Is powershell completion wanted? Is a man page required for 0.1.0?

**Acceptance.** Each generated script passes its shell's syntax check (`bash -n`, `zsh -n`, `fish -n`); a test asserts every registered subcommand and flag appears in each script; completing `get <TAB>` lists titles from a temp vault and prints no values.

**Docs.** README, CHANGELOG.

### R-09 `pandora gen`
**Status** todo · **Size** S · **Depends** –

**Why.** The generator exists only in the form (`generateSecret`, `genLengths`, `genCharsets` in `internal/tui/generate.go`).

**Scope.** `pandora gen [--length N] [--charset alnum|symbols|digits|hex] [--count N]` prints to stdout. Optionally `pandora add --generate`. The generator moves out of `tui` so both layers share it.

**Decisions.** Move to `internal/store`? No — it is not storage; a new `internal/gen` package. *Recommended.* Keep the existing lengths/charsets; reject `--length` outside 8–128 with exit code 2.

**Acceptance.** `crypto/rand` only; tests for length, charset membership and the rejection range; the form behaves exactly as before.

### R-10 `NO_COLOR` and small-terminal help
**Status** todo · **Size** S · **Depends** –

**Why.** The help overlay is taller than the window below 74 columns (known limitation in ARCHITECTURE), and there is no way to turn colour off.

**Scope.** (a) Honour `NO_COLOR` (and `TERM=dumb`): no colour, selection shown with reverse video/markers. (b) Make the help overlay scrollable (↑/↓, PgUp/PgDn) or paginated so it fits 60×18.

**Starting points.** `internal/tui/styles.go`, `renderHelpOverlay` / `helpColumns` in `view.go`, `detectASCII` in `icons.go` for the env-detection pattern.

**Decisions.** Verify whether termenv/lipgloss already honour `NO_COLOR` in the pinned versions before adding code `[INFERENCE: they do; check]`. Help: scroll offset in the model, same clamp style as `clampScroll`.

**Acceptance.** `NO_COLOR=1` output contains no SGR colour codes (selection still distinguishable); at 60×18 every help line is reachable and the view is exactly 18 lines.

---

## Milestone 0.2 — data model and workflows

### R-20 Multiline values
**Status** todo · **Size** M · **Depends** –

**Why.** SSH keys, certificates and JSON credentials are multiline; the secret field is a single-line `textinput` (`internal/tui/model.go`, `inputs`).

**Scope.** Store and show multiline values; edit them in the form; CLI `add` reads them from stdin; `get` prints them verbatim; `run` passes them in the environment intact.

**Starting points.** `inputs[fieldSecret]` in `New()`, `saveForm`, `renderDetail*` (value box), `internal/cli/commands.go` (`add`, `get`), `internal/store` (value is a string; the keyring libraries accept newlines).

**Decisions.**
- Form: swap the secret field for a `bubbles/textarea` (multi-line, `enter` = newline, `ctrl+s` saves), or keep one line and add `ctrl+e` to edit in `$EDITOR`. *Recommended: textarea*, since `$EDITOR` leaves the alternate screen and plaintext temp files.
- Detail card: masked as a fixed-size block; reveal shows at most N lines with scroll; copy copies the whole value.
- Export/import JSON already carries strings; verify newline round-trip.
- Windows keyring value size limits (Credential Manager blob ≈ 2.5 KB) → reject with a clear error rather than truncate. Check the library's actual limit.

**Ask the user.** Is a hard size limit acceptable? Any need for binary values (base64 files)? *Recommended: no.*

**Acceptance.** Round-trip test through store, CLI and export/import with newlines and trailing newline preserved; TUI test: multi-line add, edit, reveal; layout still exactly `h` lines at 60×18.

### R-21 Tags
**Status** todo · **Size** M · **Depends** –

**Why.** A single free-form category runs out quickly.

**Scope.** Zero or more tags per secret, kept alongside the existing category. Filter by tag; CLI `--tag`.

**Starting points.** `store.SecretMeta` (add `Tags []string` `omitempty`), form fields (`fieldCategory` pattern, autocomplete), `reload` (category filter), `cli list --category`.

**Decisions.**
- Additive field, category unchanged. *Recommended* (no migration, old versions ignore it; mind that an old version saving the file would drop `tags`: document that downgrade loses tags, or keep unknown fields — check how `LoadMetadata` decodes).
- Normalisation: lowercase, trimmed, unique, no commas/spaces inside; max count and length.
- Filter UI: `/` filter already exists; add a tag picker (`g`?) rather than more number keys.

**Ask the user.** Should tags eventually replace categories (and the colours/badges tied to them)?

**Acceptance.** Old `metadata.json` loads unchanged; tags survive edit/rename/export/import; filter by tag in TUI and CLI; tests for normalisation and for the old-file path.

### R-22 Search in notes and tags
**Status** todo · **Size** S · **Depends** R-21 (soft)

**Why.** `item.FilterValue()` returns only title + category (`model.go`), so notes and tags are not searchable.

**Scope.** Extend `FilterValue` with notes (and tags once R-21 exists). **Never values.**

**Decisions.** Notes are only searched when the filter is active; no change to highlight. Check that match highlighting in the delegate does not mis-highlight when the match is in a field that is not rendered (the row shows title and category/description only).

**Acceptance.** Filtering by a word that exists only in notes selects that secret; a test guards that values are never part of `FilterValue`.

### R-23 Config file
**Status** todo · **Size** M · **Depends** –

**Why.** Behaviour is configured only through environment variables (`PANDORA_LOCK_MINUTES`, `PANDORA_ASCII`), which is awkward for a TUI launched from a dock or a menu.

**Scope.** A config file next to `metadata.json`: lock minutes, ASCII, clipboard seconds, reveal seconds, expiry warning days, default generator length/charset, default sort, compact mode, and later history depth, audit log. Precedence: environment > file > built-in default.

**Starting points.** `lockTimeout` and `New()` in `model.go`, `detectASCII`, `expiryInfo` (the 14-day constant), `genLengths`; `store` for the config dir helper.

**Decisions.**
- Format: JSON (`config.json`, no new dependency, same tooling as metadata) vs TOML (friendlier to hand-edit, one dependency). *Recommended: TOML* if the user accepts the dependency; otherwise JSON.
- Invalid values: warn on the header status once and fall back to defaults; never refuse to start.
- A `pandora config` command (`path`, `get`, `set`) vs documented manual editing. *Recommended:* `path` and `get` only for now.
- Never put secrets or secret names in the config.

**Ask the user.** TOML or JSON?

**Acceptance.** Table-driven precedence test; unknown keys ignored with a warning; invalid values fall back; the existing env vars keep working.

### R-24 Value history
**Status** todo · **Size** L · **Depends** R-23 (soft)

**Why.** `UpdateSecret` overwrites the old value irreversibly; a mistaken edit loses a key.

**Scope.** Keep the last N previous values per secret (default 3, `0` disables); show a version list in the detail card/overlay; restore a previous version; the CLI can read one (`get --version -1`).

**Starting points.** `store.UpdateSecret`, `DeleteSecret`, rename path (keyring entry moves), `MissingValues`, `RestoreSecrets`, `export`/`import`.

**Decisions (design needed).**
- Storage: extra keyring entries named `<title>@v<k>` plus `versions: <count>` in metadata, vs one keyring entry holding a JSON list. *Recommended: separate entries* (each value stays a plain keyring value; smaller blast radius), with ordering by a counter in metadata.
- Everything that touches an entry must handle versions: rename moves them, delete removes them, `doctor` checks them, `export --values` excludes or includes them (decide; default exclude), orphan check ignores them.
- Atomicity: new-value write order and rollback so a failure never leaves a version without its metadata or vice versa (follow the existing order in `UpdateSecret`).
- A title containing `@v` must not collide: choose a separator that titles cannot contain, or validate titles.
- Security: old values are still secrets; delete must wipe all versions; document that history increases exposure.

**Ask the user.** Default depth; whether history is opt-in.

**Acceptance.** Update keeps N versions and prunes the oldest; rename and delete carry/remove all versions; rollback test when the metadata write fails; restore works in TUI and CLI; old metadata loads with zero versions.

### R-25 Import from other tools
**Status** todo · **Size** M · **Depends** –

**Why.** Migration is what decides whether someone adopts Pandora. Today `import` only reads Pandora's own export.

**Scope.** `pandora import --format <fmt> FILE` with `dotenv`, `1password-csv`, `bitwarden-json`, `chrome-csv` (start with dotenv + one password-manager format). Dry-run (`--dry-run`) lists what would be created and conflicts. Reuses `store.RestoreSecrets`.

**Starting points.** `internal/cli/transfer.go` (parsing, validation, `--overwrite`), `store/restore.go`.

**Decisions.**
- One pure parser per format returning `[]store.SecretMeta + value`, so each is unit-tested with small synthetic fixtures (never real exports).
- Mapping rules (which column becomes the title, category, notes; duplicates; invalid titles) are decided per format and documented in the README.
- Imported plaintext files are the user's responsibility: print a reminder to delete them; never copy them.

**Ask the user.** Which formats, in what order, and which sample exports they can provide (redacted).

**Acceptance.** Each parser has a fixture test including malformed rows; `--dry-run` writes nothing; conflicts follow `--overwrite`.

### R-26 Dotenv output and CLI ergonomics
**Status** todo · **Size** S · **Depends** –

**Scope.** `pandora env -s TITLE[=ENV] …` prints `KEY=value` lines (explicit opt-in, since it prints values); `get --copy` copies with the same clipboard deadline logic as the TUI (or prints a note that the clear needs the TUI); `get -q`/`--newline=false` for scripting.

**Decisions.** Printing values to stdout is already done by `get`; `env` just formats. Escape/quote rules for dotenv (newlines need R-20). Whether `env` refuses when stdout is a TTY without `--force`. *Recommended: yes.*

**Acceptance.** Quoting round-trips through a dotenv parser test; no value in stderr; exit codes follow the existing convention.

### R-27 External-change detection and file lock
**Status** todo · **Size** M · **Depends** –

**Why.** Known limitations: a second instance's changes are noticed only after `r`, and concurrent writers race on `metadata.json` (last wins).

**Scope.** (a) Advisory lock around the load-modify-write cycle in `store`; (b) the TUI notices an external change (mtime/size poll in the 1 s tick, no new dependency) and reloads or shows a "changed on disk — press r" hint.

**Starting points.** `store/keyring.go` (`writeMetadata`, every mutating function loads first), `tickMsg` handling in `update.go`.

**Decisions.**
- Lock: lock file with `O_EXCL` + stale detection, or `flock`/`LockFileEx` via a small per-OS file. *Recommended:* the OS lock, since a crash must not leave a stale lock.
- Auto-reload while a form is open would discard nothing (reload only refreshes the list) but must keep the selection by title; never reload underneath a delete confirmation.

**Acceptance.** Two goroutines/processes adding different secrets both survive (test with the lock); the TUI reflects a change made by the CLI within a few seconds.

### R-28 Keyring index and recovery
**Status** todo · **Size** L · **Depends** R-24 (soft)

**Why.** If `metadata.json` is lost, the secrets are still in the keyring but unlisted, and `doctor` cannot find them because go-keyring cannot enumerate.

**Scope.** Maintain an index entry in the keyring (a reserved account such as `__pandora_index__`) listing titles and enough metadata to rebuild the file; `doctor` reports and `doctor --fix` restores missing metadata from it. Resolves the "go-keyring cannot enumerate" limitation listed in ARCHITECTURE.

**Decisions (design needed).**
- Index contents: titles only (smaller exposure) vs titles + category/created/expires (full rebuild). *Recommended: titles + category + dates; notes excluded.*
- Consistency: update index and metadata in a defined order; either can be stale after a crash, so `doctor` reconciles in both directions.
- Reserved account name must be rejected as a user title.
- Index grows with the vault; keyring value-size limits (see R-20) apply.

**Ask the user.** Is storing titles in the keyring acceptable (it already is in metadata.json), and is full-metadata recovery worth the extra moving part?

**Acceptance.** Delete `metadata.json` in a test, `doctor --fix` rebuilds it; index drift in either direction is detected; adding the reserved title is rejected.

---

## Milestone 0.3 — security hardening and automation

### R-30 `expiring` command
**Status** todo · **Size** S · **Depends** –

**Scope.** `pandora expiring [--within 14d] [--json]` lists expiring/expired secrets (never values) and exits with a distinct code when any exist, for cron and CI. Reuses `SecretMeta.DaysUntilExpiry`.

**Decisions.** Exit code: `3` if any are expiring/expired, `0` otherwise (document it next to the existing 0/1/2). Default window equals the TUI's 14 days (shared constant, later R-23).

**Acceptance.** Table test for window boundaries (today, tomorrow, yesterday, no expiry); JSON output stable and documented.

### R-31 Unlock challenge
**Status** todo · **Size** L · **Depends** R-23

**Why.** The idle lock only masks the screen (see R-06); any key unlocks.

**Scope.** Optionally require a passphrase to leave the lock screen (and to reveal/copy when configured).

**Decisions (design needed).**
- It protects against a person at an unattended terminal, not against same-user malware (which can read the keyring directly). Say so in the UI and README; do not oversell.
- Verifier: a salted memory-hard hash (argon2id or scrypt from `golang.org/x/crypto`, already an indirect dependency in `go.mod`) stored in the config. Never store the passphrase.
- Biometrics (Touch ID/Windows Hello) require cgo/OS APIs: out of scope for this item.
- Failed-attempt policy: backoff, no lockout that can lose data.
- Recovery: forgetting the passphrase must not lose secrets (they remain in the keyring and the CLI still works) — state clearly that this is a UI gate.

**Ask the user.** Is a UI-only gate worth shipping given its limits, or should this be dropped?

**Acceptance.** Wrong passphrase never unlocks; correct one restores the previous mode including a half-typed form; no passphrase in logs/metadata; tests with an injected clock for backoff.

### R-32 Local audit log
**Status** todo · **Size** M · **Depends** R-23

**Scope.** Opt-in append-only JSONL file recording time, action (`reveal`, `copy`, `get`, `run`, `export`, `add`, `edit`, `rm`) and secret title — never values — plus `pandora log [--since]`.

**Decisions.**
- Opt-in via config only; default off, because a log of names and times is itself sensitive metadata.
- Size cap with rotation; `0600`.
- Written from `store`/`cli` call sites through one `audit.Record(...)` function so the TUI and CLI cannot diverge.

**Ask the user.** Retention and whether failed attempts (R-31) are logged.

**Acceptance.** Disabled by default writes nothing; enabled logs each action once; values never appear (test greps the file for the test secret); rotation test.

---

## Later

### R-40 TOTP codes
**Status** todo · **Size** L · **Depends** R-20 (soft)

**Scope.** Store an `otpauth://` URI or base32 seed as a secret of a new kind; the detail card shows the current 6-digit code with a countdown (the countdown bar code already exists); `pandora otp TITLE` prints it; copy follows the clipboard rules.

**Decisions.**
- Schema: a `kind` field (`password` default, `totp`), `omitempty`; list icon per kind.
- Implement RFC 6238 with the standard library (HMAC-SHA1/256/512), or a small dependency; test against the RFC test vectors.
- Clock skew and device time: show a warning if the code is within 2 s of rolling.
- Storing 2FA seeds beside passwords weakens 2FA; document it.

**Ask the user.** Is that tradeoff acceptable for this tool?

**Acceptance.** RFC 6238 vectors pass; code refreshes on the tick without redrawing the whole value box; old metadata loads as `password`.

### R-41 Folder-sync vault file
**Status** todo · **Size** L · **Depends** R-23, R-28

**Why.** Syncing across machines is the most requested feature of any vault, but "no network" is a stated non-goal.

**Scope.** Keep it **network-free in the binary**: `pandora sync --dir PATH` reads/writes a single age-encrypted vault file in a folder the user already syncs (iCloud Drive, Dropbox, a git checkout). Pandora never talks to a service.

**Decisions (design needed).**
- Conflict model: per-secret last-write-wins using `Updated` + a version counter, with a conflicts report; never silent overwrite.
- Encryption: reuse the age passphrase flow from `export --values`; how the passphrase is cached for non-interactive use (`PANDORA_PASSPHRASE` exists already).
- Deleted secrets need tombstones, otherwise they resurrect.
- Merge must be atomic with respect to the keyring (reuse `RestoreSecrets` rollback).

**Ask the user.** Does this respect the "no sync" intent, given that the transport is the user's folder? If not, drop it.

**Acceptance.** Two simulated devices converge after edits, deletes and a conflict; the vault file contains no plaintext titles or values; wrong passphrase fails without modifying anything.

---

## Not planned

- **Network features, telemetry, hosted sync.** An explicit non-goal; R-41 is the only sanctioned approximation and needs the user's approval.
- **`tab` inside the filter prompt** to pick a category: `1`–`9` covers it without overloading the list's own filter input.
- **Biometric unlock** (Touch ID, Windows Hello): needs cgo/OS APIs; revisit only if R-31 ships and demand exists.
- **Migrating the CLI to Cobra** just for completions (see R-08).
