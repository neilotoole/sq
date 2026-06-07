---
title: "sq config keyring"
description: "Manage source secrets stored in the OS keyring"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2038
toc: false
url: /docs/cmd/config-keyring
---
The `sq config keyring` command group manages keyring entries directly.
Most users never need it: [`sq add --store keyring`](/docs/cmd/add) and the
[`secrets.store`](/docs/config#secretsstore) option write entries
automatically. Reach for these subcommands when you need to rotate a
credential, migrate inline passwords in bulk, or inspect what's already
stored.

See [Secrets](/docs/secrets) for an overview of how `sq` handles secrets
and how the keyring scheme fits in.

## Commands

| Command                                                         | What it does                                                   |
|-----------------------------------------------------------------|----------------------------------------------------------------|
| [`sq config keyring ls`](/docs/cmd/config-keyring-ls)           | List `${keyring:<path>}` references found in source locations. |
| [`sq config keyring create`](/docs/cmd/config-keyring-create)   | Create a new entry at `PATH`. Errors if `PATH` already exists. |
| [`sq config keyring update`](/docs/cmd/config-keyring-update)   | Rotate the value at an existing `PATH`.                        |
| [`sq config keyring get`](/docs/cmd/config-keyring-get)         | Check an entry exists; with `--reveal`, print its value.       |
| [`sq config keyring rm`](/docs/cmd/config-keyring-rm)           | Delete an entry. Does not touch sources that reference it.     |
| [`sq config keyring migrate`](/docs/cmd/config-keyring-migrate) | Move inline-password sources to the keyring in bulk.           |

`sq config keyring ls` walks the YAML config; it lists only `keyring`
placeholders that some source references. Orphan entries (entries in the
keyring that no source uses) are not surfaced. Enumerating them requires
keyring-wide iteration, which is deferred to a future release.

`sq config keyring rm` deletes the keyring entry only; any remaining
`${keyring:PATH}` reference in `sq.yml` will fail to resolve on the next
connect. Run [`sq config keyring ls`](/docs/cmd/config-keyring-ls) first to
find references.

## Reference

{{< readfile file="config-keyring.help.txt" code="true" lang="text" >}}
