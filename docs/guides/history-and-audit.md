# Value history and the audit log

## Value history

When you change a secret's value, the old one is kept as a previous version (the last `history_depth`, default 3; `0` turns history off). `H` in the interface and `pandora history`, `pandora get --version` and `pandora restore` on the command line list, read and restore them. Saving the value it already has changes nothing, and renaming or editing other fields leaves the history alone.

Previous values are secrets too: they sit in the OS keyring, encrypted under the same key as the current value, and `pandora rm` / `d` deletes every one of them. They add exposure, which is why the depth is a setting. `pandora export` never includes history. `pandora doctor` checks that every previous value is still in the keyring and `--fix` forgets the ones that are not; it keeps an entry whose current value is gone but which still has previous values, so you can `restore` one.

**Going back to an older Pandora.** Versions before this feature ignore the history and drop it the next time they save the vault, which leaves those keyring entries behind unreferenced (still encrypted, but never cleaned up).

## Audit log

Off by default. With `"audit_log": true` in `config.json`, Pandora records what is done with your secrets: **when**, **what** (`reveal`, `copy`, `get`, `run`, `env`, `export`, `import`, `add`, `edit`, `rm`, `restore`) and **which secret**, in the interface and on the command line alike. It never records a value. The log is `audit.log` next to `vault.json`; each line is encrypted under the same key as the vault, so the file shows no title or time without your master password. When it reaches 1 MiB it rotates once to `audit.log.1` (the older one is discarded). Read it with `pandora log [--since 7d|12h|YYYY-MM-DD] [--json]`.

It is a record for you, not a defence: someone who can run Pandora as you can switch it off in `config.json`, and failed unlock attempts are not recorded (without the key there is nothing to encrypt them with). If the log cannot be written, the action still happens and Pandora warns you.
