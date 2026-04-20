---
title: Overview
description: Introduction to sq
lead: ''
draft: false
images: []
weight: 1010
toc: true
url: /docs/overview
---

`sq` is the missing tool for wrangling data. A swiss-army knife for data.

`sq` provides a [jq](https://jqlang.github.io/jq/)-style syntax to query, join, migrate, and export data from a variety of data sources,
such as Postgres, SQLite, SQL Server, MySQL, ClickHouse, Excel or CSV, with the ability to fall back
to actual SQL for trickier work. In essence, `sq` treats every data source as if it were a SQL database.
`sq` also provides several handy commands such as `inspect`, `ping`, or `tbl copy`.

## Key Concepts

- `sq` is the command-line utility itself.
- A *source* is a specific data source such as a database instance, or Excel or CSV file etc. Each source
  has a:
  - `driver`: such as [`postgres`](/docs/drivers/postgres), [`sqlserver`](/docs/drivers/sqlserver),
    [`clickhouse`](/docs/drivers/clickhouse), [`csv`](/docs/drivers/csv), or [`xlsx`](/docs/drivers/xlsx).
  - `location`: URI or filepath of the source, such as `postgres://sakila:****@localhost/sakila` or `/Users/neilotoole/sq/xl_demo.xlsx`.
  - `handle`: this is how `sq` refers to that particular _source_, e.g. `@sakila_pg`, `@prod/customer` or `@xl_demo`. The handle must begin with `@`.
- [Active Source](/docs/concepts/#active-source) is the default source upon which `sq` acts if no other source is specified.
- [`sq inspect`](/docs/cmd/inspect) returns _metadata_ about your source, such as table names or number of rows.

Read more in [Concepts](/docs/concepts).

## Quick start

1. [Install](/docs/install) `sq`.
1. [Add](/docs/cmd/add/) a data source. We'll download and use a sample SQLite database file.
   ```shell
   # Download the sample db
   $ wget https://sq.io/testdata/sakila.db

   # Add the source
   $ sq add ./sakila.db --handle @demo
   @demo  sqlite3  sakila.db
   ```
1. Inspect the source:
   ```shell
   $ sq inspect @demo
   SOURCE  DRIVER   NAME       FQ NAME         SIZE   TABLES  VIEWS  LOCATION
   @demo   sqlite3  sakila.db  sakila.db/main  5.6MB  16      5      sqlite3:///Users/neilotoole/work/sq/sq/sakila.db

   NAME                    TYPE   ROWS   COLS
   actor                   table  200    actor_id, first_name, last_name, last_update
   address                 table  603    address_id, address, address2, district, city_id, postal_code, phone, last_update
   category                table  16     category_id, name, last_update
   [...]
   ```
1. Run a query, getting the first three rows of the `actor` table:
   ```shell
   $ sq '@demo.actor | .[0:3]'
   actor_id  first_name  last_name  last_update
   1         PENELOPE    GUINESS    2020-02-15T06:59:28Z
   2         NICK        WAHLBERG   2020-02-15T06:59:28Z
   3         ED          CHASE      2020-02-15T06:59:28Z
   ```
1. Run the query again, but output in a different format:
   ```shell
   $ sq '@demo.actor | .[0:3]' --jsonl
   {"actor_id": "1", "first_name": "PENELOPE", "last_name": "GUINESS", "last_update": "2020-02-15T06:59:28Z"}
   {"actor_id": "2", "first_name": "NICK", "last_name": "WAHLBERG", "last_update": "2020-02-15T06:59:28Z"}
   {"actor_id": "3", "first_name": "ED", "last_name": "CHASE", "last_update": "2020-02-15T06:59:28Z"}
   ```

Next, read the [tutorial](/docs/tutorial).

## Commands

Use `sq help` to list the available commands, or consult the [reference](/docs/cmd/)
for each command.

```text
Available Commands:
  add         Add data source
  src         Get or set active data source
  group       Get or set active group
  ls          List sources and groups
  mv          Move/rename sources and groups
  rm          Remove data source or group
  inspect     Inspect data source schema and stats
  ping        Ping data sources
  sql         Execute DB-native SQL query or statement
  tbl         Useful table actions (copy, truncate, drop)
  diff        BETA: Compare sources, or tables
  driver      Manage drivers
  config      Manage config
  cache       Manage cache
  completion  Generate shell completion script
  version     Show version info
  help        Show help

Flags:
  -f, --format string                  Specify output format (default "text")
  -t, --text                           Output text
  -h, --header                         Print header row (default true)
  -H, --no-header                      Don't print header row
      --help                           Show help
  -j, --json                           Output JSON
  -A, --jsona                          Output LF-delimited JSON arrays
  -J, --jsonl                          Output LF-delimited JSON objects
  -C, --csv                            Output CSV
      --tsv                            Output TSV
      --html                           Output HTML table
      --markdown                       Output Markdown
  -r, --raw                            Output each record field in raw format without any encoding or delimiter
  -x, --xlsx                           Output Excel XLSX
      --xml                            Output XML
  -y, --yaml                           Output YAML
  -c, --compact                        Compact instead of pretty-printed output
      --format.datetime string         Timestamp format: constant such as RFC3339 or a strftime format (default "RFC3339")
      --format.datetime.number         Render numeric datetime value as number instead of string (default true)
      --format.date string             Date format: constant such as DateOnly or a strftime format (default "DateOnly")
      --format.date.number             Render numeric date value as number instead of string (default true)
      --format.time string             Time format: constant such as TimeOnly or a strftime format (default "TimeOnly")
      --format.time.number             Render numeric time value as number instead of string (default true)
      --format.excel.datetime string   Timestamp format string for Excel datetime values (default "yyyy-mm-dd hh:mm")
      --format.excel.date string       Date format string for Excel date-only values (default "yyyy-mm-dd")
      --format.excel.time string       Time format string for Excel time-only values (default "hh:mm:ss")
  -o, --output string                  Write output to <file> instead of stdout
      --insert string                  Insert query results into @HANDLE.TABLE; if not existing, TABLE will be created
      --src string                     Override active source for this query
      --src.schema string              Override active schema or catalog.schema for this query
      --ingest.driver string           Explicitly specify driver to use for ingesting data
      --ingest.header                  Treat first row of ingest data as header
      --no-cache                       Cache ingest data
      --driver.csv.empty-as-null       Treat empty CSV fields as null (default true)
      --driver.csv.delim string        CSV delimiter: one of comma, space, pipe, tab, colon, semi, period (default "comma")
      --version                        Print version info
  -M, --monochrome                     Don't colorize output
      --no-progress                    Progress bar for long-running operations
  -v, --verbose                        Verbose output
      --config string                  Load config from here
      --log                            Enable logging
      --log.file string                Path to log file; empty disables logging
      --log.level string               Log level: one of DEBUG, INFO, WARN, ERROR
      --log.format string              Log format: one of "text" or "json"
```

## Issues

File any bug reports or other issues [here](https://github.com/neilotoole/sq/issues).
When filing a bug report, submit a [log file](/docs/config#logging).

## Config

`sq` is highly configurable. See the [config](/docs/config) section.

### Logging

By default, `sq` logging is disabled. See the [logging](/docs/config#logging) section
to learn how to enable logging.
