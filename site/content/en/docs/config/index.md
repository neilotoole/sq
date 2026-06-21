---
title: Config
description: Config
lead: ''
draft: false
images:
  - sq_config_ls.png
weight: 1038
toc: true
url: /docs/config
---
`sq` aims to work out of the box with sane defaults, but allows you to configure most
everything. `sq`'s total configuration state consists of a collection of
data [sources](/docs/source) and [groups](/docs/source/#groups), and a plethora
of configuration options. That's what this section is about. There are two levels
of options:

- Base config, consisting of many options. Each option is a key-value pair,
  e.g. `format=json`, or `conn.max-open=50`
- Source-specific config. Each source can have its own value for, say, `conn.max-open`.
  If an option is not explicitly set on a source, the source inherits that
  option value from base config.

## Commands

`sq` provides commands to [locate](#location), [list](#ls), [`get`](#get),
[`set`](#set), and [`edit`](#edit) config. The config commands provide extensive
shell-completion, so feel free to hit `TAB` while
entering a command, and `sq` will guide you.

### `location`

`sq` stores its main config in a `sq.yml` file in its config dir. You don't usually
need to edit the config directly: `sq` provides several mechanisms for managing config.

The location of `sq's` config dir is OS-dependent. On macOS, it's here:

```shell
$ sq config location
/Users/neilotoole/.config/sq
```

You can specify an alternate location by setting envar `SQ_CONFIG`:

```shell
$ export SQ_CONFIG=/tmp/sq
$ sq config location
/tmp/sq
```

You can also specify the config dir via the `--config` flag:

```shell
$ sq --config=/tmp/sq2 config location
/tmp/sq2
```

### `ls`

Use `sq config ls` to list the options that have been set.

```shell
# List config options
$ sq config ls
```

![sq config ls](sq_config_ls.png)

Well, there's more. A lot more. Use `sq config ls -v` to also see unset options,
along with their default values.

![sq config ls -v](sq_config_ls_v.png)

Note in the image above that some options don't have a value. That is to say,
the option is _unset_. When _unset_, an option takes on its default value.

{{< alert icon="👉" >}}
If you want a wall of text, try `sq config ls -yv` (the `-y` flag is for
`--yaml` output). That's the maximum amount of detail available.
{{< /alert >}}

As well as listing base config, you can view config options for a source.

```shell
$ sq config ls --src @actor_csv -v
```

![sq config ls --src @actor_csv -v](sq_config_ls_src_v.png)

### `get`

`sq config get` is like the single-friend counterpart of `sq ls`. It gets the
value of a single option.

```shell
# Get base value of "format" option
$ sq config get format
text

# Get the "conn.max-open" option value for a particular source
$ sq config get --src @actor_csv conn.max-open
10
```

### `set`

Use `sq config set` or `sq config set --src` to set an option value.

```shell
# Set base option value
$ sq config set format json

# Set source-specific option value
$ sq config set --src @sakila_pg conn.max-open 20
```

To get help for a specific option, execute `sq config set OPTION --help`.

![sq config set --help](sq_config_set_help.png)

### `edit`

In the spirit of
[`kubectl edit`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#edit),
you can edit base config or source-specific config via the
default editor, as defined in envar `$EDITOR` or `$SQ_EDITOR`.

```shell
# Edit base config
$ sq config edit

# Edit config for source
$ sq config edit @sakila

# Use a different editor
$ SQ_EDITOR=nano sq config edit
```

![sq config edit src](sq_config_edit_src.png)

If you add the `-v` flag (`sq config edit -v`), the editor will show
additional help for the options.

![sq config edit v](sq_config_edit_v.png)

## Upgrades

When a new `sq` version changes the config schema, `sq` migrates `sq.yml`
automatically on first run. Before migrating, `sq` writes a verbatim backup of
the config to the same dir, named for the version being upgraded from, e.g.
`sq.v0.53.0.bak.yml`. `sq` never reads, modifies, or deletes the backup: it
exists solely so you can restore the previous config if needed.

The backup is created with user-only file permissions, but it preserves the
old config exactly, including any inline credentials. If you later move your
credentials out of the config (e.g. via
[`sq config keyring migrate`](/docs/cmd/config-keyring-migrate)), remember
that the backup still holds the plaintext values; delete it when you no
longer need it.

## Logging

By default, logging is turned off. If you need to submit a `sq`
[bug report](https://github.com/neilotoole/sq/issues), you'll likely
want to include the `sq` log file.

```shell
# Enable logging
$ sq config set log true

# Default log level is DEBUG... you can change it if you want.
# But leave it on DEBUG if you're sending bug reports.
$ sq config set log.level WARN

# The default log format is "text", a human-friendly format. You can
# also change it to "json" if you prefer.
$ sq config set log.format json

# You can also change the log file location.
$ sq config set log.file /tmp/sq.log

# Note that the default log file location is OS-dependent.
$ sq config get log.file -v
KEY       VALUE  DEFAULT
log.file         /Users/neilotoole/Library/Logs/sq/sq.log

# To output just the log file path:
$ sq config get log.file -jv | jq -r .value
/Users/neilotoole/Library/Logs/sq/sq.log
```

{{< alert icon="🤬️" >}}
If there's a problem with `sq`'s bootstrap mechanism (e.g. corrupt config file),
and logs aren't being generated, you can use envars to force logging,
overriding the config file. For example:

```shell
export SQ_LOG=true; export SQ_LOG_LEVEL=DEBUG; export SQ_LOG_FORMAT=text; export SQ_LOG_FILE=./sq.log
```

{{< /alert >}}

## Options

Below, all available options are listed. Use `sq config set OPTION` to
modify the option value.

Some config options apply only to base config. For example, `format=json` applies
to the `sq` CLI itself, and not to a particular source such as `@sakila`. However,
some options can apply to a source, and also have a base value. For example,
`conn.max-open` controls the maximum number of connections that `sq` will open
to a database. This option can be set for base config, but can also be set for
an individual source, overriding the base config.

## CLI

### `log`

{{< readfile file="../cmd/options/log.help.txt" code="true" lang="text" >}}

### `log.file`

{{< readfile file="../cmd/options/log.file.help.txt" code="true" lang="text" >}}

### `log.format`

{{< readfile file="../cmd/options/log.format.help.txt" code="true" lang="text" >}}

### `log.level`

{{< readfile file="../cmd/options/log.level.help.txt" code="true" lang="text" >}}

### `error.format`

{{< readfile file="../cmd/options/error.format.help.txt" code="true" lang="text" >}}

### `error.stack`

{{< readfile file="../cmd/options/error.stack.help.txt" code="true" lang="text" >}}

### `error.format.text.verbose`

{{< readfile file="../cmd/options/error.format.text.verbose.help.txt" code="true" lang="text" >}}

### `ping.timeout`

{{< readfile file="../cmd/options/ping.timeout.help.txt" code="true" lang="text" >}}

### `http.request.timeout`

{{< readfile file="../cmd/options/http.request.timeout.help.txt" code="true" lang="text" >}}

### `http.response.timeout`

{{< readfile file="../cmd/options/http.response.timeout.help.txt" code="true" lang="text" >}}

### `https.insecure-skip-verify`

{{< readfile file="../cmd/options/https.insecure-skip-verify.help.txt" code="true" lang="text" >}}

### `download.cache`

{{< readfile file="../cmd/options/download.cache.help.txt" code="true" lang="text" >}}

### `download.refresh.ok-on-err`

{{< readfile file="../cmd/options/download.refresh.ok-on-err.help.txt" code="true" lang="text" >}}

### `progress`

{{< readfile file="../cmd/options/progress.help.txt" code="true" lang="text" >}}

### `progress.delay`

{{< readfile file="../cmd/options/progress.delay.help.txt" code="true" lang="text" >}}

### `progress.max-bars`

{{< readfile file="../cmd/options/progress.max-bars.help.txt" code="true" lang="text" >}}

### `shell-completion.timeout`

{{< readfile file="../cmd/options/shell-completion.timeout.help.txt" code="true" lang="text" >}}

### `shell-completion.group-filter`

<!-- markdownlint-disable-next-line MD013 -->
{{< readfile file="../cmd/options/shell-completion.group-filter.help.txt" code="true" lang="text" >}}

### `shell-completion.log`

{{< readfile file="../cmd/options/shell-completion.log.help.txt" code="true" lang="text" >}}

### `config.lock.timeout`

{{< readfile file="../cmd/options/config.lock.timeout.help.txt" code="true" lang="text" >}}

<a id="formatting"></a>

## Output

### `compact`

{{< readfile file="../cmd/options/compact.help.txt" code="true" lang="text" >}}

### `format`

{{< readfile file="../cmd/options/format.help.txt" code="true" lang="text" >}}

### `format.datetime`

{{< readfile file="../cmd/options/format.datetime.help.txt" code="true" lang="text" >}}

### `format.datetime.number`

{{< readfile file="../cmd/options/format.datetime.number.help.txt" code="true" lang="text" >}}

### `format.date`

{{< readfile file="../cmd/options/format.date.help.txt" code="true" lang="text" >}}

### `format.date.number`

{{< readfile file="../cmd/options/format.date.number.help.txt" code="true" lang="text" >}}

### `format.time`

{{< readfile file="../cmd/options/format.time.help.txt" code="true" lang="text" >}}

### `format.time.number`

{{< readfile file="../cmd/options/format.time.number.help.txt" code="true" lang="text" >}}

### `format.excel.datetime`

{{< readfile file="../cmd/options/format.excel.datetime.help.txt" code="true" lang="text" >}}

<!-- markdownlint-disable-next-line MD013 -->
See also: [Excel date/time format reference](https://support.microsoft.com/en-gb/office/format-numbers-as-dates-or-times-418bd3fe-0577-47c8-8caa-b4d30c528309#bm2)

### `format.excel.date`

{{< readfile file="../cmd/options/format.excel.date.help.txt" code="true" lang="text" >}}

<!-- markdownlint-disable-next-line MD013 -->
See also: [Excel date/time format reference](https://support.microsoft.com/en-gb/office/format-numbers-as-dates-or-times-418bd3fe-0577-47c8-8caa-b4d30c528309#bm2)

### `format.excel.time`

{{< readfile file="../cmd/options/format.excel.time.help.txt" code="true" lang="text" >}}

<!-- markdownlint-disable-next-line MD013 -->
See also: [Excel date/time format reference](https://support.microsoft.com/en-gb/office/format-numbers-as-dates-or-times-418bd3fe-0577-47c8-8caa-b4d30c528309#bm2)

### `format.html.embed-assets`

{{< readfile file="../cmd/options/format.html.embed-assets.help.txt" code="true" lang="text" >}}

This option applies to [`sq inspect`](/docs/inspect)'s HTML output. As with
other options, it can be set per invocation via the matching
`--format.html.embed-assets` flag.

### `header`

{{< readfile file="../cmd/options/header.help.txt" code="true" lang="text" >}}

### `monochrome`

{{< readfile file="../cmd/options/monochrome.help.txt" code="true" lang="text" >}}

See [Output modifiers: monochrome and environment variables](/docs/output#no_color-and-force_color)
for how `--monochrome` interacts with `NO_COLOR`, `FORCE_COLOR`, and terminal
detection.

### `verbose`

{{< readfile file="../cmd/options/verbose.help.txt" code="true" lang="text" >}}

### `secrets.reveal`

Controls whether sensitive fields (passwords in source locations, stored
keyring values) are printed verbatim or redacted in output. Default is
`false`: secrets are redacted.

```shell
# Default behavior: password is redacted.
$ sq src -v
@sakila/pg12  postgres  postgres://sakila:xxxxx@192.168.50.132/sakila

# Set secrets.reveal to true.
$ sq config set secrets.reveal true

# Now the password is visible.
$ sq src -v
@sakila/pg12  postgres  postgres://sakila:p_ssW0rd@192.168.50.132/sakila
```

`secrets.reveal` is the persistent form of the global
[`--reveal`](/docs/secrets#redaction) flag. The earlier `--no-redact`
flag still works but is deprecated; use `--reveal` in new scripts.

See [Secrets](/docs/secrets) for the full picture, including how `--reveal`
relates to [`sq config export --expand`](/docs/cmd/config-export) and the
placeholder system.

{{< alert icon="🐥" >}}
**Renamed in v0.54.0.** The earlier `redact` option (default `true`) was
renamed to `secrets.reveal` (default `false`) for polarity-consistency with
the `--reveal` flag. Existing configs migrate automatically on first run.
Scripts that call `sq config get redact` or `sq config set redact ...`
need updating to the new key.
{{< /alert >}}

{{< readfile file="../cmd/options/secrets.reveal.help.txt" code="true" lang="text" >}}

### `secrets.store`

Default storage backend used by [`sq add`](/docs/cmd/add) when the source URL
carries a password: `inline` (write the URL verbatim into `sq.yml`, the
historical default) or `keyring` (write the full conn string to the OS keyring at a
fresh opaque ID and store a bare `${keyring:<id>}` placeholder in YAML).

```shell
# Make keyring the default for new sources
$ sq config set secrets.store keyring
```

The `--store` flag on `sq add` overrides this option per invocation. See
[Secrets](/docs/secrets#keyring) for the keyring scheme and the
threat model.

{{< readfile file="../cmd/options/secrets.store.help.txt" code="true" lang="text" >}}

### `result.column.rename`

{{< readfile file="../cmd/options/result.column.rename.help.txt" code="true" lang="text" >}}

The `result.column.rename` option is rather arcane: it allows you to change
the way `sq` de-duplicates column names. By default, a result set containing
duplicate column names is renamed like this:

```SQL
-- Columns returned from DB...
actor_id, first_name, last_name, last_update, actor_id, film_id, last_update

-- are renamed to
actor_id, first_name, last_name, last_update, actor_id_1, film_id, last_update_1
```

Thus, the second `actor_id` column becomes `actor_id_1`. Let's say you instead
wanted the column to be renamed to `actor_id:1`. Change the template value to
use `:` instead of `_`.

```shell
$ sq config set result.column.rename '{{.Name}}{{with .Recurrence}}:{{.}}{{end}}'
```

The option value must be a valid [Go text template](https://pkg.go.dev/text/template).
In addition to the standard Go functions, the [sprig](https://masterminds.github.io/sprig/)
functions are available. Here's an example of a template using the sprig `upper` function to
rename each column to uppercase.

```text
{{.Name | upper}}{{with .Recurrence}}:{{.}}{{end}}
```

The `.Alpha` template field maps the column index to `A, B ... Y, Z, AA, AB...`,
similar to how Microsoft Excel names columns. To use this style:

```shell
$ sq config set result.column.rename '{{.Alpha}}'
$ sq .actor
```

{{< alert icon="👉" >}}
Note that [`ingest.column.rename`](#ingestcolumnrename) and
[`result.column.rename`](#resultcolumnrename) are distinct options.
The _ingest_ option is applied to ingest data (e.g. a CSV file) column names
before the data is sent to the database (pre-processing).
The _result_ option, by contrast, is applied
to result set column names after the data is returned from the database (post-processing).
It is possible (and normal) to use both options.
{{< /alert >}}

### `diff.data.format`

{{< readfile file="../cmd/options/diff.data.format.help.txt" code="true" lang="text" >}}

### `diff.lines`

Configures the number of context lines that [`sq diff`](/docs/diff) shows before and after
a difference. You can use the `--unified` (`-U`) flag, e.g.:

```shell
$ sq diff @prod/sales.payments @staging/sales.payments -U4
```

{{< readfile file="../cmd/options/diff.lines.help.txt" code="true" lang="text" >}}

### `diff.stop`

Configures the default stop-after value for [`sq diff`](/docs/diff).
You can use the `--stop` (`-n`) flag, e.g.:

```shell
$ sq diff @prod/sales.payments @staging/sales.payments -n10
```

Note that `diff.stop` only applies to table row data diffs, not to metadata diffs.

{{< readfile file="../cmd/options/diff.stop.help.txt" code="true" lang="text" >}}

### `diff.max-hunk-size`

{{< readfile file="../cmd/options/diff.max-hunk-size.help.txt" code="true" lang="text" >}}

## Tuning

### `conn.max-idle`

{{< readfile file="../cmd/options/conn.max-idle.help.txt" code="true" lang="text" >}}

### `conn.max-idle-time`

{{< readfile file="../cmd/options/conn.max-idle-time.help.txt" code="true" lang="text" >}}

### `conn.max-lifetime`

{{< readfile file="../cmd/options/conn.max-lifetime.help.txt" code="true" lang="text" >}}

### `conn.max-open`

{{< readfile file="../cmd/options/conn.max-open.help.txt" code="true" lang="text" >}}

### `conn.open-timeout`

{{< readfile file="../cmd/options/conn.open-timeout.help.txt" code="true" lang="text" >}}

### `retry.max-interval`

{{< readfile file="../cmd/options/retry.max-interval.help.txt" code="true" lang="text" >}}

### `tuning.errgroup-limit`

{{< readfile file="../cmd/options/tuning.errgroup-limit.help.txt" code="true" lang="text" >}}

### `tuning.output-flush-threshold`

<!-- markdownlint-disable-next-line MD013 -->
{{< readfile file="../cmd/options/tuning.output-flush-threshold.help.txt" code="true" lang="text" >}}

### `tuning.record-buffer`

{{< readfile file="../cmd/options/tuning.record-buffer.help.txt" code="true" lang="text" >}}

### `tuning.buffer-spill-limit`

{{< readfile file="../cmd/options/tuning.buffer-spill-limit.help.txt" code="true" lang="text" >}}

### `tuning.scan-buffer-limit`

{{< readfile file="../cmd/options/tuning.scan-buffer-limit.help.txt" code="true" lang="text" >}}

## Ingest

### `ingest.cache`

Enable or disable the ingest cache. You can also use the
[`sq cache enable`](/docs/cmd/cache-enable) and [`sq cache disable`](/docs/cmd/cache-disable)
commands.

{{< readfile file="../cmd/options/ingest.cache.help.txt" code="true" lang="text" >}}

### `cache.lock.timeout`

{{< readfile file="../cmd/options/cache.lock.timeout.help.txt" code="true" lang="text" >}}

### `ingest.column.rename`

{{< readfile file="../cmd/options/ingest.column.rename.help.txt" code="true" lang="text" >}}

{{< alert icon="👉" >}}
Note that [`ingest.column.rename`](#ingestcolumnrename) and
[`result.column.rename`](#resultcolumnrename) are distinct options.
The _ingest_ option is applied to ingest data (e.g. a CSV file) column names
before the data is sent to the database (pre-processing).
The _result_ option, by contrast, is applied
to result set column names after the data is returned from the database (post-processing).
It is possible (and normal) to use both options.
{{< /alert >}}

### `ingest.header`

{{< readfile file="../cmd/options/ingest.header.help.txt" code="true" lang="text" >}}

### `ingest.sample-size`

{{< readfile file="../cmd/options/ingest.sample-size.help.txt" code="true" lang="text" >}}

### `driver.csv.delim`

{{< readfile file="../cmd/options/driver.csv.delim.help.txt" code="true" lang="text" >}}

### `driver.csv.empty-as-null`

{{< readfile file="../cmd/options/driver.csv.empty-as-null.help.txt" code="true" lang="text" >}}
