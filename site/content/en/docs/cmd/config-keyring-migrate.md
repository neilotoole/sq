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
shown in the output (skips are hidden unless you pass `-v`):

- File or document sources with no embedded credentials, such as CSV, SQLite,
  or Excel: `no credentials to migrate`.
- A connection URL that carries no password: `no password to migrate`.
- A location that already uses a `${...}` placeholder, so re-runs are
  idempotent: `already has a placeholder`.
- A malformed location or placeholder, surfaced rather than stamped into the
  keyring: `malformed location: <reason>` or
  `malformed placeholder syntax: <reason>`.

## Preview, then confirm

Every run targets either a single source by handle or the whole collection with
`--all`. Two flags control how it behaves, and both work with either target:

- `--dry-run` previews the plan and changes nothing (mints no IDs, writes
  nothing).
- `--yes` skips the confirmation prompt, for non-interactive use.

### `--dry-run`: preview without changes

```shell
# Preview a single source
$ sq config keyring migrate @sakila --dry-run

# Preview the whole collection
$ sq config keyring migrate --all --dry-run
```

The output is an aligned table listing only the sources that will migrate.
Sources with nothing to move (file paths, no password, already migrated) are
omitted to keep the list focused; pass `-v` to show them with their skip
reason. If nothing is eligible, migrate says so and makes no changes:

```text
HANDLE            STATUS   DETAIL
@sakila/pg        migrate  ${keyring:<new-id>}
@sakila/local/pg  migrate  ${keyring:<new-id>}
```

### The confirmation prompt and `--yes`

A real run prints the same plan, then prompts before changing anything:

```text
Proceed with migration? [y/N]
```

Answer `y`/`yes` to proceed, or `n`/`no` (or just press Enter, the `[y/N]`
default) to cancel. Cancelling touches nothing (no keyring writes, no `sq.yml`
change) and exits non-zero, so a script can tell a declined migration from a
completed one. Only `y`/`yes`/`n`/`no` (case-insensitive) are accepted; any
other answer, or no answer at all, is an error and exits non-zero without
retrying. Pass `--yes` to skip the prompt, which is what you want in scripts:

```shell
# Migrate one source without prompting
$ sq config keyring migrate @sakila --yes

# Migrate everything without prompting
$ sq config keyring migrate --all --yes
```

JSON output (`--json`) is non-interactive: it skips both the preview and the
prompt and applies directly, so `--yes` is implied.

## How changes are applied

`migrate` runs under a config lock (so it won't race another `sq` process) and
is atomic: a `--all` run either migrates every eligible source or changes
nothing. It writes each source's keyring entry and rewrites its `location` in
memory, then saves `sq.yml` once for the whole batch. If any step fails, the
run is rolled back: every keyring entry it wrote is deleted, every `location`
is restored, `sq.yml` is left untouched, and the command exits non-zero. A
failed migration never leaves your config half-converted.

## Reference

{{< readfile file="config-keyring-migrate.help.txt" code="true" lang="text" >}}
