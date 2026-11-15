# Security Policy

## Reporting a vulnerability

Please **do not open a public issue** for security problems. Report them privately through the repository's private vulnerability reporting (GitHub: *Security → Report a vulnerability*) or by contacting the maintainers directly, and include:

- what you found and its impact;
- steps to reproduce, with OS and terminal;
- a suggested fix, if you have one.

Never include real secret values in a report.

## Scope

In scope: ways Pandora could leak or corrupt secret values or metadata, expose titles, notes or values without the master password, weaken the vault encryption or its key derivation, bypass the OS keyring, leave values in the clipboard longer than documented, or write files with unsafe permissions.

Out of scope: threats Pandora does not claim to handle (see *Security model* in the [README](README.md)) — malware running as the same user, clipboard history tools, secret values visible to a program started with `pandora run` (it receives them in its environment), `PANDORA_MASTER_PASSWORD` or `PANDORA_PASSPHRASE` being readable by other processes of your user, a weak master password guessed offline, memory that is not wiped after the idle lock, an older plain-text copy of the data left on disk after migration (deletion is not secure erasure), and weaknesses in the OS keyring itself.

## Supported versions

Pandora is pre-1.0 (`v0.x`): only the latest tagged release and the `main` branch receive fixes, and the command line or data format may change between minor versions (noted in the [changelog](CHANGELOG.md)).
