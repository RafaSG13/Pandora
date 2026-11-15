# Contributing to Pandora

Thanks for helping out. Bug reports, fixes and ideas from [docs/ROADMAP.md](docs/ROADMAP.md) are all welcome.

## Getting started

```sh
git clone https://github.com/RafaSG13/Pandora.git
cd Pandora
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
- **One test touches the real keyring, and only in CI.** `internal/store/integration_test.go` (build tag `integration`, also gated on `PANDORA_INTEGRATION=1`) runs in the Linux job inside a throwaway D-Bus session. Don't run it on a machine whose keychain you care about.
- **Test behaviour, not wiring.** Cover boundaries, error paths and state transitions that a user could observe.
- **Never log or print secret values**, including in error messages.
- **Persisted format is an API.** `vault.json` (version 2, encrypted) holds `store.SecretMeta` records; additions must be backwards compatible (missing field = default), and a format change needs a new `version` and a migration.
- **Order keyring and vault writes so a failure cannot lose a value** (write the new thing, then remove the old one).
- **Plaintext titles, categories and notes never reach disk, the keyring or logs**; control characters are rejected in them (`store.ValidateText`).
- Keep exported identifiers documented and run `gofmt`.
- Match the existing UI conventions: palette in `internal/tui/styles.go`, `✓`/`✗` status messages, English text.

## Reporting bugs

Include your OS, terminal, Pandora version (`pandora --version`), what you did and what you expected. **Do not paste secret values**; `vault.json` is encrypted, but redact names if they are sensitive.

Security issues: see [SECURITY.md](SECURITY.md); do not open a public issue.

## Commit messages

Short imperative subject (`Add delete confirmation`), with a body explaining *why* when it isn't obvious. Update [CHANGELOG.md](CHANGELOG.md) under **Unreleased** for user-visible changes.

## Cutting a release

Maintainers only. Releases are built by `.github/workflows/release.yml` (GoReleaser) when a `v*` tag is pushed.

1. Run the [QA checklist](docs/QA.md) on macOS and record the result in the release PR.
2. In [CHANGELOG.md](CHANGELOG.md), rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` and add a fresh empty `## [Unreleased]` above it. The release job fails if the section for the tag is missing (`scripts/release-notes.sh X.Y.Z` prints what will be published).
3. Merge to `main`, then `git tag vX.Y.Z && git push origin vX.Y.Z`.
4. The workflow publishes the archives, `checksums.txt` and the changelog section as release notes. Check that the artefacts run (`pandora --version` must print `X.Y.Z`, not `dev`).

To rehearse locally: `goreleaser release --snapshot --clean` (output in `dist/`). Pull requests run the same dry run in CI.
