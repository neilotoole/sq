---
title: Tutorial
description: sq tutorial
lead: ''
draft: false
images: []
weight: 1070
toc: true
url: /docs/tutorial
---

This tutorial walks through `sq`'s features.

If you haven't installed `sq`, see [here](/docs/install). If you've already
installed `sq`, check that you're on a
recent version like so:

```shell
$ sq version
sq v0.40.0
```

If your version is older than that, please [upgrade](/docs/install).
Then start with `sq help`.

## Basics

Let's set out with an example. We'll use a SQLite database,
prepopulated with the [Sakila](/docs/develop/sakila) dataset. Note that for many
of these examples, the output will be abbreviated for brevity.

```shell
# No data source added to sq yet, so "sq ls" is empty.
$ sq ls

# Download the sample db.
$ wget https://sq.io/testdata/sakila.db

# Add a new source
$ sq add ./sakila.db --handle @tutorial_db
@tutorial_db  sqlite3  sakila.db

# The new source should show up in "sq ls".
$ sq ls
@tutorial_db  sqlite3  sakila.db

# For many commands, add -v (--verbose) to see more detail.
$ sq ls -v
HANDLE        ACTIVE  DRIVER   LOCATION                                                           OPTIONS
@tutorial_db  active  sqlite3  sqlite3:///Users/neilotoole/work/sq/sq/scratch/tutorial/sakila.db

# Now, let's have a look at our new source. Output abbreviated for brevity.
$ sq inspect @tutorial_db
SOURCE        DRIVER   NAME       FQ NAME         SIZE   TABLES  VIEWS  LOCATION
@tutorial_db  sqlite3  sakila.db  sakila.db/main  5.6MB  16      5      sqlite3:///Users/neilotoole/work/sq/sq/scratch/tutorial/sakila.db

NAME                    TYPE   ROWS   COLS
actor                   table  200    actor_id, first_name, last_name, last_update
address                 table  603    address_id, address, address2, district, city_id, postal_code, phone, last_update
category                table  16     category_id, name, last_update
city                    table  600    city_id, city, country_id, last_update
country                 table  109    country_id, country, last_update
customer                table  599    customer_id, store_id, first_name, last_name, email, address_id, active, create_date, last_update
film                    table  1000   film_id, title, description, release_year, language_id, original_language_id, rental_duration, rental_rate, length, replacement_cost, rating, special_features, last_update
film_actor              table  5462   actor_id, film_id, last_update
```

Let's step through the above:

