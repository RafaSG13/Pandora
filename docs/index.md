---
title: Pandora
hide:
  - navigation
  - toc
---

<div class="pd-landing" markdown>

<div class="pd-hero" markdown>
<div class="pd-banner"><img src="assets/pandora-logo.svg" alt="Pandora"></div>

# A terminal vault for developer secrets, with no cloud attached.

Pandora keeps API keys, tokens, passwords and private keys encrypted behind a master password, with the values stored in your operating system's keyring. A fast terminal interface and a scriptable command line. One static binary, no account, no network, no telemetry.

<a class="pd-btn fill" href="getting-started/installation/">Install</a>
<a class="pd-btn line" href="security/threat-model/">Read the threat model</a>
<a class="pd-btn line" href="https://github.com/RafaSG13/Pandora">GitHub</a>
</div>

## Why Pandora

<div class="pd-grid" markdown>
<div class="pd-card" style="--c: var(--pd-purple)" markdown>
### Encrypted and local
Names, notes and values are encrypted with Argon2id and XChaCha20-Poly1305. Values also live in the OS keyring under random IDs, so even the keychain shows nothing readable.
</div>
<div class="pd-card" style="--c: var(--pd-gold)" markdown>
### Built for the shell
`pandora run -- cmd` injects a secret into a single process. `get`, `add`, `env`, `export` and `import` make it scriptable, with stable exit codes for CI.
</div>
<div class="pd-card" style="--c: var(--pd-cyan)" markdown>
### Safe by default
Values are masked until revealed, the clipboard is cleared after 15 seconds and hidden from clipboard history on macOS and Windows, and the screen locks itself when idle.
</div>
<div class="pd-card" style="--c: var(--pd-green)" markdown>
### Share without a server
Send chosen secrets to another machine as a file, signed by you and encrypted for the recipient with a post-quantum hybrid key. No passphrase, no network.
</div>
<div class="pd-card" style="--c: var(--pd-magenta)" markdown>
### Fast to find things
Fuzzy search with `tag:`, `cat:` and `user:` qualifiers, category and tag pickers, expiry warnings and value history you can roll back.
</div>
<div class="pd-card" style="--c: var(--pd-red)" markdown>
### Honest about its limits
The [threat model](security/threat-model.md) says what it does not cover: malware running as you, clipboard managers that ignore the concealed marker, and lost master passwords.
</div>
</div>

## See it

<div class="pd-demo" markdown>
<img src="assets/demo.gif" alt="The terminal interface: browsing, revealing, filtering, adding a secret">

The terminal interface: browse, reveal, filter by tag, add a secret with the generator.
</div>

<div class="pd-demo" markdown>
<img src="assets/demo-cli.gif" alt="The command line: run, id, peer add, export --to, import">

The command line: inject a secret into one command, then share it with another machine.
</div>

<div class="pd-shots" markdown>
<figure><img src="assets/screenshots/browse.png" alt="Browse mode"><figcaption>Browse</figcaption></figure>
<figure><img src="assets/screenshots/reveal.png" alt="A revealed value"><figcaption>Reveal</figcaption></figure>
<figure><img src="assets/screenshots/picker.png" alt="Category and tag picker"><figcaption>Filter picker</figcaption></figure>
<figure><img src="assets/screenshots/compact.png" alt="Compact rows"><figcaption>Compact rows</figcaption></figure>
<figure><img src="assets/screenshots/form.png" alt="New secret form"><figcaption>New secret</figcaption></figure>
<figure><img src="assets/screenshots/help.png" alt="Help overlay"><figcaption>Help</figcaption></figure>
</div>

## Get started

```sh
brew install RafaSG13/tap/pandora   # or: go install github.com/RafaSG13/Pandora/cmd/pandora@latest
pandora
```

## Find your way

<div class="pd-grid" markdown>
<div class="pd-card" style="--c: var(--pd-purple)" markdown>
### [Getting started](getting-started/installation.md)
Install, verify a download and take the first steps.
</div>
<div class="pd-card" style="--c: var(--pd-gold)" markdown>
### [Security](security/threat-model.md)
Threat model, cryptography, vault format, sharing and hardening.
</div>
<div class="pd-card" style="--c: var(--pd-cyan)" markdown>
### [Guides](guides/scripting-and-ci.md)
Scripting and CI, backup and recovery, importing, history and audit.
</div>
<div class="pd-card" style="--c: var(--pd-green)" markdown>
### [Reference](reference/cli.md)
Every command and flag, the configuration file and the keybindings.
</div>
</div>

!!! warning "Pandora is pre-1.0"
    The command line, the JSON formats and the vault format may still change between minor versions. The [roadmap](ROADMAP.md) tracks what must be frozen before 1.0.

To report a vulnerability, follow the [security policy](https://github.com/RafaSG13/Pandora/blob/main/SECURITY.md).

</div>
