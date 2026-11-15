# Contributing to Pandora

Thanks for helping out. Bug reports, fixes and ideas from [docs/ROADMAP.md](docs/ROADMAP.md) are all welcome.

## Getting started

```sh
git clone <repository-url> pandora
cd pandora
make build && make test
```

You need the Go version in `go.mod`. On Linux you also need a Secret Service provider to run the app (tests do not).

## Before you open a pull request

```sh
gofmt -l .      # must print nothing
make vet
make test
```

Run the app and exercise the path you changed. For UI changes, try the smallest supported terminal (60×18) as well as a large one, and `PANDORA_ASCII=1`.

## Guidelines

- **Keep the layers separate.** `internal/store` knows nothing about the UI; `internal/tui` never talks to the keyring or the filesystem directly, only through `store`; `internal/cli` reaches the keyring through `store` and touches the filesystem only for export/import files.
- **Never test against the real keychain or clipboard.** Tests must call `keyring.MockInit()` and point `HOME` / `XDG_CONFIG_HOME` / `AppData` at `t.TempDir()`; TUI tests also stub `writeClipboard`. Copy the `setup` helper in `internal/store/keyring_test.go` or `newTestModel` in `internal/tui/update_test.go`.
- **Test behaviour, not wiring.** Cover boundaries, error paths and state transitions that a user could observe.
- **Never log or print secret values**, including in error messages.
- **Persisted format is an API.** `store.SecretMeta` JSON field names live in users' `metadata.json` files; additions must be backwards compatible (missing field = default).
- **Order keyring and metadata writes so a failure cannot lose a value** (write the new thing, then remove the old one).
- Keep exported identifiers documented and run `gofmt`.
- Match the existing UI conventions: palette in `internal/tui/styles.go`, `✓`/`✗` status messages, English text.

## Reporting bugs

Include your OS, terminal, Pandora version (commit), what you did and what you expected. **Do not paste secret values or the contents of `metadata.json`** — redact names if they are sensitive.

Security issues: see [SECURITY.md](SECURITY.md); do not open a public issue.

## Commit messages

Short imperative subject (`Add delete confirmation`), with a body explaining *why* when it isn't obvious. Update [CHANGELOG.md](CHANGELOG.md) under **Unreleased** for user-visible changes.
