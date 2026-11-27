# Using Pandora safely

The [threat model](threat-model.md) says what Pandora can and cannot do. This page turns it into habits.

## Choose a strong master password

Everything rests on it. An attacker who copies `vault.json` can guess offline, limited only by Argon2id (64 MiB, 3 passes) and by the password's strength.

- Use a long, unique passphrase: several random words, or at least 16 characters. The minimum Pandora accepts is 10.
- Do not reuse a password from any other service.
- Write it down somewhere safe, offline. It **cannot be recovered**.

## Keep a backup that does not need the vault

`pandora export --values -o backup.age` writes an age-encrypted backup under a separate passphrase. Store it away from the machine. It is your only way back if the vault is lost or the password forgotten. See [Backup and recovery](../guides/backup-and-recovery.md).

## Lock when you leave

The interface locks itself after 5 idle minutes (`lock_minutes`; `0` disables it, which is not recommended). Press `L` to lock at once. Locking drops the data key from memory, empties the list and the form, and clears a pending clipboard.

## Mind the clipboard

- Copies are cleared after 15 seconds (`clipboard_seconds`), or right away with `x`, but only if the clipboard still holds what Pandora copied.
- On macOS and Windows copies are marked so clipboard history skips them. On Linux there is no such convention. If you run a clipboard manager, check whether it records everything.
- `pandora get --copy` does **not** clear the clipboard afterwards, because the process exits at once. Prefer the interface for copying.

## Prefer `run` to files, and the prompt to the environment

| Instead of | Prefer | Why |
| ---------- | ------ | --- |
| A `.env` file with real values | `pandora run -s NAME=VAR -- cmd` | The variable exists only for that process and its children. |
| `PANDORA_MASTER_PASSWORD` in your shell profile | The hidden prompt | Environment variables can be visible to other processes of your user. |
| `PANDORA_PASSPHRASE` | The prompt, when interactive | Same reason. |
| `pandora get x` in a shell where history is recorded | `pandora run`, or `$(pandora get -q x)` in a script | Keeps the value out of your terminal scrollback. |

If you must use `pandora env > .env`, delete the file when you are done. `pandora env` refuses to print values to a terminal unless you pass `--force`.

## Do not put secrets in names or notes

Titles, categories, notes and tags are encrypted in the vault, but they are shown in clear on screen and written in clear by `pandora export` without `--values`. A secret in the notes field is a secret in plain text in that export.

## Treat imported files as secrets

`dotenv` and Bitwarden JSON exports hold your values in plain text. Run `pandora import --dry-run` first, then delete the source file once you have checked the result.

## Keep it current, verify what you install

Download releases only from the official [Releases page](https://github.com/RafaSG13/Pandora/releases), and check the archive against `checksums.txt` (see [Installation](../getting-started/installation.md)). Binaries are not code-signed or notarised yet.

## Shared machines

Pandora protects against casual exposure, not against other users with root, or against malware running as you. Do not keep a vault on a machine you do not control.
