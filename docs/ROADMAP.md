# Roadmap

Releases 0.1.x, 0.2.0, 0.3.0 and 0.6.0 are published (0.6.0 also carries what was planned as 0.4.0 and 0.5.0; see [Release plan](#release-plan)). This file is the forward plan, written to be **read by an engineer or an AI agent that then produces a detailed implementation plan** for the next item. Read [How to use this document](#how-to-use-this-document) first.

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
- **Secret values live only in the OS keyring** (service `pandora-cli`, encrypted under the vault key, account = an opaque random ID), never in the vault, logs, error messages, toasts or test output. Names, categories and notes are only ever stored encrypted. Exports with values are encrypted.
- **`vault.json` is a persisted format** (version `2`, encrypted). New record fields are `omitempty` inside the encrypted JSON and a missing field must have a sensible default. A format change needs a new `version` and a migration; `store.SecretMeta` is also the export/import schema. Any item that cannot honour this must say so in its plan and ask the user.
- **Layering.** `internal/store` has no UI dependency; `internal/tui` reaches the keyring and files only through `store`; `internal/cli` takes its streams as arguments and returns an exit code (`0` ok, `1` runtime, `2` usage).
- **Terminal floor.** The TUI must stay usable at 60×18, have an ASCII fallback (`PANDORA_ASCII=1`), and keep the view exactly the window height.
- **Time-based behaviour uses deadlines** driven by the 1 s `tickMsg` and the injectable `model.now`, not per-action timers.

### Testing rules

Never touch the real keychain, config directory or clipboard. Use `keyring.MockInit()`, temp `HOME` / `XDG_CONFIG_HOME` / `AppData`, and the stubbed `writeClipboard` / `writeSecretClipboard` (see `internal/store/keyring_test.go`, `internal/tui/update_test.go`, `internal/tui/features_test.go`). Test behaviour, boundaries and error paths; do not pin wording or wiring.

## Index

| ID | Item | Release | Size | Depends | Status |
| --- | --- | --- | --- | --- | --- |
| R-01 | Version information | 0.1.0 | S | – | done (v0.1.0, PR #6) |
| R-02 | Go module path | 0.1.0 | S | – | done (v0.1.0, PR #6) |
| R-03 | Release pipeline | 0.1.0 | M | R-01, R-02 | done (v0.1.0) |
| R-04 | Homebrew tap and install docs | 0.1.0 | S | R-03 | done (v0.1.1) |
| R-06 | Security model hardening (docs) | 0.1.0 | S | – | done (v0.1.0, PR #6) |
| R-07 | Clipboard hygiene | 0.1.0 | M | – | done, part (a) only; part (b) is R-11 |
| R-08 | Shell completions | 0.1.0 | M | – | done (v0.1.0, PR #6) |
| R-09 | `pandora gen` | 0.1.0 | S | – | done (v0.1.0, PR #6) |
| R-10 | `NO_COLOR` and small-terminal help | 0.1.0 | S | – | done (v0.1.0, PR #6) |
| R-31 | Unlock challenge | 0.2.0 | L | – | done (v0.2.0; master password) |
| R-20 | Multiline values | 0.3.0 | M | – | done (v0.3.0) |
| R-21 | Tags | 0.3.0 | M | – | done (v0.3.0) |
| R-22 | Search in notes and tags | 0.3.0 | S | R-21 (soft) | done (v0.3.0) |
| R-23 | Config file | 0.3.0 | M | – | done (v0.3.0) |
| R-27 | External-change detection and file lock | 0.3.0 | M | – | done (v0.3.0) |
| R-26 | Dotenv output and CLI ergonomics | 0.4.0 | S | – | done |
| R-30 | `expiring` command | 0.4.0 | S | R-23 (done) | done |
| R-25 | Import from other tools (dotenv + one password-manager format) | 0.4.0 | M | R-26 (done) | done (dotenv, bitwarden-json) |
| R-28 | `doctor` vault recovery (re-scoped, option B) | 0.5.0 | M | – | done |
| R-24 | Value history | 0.5.0 | L | R-23 (done) | done |
| R-32 | Local audit log | 0.6.0 | M | R-23 (done), R-25, R-26 | done |
| R-50 | API freeze audit (CLI, JSON, exit codes, formats) | 0.7.0 | M | R-25, R-28, R-30, R-24, R-32 | todo |
| R-51 | Format compatibility test corpus | 0.7.0 | M | R-24 | todo |
| R-52 | TUI polish and known bugs | 0.7.0 | S | – | todo |
| R-05 | Real-environment QA, all three OSes | 1.0.0 gate | M | – | in-progress (macOS keyring and clipboard done; other terminals and OSes open) |
| R-11 | Concealed clipboard copies, Windows check | 1.0.0 gate | S | – | in-progress (macOS verified; Windows manual QA open) |
| R-53 | 1.0 release (docs, security review, tag) | 1.0.0 | S | R-05, R-11, R-50, R-51, R-52 | todo |
| R-40 | TOTP codes | post-1.0 (1.1) | L | R-20 (done) | todo |
| R-41 | Folder-sync vault file | post-1.0 (1.2) | L | R-23 (done) | todo |

The index is ordered by release, then by the order to build inside it; "first `todo` item whose `Depends` are `done`" therefore walks the plan top to bottom. The section order below matches.

## Release plan

Versions are SemVer `0.x` until 1.0.0, which freezes the public surface (see R-50 and `docs/STABILITY.md`); from 1.0.0 on only additive changes land in minor releases. `v0.1.0`, `v0.1.1`, `v0.2.0` and `v0.3.0` are published; `0.3.0` rather than `0.2.1` because it added features and extended the persisted records (tags, values split over several keyring entries, `config.json`). Cutting a release is in CONTRIBUTING; **no tag is pushed unless the user asks for it**. Every release needs: `[Unreleased]` renamed to `## [X.Y.Z] - date` in the changelog (the heading format matters to `scripts/release-notes.sh`), the R-05 checklist (`docs/QA.md`) run on macOS for what changed, the `keyring-linux` CI job green, and an upgrade note when a downgrade would lose data.

| Release | Theme | Items (build in this order) | Gate before tagging |
| --- | --- | --- | --- |
| **0.3.0** (published) | Multi-line secrets, tags and search, config file, safe concurrent writers, concealed clipboard copies | R-20, R-21, R-22, R-23, R-27, R-11 | Shipped with the automated suite and CLI and pty smoke runs; the full `docs/QA.md` macOS pass and the Windows concealed-copy check were **not** done before tagging (R-11 and R-05 stay open for them). Known open bug, not from this release: the empty-vault welcome screen is 23 lines tall at 60×18. |
| **0.4.0** (shipped in v0.6.0) | Scripting and adoption, no format change | R-26 (done), then R-30, then R-25 | R-30 first: smallest, and it reads the existing `expiry_warn_days`. R-26's dotenv quoting (`dotenvValue` in `internal/cli/dotenv.go`) is reused, inverted, by the R-25 dotenv parser. R-25 starts with `dotenv` plus one password-manager format and needs redacted sample exports from the user. New commands go through `commands()` so the completion drift tests cover them; README command table updated. |
| **0.5.0** (shipped in v0.6.0) | Data safety | R-28 (option B), then R-24 (design first) | R-24 adds an `omitempty` field to the encrypted record and a `history_depth` config key; the vault stays `version: 2` unless the plan finds a reason to bump it. The plan must state what a downgrade does with history entries. R-24 comes before the freeze because it touches the persisted format. R-28 is independent of R-24: if R-24 slips, tag 0.5.0 with R-28 alone. |
| **0.6.0** (published) | Auditability | R-32 | After 0.4.0 so every new command (`env`, `get --copy`, imports, `expiring`) calls the single audit writer from the start. Opt-in through an `audit_log` config key; a test greps the file for the test secret. Shipped as `v0.6.0` together with 0.4.0 and 0.5.0, without the full `docs/QA.md` macOS pass or the Windows concealed-copy check (R-05 and R-11 stay open). |
| **0.7.0** | Stabilise (release candidate quality, no new features) | R-50, R-51, R-52 | R-50 last among features so nothing it freezes still moves. R-51 proves the formats still load; R-52 fixes known layout bugs. |
| **1.0.0** | Stable | R-53 plus the R-05 and R-11 gates | QA gates run last, against the final release artefacts. Tag only when the user asks. |
| 1.1 (post-1.0) | TOTP | R-40 | Additive `kind` field; needs the user's answer to its `Ask the user` question before planning. |
| 1.2 (post-1.0) | Folder sync | R-41 | Needs the user's answer to its `Ask the user` question; additive only. |

**1.0.0 criteria:** R-05 and R-11 done on macOS, Linux and Windows; persisted formats (`vault.json` v2, export JSON) unchanged since 0.5 apart from additive `omitempty` fields, proved by the R-51 corpus; the R-50 stability document published; no `blocked` item; `keyring-linux` CI job green.

**Order rationale.**
- R-30 first: smallest item, reads the existing `expiry_warn_days`.
- R-25 after R-26: reuses its dotenv quoting.
- R-24 before the freeze: it touches the persisted format.
- R-32 after the commands exist, so each calls `audit.Record` once.
- R-50 last among features, so nothing it freezes still moves.
- QA gates last: they run against the final artefacts.

Sizing rule used for the cut: a release holds at most one L item, or up to three S/M items, so a slipping item never holds back a finished one; move it to the next release instead. 0.5.0 holds R-24 (L) plus R-28 (M): acceptable because R-28 is independent.

---

## Shipped in 0.1.x — publishable

### R-01 Version information
**Status** done · **Size** S · **Depends** –

**Outcome.** `internal/version` (link-time values, `debug.ReadBuildInfo` fallback, pseudo-versions of untagged builds report `dev`), `pandora version` / `--version`, ldflags in the Makefile and `.goreleaser.yaml`, version line in the help overlay.

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
**Status** done · **Size** S · **Depends** –

**Outcome.** Module is `github.com/RafaSG13/Pandora`; every import rewritten. Verified after the release: `go install github.com/RafaSG13/Pandora/cmd/pandora@v0.1.0` installs and reports `0.1.0`.

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
**Status** done · **Size** M · **Depends** R-01, R-02

**Outcome.** `.goreleaser.yaml`, `.github/workflows/release.yml` (tag trigger; fails when the changelog has no section for the tag; vet and tests; GoReleaser; then `gh release edit --notes-file`), `scripts/release-notes.sh`, a `release-snapshot` CI job, and "Cutting a release" in CONTRIBUTING. `v0.1.0` was published by it: six archives plus `checksums.txt`; the darwin/arm64 archive verified against the checksum and reports `0.1.0`. Decisions taken as the recommended defaults: SemVer with `v0.x` allowed to break, checksums only, no notarisation.
**Lesson.** Passing `--release-notes` to GoReleaser left the v0.1.0 release body empty (its changelog is disabled), so the notes are now set with `gh release edit`; the v0.1.0 body was fixed by hand.

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
**Status** done · **Size** S · **Depends** R-03

**Outcome.** `RafaSG13/homebrew-tap` exists; `.goreleaser.yaml` has a `brews` section (skipped while `HOMEBREW_TAP_TOKEN` is empty) and the secret was added by the user. Releasing `v0.1.1` pushed `Formula/pandora.rb`; `brew install RafaSG13/tap/pandora` installed 0.1.1 on macOS arm64 and `pandora --version` reported it. The README Install section lists Homebrew, `go install` and release archives. No `scoop` bucket (not requested).

**Why.** Homebrew is the natural channel for a macOS terminal tool.

**Scope.** A `RafaSG13/homebrew-tap` repository updated by the release pipeline; an Install section in the README covering brew, `go install`, and downloaded archives.

**Decisions.** Formula via GoReleaser's `brews` section. *Recommended.* Needs a token secret with write access to the tap repo.

**Ask the user.** Create the tap repository and provide the token secret (the agent cannot create secrets). Is a `scoop` bucket for Windows wanted?

**Acceptance.** `brew install RafaSG13/tap/pandora` installs a binary that prints the released version (verified by the user or in a macOS CI job).

**Docs.** README.

### R-05 Real-environment QA
**Status** in-progress · **Size** M · **Depends** –

**Outcome so far.** `docs/QA.md` (keyring, clipboard, terminals, artefacts, how to record results) and `internal/store/integration_test.go` (build tag `integration`, only runs with `PANDORA_INTEGRATION=1`) wired to a `keyring-linux` CI job; it passed on GitHub against gnome-keyring (add, read with a newline, rename, delete). The macOS checklist was run on 2026-11-15 for keyring, `run`, `doctor`, export and clipboard (see PR #6). **Left:** other terminals (Terminal.app, iTerm2, tmux, Windows Terminal), Windows Credential Manager, and the artefact checks against the real release.

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
**Status** done · **Size** S · **Depends** –

**Outcome.** README Security model gained "Things that surprise people" (run environment, lock is a screen mask, names in plain text, threat model) and `SECURITY.md` scope and supported versions were updated. No behaviour change.

**Why.** The README security model is accurate but silent on three real exposures.

**Scope.** Document, in README and SECURITY.md, honestly and briefly:
1. `pandora run` puts values in the child's environment: visible to the child, its children, and (on some OSes) `ps eww`/`/proc/<pid>/environ` to same-user processes.
2. *(Historical; replaced by the master password, R-31.)* The idle lock masked the screen only; any key unlocked.
3. *(Historical.)* Secret *names*, categories, notes and expiry dates were stored in plain text in `metadata.json` and its `.bak`; `export` without `--values` still writes them to the file you choose.
Also state the threat model in one paragraph: shoulder surfing, accidental screen sharing and shell-history leaks are in; same-user malware is out.

**Starting points.** `README.md` "Security model", `SECURITY.md` "Scope", `internal/cli/run.go`.

**Decisions.** Whether `run` should warn once on stderr when stdout is a TTY. *Recommended: no*, documentation only.

**Acceptance.** README and SECURITY say the three points above; no behaviour change.

### R-07 Clipboard hygiene
**Status** done for part (a); part (b) moved to R-11 · **Size** M · **Depends** –

**Outcome.** `readClipboard` next to `writeClipboard`; copies store a SHA-256 only; deadline, `x` and lock clear only when the clipboard still matches (trailing newlines ignored; a read error clears anyway; `x` toasts when it leaves foreign content alone).

**Why.** `clearClipboard` and the lock path call `writeClipboard("")` unconditionally (`internal/tui/update.go`), so if the user copied something else in the meantime it is wiped, and the secret is visible to clipboard-history tools while it sits there.

**Scope.** (a) Clear only if the clipboard still holds the secret Pandora put there; (b) where the OS supports it, mark the copy as concealed/transient so clipboard managers skip it. Out: wiping process memory.

**Starting points.** `writeClipboard` / `clipboard.WriteAll` (`github.com/atotto/clipboard` v0.1.4, which also has `ReadAll`), `copySelected` and `clearClipboard` in `internal/tui/update.go`, `clipUntil` in `model.go`.

**Decisions.**
- (a) Remember a hash (not the value) of what was copied; at clear time `ReadAll`, compare, clear only on match; if the read fails, fall back to clearing. Add a `readClipboard` variable stubbed in tests next to `writeClipboard`. *Recommended.*
- (b) macOS `org.nspasteboard.ConcealedType`, Windows `ExcludeClipboardContentFromMonitorProcessing`/`CanIncludeInClipboardHistory`, Linux has no standard. atotto cannot set these; options are a small per-OS file with build tags (exec `pbcopy` cannot set types; needs `osascript`/cgo or a library) or leaving (b) as documented limitation. Evaluate cost before committing; ship (a) alone if (b) needs cgo.

**Ask the user.** Accept per-OS code (and possibly a new dependency) for (b)?

**Acceptance.** Test: copy secret → copy something else → deadline passes → the other content is untouched. Test: clipboard read failure still clears. README security text updated to match what (b) achieves.

### R-08 Shell completions
**Status** done · **Size** M · **Depends** –

**Outcome.** One command table (`internal/cli/registry.go`) drives usage, dispatch and bash/zsh/fish/PowerShell scripts (`completion.go`), with a drift test against each command's real flags and a hidden `__complete titles|categories` helper. Verified: `bash -n`, `zsh -n`, a manual bash completion session and partial zsh. **Not verified:** fish and PowerShell scripts (not installed where this was built). No man page (deferred).

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
**Status** done · **Size** S · **Depends** –

**Outcome.** `internal/gen` (`Generate`, `Lookup`, `Charsets` keyed `alnum`/`symbols`/`hex`, lengths 8–128); the TUI uses it; `pandora gen [--length] [--charset] [--count]` (count 1–100) with usage errors for out-of-range values. `add --generate` was not added. The `digits` charset was dropped as too weak to offer.

**Why.** The generator exists only in the form (`generateSecret`, `genLengths`, `genCharsets` in `internal/tui/generate.go`).

**Scope.** `pandora gen [--length N] [--charset alnum|symbols|digits|hex] [--count N]` prints to stdout. Optionally `pandora add --generate`. The generator moves out of `tui` so both layers share it.

**Decisions.** Move to `internal/store`? No — it is not storage; a new `internal/gen` package. *Recommended.* Keep the existing lengths/charsets; reject `--length` outside 8–128 with exit code 2.

**Acceptance.** `crypto/rand` only; tests for length, charset membership and the rejection range; the form behaves exactly as before.

### R-10 `NO_COLOR` and small-terminal help
**Status** done · **Size** S · **Depends** –

**Outcome.** termenv v0.16 already honours `NO_COLOR` (Ascii profile, which also drops bold and reverse); `TERM=dumb` with `COLORTERM` set was not covered, so `detectNoColor` / `applyColorMode` in `icons.go` force it. Selection gets a `▌` / `>` gutter marker, compact expired rows show `◷!`. The help overlay scrolls (`↑/↓/j/k`, `PgUp/PgDn`, `Home/End`) only when it does not fit and shows a position hint.

**Why.** The help overlay is taller than the window below 74 columns (known limitation in ARCHITECTURE), and there is no way to turn colour off.

**Scope.** (a) Honour `NO_COLOR` (and `TERM=dumb`): no colour, selection shown with reverse video/markers. (b) Make the help overlay scrollable (↑/↓, PgUp/PgDn) or paginated so it fits 60×18.

**Starting points.** `internal/tui/styles.go`, `renderHelpOverlay` / `helpColumns` in `view.go`, `detectASCII` in `icons.go` for the env-detection pattern.

**Decisions.** Verify whether termenv/lipgloss already honour `NO_COLOR` in the pinned versions before adding code `[INFERENCE: they do; check]`. Help: scroll offset in the model, same clamp style as `clampScroll`.

**Acceptance.** `NO_COLOR=1` output contains no SGR colour codes (selection still distinguishable); at 60×18 every help line is reachable and the view is exactly 18 lines.

---

## Shipped in 0.2.0 — master password

### R-31 Unlock challenge
**Status** done (v0.2.0; superseded by the master-password vault) · **Size** L · **Depends** –

**Outcome.** Instead of a UI-only gate, a master password now protects the data itself: `vault.json` holds encrypted names and metadata, keyring values are encrypted under the same key, and both the TUI (including after the idle lock) and the CLI ask for the password. Argon2id + XChaCha20-Poly1305 (`golang.org/x/crypto`); no recovery, so `export --values` is the backup route. See README "Security model" and `docs/ARCHITECTURE.md` "Persisted data". Biometrics and a failed-attempt backoff were not built; the KDF cost is the brake.

---

## Shipped in 0.3.0

### R-20 Multiline values
**Status** done (v0.3.0) · **Size** M · **Depends** –

**Outcome.** The form's secret is a `textarea` (`enter` = newline, `ctrl+s` saves, `ctrl+r` shows/hides, masked as bullets otherwise); `add` reads any number of lines from stdin; `get`/`run`/export/import round-trip them. Decisions taken: textarea (not `$EDITOR`); text only, no binary values; a 32 KB cap (`store.MaxValueBytes`). The Windows (2560 B) and macOS (~4 KB `security` command) per-entry limits, made tighter by the encrypted base64 form, are handled by splitting the stored value over several keyring entries (`chunks` in the vault record) rather than by a small hard limit. The detail card shows the first lines of a long revealed value and says how many more (no scrolling). Details: `docs/ARCHITECTURE.md` "Persisted data" and "Secret field".

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
**Status** done (v0.3.0) · **Size** M · **Depends** –

**Outcome.** `store.SecretMeta.Tags` (`omitempty`, so the vault stays at version 2 and old vaults and exports load unchanged); writers normalise through `store.NormalizeTags` (lowercase, trimmed, unique, sorted; no whitespace or commas inside; at most 10 tags of 24 characters) and `parseExport` does too. TUI: **Tags** form field after Notes (comma-separated), `T` cycles the tag filter (one tag at a time, combined with the category filter, shown in the list title; `g`/`G` stay the list's first/last keys), tags in the detail card. CLI: `add --tag/-t`, `list --tag/-t` (repeat or comma-separate: all of them), `list --json` always carries `tags`, plain `list` has a trailing `TAGS` column. Decisions taken: additive field, category unchanged and kept alongside (tags are not meant to replace categories); a cycling key instead of a picker overlay, since `/` search will cover tags (R-22). Downgrade caveat: an older version saving the vault drops `tags` (documented in `docs/ARCHITECTURE.md`). Not done: shell completion of existing tags, tag autocomplete in the form.

**Why.** A single free-form category runs out quickly.

**Scope.** Zero or more tags per secret, kept alongside the existing category. Filter by tag; CLI `--tag`.

**Acceptance (met).** Old vaults load unchanged; tags survive edit/rename/export/import; filter by tag in TUI and CLI; tests for normalisation and for the old-record shape.

### R-22 Search in notes and tags
**Status** done (v0.3.0) · **Size** S · **Depends** R-21 (soft)

**Outcome.** `item.FilterValue()` (`model.go`) is title, category, tags, then notes. The list holds metadata only, so values cannot be searched; `TestFilterNeverSearchesTheSecretValue` guards it. The row underlines only matches inside the title: indexes from a tag or note match fall past the title and mark nothing (`TestMatchInNotesDoesNotHighlightTheTitle`). Trade-off: the filter is fuzzy (subsequence), so a short query can match across a long note; results are ranked, name matches first, but some noise remains. A substring-only match for tags and notes would remove it, at the cost of a custom `list.Filter`.

**Why.** `item.FilterValue()` returned only title + category, so notes and tags were not searchable.

**Scope.** Extend `FilterValue` with notes and tags. **Never values.**

**Acceptance (met).** Filtering by a word that exists only in notes (or only in a tag) selects that secret; a test guards that values are never part of `FilterValue`.

### R-23 Config file
**Status** done (v0.3.0) · **Size** M · **Depends** –

**Why.** Behaviour is configured only through environment variables (`PANDORA_LOCK_MINUTES`, `PANDORA_ASCII`), which is awkward for a TUI launched from a dock or a menu.

**Scope.** A config file next to `metadata.json`: lock minutes, ASCII, clipboard seconds, reveal seconds, expiry warning days, default generator length/charset, default sort, compact mode, and later history depth, audit log. Precedence: environment > file > built-in default.

**Starting points.** `lockTimeout` and `New()` in `model.go`, `detectASCII`, `expiryInfo` (the 14-day constant), `genLengths`; `store` for the config dir helper.

**Decisions.**
- Format: JSON (`config.json`, no new dependency, same tooling as metadata) vs TOML (friendlier to hand-edit, one dependency). *Recommended: TOML* if the user accepts the dependency; otherwise JSON.
- Invalid values: warn on the header status once and fall back to defaults; never refuse to start.
- A `pandora config` command (`path`, `get`, `set`) vs documented manual editing. *Recommended:* `path` and `get` only for now.
- Never put secrets or secret names in the config.

**Decision taken.** JSON, `path` and `get` only, only settings that already exist as behaviour.

**Outcome (done).** JSON `config.json` in `internal/config`; nine settings (`lock_minutes`, `ascii`, `clipboard_seconds`, `reveal_seconds`, `expiry_warn_days`, `gen_length`, `gen_charset`, `sort`, `compact`); read-only `pandora config path|get`. History depth and the audit log are not settings yet: R-24 and R-32 add their keys when they ship.

**Acceptance (met).** Table-driven precedence test (`internal/config`); unknown keys ignored with a warning; invalid values fall back; the existing env vars keep working.

### R-27 External-change detection and file lock
**Status** done (v0.3.0) · **Size** M · **Depends** –

**Why.** A second instance's changes were noticed only after `r`, and concurrent writers raced on `vault.json` (last wins, losing the other's secret).

**Scope.** (a) Advisory lock around the load-modify-write cycle in `store`; (b) the TUI notices an external change (mtime/size poll in the 1 s tick, no new dependency) and reloads or shows a "changed on disk — press r" hint.

**Starting points.** `store/lock.go` (`lockVault`, `VaultStamp`), every mutating function in `store` (`keyring.go`, `restore.go`, `migrate.go`, `vault.go`), `tick` and `reload` in `internal/tui`.

**Decisions.**
- Lock: lock file with `O_EXCL` + stale detection, or `flock`/`LockFileEx` via a small per-OS file. *Recommended:* the OS lock, since a crash must not leave a stale lock.
- Auto-reload while a form is open would discard nothing (reload only refreshes the list) but must keep the selection by title; never reload underneath a delete confirmation.

**Acceptance.** Two goroutines/processes adding different secrets both survive (test with the lock); the TUI reflects a change made by the CLI within a few seconds.

### R-11 Concealed clipboard copies
**Status** in-progress (macOS verified; Windows needs the `docs/QA.md` check on a real machine) · **Size** M · **Depends** –

**Outcome so far.** Secret copies go through `writeSecretClipboard` (`internal/tui/clipboard.go`); `writeClipboard`/`readClipboard` stay for reads and clears. Per-OS files, no cgo, no new dependency: macOS runs `/usr/bin/osascript` (JXA, value on stdin) to set `public.utf8-plain-text` plus `org.nspasteboard.ConcealedType` (~100 ms per copy); Windows sets `CF_UNICODETEXT` plus `ExcludeClipboardContentFromMonitorProcessing`, `CanIncludeInClipboardHistory` = 0 and `CanUploadToCloudClipboard` = 0 through `golang.org/x/sys/windows`; Linux and others write plainly. Any failure before the text is set falls back to `atotto/clipboard`. Verified on macOS in a real TUI (pasteboard types include the marker; `x` still clears); Windows only cross-builds and vets so far.

**Why.** Part (b) of the old R-07: while a secret sits on the clipboard, clipboard-history tools can record it.

**Scope.** Mark the copy as concealed/transient where the OS supports it so managers skip it: macOS `org.nspasteboard.ConcealedType`, Windows `ExcludeClipboardContentFromMonitorProcessing` / `CanIncludeInClipboardHistory`. Linux has no standard.

**Starting points.** `writeClipboard` in `internal/tui/update.go` (currently `atotto/clipboard`, which cannot set types), `readClipboard`.

**Decisions.** Per-OS files with build tags and a native call (cgo or a small dependency) versus leaving it documented as a limitation. Evaluate the cost first; a cgo dependency would also break the CGO-off release builds, so prefer pure-Go options (e.g. a clipboard library that supports it).

**Ask the user.** Accept per-OS code and possibly a new dependency?

**Acceptance.** On each supported OS the copy carries the concealed marker (verified manually per `docs/QA.md`); release builds stay `CGO_ENABLED=0`; README security text updated.

---

## Release 0.4.0 — workflows and adoption

### R-26 Dotenv output and CLI ergonomics
**Status** done · **Size** S · **Depends** –

**Scope.** `pandora env -s TITLE[=ENV] …` prints `KEY=value` lines (explicit opt-in, since it prints values); `get --copy` copies with the same clipboard deadline logic as the TUI (or prints a note that the clear needs the TUI); `get -q`/`--newline=false` for scripting.

**Decisions.** Printing values to stdout is already done by `get`; `env` just formats. Escape/quote rules for dotenv (newlines need R-20). Whether `env` refuses when stdout is a TTY without `--force`. *Recommended: yes.*

**Acceptance.** Quoting round-trips through a dotenv parser test; no value in stderr; exit codes follow the existing convention.

**Outcome (done).** `env` (`internal/cli/dotenv.go`): `dotenvValue` is the quoting rule R-25's dotenv parser must invert (bare `[A-Za-z0-9_./:@%+-]*`, else double-quoted with `\\ \" \n \r \$` escaped); names default via `EnvName` like `run`, and must match `[A-Za-z0-9_.-]+`. Refuses on a terminal stdout without `--force` (exit 2, `app.isOutTTY`). `get -q`/`--newline=false`/`--copy`: the copy has no clear deadline in the CLI (process exits), noted on stderr. The concealed clipboard code moved from `internal/tui` to `internal/secretclip` so the CLI can share it without importing the TUI. Verified: `go test -race` for cli and tui, Windows/Linux/CGO-off builds, pty refusal run by hand; the success path ran only against the mock keyring (tests), not a real keychain.

### R-30 `expiring` command
**Status** done · **Size** S · **Depends** R-23 (done)

**Scope.** `pandora expiring [--within 14d] [--json]` lists expiring/expired secrets (never values) and exits with a distinct code when any exist, for cron and CI. Reuses `SecretMeta.DaysUntilExpiry`.

**Decisions.** Exit code: `3` if any are expiring/expired, `0` otherwise (document it next to the existing 0/1/2). The default window is the `expiry_warn_days` setting from `config.json` (R-23, default 14), so the TUI and the command agree; `--within` overrides it.

**Acceptance.** Table test for window boundaries (today, tomorrow, yesterday, no expiry); JSON output stable and documented.

**Outcome (done).** `internal/cli/expiring.go`; `app.now` is the injectable clock (`harness.now` in tests). Window is `--within N[d]` (0–3650, shares `config.MaxExpiryWarnDays`) or `expiry_warn_days`. Output sorted soonest first; JSON is `[{title, category, expires, days, status}]` with `status` `expired` or `expiring` (today is expiring, like the TUI) and `[]` when empty. Exit `3` (`exitExpiring`) whenever the list is not empty, in table and JSON mode. Verified: `go test -race ./...`, Windows/Linux/CGO-off builds, the built binary for `help`, `-h`, a bad `--within` (exit 2) and bash/zsh completions; the listing path ran only against the mock keyring (tests), not a real keychain.

### R-25 Import from other tools
**Status** done · **Size** M · **Depends** R-26 (done)

**Why.** Migration is what decides whether someone adopts Pandora. Today `import` only reads Pandora's own export.

**Scope.** `pandora import --format <fmt> FILE` with `dotenv`, `1password-csv`, `bitwarden-json`, `chrome-csv` (start with dotenv + one password-manager format). Dry-run (`--dry-run`) lists what would be created and conflicts. Reuses `store.RestoreSecrets`.

**Starting points.** `internal/cli/transfer.go` (parsing, validation, `--overwrite`), `store/restore.go`.

**Decisions.**
- One pure parser per format returning `[]store.SecretMeta + value`, so each is unit-tested with small synthetic fixtures (never real exports).
- Mapping rules (which column becomes the title, category, notes; duplicates; invalid titles) are decided per format and documented in the README.
- Imported plaintext files are the user's responsibility: print a reminder to delete them; never copy them.

**Decisions taken (user, 2026-11-22).** Formats: `dotenv` plus `bitwarden-json`; no sample export (fixtures follow Bitwarden's public JSON layout, so validate against a real export when one is available); user name and first URL go into the notes. `1password-csv` and `chrome-csv` stay open for a later release.

**Acceptance.** Each parser has a fixture test including malformed rows; `--dry-run` writes nothing; conflicts follow `--overwrite`.

**Outcome (done).** `internal/cli/importers.go` (`parseDotenv`, `parseBitwardenJSON`, `externalParsers`); `cmdImport` takes `--format`, `--dry-run`, `--category` and shares `normalizeSecret` with `parseExport`. File-level problems (syntax, encrypted or wrong kind of file) abort before anything is written and name only a line number, never content; single entries that cannot be used (empty value, over 32 KB, card, identity, control characters) are skipped and listed on stderr with a reason that never contains a value. Dotenv: `export ` prefix, comments, bare, `'literal'` and `"escaped"` values (multi-line allowed), duplicate key is an error, empty value is skipped; the old test-only dotenv reader in `env_test.go` was replaced by the real parser. Bitwarden: login password or secure-note text is the value, folder is the category, notes are one line (user name, first URL), original dates kept, duplicate names get ` (2)`, TOTP seeds and custom fields are never imported, encrypted exports are rejected. Verified: `go test -race ./...`, Windows/Linux/CGO-off builds, the built binary for the usage errors and completions; import itself ran only against the mock keyring (tests), not a real keychain, and never against a real Bitwarden export.

---

## Release 0.5.0 — history and recovery

### R-24 Value history
**Status** done · **Size** L · **Depends** R-23 (done)

**Why.** `UpdateSecret` overwrites the old value irreversibly; a mistaken edit loses a key.

**Scope.** Keep the last N previous values per secret (default 3, `0` disables); show a version list in the detail card/overlay; restore a previous version; the CLI can read one (`get --version -1`).

**Starting points.** `store.UpdateSecret` and `RestoreSecrets` (they already write the replacement under a fresh ID and remove the old entries only after the vault points at the new ones: history keeps those old IDs instead of removing them), `DeleteSecret`, `chunks.go` (one version can span several keyring entries), `MissingValues`, `export`/`import`. *Revised 2026-11-18: the original text assumed title-named keyring entries.*

**Decisions (design needed).**
- Storage: a `history` list in the encrypted vault record (`omitempty`; per version the keyring ID, chunk count and replaced-on date, newest first) pointing at the old keyring entries. No new keyring naming scheme and no title collisions; rename touches nothing. A reader that does not know the field ignores it and would leave those entries behind (harmless but never removed): note it in the downgrade text.
- Everything that touches an entry handles history: delete and pruning remove the entries only after the vault write, `doctor` checks them, `export --values` excludes them by default, the orphan check ignores them.
- Atomicity: the order of `UpdateSecret` (store new, write vault, remove only what the vault no longer references); a failed vault write removes just what this call stored.
- Security: old values are still secrets; delete must wipe every version; document that history increases exposure.

**Decisions taken (user, 2026-11-22).** On by default: `history_depth = 3` (`0` disables). CLI: `pandora history TITLE [--json]`, `pandora get TITLE --version -1` and `pandora restore TITLE [--version -1]`; no confirmation on restore because it only swaps ids and can be undone. Restore swaps the current value with the chosen version and puts the replaced one on top of the history. `H` is the TUI key (`v` was already reveal).

**Acceptance.** Update keeps N versions and prunes the oldest; rename and delete carry/remove all versions; rollback test when the metadata write fails; restore works in TUI and CLI; old metadata loads with zero versions.

**Outcome (done).** `internal/store/history.go` (`version`, `SetHistoryDepth`, `record.replaceValue`, `Versions`, `GetVersion`, `RestoreVersion`, `VersionCounts`, `MissingVersions`, `DropMissingVersions`), `record.History` (`omitempty`, vault stays `version: 2`), `UpdateSecret` / `RestoreSecrets` / `DeleteSecret` changed, config key `history_depth` (0 to 20, default 3), CLI `history` / `restore` / `get --version`, `doctor` extension, TUI `modeHistory` (`internal/tui/history.go`). Unchanged values create no version and are not rewritten. The depth is a package setting applied by TUI `New` and CLI `import`. `store.DropEntriesForTests` removes keyring entries behind the vault's back for the CLI tests. Not done: `export --values` leaves history out and there is no flag to include it. Verified: `go test -race ./...`, 60×18, 80×24 and 100×30 view-size test of the history screen with 12 versions, cross-builds; everything ran against the mock keyring only, never a real keychain, and the TUI was not driven in a real terminal.

### R-28 `doctor` vault recovery
**Status** done · **Size** M · **Depends** –

**Why.** Since 0.2.0 titles and metadata are encrypted inside `vault.json`, keyring accounts are random IDs, and the data key exists only wrapped by the master password inside `vault.json`. The old scope (a keyring index rebuilding `metadata.json`) no longer applies. What remains recoverable is the vault file itself: every write already keeps `vault.json.bak`.

**Decision taken.** Re-scoped to vault-file recovery (option B), done before 1.0.

**Scope.** `pandora doctor` verifies that `vault.json` and `vault.json.bak` parse and decrypt (needs the master password; a wrong password never triggers a restore). `doctor --fix` restores `vault.json` from the `.bak` after confirmation. Nothing is overwritten when both files are bad. Keyring entries that no vault record points to still cannot be listed (go-keyring cannot enumerate); that limitation in ARCHITECTURE stays.

**Starting points.** `internal/store/doctor.go`, `internal/store/vault.go`, the `doctor` entry in `internal/cli/registry.go`.

**Outcome (done).** `internal/store/recover.go` (`InspectVault`, `RestoreVaultBackup`, `ErrVaultCorrupt`), `internal/cli/doctor.go` (`cmdDoctor` moved out of `commands.go`). `doctor` asks for the master password against both files instead of unlocking first, so it works when vault.json is unreadable. Restore only when the password opens the backup and vault.json is missing or corrupt (not merely locked by another password, not from a newer version); the damaged file is kept as `vault.json.corrupt` and the `.bak` is left as is. After a restore `doctor` never removes entries without a value in the same run (the backup predates the last change). Any command that meets a damaged vault prints a hint to run `doctor`. Verified: `go test -race ./...`, cross-builds, and the built binary against a temp HOME with a vault that has no secrets (damaged, wrong password with `--fix -y`, restore, `list` afterwards); the TUI still shows its own "could not load" screen and does not mention `doctor`.

**Acceptance.** A corrupt `vault.json` with a valid `.bak` is reported by `doctor` and restored by `doctor --fix`; a wrong password never triggers a restore; nothing is overwritten when both files are bad.

---

## Release 0.6.0 — auditability

### R-32 Local audit log
**Status** done · **Size** M · **Depends** R-23 (done); schedule after R-25 and R-26 so the new `env`, `get --copy` and import paths are logged from day one

**Scope.** Opt-in append-only JSONL file recording time, action (`reveal`, `copy`, `get`, `run`, `export`, `add`, `edit`, `rm`) and secret title — never values — plus `pandora log [--since]`.

**Decisions.**
- Opt-in via a new `audit_log` key in `config.json` (R-23 is shipped); default off, because a log of names and times is itself sensitive metadata.
- Size cap with rotation; `0600`.
- Written from `store`/`cli` call sites through one `audit.Record(...)` function so the TUI and CLI cannot diverge.

**Decisions taken (user, 2026-11-23).** Retention: a 1 MiB cap with one rotated file, no settings. Failed unlock attempts are not logged (no key to encrypt with, and a plaintext line would leak attempt times). **Deviation from the original scope:** the log is not plain JSONL: every line is encrypted under the data key (AGENTS.md forbids plaintext titles on disk). The writer is `store.RecordAudit` rather than a separate `audit` package, because it needs the session key.

**Acceptance.** Disabled by default writes nothing; enabled logs each action once; values never appear (test greps the file for the test secret); rotation test.

**Outcome (done).** `internal/store/audit.go` (`RecordAudit`, `ReadAudit`, `SetAuditEnabled`), `audit_log` config key, `internal/cli/audit.go` (`app.audit`, `pandora log`), `internal/tui/audit.go` (`model.audit`, failure shown as a toast by the `Update` wrapper). CLI call sites: `get`/`copy`, `run`, `env`, `export` (one entry per secret with `--values`, one empty-title entry otherwise), `import` (per secret written), `add`, `rm`, `restore`, and `doctor --fix` removals as `rm`. TUI: reveal (also of previous values), copy, add, edit, delete, restore. The CLI now loads `config.json` once per process (memoised) and applies `history_depth` and `audit_log` when it unlocks, instead of only in `import`. Verified: `go test -race ./...` (three runs), cross-builds; the tests use the mock keyring, so nothing was run against a real keychain or in a real terminal.

---

## Release 0.7.0 — stabilise

No new features: this release is the 1.0 release candidate. Everything here is about making the public surface boring and provable.

### R-50 API freeze audit
**Status** todo · **Size** M · **Depends** R-25, R-28, R-30, R-24, R-32

**Scope.** Enumerate from `commands()` (`internal/cli/registry.go`) every command, flag, exit code (`0/1/2/3`), stdout format and JSON shape (`list --json`, `expiring --json`, export JSON `SecretMeta`, `config get`). Write them into a new `docs/STABILITY.md`: *frozen from 1.0* (commands, flags, exit codes, JSON schemas, `vault.json` v2, export format) and *may change in minor releases* (TUI keys, layout, wording of messages). Rename or remove anything awkward now (for example inconsistent flag names between `get`, `env` and `run`) as a clean cutover with a changelog note; after 1.0 only additive changes.

**Ask the user.** Which awkward names to change, once the audit lists them.

**Acceptance.** `docs/STABILITY.md` exists and matches the code; a test snapshots `commands()` flag names and the exit-code constants so accidental changes fail CI.

### R-51 Format compatibility test corpus
**Status** todo · **Size** M · **Depends** R-24

**Scope.** Check in `internal/store/testdata/` a `vault.json` fixture per shipped format state (0.2.0 record, 0.3.0 with tags and chunks, 0.5.0 with history), created with fixed KDF parameters and a known password, plus an export JSON from each. Tests load every fixture with the current code. Fixtures use synthetic titles and values only.

**Acceptance.** All fixtures unlock and list the expected titles; adding a field without `omitempty` or breaking an old record fails a test. If R-24 bumps the vault version, the corpus gains a migration fixture.

### R-52 TUI polish and known bugs
**Status** todo · **Size** S · **Depends** –

**Scope.** Fix the known bug "empty-vault welcome screen is 23 lines tall at 60×18" (0.3.0 gate row) and re-run the size test for every `mode`.

**Acceptance.** View height equals window height in every `mode` at 60×18 and 80×24.

---

## Release 1.0.0

### R-53 1.0 release
**Status** todo · **Size** S · **Depends** R-05, R-11, R-50, R-51, R-52

**Scope.** Gate checklist, then the release:
- R-05: `docs/QA.md` executed on macOS, Linux and Windows against the real release artefacts, results recorded.
- R-11: Windows concealed-copy check done.
- No `blocked` item in the index.
- Final read-through of `SECURITY.md`; the "pre-1.0" supported-versions text becomes "1.x".
- Changelog heading `## [1.0.0] - date`.
- Tag only when the user asks.

**Acceptance.** Every gate above is recorded as met; the release workflow publishes the archives, checksums and notes.

---

## After 1.0

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
**Status** todo · **Size** L · **Depends** R-23 (done); post-1.0 (1.2)

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
