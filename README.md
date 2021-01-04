# sq: swiss army knife for data

`sq` is a command line tool that provides `jq`-style access to
structured data sources such as SQL databases,
or document formats like CSV or Excel.

`sq` can perform cross-source joins,
execute database-native SQL, and output to a multitude of formats including JSON,
Excel, CSV, HTML, Markdown and XML, or output directly to a SQL database.
`sq` can also inspect sources to see metadata about the source structure (tables,
columns, size) and has commands for common database operations such as copying
or dropping tables.


## Install

For other installation options, see [here](https://github.com/neilotoole/sq/wiki/Home#Install).

### macOS

```shell script
brew tap neilotoole/sq && brew install sq
```

### Windows

```
scoop bucket add sq https://github.com/neilotoole/sq
scoop install sq
```

### Linux

```shell script
curl -fsSLO https://github.com/neilotoole/sq/releases/latest/download/sq-linux-amd64.deb && sudo apt install -y ./sq-linux-amd64.deb && rm ./sq-linux-amd64.deb
```

Or:

```shell script
sudo rpm -i https://github.com/neilotoole/sq/releases/latest/download/sq-linux-amd64.rpm
```

Or:

```shell script
yum localinstall -y https://github.com/neilotoole/sq/releases/latest/download/sq-linux-amd64.rpm
```

## Quickstart

Use `sq help` to see command help. Note that the [tutorial](https://github.com/neilotoole/sq/wiki/Tutorial) has a more detail, but here's the basics:

`sq` operates on data sources, which are treated as SQL databases (even if the source is really a CSV or XLSX file etc). In a nutshell, you add a source (giving it a `handle`), and then execute commands against the source.


### Sources

Initially there are no sources.

```sh
$ sq ls

```

Let's add a source. First we'll add a SQLite database, but this could also be Postgres,
SQL Server, Excel, etc. Let's download the sample DB, and add the source. We
use `-h` to specify a handle to use.

```sh
$ wget https://sq.io/testdata/sakila.db

$ sq add ./sakila.db -h @sakila_sl3
@sakila_sl3  sqlite3  sakila.db

$ sq ls -v
HANDLE           DRIVER   LOCATION                 OPTIONS
@sakila_sl3*  sqlite3  sqlite3:/root/sakila.db

$ sq ping @sakila_sl3
@sakila_sl3  1ms  pong

$ sq src
@sakila_sl3  sqlite3  sakila.db
```

The `sq ping` command simply pings the source to verify that it's available.

`sq src` lists the _active source_, which in our case is `@sakila_sl33`. You can change the active source using `sq src @other_src`. When there's an active source set, you can usually omit the handle from commands. Thus you could instead do:

```sh
$ sq ping
@sakila_sl3  1ms  pong
```

### Query

The most fundamental functionality is querying data. Using our jq-style syntax:

```sh
$ sq '.actor | .actor_id < 100 | .[0:3]'
actor_id  first_name  last_name     last_update
1         PENELOPE    GUINESS       2020-02-15T06:59:28Z
2         NICK        WAHLBERG      2020-02-15T06:59:28Z
3         ED          CHASE         2020-02-15T06:59:28Z
```


The above query selected some rows from the `actor` table. You could also use native SQL, e.g.:

```sh
$ sq sql 'SELECT * FROM actor WHERE actor_id < 100 LIMIT 3'
actor_id  first_name  last_name  last_update
1         PENELOPE    GUINESS    2020-02-15T06:59:28Z
2         NICK        WAHLBERG   2020-02-15T06:59:28Z
3         ED          CHASE      2020-02-15T06:59:28Z
```

But we're flying a bit blind here: how did we know about the `actor` table?

### Inspect

`sq inspect` is your friend (output abbreviated):

```sh
sq inspect
HANDLE          DRIVER   NAME       FQ NAME         SIZE   TABLES  LOCATION
@sakila_sl3     sqlite3  sakila.db  sakila.db/main  5.6MB  21      sqlite3:///root/sakila.db

TABLE                   ROWS   TYPE   SIZE  NUM COLS  COL NAMES                                                                          COL TYPES
actor                   200    table  -     4         actor_id, first_name, last_name, last_update                                       numeric, VARCHAR(45), VARCHAR(45), TIMESTAMP
address                 603    table  -     8         address_id, address, address2, district, city_id, postal_code, phone, last_update  int, VARCHAR(50), VARCHAR(50), VARCHAR(20), INT, VARCHAR(10), VARCHAR(20), TIMESTAMP
category                16     table  -     3         category_id, name, last_update
```

Use the `--json` flag to output in JSON (output abbreviated):

```json
sq inspect -j
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
      ]
    }
  ]
}
```

Combine `sq inspect` with [jq](https://stedolan.github.io/jq/) for some very useful capabilities. Here's how to [list](https://github.com/neilotoole/sq/wiki/Cookbook#list-name-of-each-table-in-a-source) all the table names in the active source:

```sh
$ sq inspect -j | jq -r '.tables[] | .name'
actor
address
category
city
country
customer
[...]
```

And here's how you could [export](https://github.com/neilotoole/sq/wiki/Cookbook#export-all-tables-to-csv) each table to a CSV file:

```sh
$ sq inspect -j | jq -r '.tables[] | .name' | xargs -I % sq .% --csv --output %.csv
$ ls
actor.csv     city.csv	    customer_list.csv  film_category.csv  inventory.csv  rental.csv		     staff.csv
address.csv   country.csv   film.csv	       film_list.csv	  language.csv	 sales_by_film_category.csv  staff_list.csv
category.csv  customer.csv  film_actor.csv     film_text.csv	  payment.csv	 sales_by_store.csv	     store.csv
```

Note that you can also inspect an individual table:

```sh
$ sq inspect @sakila_sl3.actor
TABLE  ROWS  TYPE   SIZE  NUM COLS  COL NAMES                                     COL TYPES
actor  200   table  -     4         actor_id, first_name, last_name, last_update  numeric, VARCHAR(45), VARCHAR(45), TIMESTAMP

```

### Copy Table Across Sources

`sq` query results can be output in various formats (JSON, XML, CSV, etc), and can also be "outputted" as an *insert* into database sources.

Thus, you can use `sq` to copy a Postgres table into MySQL, or to copy an Excel worksheet into a SQLite table, or a CSV file into SQL Server table.

> **Note:** If you want to copy a table inside the same (database) source, use `sq tbl copy` instead, which uses the database's native table copy functionality.

For this example, we'll insert an Excel worksheet into our SQLite DB. First, we download the XLSX file, and `sq add` it as a source.

```sh
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

Now, we'll insert that output into a (new) table in `@sakila_sl3`:

```shell
$ sq @xl_demo_xlsx.person --insert @sakila_sl3.person
Inserted 7 rows into @sakila_sl3.person

$ sq inspect @sakila_sl3.person
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

`sq` has rudimentary support for cross-source joins. That is, you can join an Excel sheet with a CSV file, or Postgres table, etc.

> Note that the current mechanism for these joins is highly naive: it basically copies the joined table from each source to a "scratch database" (SQLite by default), and then performs the JOIN using the scratch database's SQL interface. Thus, performance is currently abysmal for larger tables.

See the [tutorial](https://github.com/neilotoole/sq/wiki/Tutorial#join) for further details, but given an Excel source `@xl_demo` and a CSV source `@csv_demo`, you can do:

```sh
$ sq '@csv_demo.data, @xl_demo.address | join(.D == .address_id) | .C, .city'
C                      city
neilotoole@apache.org  Washington
kaiser@soze.org        Ulan Bator
nikola@tesla.rs        Washington
augustus@caesar.org    Ulan Bator
plato@athens.gr        Washington
```


### Table Commands

`sq` provides several handy commands for working with tables. Note that these commands work directly against SQL database sources, using their native SQL commands.

```sh
$ sq tbl copy .actor .actor_copy
Copied table: @sakila_sl3.actor --> @sakila_sl3.actor_copy (200 rows copied)

$ sq tbl truncate .actor_copy
Truncated 200 rows from @sakila_sl3.actor_copy

$ sq tbl drop .actor_copy
Dropped table @sakila_sl3.actor_copy
```



### UNIX Pipes

For file-based sources (such as CSV or XLSX), you can `sq add` the source file, but you can also pipe it, e.g. `cat ./example.xlsx | sq .Sheet1`.

Similarly you can inspect, e.g. `cat ./example.xlsx | sq inspect`.


## Data Source Drivers
`sq` implements support for data source types via a _driver_. To view the installed/supported drivers:

```sh
$ sq drivers
DRIVER     DESCRIPTION                            USER-DEFINED  DOC
sqlite3    SQLite                                 false         https://github.com/mattn/go-sqlite3
postgres   PostgreSQL                             false         https://github.com/jackc/pgx
sqlserver  Microsoft SQL Server                   false         https://github.com/denisenkom/go-mssqldb
mysql      MySQL                                  false         https://github.com/go-sql-driver/mysql
csv        Comma-Separated Values                 false         https://en.wikipedia.org/wiki/Comma-separated_values
tsv        Tab-Separated Values                   false         https://en.wikipedia.org/wiki/Tab-separated_values
json       JSON                                   false         https://en.wikipedia.org/wiki/JSON
jsona      JSON Array: LF-delimited JSON arrays   false         https://en.wikipedia.org/wiki/JSON
jsonl      JSON Lines: LF-delimited JSON objects  false         https://en.wikipedia.org/wiki/JSON_streaming#Line-delimited_JSON
xlsx       Microsoft Excel XLSX                   false         https://en.wikipedia.org/wiki/Microsoft_Excel
```


## Output Formats
`sq` supports these output formats:

- `--csv`: Text/Table
- `--json`: JSON
- `--jsona`: JSON Array
- `--jsonl`: JSON Lines
- `--csv` / `--tsv` : CSV / TSV
- `--xlsx`: XLSX (Microsoft Excel)
- `--html`: HTML
- `--xml`: XML
- `--markdown`: Markdown
- `--raw`: Raw (bytes)


## Acknowledgements

- Much inspiration is owed to [jq](https://stedolan.github.io/jq/).
- See [`go.mod`](https://github.com/neilotoole/sq/blob/master/go.mod) for a list of third-party packages.
- Additionally, `sq` incorporates modified versions of:
    - [`olekukonko/tablewriter`](https://github.com/olekukonko/tablewriter)
    - [`segmentio/encoding`](https://github.com/segmentio/encoding) for JSON encoding.
- The [_Sakila_](https://dev.mysql.com/doc/sakila/en/) example databases were lifted from [jOOQ](https://github.com/jooq/jooq), which in turn owe their heritage to earlier work on Sakila.

## Similar / Related / Noteworthy Projects

- [usql](https://github.com/xo/usql)
- [textql](https://github.com/dinedal/textql)
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [octosql](https://github.com/cube2222/octosql)
- [rq](https://github.com/dflemstr/rq)


