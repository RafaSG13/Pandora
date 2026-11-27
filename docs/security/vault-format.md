# Vault format

`vault.json` is Pandora's one persistent file of metadata. **The format is an API**: it is versioned, additive and covered by tests. This page describes version 2.

## Location and permissions

`<user config dir>/pandora/vault.json`. The directory is `0700` and the file `0600` on Unix. Windows has no POSIX modes and relies on the user profile's ACLs.

Next to it:

| File | Purpose |
| ---- | ------- |
| `vault.json.bak` | The previous version, still encrypted. Copied before every write. |
| `vault.json.corrupt` | A damaged `vault.json` kept aside by `pandora doctor --fix` when it restores the backup. |
| `vault.lock` | Exclusive write lock (`flock`, or `LockFileEx` on Windows). Never deleted. The kernel drops the lock when a process dies, so there is no stale-lock cleanup. |
| `config.json` | Optional settings. Never contains secrets. |
| `audit.log`, `audit.log.1` | Optional encrypted audit log, rotated at 1 MiB. |

## Structure

```json
{"version": 2,
 "kdf": {"alg": "argon2id", "salt": "<b64 16 B>", "time": 3, "memory_kib": 65536, "threads": 4},
 "wrapped_key": "<b64 nonce24 || ciphertext>",
 "data": "<b64 nonce24 || ciphertext>",
 "sharing": "<b64 nonce24 || ciphertext>"}
```

- `wrapped_key` is the random 32-byte data key sealed under the key Argon2id derives from the password. A failed open means "wrong password".
- `data` is the records array sealed under the data key with a fresh nonce on every write.
- `sharing` is present once you have used `pandora id` or `pandora peer`. It holds your share identity and the trusted peers, sealed under the data key.

## Records

The decrypted `data` is a JSON array of records:

```json
{"id": "<32 hex>", "chunks": 2, "title": "github-token", "category": "Tokens",
 "username": "rafa", "created": "2026-11-15", "updated": "2026-11-20",
 "expires": "2027-01-31", "notes": "https://github.com/settings/tokens",
 "tags": ["ci", "prod"],
 "history": [{"id": "<32 hex>", "chunks": 1, "replaced": "2026-11-20"}]}
```

| Field | Meaning |
| ----- | ------- |
| `id` | Keyring account of the first piece of the value. Random, 32 lowercase hex characters. |
| `chunks` | Number of keyring entries holding the value. Omitted for one. |
| `title` | The secret's name, unique within the vault. |
| `category`, `username`, `notes` | Free-form text. Never put a secret in the notes. |
| `created`, `updated`, `expires` | `YYYY-MM-DD`. `updated` and `expires` are optional. |
| `tags` | Normalised: trimmed, lowercase, unique, sorted. At most 10 tags of at most 24 characters, with no spaces or commas. |
| `history` | Previous values, newest first. Each points at keyring entries encrypted under the same data key with their own ID as AAD, so nothing is re-encrypted when a value is replaced or restored. |

Titles, categories, user names and notes must not contain control characters, so no stored text can inject terminal escape sequences.

## Compatibility rules

- **New record fields are `omitempty` with a safe default.** Older records load unchanged, and a record that does not use the field is written in the old shape.
- A real format change gets a new `version` and a migration.
- An older Pandora that does not know a field ignores it and **drops it on its next write**. This applies to tags, user names, value history and the share identity. Downgrading after using a feature can lose that data. Keyring entries that history pointed at stay behind, unreferenced and still encrypted.
- Plain-text stores from early versions (`metadata.json` plus keyring entries named after the titles) are migrated on first start. On any failure the old files are left untouched.

## Integrity and failure behaviour

- A failed decrypt of `data` is an error, never an empty vault. Mutating commands abort rather than overwrite a vault they cannot read.
- Writes go to a temporary file in the same directory followed by `rename`, so a crash cannot leave a half-written file.
- `pandora doctor` classifies `vault.json` and `vault.json.bak` as OK, missing, corrupt, newer than this Pandora, or wrong password. It restores the backup only when your password opens the backup and `vault.json` is missing or corrupt. See [Backup and recovery](../guides/backup-and-recovery.md).

For the full code-level description, including the order of keyring and vault writes, see the [architecture notes](../ARCHITECTURE.md).