- `sq ls`: list the current [sources](/docs/source). There are none.
- `wget`: download a [SQLite](/docs/drivers/sqlite) datafile to use for this demo.
- `sq add`: add a source. The [_driver type_](/docs/concepts/#driver-type) is [detected](/docs/detect/#driver-type)
  to be `sqlite3`, and we give this _source_ the handle `@tutorial_db`.
- `sq ls`: lists the sources again; this time we see our new `@tutorial_db` source.
- `sq ls -v`: lists the sources yet again, this time [verbosely](/docs/config/#verbose) (`-v`).
- `sq inspect`: [inspects](/docs/inspect) `@tutorial_db`, showing the structure of the source.

{{< alert icon="👉" >}}
Most `sq` commands feature sophisticated shell completion. Try it out
by hitting `TAB` when typing a command.
{{< /alert >}}

## Query

Now that we have added a source to `sq`, we can [query](/docs/query) it. Let's select
everything from the `actor` table.

```shell
$ sq @tutorial_db.actor
actor_id  first_name   last_name     last_update
1         PENELOPE     GUINESS       2020-02-15T06:59:28Z
2         NICK         WAHLBERG      2020-02-15T06:59:28Z
3         ED           CHASE         2020-02-15T06:59:28Z

# Being that "@tutorial_db" is the active source, you can omit the handle:
$ sq .actor
actor_id  first_name   last_name     last_update
1         PENELOPE     GUINESS       2020-02-15T06:59:28Z
2         NICK         WAHLBERG      2020-02-15T06:59:28Z
3         ED           CHASE         2020-02-15T06:59:28Z
```

That listed the contents of the `actor` table.

The same query can be executed in [native SQL](/docs/cmd/sql/):

```shell
$ sq sql "SELECT * FROM actor"
actor_id  first_name   last_name     last_update
1         PENELOPE     GUINESS       2020-02-15T06:59:28Z
2         NICK         WAHLBERG      2020-02-15T06:59:28Z
3         ED           CHASE         2020-02-15T06:59:28Z
```

Let's look at some examples of using the _SLQ_ query language. See
the [query guide](/docs/query) for in-depth documentation.

```shell
$ sq '.actor | where(.first_name == "MARY")'
actor_id  first_name  last_name  last_update
66        MARY        TANDY      2020-02-15T06:59:28Z
198       MARY        KEITEL     2020-02-15T06:59:28Z
```

It should be obvious that the above query effectively performs a `WHERE first_name = 'MARY'`.

```shell
$ sq '.actor | .first_name, .last_name | .[2:5]'
first_name  last_name
ED          CHASE
JENNIFER    DAVIS
JOHNNY      LOLLOBRIGIDA
```

Above we select (zero-indexed) rows 2-5, and output columns `first_name` and `last_name`.
The same could be accomplished by:

```shell
$ sq sql 'SELECT first_name, last_name FROM actor LIMIT 3 OFFSET 2'
first_name  last_name
ED          CHASE
JENNIFER    DAVIS
JOHNNY      LOLLOBRIGIDA
```

We could also output in [JSON](/docs/output#json) using the `-j` (`--json`) flag:

```json
[
  {
    "first_name": "ED",
    "last_name": "CHASE"
  },
  {
    "first_name": "JENNIFER",
    "last_name": "DAVIS"
  },
  {
    "first_name": "JOHNNY",
    "last_name": "LOLLOBRIGIDA"
  }
]
```

The `--jsonl` (JSON lines) format is sometimes more convenient:

```json lines
{"first_name": "ED", "last_name": "CHASE"}
{"first_name": "JENNIFER", "last_name": "DAVIS"}
{"first_name": "JOHNNY", "last_name": "LOLLOBRIGIDA"}
```

There are several other [output formats](/docs/output) available.

## Join

We can [join](/docs/query#joins) across the tables of the database.:

```shell
$ sq '.actor | join(.film_actor, .actor_id) | join(.film, .film_id) | .first_name, .last_name, .title:film_title'
first_name   last_name     film_title
PENELOPE     GUINESS       ACADEMY DINOSAUR
PENELOPE     GUINESS       ANACONDA CONFESSIONS
PENELOPE     GUINESS       ANGELS LIFE
PENELOPE     GUINESS       BULWORTH COMMANDMENTS
```

## Stdin

Let's grab another data source, this time in [CSV](/docs/output#csv). We'll download the file.

```shell
$ wget https://sq.io/testdata/film.csv
```

Now let's take a look at it:

```shell
$ cat film.csv | sq inspect -v
SOURCE  DRIVER  NAME    FQ NAME  SIZE     TABLES  VIEWS  LOCATION
@stdin  csv     @stdin  @stdin   189.6KB  1       0      @stdin

NAME  TYPE   ROWS  COLS  NAME                  TYPE      PK
data  table  1000  13    film_id               INTEGER
                         title                 TEXT
                         description           TEXT
                         release_year          INTEGER
                         language_id           INTEGER
                         original_language_id  TEXT
                         rental_duration       INTEGER
                         rental_rate           NUMERIC
                         length                INTEGER
                         replacement_cost      NUMERIC
                         rating                TEXT
                         special_features      TEXT
                         last_update           DATETIME
```

> Note that because CSV is [_monotable_](/docs/concepts/#monotable) (only has one table of data),
> its data is represented as a single table
> named `data`.

We can pipe that CSV file to `sq` and performs the usual actions on it:

```shell
$ cat film.csv | sq '.data | .title, .release_year | .[2:5]'
title             release_year
ADAPTATION HOLES  2006
AFFAIR PREJUDICE  2006
AFRICAN EGG       2006
```

We could continue to `cat` the CSV file to `sq`, but being that we'll use it later
in this tutorial, we'll add it as a source:

```shell
$ sq add film.csv --handle @film_csv
@film_csv  csv  film.csv
```

{{< alert icon="👉" >}}
`stdin` sources can't take advantage of [ingest caching](/docs/source#ingest), because
the `stdin` pipe is "anonymous", and `sq` can't do a cache lookup for it. If you're going to
repeatedly query the same `stdin` data, you should probably just [`sq add`](/docs/source#add) it.
{{< /alert >}}

We've now got two sources: a SQLite database (`@tutorial_db`), and
a CSV file (`@film_csv`). We can join across those sources:

```shell
$ sq '.actor | join(.film_actor, .actor_id) | join(@film_csv.data, .film_id) | .first_name, .last_name, .title | .[0:5]'
first_name  last_name  title
PENELOPE    GUINESS    ACADEMY DINOSAUR
PENELOPE    GUINESS    ANACONDA CONFESSIONS
PENELOPE    GUINESS    ANGELS LIFE
PENELOPE    GUINESS    BULWORTH COMMANDMENTS
PENELOPE    GUINESS    CHEAPER CLYDE
```

## Active Source

Now that we've added multiple sources, let's see what `sq ls` has to say:

```shell
$ sq ls
@film_csv      csv      film.csv
@tutorial_db*  sqlite3  sakila.db
```

Note that `@tutorial_db` is the active source (it has an asterisk beside it, and renders
in a different color on a color terminal).

We can do this with `@film_csv`:

```shell
$ sq '@film_csv.data | .title | .[0:2]'
title
ACADEMY DINOSAUR
ACE GOLDFINGER
```

But not this:

```shell
$ sq '.data | .title | .[0:2]'
sq: SQL query against @tutorial_db failed: SELECT "title" FROM "data" LIMIT 2 OFFSET 0: no such table: data
```

Because the active source is still `@tutorial_db`. To see the active source:

```shell
$ sq src
@tutorial_db  sqlite3  sakila.db
```

Let's switch the active source to the CSV file:

```shell
$ sq src @film_csv
@film_csv  csv  film.csv
```

Now we can use the shorthand form (omit the `@film_csv` handle) and `sq` will look for
table `.data` in the active
source (which is now `@film_csv`):

```shell
$ sq '.data | .title | .[0:2]'
title
ACADEMY DINOSAUR
ACE GOLDFINGER
```

## Ping

A useful feature is to [ping](/docs/cmd/ping) the sources to verify that they're accessible:

```shell
# Ping sources in the root group, i.e. all sources.
$ sq ping /
@film_csv           0s  pong
@tutorial_db       1ms  pong
```

Instead of pinging all the sources in the [group](/docs/source/#groups), we can specify the
sources explicitly:

```shell
$ sq ping @film_csv @tutorial_db
@film_csv           0s  pong
@tutorial_db       1ms  pong
```

## SQL Sources

Having read this far, you can be forgiven for thinking that `sq` only deals with file formats such as CSV
or even SQLite, but that is not the case. Let's add some SQL databases.

First we'll do [Postgres](/docs/drivers/postgres); we'll start a pre-built [Sakila](https://dev.mysql.com/doc/sakila/en/sakila-introduction.html)
database via docker on port (note that it will take a moment for the Postgres container to start up):

```shell
$ docker run -d -p 5432:5432 sakiladb/postgres:latest
```

Let's add that Postgres database as a source:

```shell
$ sq add 'postgres://sakila:p_ssW0rd@localhost/sakila' --handle @sakila_pg
@sakila_pg  postgres  sakila@localhost/sakila
```

> If you don't want to type the password on the command line, use `-p`
> to be [prompted](/docs/cmd/add/#password-visibility):
>
>
> ```shell
> $ sq add 'postgres://sakila@localhost/sakila' -p
> Password: [ENTER]
> ```

The new source should show up in `sq ls`:

```shell
@film_csv*    csv       film.csv
@sakila_pg    postgres  sakila@localhost/sakila
@tutorial_db  sqlite3   sakila.db
```

Ping the new source just for fun:

```shell
$ sq ping @sakila_pg
@sakila_pg      29ms  pong
```


Now we'll add and start a MySQL instance of _Sakila_:

```shell
$ docker run -d -p 3306:3306 sakiladb/mysql:latest
$ sq add "mysql://sakila:p_ssW0rd@localhost/sakila" --handle @sakila_my
@sakila_my  mysql  sakila@localhost/sakila
```

And get some data from `@sakila_my`:

```shell
$ sq '@sakila_my.actor | .[0:2]'
actor_id  first_name  last_name  last_update
1         PENELOPE    GUINESS    2006-02-15 04:34:33 +0000 UTC
2         NICK        WAHLBERG   2006-02-15 04:34:33 +0000 UTC
```

## Insert & Modify

In addition to JSON, CSV, etc., `sq` can write query results to database tables.

We'll use the `film_category` table as an example: the table is in both `@sakila_pg` and `@sakila_my`. We're going to
truncate the table in `@sakila_my` and then repopulate via a query against `@sakila_pg`.

```shell
# First, set the active source to @sakila_my for convenience.
$ sq src @sakila_my
@sakila_my  mysql  sakila@localhost/sakila

# Now, let's confirm the counts
$ sq '@sakila_pg.film_category | count'
count
1000

$ sq '@sakila_my.film_category | count'
count
1000
````

Make a copy of the table as a backup.

```shell
$ sq tbl copy .film_category .film_category_bak
Copied table: @sakila_my.film_category --> @sakila_my.film_category_bak (1000 rows copied)
```

> Note that the `sq tbl copy` makes use each database's own table copy functionality. Thus `sq tbl copy` can't be used
> to directly copy a table from one database to another. But `sq` provides a means of doing this, keep reading.

Truncate the `@sakila_my.film_category` table:

```shell
$ sq tbl truncate @sakila_my.film_category
Truncated 1000 rows from @sakila_my.film_category

$ sq '@sakila_my.film_category | count'
count
0
```

The `@sakila_my.film_category` table is now empty. But we can repopulate it via a query against `@sakila_pg`. For this
example, we'll just do `500` rows.

```shell
$ sq '@sakila_pg.film_category | .[0:500]' --insert @sakila_my.film_category
Inserted 500 rows into @sakila_my.film_category

$ sq '@sakila_my.film_category | count'
count
500
```

We can now use the `sq tbl` commands to restore the previous state.

```shell
$ sq tbl drop .film_category
Dropped table @sakila_my.film_category

# Restore the film_category table from the backup table we made earlier
$ sq tbl copy .film_category_bak .film_category
Copied table: @sakila_my.film_category_bak --> @sakila_my.film_category (1000 rows copied)

# Verify that the table is restored
$ sq '.film_category | count'
count
1000

# Get rid of the backup table
$ sq tbl drop .film_category_bak
Dropped table @sakila_my.film_category_bak
```

## jq

Note that `sq` plays nicely with jq. For example,
list the names of the columns in table `@sakila_pg.actor`:

```shell
$ sq inspect --json @sakila_pg.actor | jq -r '.columns[] | .name'
actor_id
first_name
last_name
last_update
```
