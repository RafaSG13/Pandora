# Threat model

Pandora is designed to **reduce accidental exposure**, not to defend against a compromised account. This page states what it protects, against whom, and where the limits are.

## Assets

- The **values** of your secrets (tokens, passwords, keys).
- Their **metadata**: titles, categories, user names, notes, tags and dates. A title like `aws-prod-root` is itself sensitive.
- The **master password** and the **data key** it unlocks.

## Adversaries

| Adversary | In scope? | Outcome |
| --------- | --------- | ------- |
| Someone who **sees your screen** (shoulder surfing, screen sharing, recordings) | Yes | Values are masked until revealed and hide again after 10 s or when you move. |
| **Shell history, dotfiles, notes apps, git** | Yes | Secrets live in an encrypted vault and a keyring, not in `.bashrc`, `.env` files or notes. |
| Someone with a **copy of your disk or backups** (stolen laptop, cloud backup of the config directory) | Yes | `vault.json` is ciphertext. Without the master password they see no titles and no values. Offline guessing is slowed by Argon2id (64 MiB, 3 passes). |
| Someone who can **read your keyring entries** but not your vault password | Yes | Entries are opaque: a random ID as the account name, an encrypted blob as the value. |
| A **clipboard history manager** | Partly | Copies are marked as concealed on macOS, Windows and Wayland (`wl-copy --sensitive`, needs `wl-clipboard`). A tool that ignores the marker still records them. X11 has no such convention. |
| **Another machine** you share secrets with | Partly | Shares are signed by the sender and encrypted for the recipient. See [Sharing](sharing.md). |
| **Malware or another process running as your user** | **No** | It can read the keyring (subject to OS prompts) and log your keystrokes. On Linux Pandora is non-dumpable, so `/proc/<pid>/{mem,environ}` and same-user ptrace are refused, and on every Unix core dumps are disabled; macOS and Windows have no such protection here. |
| An attacker with **root or physical access to an unlocked session** | **No** | Out of scope. |
| An attacker who can **modify the Pandora binary** | **No** | Verify downloads against `checksums.txt`. |

## What Pandora does

- Requires a master password to see anything, and keeps values in the OS keyring instead of files.
- Masks values on screen until revealed.
- Clears the clipboard 15 seconds after copying, but only if the clipboard still holds what Pandora copied. It compares a SHA-256 and keeps no plaintext, so something you copied in the meantime is left alone. A detached helper process does the clearing, so it still happens if you quit the interface, and for `pandora get --copy`. The helper receives only the hash, over a pipe, and none of the `PANDORA_*` variables.
- Marks copied values as concealed: `org.nspasteboard.ConcealedType` on macOS, and excluded from clipboard history (`Win+V`) and cloud clipboard sync on Windows.
- Never logs, prints or transmits values and never opens network connections.
- Writes the vault atomically with owner-only permissions (`0600` file, `0700` directory on macOS and Linux; Windows relies on the user profile's ACLs).
- Encrypts names, categories, notes, dates and values under a key only your master password unlocks, and stores keyring entries under random IDs.
- Rejects control characters in titles, categories and notes, so an imported file cannot inject terminal escape sequences.
- Treats the vault file as **untrusted input**: KDF parameters are bounded before any key derivation, and a corrupt or tampered vault is reported, never overwritten.

## What Pandora does not do

- Protect against malware or other processes running as your user.
- Control clipboard managers or other tools that record clipboard history.
- Wipe memory reliably after use. Go does not guarantee it.
- Stop someone who can read your keyring **and** guess a weak master password offline. Use a long, unique one.
- Sync or back up for you. [`pandora export`](../guides/backup-and-recovery.md) is manual, and an export made with `--values` is only as safe as its passphrase.

## Things that surprise people

- **`pandora run` uses the environment.** Values reach the child process (and everything it starts) as environment variables. On some systems other processes of the same user can read them (`ps eww`, `/proc/<pid>/environ`). Use it for commands you trust.
- **The idle lock asks for your master password.** After the idle timeout (default 5 minutes) the data key is dropped from memory (best effort), the list is emptied and the unlock screen needs the password again. An unsaved form is discarded.
- **Lose the master password, lose the vault.** Nothing can recover it. `pandora export --values` makes an encrypted backup under a separate passphrase.
- **`PANDORA_MASTER_PASSWORD` is a trade-off.** It lets scripts and shell completion work without a terminal, but environment variables can be visible to other processes of your user. Without it, completion offers no titles or categories because they are encrypted.
- **`pandora export` without `--values` is plain text.** It writes names, categories, notes and dates, unencrypted, to the file you choose. Never put a secret in the notes.
- **The audit log is a record for you, not a defence.** Someone who can run Pandora as you can switch it off.

## Known limitations

- `ChangePassword` re-wraps the same data key. Someone who knew the old password and kept an old copy of `vault.json` can still open that old copy. Rotating the data key would require re-encrypting every keyring value. Right after `passwd`, `vault.json.bak` also still holds the old wrapping, until the next write replaces it.
- A change made by another process is detected by polling the size and modification time of `vault.json` once per second. On a file system with one-second timestamps, a same-size change within the same second could go unnoticed; `r` always reloads.
- The write lock is advisory: it protects against other Pandora processes, not against something editing `vault.json` or the keyring by hand. On a network share, locking may not exclude other machines.
- The keyring library cannot enumerate entries, so `pandora doctor` finds vault entries without a value but not keyring entries without a vault record.
- Values are text up to 32 KB. There is no binary value type.
- The clipboard helper is a second short-lived process. If it cannot start, or the machine sleeps or shuts down before the deadline, the copy stays on the clipboard until something replaces it.
- The concealed clipboard marker helps only with managers that honour it. On macOS each copy starts `osascript` (about 100 ms); if it fails, the copy falls back to an unmarked write.
- Removing keyring entries is not secure erasure on SSD or APFS.
- No independent security audit is on record. A security review is part of the [1.0 gate](../ROADMAP.md); until then, the design is documented here so that it can be reviewed, and reviews are welcome.
