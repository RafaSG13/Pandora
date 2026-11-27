# Backup and recovery

The master password cannot be recovered, so a backup is the only way back from a lost or damaged vault.

## Encrypted backups

`export --values` never writes plaintext values. The document, titles included, is encrypted with [age](https://age-encryption.org) using a scrypt passphrase recipient and ASCII-armored; `age -d backup.age` can decrypt it too. The passphrase must be at least 10 characters when exporting; it is prompted for twice without echo, or read from `PANDORA_PASSPHRASE` when there is no terminal (environment variables can be visible to other processes of your user, so prefer the prompt where possible; a warning is printed and `pandora run` removes the variable from the child's environment). Export files are created with mode `0600`, and an existing file is only replaced with `--force`. Export without `--values` is **plain text**: names, categories, notes and dates, but no values.

## Recovering a damaged vault

If `pandora list` says the vault file is corrupt, run `pandora doctor`. When `vault.json.bak` opens with your master password, `pandora doctor --fix` puts it back; the damaged file is kept as `vault.json.corrupt`. The backup is the state before your last change, so that change is lost. A wrong password never restores anything, and with no usable backup only an export made with `pandora export --values` can bring your secrets back.

!!! note "Doctor limitation"
    `doctor` can only find vault entries without a value. The keyring API Pandora uses cannot list entries, so values that exist in the keyring without a vault entry cannot be detected.
