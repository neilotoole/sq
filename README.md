[//]: # ([![Go Coverage]&#40;https://github.com/neilotoole/sq/wiki/coverage.svg&#41;]&#40;https://raw.githack.com/wiki/neilotoole/sq/coverage.html&#41;)
[![Go Reference](https://pkg.go.dev/badge/github.com/neilotoole/sq.svg)](https://pkg.go.dev/github.com/neilotoole/sq)
[![Go Report Card](https://goreportcard.com/badge/neilotoole/sq)](https://goreportcard.com/report/neilotoole/sq)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/neilotoole/sq/blob/master/LICENSE)
![Main pipeline](https://github.com/neilotoole/sq/actions/workflows/main.yml/badge.svg)


# sq data wrangler

`sq` is a command line tool that provides jq-style access to
structured data sources: SQL databases, or document formats like CSV or Excel.

![sq](.images/splash.png)

`sq` executes jq-like [queries](https://sq.io/docs/query), or database-native [SQL](https://sq.io/docs/cmd/sql/).
It can perform cross-source [joins](https://sq.io/docs/query/#cross-source-joins).

`sq` outputs to a multitude of [formats](https://sq.io/docs/output#formats)
including [JSON](https://sq.io/docs/output#json),
[Excel](https://sq.io/docs/output#xlsx), [CSV](https://sq.io/docs/output#csv),
[HTML](https://sq.io/docs/output#html), [Markdown](https://sq.io/docs/output#markdown) 
and [XML](https://sq.io/docs/output#xml), and can [insert](https://sq.io/docs/output#insert) query 
results directly to a SQL database.
`sq` can also [inspect](https://sq.io/docs/cmd/inspect) sources to view metadata about the source structure (tables,
columns, size) and has commands for common database operations to
[copy](https://sq.io/docs/cmd/tbl-copy), [truncate](https://sq.io/docs/cmd/tbl-truncate),
and [drop](https://sq.io/docs/cmd/tbl-drop) tables.

Find out more at [sq.io](https://sq.io).


## Install

### macOS

```shell
brew install neilotoole/sq/sq
```

### Linux

```shell
/bin/sh -c "$(curl -fsSL https://sq.io/install.sh)"
```

### Windows

```shell
scoop bucket add sq https://github.com/neilotoole/sq
scoop install sq
```

### Go

```shell
go install github.com/neilotoole/sq
```

See other [install options](https://sq.io/docs/install/).

## Overview

Use `sq help` to see command help. Docs are over at [sq.io](https://sq.io).
Read the [overview](https://sq.io/docs/overview/), and 
[tutorial](https://sq.io/docs/tutorial/). The [cookbook](https://sq.io/docs/cookbook/) has
recipes for common tasks, and the [query guide](https://sq.io/docs/query) covers `sq`'s query language.

The major concept is: `sq` operates on data sources, which are treated as SQL databases (even if the
source is really a CSV or XLSX file etc.).

In a nutshell, you [`sq add`](https://sq.io/docs/cmd/add) a source (giving it a [`handle`](https://sq.io/docs/concepts#handle)), and then execute commands against the
source.

### Sources

Initially there are no [sources](https://sq.io/docs/source).

```shell
$ sq ls

```

Let's [add](https://sq.io/docs/cmd/add) a source. First we'll add a [SQLite](https://sq.io/docs/drivers/sqlite)
database, but this could also be [Postgres](https://sq.io/docs/drivers/postgres),
[SQL Server](https://sq.io/docs/drivers/sqlserver), [Excel](https://sq.io/docs/drivers/xlsx), etc.
Download the sample DB, and `sq add` the source. 

```shell
$ wget https://sq.io/testdata/sakila.db

$ sq add ./sakila.db
@sakila  sqlite3  sakila.db

$ sq ls -v
HANDLE   ACTIVE  DRIVER   LOCATION                         OPTIONS
@sakila  active  sqlite3  sqlite3:///Users/demo/sakila.db

$ sq ping @sakila
@sakila       1ms  pong

$ sq src
@sakila  sqlite3  sakila.db
```

The [`sq ping`](https://sq.io/docs/cmd/ping) command simply pings the source
to verify that it's available.

[`sq src`](https://sq.io/docs/cmd/src) lists the [_active source_](https://sq.io/docs/source#active-source), which in our
case is `@sakila`.
You can change the active source using `sq src @other_src`.
When there's an active source specified, you can usually omit the handle from `sq` commands.
Thus you could instead do:

```shell
$ sq ping
@sakila  1ms  pong
```

### Query

Fundamentally, `sq` is for querying data. The jq-style syntax is covered in
detail in the [query guide](https://sq.io/docs/query).

```shell
$ sq '.actor | where(.actor_id < 100) | .[0:3]'
actor_id  first_name  last_name     last_update
1         PENELOPE    GUINESS       2020-02-15T06:59:28Z
2         NICK        WAHLBERG      2020-02-15T06:59:28Z
3         ED          CHASE         2020-02-15T06:59:28Z
```

The above query selected some rows from the `actor` table. You could also
use [native SQL](https://sq.io/docs/cmd/sql), e.g.:

```shell
$ sq sql 'SELECT * FROM actor WHERE actor_id < 100 LIMIT 3'
actor_id  first_name  last_name  last_update
1         PENELOPE    GUINESS    2020-02-15T06:59:28Z
2         NICK        WAHLBERG   2020-02-15T06:59:28Z
3         ED          CHASE      2020-02-15T06:59:28Z
```

But we're flying a bit blind here: how did we know about the `actor` table?

### Inspect

[`sq inspect`](https://sq.io/docs/cmd/inspect) is your friend (output abbreviated):

```shell
$ sq inspect
HANDLE   DRIVER   NAME       FQ NAME         SIZE   TABLES  LOCATION
@sakila  sqlite3  sakila.db  sakila.db/main  5.6MB  21      sqlite3:///Users/demo/sakila.db

TABLE                   ROWS   COL NAMES
actor                   200    actor_id, first_name, last_name, last_update
address                 603    address_id, address, address2, district, city_id, postal_code, phone, last_update
category                16     category_id, name, last_update
```

Use [`sq inspect -v`](https://sq.io/docs/output#verbose) to see more detail.
Or use [`-j`](https://sq.io/docs/output#json) to get JSON output:

![sq inspect -j](https://sq.io/images/sq_inspect_sakila_sqlite_json.png)

Combine `sq inspect` with [jq](https://stedolan.github.io/jq/) for some useful capabilities.
Here's how to [list](https://sq.io/docs/cookbook/#list-table-names)
all the table names in the active source:

```shell
$ sq inspect -j | jq -r '.tables[] | .name'
actor
address
category
city
country
customer
[...]
```

And here's how you
could [export](https://sq.io/docs/cookbook/#export-all-table-data-to-csv) each table
to a CSV file:

```shell
$ sq inspect -j | jq -r '.tables[] | .name' | xargs -I % sq .% --csv --output %.csv
$ ls
actor.csv     city.csv	    customer_list.csv  film_category.csv  inventory.csv  rental.csv		     staff.csv
address.csv   country.csv   film.csv	       film_list.csv	  language.csv	 sales_by_film_category.csv  staff_list.csv
category.csv  customer.csv  film_actor.csv     film_text.csv	  payment.csv	 sales_by_store.csv	     store.csv
```

Note that you can also inspect an individual table:

```shell
$ sq inspect @sakila.actor
TABLE  ROWS  TYPE   SIZE  NUM COLS  COL NAMES
actor  200   table  -     4         actor_id, first_name, last_name, last_update
```

### Diff

Use [`sq diff`](https://sq.io/docs/diff) to compare source metadata, or row data.

![sq diff](.images/sq_diff_table_data.png)

### Insert query results

`sq` query results can be [output](https://sq.io/docs/output) in various formats 
(JSON, XML, CSV, etc), and can also be "outputted" as an
[*insert*](https://sq.io/docs/output#insert) into database sources.

That is, you can use `sq` to insert results from a Postgres query into a MySQL table,
or copy an Excel worksheet into a SQLite table, or a push a CSV file into
a SQL Server table etc.

> **Note:** If you want to copy a table inside the same (database) source,
> use [`sq tbl copy`](https://sq.io/docs/cmd/tbl-copy) instead, which uses the database's native table copy functionality.

For this example, we'll insert an Excel worksheet into our `@sakila`
SQLite database. First, we
download the XLSX file, and `sq add` it as a source.

```shell
$ wget https://sq.io/testdata/xl_demo.xlsx

$ sq add ./xl_demo.xlsx --ingest.header=true
@xl_demo  xlsx  xl_demo.xlsx

$ sq @xl_demo.person
uid  username    email                  address_id
1    neilotoole  neilotoole@apache.org  1
2    ksoze       kaiser@soze.org        2
3    kubla       kubla@khan.mn          NULL
[...]
```

Now, execute the same query, but this time `sq` inserts the results into a new 
table (`person`)
in the SQLite `@sakila` source:

```shell
$ sq @xl_demo.person --insert @sakila.person
Inserted 7 rows into @sakila.person

$ sq inspect @sakila.person
TABLE   ROWS  COL NAMES
person  7     uid, username, email, address_id

$ sq @sakila.person
uid  username    email                  address_id
1    neilotoole  neilotoole@apache.org  1
2    ksoze       kaiser@soze.org        2
3    kubla       kubla@khan.mn          NULL
[...]
```

### Cross-source join

`sq` has rudimentary support for cross-source [joins](https://sq.io/docs/query#join). That is, you can join an Excel worksheet with a
CSV file, or Postgres table, etc.

See the [tutorial](https://sq.io/docs/tutorial/#join) for further details, but
given an Excel source `@xl_demo` and a CSV source `@csv_demo`, you can do:

```shell
$ sq '@csv_demo.data, @xl_demo.address | join(.D == .address_id) | .C, .city'
C                      city
neilotoole@apache.org  Washington
kaiser@soze.org        Ulan Bator
nikola@tesla.rs        Washington
augustus@caesar.org    Ulan Bator
plato@athens.gr        Washington
```

### Table commands

`sq` provides several handy commands for working with tables:
[`tbl copy`](/docs/cmd/tbl-copy), [`tbl truncate`](/docs/cmd/tbl-truncate)
and [`tbl drop`](/docs/cmd/tbl-drop).
Note that these commands work directly
against SQL database sources, using their native SQL commands.

```shell
$ sq tbl copy .actor .actor_copy
Copied table: @sakila.actor --> @sakila.actor_copy (200 rows copied)

$ sq tbl truncate .actor_copy
Truncated 200 rows from @sakila.actor_copy

$ sq tbl drop .actor_copy
Dropped table @sakila.actor_copy
```

### UNIX pipes

For file-based sources (such as CSV or XLSX), you can `sq add` the source file,
but you can also pipe it:

```shell
$ cat ./example.xlsx | sq .Sheet1
```

Similarly, you can inspect:

```shell
$ cat ./example.xlsx | sq inspect
```

## Drivers

`sq` knows how to deal with a data source type via a [driver](https://sq.io/docs/drivers)
implementation. To view the installed/supported drivers:

```shell
$ sq driver ls
DRIVER     DESCRIPTION                          
sqlite3    SQLite                               
postgres   PostgreSQL                           
sqlserver  Microsoft SQL Server / Azure SQL Edge
mysql      MySQL                                
csv        Comma-Separated Values               
tsv        Tab-Separated Values                 
json       JSON                                 
jsona      JSON Array: LF-delimited JSON arrays 
jsonl      JSON Lines: LF-delimited JSON objects
xlsx       Microsoft Excel XLSX                 
```

## Output formats

`sq` has many [output formats](https://sq.io/docs/output):

- `--text`: [Text](https://sq.io/docs/output#text)
- `--json`: [JSON](https://sq.io/docs/output#json)
- `--jsona`: [JSON Array](https://sq.io/docs/output#jsona)
- `--jsonl`: [JSON Lines](https://sq.io/docs/output#jsonl)
- `--csv` / `--tsv` : [CSV](https://sq.io/docs/output#csv) / [TSV](https://sq.io/docs/output#tsv)
- `--xlsx`: [XLSX](https://sq.io/docs/output#xlsx) (Microsoft Excel)
- `--html`: [HTML](https://sq.io/docs/output#html)
- `--xml`: [XML](https://sq.io/docs/output#xml)
- `--yaml`: [YAML](https://sq.io/docs/output#yaml)
- `--markdown`: [Markdown](https://sq.io/docs/output#markdown)
- `--raw`: [Raw](https://sq.io/docs/output#raw) (bytes)

## CHANGELOG

See [CHANGELOG.md](./CHANGELOG.md).

## Acknowledgements

- Thanks to [Diego Souza](https://github.com/diegosouza) for creating
  the [Arch Linux package](https://aur.archlinux.org/packages/sq-bin).
- Much inspiration is owed to [jq](https://stedolan.github.io/jq/).
- See [`go.mod`](https://github.com/neilotoole/sq/blob/master/go.mod) for a list of third-party
  packages.
- Additionally, `sq` incorporates modified versions of:
	- [`olekukonko/tablewriter`](https://github.com/olekukonko/tablewriter)
	- [`segmentio/encoding`](https://github.com/segmentio/encoding) for JSON encoding.
- The [_Sakila_](https://dev.mysql.com/doc/sakila/en/) example databases were lifted
  from [jOOQ](https://github.com/jooq/jooq), which in turn owe their heritage to earlier work on
  Sakila.
- Date rendering via [`ncruces/go-strftime`](https://github.com/ncruces/go-strftime).

## Similar, related, or noteworthy projects

- [usql](https://github.com/xo/usql)
- [textql](https://github.com/dinedal/textql)
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [octosql](https://github.com/cube2222/octosql)
- [rq](https://github.com/dflemstr/rq)
