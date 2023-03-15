# sq: data wrangler

`sq` is a command line tool that provides `jq`-style access to
structured data sources: SQL databases, or document formats like CSV or Excel.

`sq` can perform cross-source joins,
execute database-native SQL, and output to a multitude of formats including JSON,
Excel, CSV, HTML, Markdown and XML, or insert directly to a SQL database.
`sq` can also inspect sources to view metadata about the source structure (tables,
columns, size) and has commands for common database operations such as copying
or dropping tables.

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

## Quickstart

Use `sq help` to see command help. Docs are over at [sq.io](https://sq.io).
Read the [overview](https://sq.io/docs/overview/), and the
[tutorial](https://sq.io/docs/tutorial/). The [cookbook](https://sq.io/docs/cookbook/) has
recipes for common tasks.

The major concept is: `sq` operates on data sources, which are treated as SQL databases (even if the
source is really a CSV or XLSX file etc.).

In a nutshell, you `sq add` a source (giving it a `handle`), and then execute commands against the
source.

### Sources

Initially there are no sources.

```shell
$ sq ls

```

Let's add a source. First we'll add a SQLite database, but this could also be Postgres,
SQL Server, Excel, etc. Download the sample DB, and `sq add` the source. We
use `-h` to specify a _handle_ to use.

```shell
$ wget https://sq.io/testdata/sakila.db

$ sq add ./sakila.db -h @sakila_sl3
@sakila_sl3  sqlite3  sakila.db

$ sq ls -v
HANDLE       DRIVER   LOCATION                 OPTIONS
@sakila_sl3* sqlite3  sqlite3:/root/sakila.db

$ sq ping @sakila_sl3
@sakila_sl3  1ms  pong

$ sq src
@sakila_sl3  sqlite3  sakila.db
```

The `sq ping` command simply pings the source to verify that it's available.

`sq src` lists the _active source_, which in our case is `@sakila_sl3`.
You can change the active source using `sq src @other_src`.
When there's an active source specified, you can usually omit the handle from `sq` commands.
Thus you could instead do:

```shell
$ sq ping
@sakila_sl3  1ms  pong
```

### Query

Fundamentally, `sq` is for querying data. Using our jq-style syntax:

```shell
$ sq '.actor | .actor_id < 100 | .[0:3]'
actor_id  first_name  last_name     last_update
1         PENELOPE    GUINESS       2020-02-15T06:59:28Z
2         NICK        WAHLBERG      2020-02-15T06:59:28Z
3         ED          CHASE         2020-02-15T06:59:28Z
```

The above query selected some rows from the `actor` table. You could also
use native SQL, e.g.:

```shell
$ sq sql 'SELECT * FROM actor WHERE actor_id < 100 LIMIT 3'
actor_id  first_name  last_name  last_update
1         PENELOPE    GUINESS    2020-02-15T06:59:28Z
2         NICK        WAHLBERG   2020-02-15T06:59:28Z
3         ED          CHASE      2020-02-15T06:59:28Z
```

But we're flying a bit blind here: how did we know about the `actor` table?

### Inspect

`sq inspect` is your friend (output abbreviated):

```shell
HANDLE       DRIVER   NAME       FQ NAME         SIZE   TABLES  LOCATION
@sakila_sl3  sqlite3  sakila.db  sakila.db/main  5.6MB  21      sqlite3:/Users/neilotoole/work/sq/sq/drivers/sqlite3/testdata/sakila.db

TABLE                   ROWS   COL NAMES
actor                   200    actor_id, first_name, last_name, last_update
address                 603    address_id, address, address2, district, city_id, postal_code, phone, last_update
category                16     category_id, name, last_update
```

Use the `--verbose` (`-v`) flag to see more detail. And use `--json` (`-j`) to output in JSON (
output abbreviated):

```shell
$ sq inspect -j
{
  "handle": "@sakila_sl3",
  "name": "sakila.db",
  "driver": "sqlite3",
  "db_version": "3.31.1",
  "location": "sqlite3:///root/sakila.db",
  "size": 5828608,
  "tables": [
    {
      "name": "actor",
      "table_type": "table",
      "row_count": 200,
      "columns": [
        {
          "name": "actor_id",
          "position": 0,
          "primary_key": true,
          "base_type": "numeric",
          "column_type": "numeric",
          "kind": "decimal",
          "nullable": false
        }
```

Combine `sq inspect` with [jq](https://stedolan.github.io/jq/) for some useful capabilities. Here's
how to [list](https://sq-web.netlify.app/docs/cookbook/#list-table-names)
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
$ sq inspect -v @sakila_sl3.actor
TABLE  ROWS  TYPE   SIZE  NUM COLS  COL NAMES                                     COL TYPES
actor  200   table  -     4         actor_id, first_name, last_name, last_update  numeric, VARCHAR(45), VARCHAR(45), TIMESTAMP

```

### Insert Output Into Database Source

`sq` query results can be output in various formats (JSON, XML, CSV, etc), and can also be "
outputted" as an *insert* into database sources.

That is, you can use `sq` to insert results from a Postgres query into a MySQL table, or copy an
Excel worksheet into a SQLite table, or a push a CSV file into a SQL Server table etc.

> **Note:** If you want to copy a table inside the same (database) source, use `sq tbl copy`
> instead, which uses the database's native table copy functionality.

For this example, we'll insert an Excel worksheet into our `@sakila_sl3` SQLite database. First, we
download the XLSX file, and `sq add` it as a source.

```shell
$ wget https://sq.io/testdata/xl_demo.xlsx

$ sq add ./xl_demo.xlsx --opts header=true
@xl_demo_xlsx  xlsx  xl_demo.xlsx

$ sq @xl_demo_xlsx.person
uid  username    email                  address_id
1    neilotoole  neilotoole@apache.org  1
2    ksoze       kaiser@soze.org        2
3    kubla       kubla@khan.mn          NULL
[...]
```

Now, execute the same query, but this time `sq` inserts the results into a new table (`person`)
in `@sakila_sl3`:

```shell
$ sq @xl_demo_xlsx.person --insert @sakila_sl3.person
Inserted 7 rows into @sakila_sl3.person

$ sq inspect -v @sakila_sl3.person
TABLE   ROWS  TYPE   SIZE  NUM COLS  COL NAMES                         COL TYPES
person  7     table  -     4         uid, username, email, address_id  INTEGER, TEXT, TEXT, INTEGER

$ sq @sakila_sl3.person
uid  username    email                  address_id
1    neilotoole  neilotoole@apache.org  1
2    ksoze       kaiser@soze.org        2
3    kubla       kubla@khan.mn          NULL
[...]
```

### Cross-Source Join

`sq` has rudimentary support for cross-source joins. That is, you can join an Excel worksheet with a
CSV file, or Postgres table, etc.

> **Note:** The current mechanism for these joins is highly naive: `sq` copies the joined table from
> each source to a "scratch database" (SQLite by default), and then performs the JOIN using the
> scratch database's SQL interface. Thus, performance is abysmal for larger tables. There are
> massive
> optimizations to be made, but none have been implemented yet.

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

### Table Commands

`sq` provides several handy commands for working with tables. Note that these commands work directly
against SQL database sources, using their native SQL commands.

```shell
$ sq tbl copy .actor .actor_copy
Copied table: @sakila_sl3.actor --> @sakila_sl3.actor_copy (200 rows copied)

$ sq tbl truncate .actor_copy
Truncated 200 rows from @sakila_sl3.actor_copy

$ sq tbl drop .actor_copy
Dropped table @sakila_sl3.actor_copy
```

### UNIX Pipes

For file-based sources (such as CSV or XLSX), you can `sq add` the source file, but you can also
pipe it:

```shell
$ cat ./example.xlsx | sq .Sheet1
```

Similarly, you can inspect:

```shell
$ cat ./example.xlsx | sq inspect
```

## Data Source Drivers

`sq` knows how to deal with a data source type via a _driver_ implementation. To view the
installed/supported drivers:

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

## Output Formats

`sq` has many output formats:

- `--table`: Text/Table
- `--json`: JSON
- `--jsona`: JSON Array
- `--jsonl`: JSON Lines
- `--csv` / `--tsv` : CSV / TSV
- `--xlsx`: XLSX (Microsoft Excel)
- `--html`: HTML
- `--xml`: XML
- `--markdown`: Markdown
- `--raw`: Raw (bytes)

## Changelog

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

## Similar / Related / Noteworthy Projects

- [usql](https://github.com/xo/usql)
- [textql](https://github.com/dinedal/textql)
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [octosql](https://github.com/cube2222/octosql)
- [rq](https://github.com/dflemstr/rq)
