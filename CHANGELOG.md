# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Edit a secret (`e`): change name, category and optionally the value; renaming moves the keyring value.
- Delete a secret (`d`) with a confirmation dialog.
- Reload the list from disk (`r`).
- Validation: name and secret are required when adding; names must be unique.
- Status messages for save, read, update, delete and reload errors.
- Responsive two-pane layout, adaptive light/dark palette, contextual footer help.
- Tests for `internal/store` and `internal/tui` using an in-memory keyring.
- Documentation: README, architecture notes, roadmap, contributing guide and security policy.

### Changed
- Project restructured into `cmd/pandora`, `internal/store` and `internal/tui`.
- UI text is now in English.
- The add form is reset each time it opens; `enter` advances to the next field.
- New and edited secrets are selected in the list after saving.
- Metadata is written atomically, and a corrupt `metadata.json` is reported instead of being overwritten.

### Fixed
- Adding a secret with an existing name no longer silently overwrites its value or duplicates the metadata entry; duplicates written by older versions are collapsed on load.
- Save and read failures are no longer silently ignored.
