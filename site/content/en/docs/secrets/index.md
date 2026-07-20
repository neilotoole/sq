---
title: Secrets
description: How sq handles passwords, the OS keyring, and ${scheme:path} placeholders.
lead: ""
draft: false
images: []
weight: 1037
toc: true
url: /docs/secrets
---

`sq` treats credentials with two independent mechanisms: **redaction** keeps
secrets out of display output, and **placeholders** keep secrets out of the
[config](../config) file itself. The two stack rather than compete. This page
explains both, plus the `--reveal` and `--expand` flags that opt out.

## Overview

Redaction is the display default. Anywhere `sq` prints a source location,
URL-style passwords show as `xxxxx`:

```text
@sakila/pg  postgres  postgres://alice:xxxxx@db.acme.com/sakila
```

Placeholders are opt-in. Replace an inline credential with `${scheme:path}`
and `sq` fetches the real value at connect time:

```yaml
location: postgres://alice:${keyring:j2k7m3pxtz}@db.acme.com/sakila
```

The password isn't in `sq.yml`; `sq` reads it from the OS keyring at the
opaque ID `j2k7m3pxtz` whenever the source connects. Four schemes ship in
the box: [`keyring`](#keyring), [`env`](#env), [`file`](#file), and
[`op`](#op) for 1Password.

Two global flags opt out of the defaults:

- [`--reveal`](#redaction) prints the redacted secret instead of `xxxxx`.
- [`--expand`](#expanding-placeholders) resolves placeholders and prints the
  value they fetch.

Both can expose plaintext, but in different ways: `--reveal` shows what
`sq` already has loaded, while `--expand` pulls plaintext **out** of an
external resolver into the display.

[`sq config export`](/docs/cmd/config-export) is a backup tool, not a
display tool, and writes locations verbatim: placeholders are preserved
and inline plaintext is dumped as-is. Treat the exported file the same as
`sq.yml` itself (mode `0600` by default).

## Redaction

`sq` redacts URL-style passwords in the location of a source when that
location is printed. Secret-bearing query parameters, such as Postgres's
`?sslpassword=` or SQLite's `?_auth_pass=`, are masked the same way.
For a source

```yaml
- handle: "@sakila/pg"
  driver: postgres
  location: postgres://alice:hunter2@db.acme.com/sakila
```

`sq ls -v` displays the password as `xxxxx`:

```shell
$ sq ls -v
HANDLE      DRIVER    LOCATION
@sakila/pg  postgres  postgres://alice:xxxxx@db.acme.com/sakila
```

To see the underlying value, pass the global `--reveal` flag:

```shell
$ sq ls -v --reveal
HANDLE      DRIVER    LOCATION
@sakila/pg  postgres  postgres://alice:hunter2@db.acme.com/sakila
```

`--reveal` is a display flip on data that `sq` already has loaded. It applies
to every command that prints a source location, plus
[`sq config keyring get`](/docs/cmd/config-keyring-get), where it controls
whether the stored value is printed.

`--reveal` does **not** fetch placeholders. If a source's location is
`postgres://alice:${keyring:abc}@db/sakila`, `--reveal` prints the placeholder
itself, not the keyring value. Use [`sq config keyring get`](/docs/cmd/config-keyring-get)
to inspect a keyring value directly.

{{< alert icon="👉" >}}
The [`secrets.reveal`](/docs/config#secretsreveal) config option (default
`false`) controls the same behavior persistently:

```shell
# Turn off redaction for this user, persistently
$ sq config set secrets.reveal true
```

`--reveal` is the per-invocation form of the same control. Setting
`secrets.reveal: true` is equivalent to passing `--reveal` on every
command.
{{< /alert >}}

## Placeholders

A source's `location` may contain one or more `${scheme:path}` placeholders.
Each placeholder is resolved at connect time by the named scheme:

```yaml
location: postgres://alice:${keyring:j2k7m3pxtz}@db/sakila
location: postgres://alice:${env:DB_PROD_PASSWORD}@db/sakila
location: postgres://alice:${file:/run/secrets/db_prod_pw}@db/sakila
location: postgres://alice:${file:~/.sq/db_prod_pw}@db/sakila
```

A placeholder can fill any segment of the location, or the whole thing:

```yaml
# Whole conn string: the keyring entry holds the entire URL
location: ${keyring:j2k7m3pxtz}

# Composition: only the password is fetched
location: postgres://alice:${env:DB_PW}@db.acme.com/sakila
```

`sq` ships with four schemes: `keyring`, `env`, `file`, and `op`.

### Literal dollar signs

The `location` value is a template: write `$$` for a literal `$`. At connect
time, `sq` reduces each `$$` to `$` and resolves any `${scheme:path}`
placeholders, so the driver receives the literal form.

```yaml
# Driver receives: postgres://alice:pa$$word@db/sakila
location: postgres://alice:pa$$$$word@db/sakila

# Driver receives the literal text ${env:HOME}, not the env var value
location: postgres://alice:$${env:HOME}@db/sakila
```

A lone `$` that doesn't form a `${scheme:path}` placeholder (e.g. an inline
password like `p$ssw0rd`) is already literal and needs no escaping.

Escaping applies to location values you write yourself (the `sq add` location
argument, or hand-edited config). A password supplied via
[`sq add -p`](/docs/cmd/add) is a literal: `sq` escapes it for you before
splicing it into the stored location.

When upgrading a config created before `v0.54.0` (which had no placeholder
syntax), `sq` automatically escapes any source location that would otherwise
be reinterpreted, so existing sources connect exactly as before. The
pre-upgrade config is [backed up](/docs/config#upgrades) verbatim first.

### URL encoding

When a placeholder lands inside URL userinfo (the `user:password@host` part),
`sq` automatically percent-encodes the resolved value so that characters
URL reserves (`@`, `:`, `/`, `?`, `#`, `&`, `+`, `%`, etc.) round-trip
correctly. Store the raw, unencoded password in the resolver; do not
pre-encode it, or you'll end up with a double-encoded value at connect
time.

Whole-conn-string placement (`location: ${keyring:abc}`) skips userinfo
splicing entirely: the resolved value is used as the complete location
string, so the resolver is responsible for any escaping the driver
requires.

### Choosing a scheme

| Environment            | Recommended scheme    | Why                                                                               |
| ---------------------- | --------------------- | --------------------------------------------------------------------------------- |
| Dev laptop             | [`keyring`](#keyring) | Plaintext never lives on disk; OS handles the storage and prompting.              |
| CI runner              | [`env`](#env)         | CI systems already inject secrets as environment variables.                       |
| Container / Kubernetes | [`file`](#file)       | Secrets are typically mounted into the container as files (e.g. `/run/secrets/`). |
| Shared / team secrets  | [`op`](#op)           | 1Password is the team source of truth; `sq` reads it via the `op` CLI.            |

The schemes are not mutually exclusive: an `sq.yml` may use `keyring` for one
source, `env` for another, and inline plaintext for a third.

### `keyring`

An _OS keyring_ is the operating system's encrypted credential store, unlocked
by the user's login session and reachable by apps running as that user.
Storing a secret there keeps it off disk in plaintext while letting `sq`
fetch it at connect time. Every major OS ships one, under a different name:

- **macOS:** [Keychain][keychain]
- **Windows:** [Credential Manager][cred-mgr]
- **Linux:** [Secret Service][secret-svc] ([GNOME Keyring][gnome-keyring], [KWallet][kwallet], etc.)

`${keyring:<path>}` reads from whichever of these is active on the host
running `sq`.

[keychain]: https://support.apple.com/guide/keychain-access/welcome/mac
[cred-mgr]: https://learn.microsoft.com/en-us/windows/win32/secauthn/credential-manager
[secret-svc]: https://specifications.freedesktop.org/secret-service-spec/latest/
[gnome-keyring]: https://wiki.gnome.org/Projects/GnomeKeyring
[kwallet]: https://apps.kde.org/kwalletmanager5/

The typical pattern is [`sq add --store keyring DSN`](/docs/cmd/add), which
mints an opaque 10-character ID (e.g. `j2k7m3pxtz`), writes the full conn
string to the keyring at that ID, and stores a bare placeholder in YAML:

```yaml
- handle: "@sakila/pg"
  driver: postgres
  location: ${keyring:j2k7m3pxtz}
```

The driver type stays in YAML (so `sq` can pick the right driver before
resolving the keyring), but everything else (username, host, port,
database, password, query parameters) lives in the keyring entry. One
keyring entry, one source, no composition. The same layout is produced
by [`sq config keyring migrate`](/docs/cmd/config-keyring-migrate).

For shared credentials or password-only composition (e.g.
`postgres://alice:${keyring:my_db_pw}@db/sakila`), hand-create an entry
with a meaningful path:

```shell
$ sq config keyring create my_db_pw -p < secret.txt
```

then reference it by hand in `sq.yml`:

```yaml
location: postgres://alice:${keyring:my_db_pw}@db/sakila
```

For management operations (rotate, migrate inline passwords, list
references), see the [`sq config keyring`](/docs/cmd/config-keyring)
command group.

### `env`

`${env:<VAR>}` reads the named environment variable at connect time.

```yaml
location: postgres://alice:${env:DB_PROD_PASSWORD}@db.acme.com/sakila
location: ${env:DB_PROD_CONN_STR}
```

If the variable is unset, the source fails to connect with an error naming
the variable. `env` is read-only: `sq` consults the value but never sets it.

### `file`

`${file:<path>}` reads file contents at connect time, trimming a single
trailing newline. Path forms accepted:

- Absolute path: `${file:/run/secrets/db_pw}`
- Tilde home: `${file:~/.sq/db_pw}` (current user's home)
- Empty-authority file URI: `${file:///run/secrets/db_pw}`

Relative paths and remote `file://host/path` forms are rejected.

`file` is read-only: `sq` consults the file but never writes to it.

### `op`

`${op://<vault>/<item>/[<section>/]<field>}` reads a value from 1Password
via the [`op` CLI](https://developer.1password.com/docs/cli/) using
1Password's
[secret-reference syntax](https://developer.1password.com/docs/cli/secret-reference-syntax/)
verbatim. The user must already be signed in: biometric, `op signin`, or a
service-account token in `OP_SERVICE_ACCOUNT_TOKEN`.

```yaml
- handle: "@sakila"
  driver: postgres
  location: ${op://Private/sakila/dsn} # whole DSN
- handle: "@sakila/composed"
  driver: postgres
  location: postgres://alice:${op://Private/sakila/password}@db/sakila
```

Notes:

- Requires `op` v2 or newer on `PATH`.
- The URI body is passed to `op read` verbatim; `sq` does not parse vault,
  item, section, or field names.
- Within one `sq` invocation, the same `${op://...}` placeholder is
  resolved at most once (per-process cache), so biometric prompts and
  network round-trips do not multiply.
- `op` is read-only: `--store op` is not a thing. To put a secret into
  1Password, use the `op` CLI directly.
- "Item not found" surfaces as the standard `secret not found` message;
  other failures (not signed in, network) surface `op`'s own stderr.

**`sq add` shortcut.** [`sq add`](/docs/cmd/add) accepts the bare
`op://<vault>/<item>/<field>` form that 1Password's "Copy Secret Reference"
puts on the clipboard, as sugar for the full `${op://...}` placeholder:

```shell
# Both forms are equivalent; the bare form is stored as ${op://...} in sq.yml.
$ sq add 'op://Private/sakila/dsn'
$ sq add '${op://Private/sakila/dsn}'
```

If the resolved value is a bare credential rather than a full DSN (1Password's
default field is named `password`, so people commonly drop a DSN into it
verbatim), `sq add` fails with an error and adds no source. The error names
the placeholder and points at the three recovery paths:

1. Store a full DSN at that field, e.g. `postgres://alice:hunter2@db/sakila`.
2. Use composition: `sq add 'postgres://alice:${op://Private/sakila/password}@db/sakila'`.
3. Pass `--driver <type>` to skip driver inference entirely.

<a id="verifying-a-source"></a>

### Verifying placeholders resolve

Once a source uses placeholders, the obvious question is "did sq actually
resolve them correctly?" The answer is [`sq ping`](/docs/cmd/ping):

```shell
$ sq ping @sakila/pg
@sakila/pg  130ms  pong
```

`sq ping` opens a real connection, which forces full resolution of every
placeholder in the source's location. If a keyring entry is missing, an
env var is unset, or a file path doesn't exist, `sq ping` returns the
underlying error with the failing source's handle. There is no separate
`verify` or `test` subcommand: `sq ping` is the end-to-end check.

<a id="substitution"></a>

## Expanding placeholders

`--expand` resolves `${scheme:path}` placeholders against the configured
resolvers (`keyring`, `env`, `file`, or `op` for 1Password), and
substitutes the resolved value into whatever location string `sq` is about
to display. It is a persistent root flag: every subcommand accepts it,
and the commands that print a source location act on it.

### On display commands

[`sq src`](/docs/cmd/src), [`sq ls`](/docs/cmd/ls),
[`sq inspect`](/docs/cmd/inspect), [`sq add`](/docs/cmd/add)'s post-add
echo, [`sq mv`](/docs/cmd/mv)'s post-mv echo, and
[`sq ping`](/docs/cmd/ping) in JSON or YAML output each show a source
location. `--expand` decides whether that location is the verbatim
placeholder or the resolved value. `--reveal` is the orthogonal axis: it
flips the redaction filter on whatever string ends up being displayed.

{{< alert icon="👉" >}}
`sq ping`'s text and CSV output do not include a Location column, so
`--expand` has no visible effect there. Use `--json` or `--yaml` to see
the per-source location.
{{< /alert >}}

For a source whose YAML location is `${keyring:abc}` and whose keyring entry
holds `postgres://alice:hunter2@db.acme.com/sakila`:

| Flags               | Displayed location                            |
| ------------------- | --------------------------------------------- |
| (none)              | `${keyring:abc}`                              |
| `--reveal`          | `${keyring:abc}`                              |
| `--expand`          | `postgres://alice:xxxxx@db.acme.com/sakila`   |
| `--expand --reveal` | `postgres://alice:hunter2@db.acme.com/sakila` |

The display-expansion step is lenient: a per-source resolver failure
(missing keyring entry, unset env var, unreadable file) leaves that
source's placeholder verbatim and the listing continues. This applies to
the display step only; commands that must resolve to connect (e.g.
`sq inspect`, `sq ping`) still fail at connect time when a missing secret
prevents the connection. The lenient display fallback is deliberate:
`--expand` on a listing command is a diagnostic, and aborting the whole
listing because one source's resolver is offline would hide every other
source's state.

### On `sq config export`

[`sq config export`](/docs/cmd/config-export) dumps the live config to YAML
for backups. By default the export is a faithful copy of `sq.yml`:
`${scheme:path}` placeholders are written verbatim, and inline credentials
(plaintext URLs) are dumped exactly as they appear in the file.

With `--expand`, every `${scheme:path}` placeholder is resolved and the
resolved value is spliced inline into the exported location. The output is
a self-contained snapshot suitable for moving between machines, at the cost
of writing every referenced secret in plaintext (which is exactly the point
of `--expand`).

Because the exported file is itself a config, its locations are templates:
any `$` in a resolved value is written as
[`$$`](#literal-dollar-signs), so that the export connects byte-identically
when used as `sq.yml`. To see resolved values in raw literal form, use the
display commands, e.g. `sq ls -v --expand`.

```yaml
# Live config: location uses a keyring placeholder
- handle: "@sakila/pg"
  driver: postgres
  location: postgres://alice:${keyring:j2k7m3pxtz}@db.acme.com/sakila
```

```shell
# Default: placeholder preserved
$ sq config export | grep '@sakila/pg' -A1
- handle: '@sakila/pg'
  location: postgres://alice:${keyring:j2k7m3pxtz}@db.acme.com/sakila

# --expand: keyring value fetched and inlined
$ sq config export --expand | grep '@sakila/pg' -A1
- handle: '@sakila/pg'
  location: postgres://alice:hunter2@db.acme.com/sakila
```

When `--output` is used, the exported file is created with mode `0600`,
since it may contain credentials regardless of whether `--expand` was set.

`sq config export --expand` keeps strict-abort semantics on resolver
failure (unlike the lenient display commands). An export is a snapshot for
transfer, and a half-resolved snapshot is the wrong artifact.

### Reveal vs expand

Both flags can produce plaintext secrets, but they're not interchangeable.

| Concern                        | `--reveal`                                           | `--expand`                                                                                    |
| ------------------------------ | ---------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Purpose                        | Show secret data already loaded                      | Fetch placeholder values and splice them in                                                   |
| Source of the printed value    | The current in-memory config                         | External resolvers: OS keyring, env, file                                                     |
| Applies to                     | Any command that prints a location or keyring value  | Any command that prints a source location; also [`sq config export`](/docs/cmd/config-export) |
| What you see for a placeholder | The placeholder text (e.g. `${keyring:abc}`)         | The resolved value (e.g. `hunter2`)                                                           |
| Failure mode                   | Cannot fail; it is just a display flip               | Lenient per source on display commands; strict-abort on `sq config export`                    |
| Risk                           | Reveals plaintext already in `sq.yml` or the keyring | Pulls plaintext **out** of resolvers into terminal, log, or exported-file output              |

Use `--reveal` to read configuration; use `--expand` to take a portable
backup or to diagnose what a source actually resolves to. Don't reach for
`--expand` when `--reveal` is what you want.

## Managing keyring entries

Most users never touch the keyring directly: [`sq add --store keyring`](/docs/cmd/add)
and the [`secrets.store`](/docs/config#secretsstore) option write entries
automatically. For management operations (rotate, migrate inline passwords,
list references), see the [`sq config keyring`](/docs/cmd/config-keyring)
command group.

## Threat model

The keyring scheme is a useful step up from plaintext in `sq.yml`, but it
is not a sandbox. The threats it does, and does not, address:

**Protects against:**

- Accidentally committing `sq.yml` to git with plaintext credentials.
- Plaintext credentials sitting unprotected on disk in `~/.config/sq/sq.yml`.
- Casual inspection of `sq.yml` by a sibling process or backup tool that
  doesn't also have the user's keyring unlocked.

**Does not protect against:**

- A compromised user account: anything running as the user can read the
  user's keyring once it is unlocked.
- Malware in the user's session: it sees the same keyring `sq` does.
- A user who runs `sq config keyring get --reveal`,
  `sq config export --expand`, or any display command with
  `--reveal --expand` and pipes the output somewhere unsafe (terminal
  buffer, shell recording, CI log, screen share).
- Memory inspection: while a source is connected, the resolved conn string
  exists in `sq`'s process memory.

For the higher bar (per-application secrets, hardware-backed keys, rotated
short-lived credentials), an external secret manager is the right answer:
referenced via the `env` or `file` scheme from a CI / container runtime,
or via the `op` scheme on a dev laptop with 1Password. The `keyring` scheme
targets the dev-laptop case when no external manager is in use.

## Related

- [`sq config keyring`](/docs/cmd/config-keyring): keyring command group.
- [`sq add`](/docs/cmd/add): the `--store inline|keyring` flag.
- [`sq config export`](/docs/cmd/config-export) (and other display
  commands): the `--expand` flag.
- [`secrets.store`](/docs/config#secretsstore): default storage backend config.
- [`secrets.reveal`](/docs/config#secretsreveal): persistent disclosure control config.
