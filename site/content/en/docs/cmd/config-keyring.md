---
title: "sq config keyring"
description: "Manage source secrets stored in the OS keyring"
group: config
draft: false
images: []
menu:
  docs:
    parent: "cmd"
toc: false
url: /docs/cmd/config-keyring
---
The `sq config keyring` command group manages keyring entries directly.
[`sq add --store keyring`](/docs/cmd/add) and the
[`secrets.store`](/docs/config#secretsstore) option write entries automatically;
reach for these subcommands to rotate a credential, migrate inline passwords in
bulk, or inspect (or prune) what's already stored.

See [Secrets](/docs/secrets) for an overview of how `sq` handles secrets
and how the keyring scheme fits in.

{{< alert icon="🐥" >}}
You should rarely, if ever, need this command group directly. `sq config
keyring` is a thin, platform-independent wrapper over your OS keychain (macOS
Keychain, Windows Credential Manager, the Secret Service on Linux); `sq` reads
and writes these entries for you, and the secret handling is meant to stay
invisible. Reach in here only to inspect, rotate, or clean up entries by hand.

This keyring support is beta and may change in a future release.
{{< /alert >}}

## Commands

`sq config keyring` is a command group rather than a command itself: run
on its own, it just prints help. Use one of its subcommands:

| Command                                                         | What it does                                                   |
|-----------------------------------------------------------------|----------------------------------------------------------------|
| [`sq config keyring ls`](/docs/cmd/config-keyring-ls)           | List every entry, tagged referenced, orphan, or missing.       |
| [`sq config keyring prune`](/docs/cmd/config-keyring-prune)     | Delete orphaned entries (those no source references).          |
| [`sq config keyring create`](/docs/cmd/config-keyring-create)   | Create a new entry at `PATH`. Errors if `PATH` already exists. |
| [`sq config keyring update`](/docs/cmd/config-keyring-update)   | Rotate the value at an existing `PATH`.                        |
| [`sq config keyring get`](/docs/cmd/config-keyring-get)         | Check an entry exists; with `--reveal`, print its value.       |
| [`sq config keyring rm`](/docs/cmd/config-keyring-rm)           | Delete an entry. Does not touch sources that reference it.     |
| [`sq config keyring migrate`](/docs/cmd/config-keyring-migrate) | Move inline-password sources to the keyring in bulk.           |

`sq config keyring ls` reconciles the keyring against your config: it tags each
entry a source references as `referenced`, each entry no source uses as
`orphan`, and each reference whose entry is absent from the keyring as
`missing`. Use [`sq config keyring prune`](/docs/cmd/config-keyring-prune) to
delete the orphans.

`sq config keyring rm` deletes the keyring entry only; any remaining
`${keyring:PATH}` reference in `sq.yml` will fail to resolve on the next
connect. Run [`sq config keyring ls`](/docs/cmd/config-keyring-ls) first to
find references.

## Reference

{{< readfile file="config-keyring.help.txt" code="true" lang="text" >}}
