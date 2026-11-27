# Pandora

Pandora is a terminal vault for developer secrets: API keys, tokens, passwords and private keys. A master password protects everything. Names, categories, notes, dates and values are encrypted; the values themselves live in your operating system's keyring under random IDs. Pandora makes no network connections and sends no telemetry.

It ships as one static binary with two faces: an interactive terminal interface (`pandora`) and a scriptable command line (`pandora get`, `add`, `run`, `export`, …).

## Where to start

<div class="grid cards" markdown>

- **New here?** [Install Pandora](getting-started/installation.md) and follow the [quick start](getting-started/quickstart.md).
- **Evaluating the security?** Read the [threat model](security/threat-model.md), then the [cryptography](security/cryptography.md) and [vault format](security/vault-format.md).
- **Using it in scripts or CI?** See [scripting and CI](guides/scripting-and-ci.md) and the [command-line guide](reference/cli.md).
- **Moving secrets between machines?** [Sharing](security/sharing.md) explains the signed, peer-encrypted files.

</div>

## What these docs cover

| Section | Contents |
| ------- | -------- |
| **Getting started** | Install, verify a download, first run |
| **Concepts** | What is stored where, and how the pieces fit |
| **Security** | Threat model, cryptography, the on-disk format, sharing, hardening |
| **Guides** | Scripts and CI, backup and recovery, importing, history and the audit log |
| **Reference** | Every command and flag, the configuration file, keybindings |
| **Project** | Architecture notes, QA checklist, roadmap, contributing |

!!! warning "Pandora is pre-1.0"
    The command line, the JSON formats and the vault format are still allowed to change between minor versions. The [roadmap](ROADMAP.md) tracks what must be frozen before 1.0, and the changelog lists every change.

## Reporting a vulnerability

Please do not open a public issue. Follow the [security policy](https://github.com/RafaSG13/Pandora/blob/main/SECURITY.md).
