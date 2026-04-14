---
title : "sq"
description: "sq data wrangler"
lead: "wrangle data"
draft: false
images: []
---

{{< asciicast src="/casts/home-quick.cast"  poster="npt:0:25" rows=10 autoPlay=true speed=3 idleTimeLimit=3 >}}

`sq` is a free/libre [open-source](https://github.com/neilotoole/sq) data wrangling swiss-army knife
to inspect, query, join, import, and export data. You could think of `sq`
as [jq](https://jqlang.github.io/jq/) for databases and documents, facilitating one-liners
like:

```shell
sq '@postgres_db | .actor | .first_name, .last_name | .[0:5]'
```

## Installation

{{< tabs name="sq-install" >}}
{{{< tab name="mac" codelang="shell" >}}brew install sq{{< /tab >}}
{{< tab name="linux" codelang="shell" >}}/bin/sh -c "$(curl -fsSL https://sq.io/install.sh)"{{< /tab >}}}
{{< tab name="win" codelang="shell">}}scoop bucket add sq https://github.com/neilotoole/sq
scoop install sq{{< /tab >}}}
{{% tab name="more" %}}Install options for `apt`, `yum`, `apk`, `pacman`, `yay` over [here](/docs/install).{{% /tab %}}}
{{< /tabs >}}


For help, `sq help` is your starting point. And then see the [docs](/docs/overview).

### Let's get this out of the way

`sq` is pronounced like _seek_. Its query language, `SLQ`, is pronounced like _sleek_.


## Feature Highlights

Some feature highlights are shown below. For more, see the [docs](/docs/overview),
including the [query guide](/docs/query), [tutorial](/docs/tutorial) and [cookbook](/docs/cookbook).

### Diff database tables

Use the [diff](/docs/diff) command to compare source metadata or row values.

![sq diff](/images/sq_diff_table_data.png)

### Import Excel worksheet into Postgres table

[Insert](/docs/output#insert) the contents of an Excel XLSX worksheet (from a sheet named `actor`) into
a new Postgres table named `xl_actor`. Note that the import mechanism
is reasonably sophisticated in that it tries to preserve data types.

{{< asciicast src="/casts/excel-to-postgres.cast" poster="npt:0:5" rows=5 >}}


### View metadata for a database

The `--json` flag to [`sq inspect`](/docs/inspect) outputs schema and other metadata in JSON.
Typically the output is piped to jq to select the interesting elements.

{{< asciicast src="/casts/inspect-sakila-mysql-json.cast" poster="npt:0:9" rows=10 >}}

### Get names of all columns in a MySQL table

{{< asciicast src="/casts/table-column-names-mysql.cast" poster="npt:0:11" rows=8 >}}

Even easier, just get the metadata for the table you want:

```shell
sq inspect @sakila_my.actor -j | jq -r '.columns[] | .name'
```

### Execute SQL query against SQL Server, insert results to SQLite

This snippet adds a (pre-existing) SQL Server source, and creates a
new [SQLite](/docs/drivers/sqlite) source. Then, a raw native SQL query is executed against
[SQL Server](/docs/drivers/sqlserver), and the results are inserted into SQLite.

{{< asciicast src="/casts/sql-query-then-insert.cast" poster="npt:0:55" rows=6 >}}

### Export all database tables to CSV

Get the (JSON) metadata for the active source; pipe that JSON to jq and
extract the table names; pipe the table names
to `xargs`, invoking `sq` once for each table, outputting a CSV file per table. This snippet
was tested on macOS.

{{< asciicast src="/casts/export-all-tables-to-csv.cast" poster="npt:0:22" idleTimeLimit=0.5 rows=6 speed=2.5 >}}

If you instead wanted to use `sql` mode:

```shell
sq inspect -j | jq -r '.tables[] | .name' | xargs -I % sq sql 'SELECT * FROM %' --csv --output %.csv
```

### Source commands

Commands to [add](/docs/cmd/add), [activate](/docs/cmd/src), [move](/docs/cmd/mv),
[list](/docs/cmd/ls), [group](/docs/cmd/group), [ping](/docs/cmd/ping)
or [remove](/docs/cmd/rm) sources.

```shell
$ sq src                      # show active source
$ sq add ./actor.tsv          # add a source
$ sq src @actor_tsv           # set active source
$ sq ls                       # list sources
$ sq ls -v                    # list sources (verbose)
$ sq group                    # get active group
$ sq group prod               # set active group
$ sq mv @sales @prod/sales    # rename a source
$ sq ping --all               # ping all sources
$ sq rm @actor_tsv            # remove a source
```

{{< asciicast src="/casts/source-cmds.cast" poster="npt:0:20" idleTimeLimit=0.5 rows=10 speed=2 >}}

### Database table commands

Convenient commands that act on database tables: [copy](/docs/cmd/tbl-copy), [truncate](/docs/cmd/tbl-truncate), [drop](/docs/cmd/tbl-drop).

Note that `sq tbl copy` only applies within a single database.
If you want to copy a table from one database to another,
use the [`--insert`](/docs/output#insert) mechanism.

```shell
$ sq tbl copy .actor .actor2  # copy table "actor" to "actor2", creating if necessary
$ sq tbl truncate .actor2     # truncate table "actor2"
$ sq tbl drop .actor2         # drop table "actor2"
```

{{< asciicast src="/casts/table-cmds.cast" poster="npt:0:20" idleTimeLimit=0.5 rows=8 speed=2 >}}

### Query JSONL (e.g. log files)

[JSONL](/docs/output#jsonl) output is a row of JSON per line (hence "JSON Lines"). Lots of log output is like this.
We can use `sq`'s own [log](/docs/config/#logging) output as an example:

```json lines
{"level":"debug","time":"00:07:48.799992","caller":"sqlserver/sqlserver.go:452:(*database).Close","msg":"Close database: @sakila_mssql | sqlserver | sqlserver://sakila:xxxxx@localhost?database=sakila"}
{"level":"debug","time":"00:07:48.800016","caller":"source/files.go:323:(*Files).Close","msg":"Files.Close invoked: has 1 clean funcs"}
{"level":"debug","time":"00:07:48.800031","caller":"source/files.go:61:NewFiles.func1","msg":"About to clean fscache from dir: /var/folders/68/qthwmfm93zl4mqdw_7wvsv7w0000gn/T/sq_files_fscache_2273841732"}
```
{{< asciicast src="/casts/query-jsonl-log-file.cast" poster="npt:0:17" idleTimeLimit=0.5 rows=5 speed=2 >}}
