---
title: "sq config keyring migrate"
description: "Migrate inline-credential sources to the keyring"
group: config
draft: false
images: []
menu:
  docs:
    parent: "cmd"
toc: false
url: /docs/cmd/config-keyring-migrate
---
Part of the [`sq config keyring`](/docs/cmd/config-keyring) command group;
see [Secrets](/docs/secrets) for the broader picture.

`migrate` is the one keyring command a typical user is likely to run. It moves
plaintext credentials out of `sq.yml` and into the OS keyring in bulk. For each
source it writes the source's full Location (the connection string, credentials
and all) to a fresh opaque keyring entry, then rewrites that source's
`location` in `sq.yml` to a bare `${keyring:<id>}` placeholder. The driver type
stays in the `driver:` field; the keyring entry holds the whole DSN.

## Back up your config first

`migrate` rewrites `sq.yml` in place and keeps no backup of its own. Before a
real run, export your current config so you can restore it if anything looks
wrong:

```shell
$ sq config export -o sq.bak.yml
```

[`sq config export`](/docs/cmd/config-export) copies the config verbatim,
including the inline credentials `migrate` is about to move, so the backup is a
complete pre-migration snapshot. If you want a self-contained copy with every
secret resolved in-line (for example, before moving to a machine whose keyring
won't hold these entries), add `--expand`. Note that this writes every secret
in plaintext:

```shell
$ sq config export --expand -o sq.bak.yml
```

## What gets migrated

Name a single source by handle, or pass `--all` for the whole collection:

```shell
$ sq config keyring migrate @sakila
$ sq config keyring migrate --all
```

Sources with nothing to move are skipped automatically, each with its reason
shown in the output:

- Non-URL locations (file paths, SQLite, Excel, and so on).
- URLs with no password component.
- Locations that already contain a `${...}` placeholder, so re-runs are
  idempotent.
- Locations with malformed placeholder syntax, surfaced rather than stamped
  into the keyring.

## Preview, then confirm

Preview first with `--dry-run`. It mints no IDs and writes nothing:

```shell
$ sq config keyring migrate --all --dry-run
```

A real run prints the same plan and then prompts before changing anything:

```text
Proceed with migration? [y/N]
```

Anything other than `y` or `yes` (including just pressing Enter) aborts without
touching the keyring or `sq.yml`. Pass `--yes` to skip the prompt in scripts.
JSON output (`--json`) is non-interactive: it skips the preview and prompt and
applies directly.

## How changes are applied

Sources are migrated one at a time: mint an ID, write the keyring entry,
rewrite the source's `location`, and save `sq.yml`. If saving fails for a
source, `migrate` rolls that source back, restoring its original `location` and
deleting the keyring entry it just wrote, then reports the source as failed and
continues with the rest. A run that hits a failure still reports which sources
succeeded, and exits non-zero.

## Reference

{{< readfile file="config-keyring-migrate.help.txt" code="true" lang="text" >}}
