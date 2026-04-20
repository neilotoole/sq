---
title: Detect
description: Detect driver type, headers, and column kinds
lead: ''
draft: false
images: []
weight: 1039
toc: true
url: /docs/detect
---
In an ideal world, all data is strongly typed, and there's no ambiguity.
Take a DB source:

```shell
$ sq add postgres://sakila@localhost/sakila -p <<< $PASSWD
```

The [driver type](/docs/concepts/#driver-type) is known (`postgres`),
each of the tables in the DB has
a distinct name (e.g. `actor`), and each column in the table has a distinct
name (`actor_id`) and type (`INTEGER`), as well as other attributes such
as nullability.

However, this doesn't hold for all sources. In particular, it's not
the case for some file-based sources such as `CSV`. Let's say we add a CSV file:

```shell
$ sq add ./actor.csv
```

That CSV file may contain the same values as the Postgres `actor` table, but
being an untyped text-based format, the CSV data doesn't have all the goodness
of the Postgres table. `sq` uses various "detectors" to bridge that gap.

## Driver type

When [adding](/docs/source/#add) a file-based source, you can use
the `--driver` flag (e.g. `--driver=csv`)
to explicitly specify the file's driver type. However, you can omit
that flag, and `sq` will generally figure out the appropriate driver. This
mechanism works fairly reliably, but you can always fall back to explicitly
supplying the driver type.

```shell
# Explicit
$ sq add --driver=sqlite3 ./sakila.db
@sakila  sqlite3  sakila.db

# Auto-detected
$ sq add ./sakila.db
@sakila  sqlite3  sakila.db
```

`sq` has driver-type detection for [SQLite](/docs/drivers/sqlite), [Excel](/docs/drivers/xlsx),
the three [JSON](/docs/drivers/json/) variants
([JSON](/docs/drivers/json/#json), [JSONA](/docs/drivers/json/#jsona), [JSONL](/docs/drivers/json/#jsonl)),
and [CSV](/docs/drivers/csv)/[TSV](/docs/drivers/csv).

## Header row

When adding a [CSV](/docs/drivers/csv) or [Excel](/docs/drivers/xlsx) source,
the source datafile doesn't explicitly state whether
the first row of data is a header row. This is important to determine, so that
the header row isn't treated as a data row. Take two distinct CSV files, `actor_header.csv`:

```text
actor_id,first_name,last_name,last_update
1,PENELOPE,GUINESS,2020-02-15T06:59:28Z
2,NICK,WAHLBERG,2020-02-15T06:59:28Z
```

and `actor_no_header.csv`:

```text
1,PENELOPE,GUINESS,2020-02-15T06:59:28Z
2,NICK,WAHLBERG,2020-02-15T06:59:28Z
```

For the latter case, `sq` automatically assigns generated
column names `A`, `B`, `C`. (Note that the
generated column names can be [configured](/docs/config/#ingestcolumnrename).)

For the first case, the column names are present in the first row of the file,
and `sq` is generally able to figure this out.
It does this by running an algorithm on a sample of the data, and if
the first row of data appears to be different from the other rows, `sq` marks
that first row as a header row, and thus can determine the correct column names.

The header-detection algorithm is relatively naive, and can be defeated if the number
of sample rows is small, or if the header row and data rows are all the same data kind
(e.g. everything is a string). If that happens, you can explicitly tell
`sq` that a header row is present (or not):

```shell
# Explicitly specify that a header row exists
$ sq add --ingest.header ./actor_header.csv

# Explicitly specify no header row
$ sq add --ingest.header=false ./actor_no_header.csv
```

{{< alert icon="👉" >}}
If `sq` fails to detect the header row correctly in your data, but it seems
like it should be able to, please [open an issue](https://github.com/neilotoole/sq/issues/new/choose), and attach a copy of your data (sanitized
if necessary).
{{< /alert >}}

## Column kind

When ingesting untyped data, `sq` determines a lowest-common-denominator "kind"
for each column, e.g. `int`, `float`, `datetime`, etc. If a kind cannot be
determined, the column typically is treated as `text`. You can think of `sq`'s
"kind" mechanism as an internal, generalized, intermediate column type.

Let's go back to the CSV example:

```text
actor_id,first_name,last_name,last_update
1,PENELOPE,GUINESS,2020-02-15T06:59:28Z
2,NICK,WAHLBERG,2020-02-15T06:59:28Z
```

And [inspect](/docs/inspect) that source:

```shell
$ sq inspect -v  @actor.data
NAME  TYPE   ROWS  COLS  NAME         TYPE      PK
data  table  200   4     actor_id     INTEGER
                         first_name   TEXT
                         last_name    TEXT
                         last_update  DATETIME
```

Note the `TYPE` value for each column. This is the type of the column in the
[ingest DB](/docs/concepts/#ingest-db) table that `sq` ingests the CSV data into.
In this case, the ingest DB is actually a SQLite DB, and thus the `TYPE` is
SQLite data type `INTEGER`.

For the equivalent Postgres source, note the different `TYPE` value:

```shell
$ sq inspect @sakila/pg12.actor -v
NAME   TYPE   ROWS  COLS  NAME         TYPE       PK
actor  table  200   4     actor_id     int4       pk
                          first_name   varchar
                          last_name    varchar
                          last_update  timestamp
```

In the CSV case, the `actor_id` column ultimately maps to SQLite `INTEGER`,
while in Postgres, the type is `int4`. Because `sq` supports multiple backend
DB implementations, there is an internal `sq` representation of data types: _**kind**_.

If we run `inspect` again on the sources, but this time output in YAML, we can see
the kind for each column.

```yaml
# sq inspect --yaml @actor.data
name: data
table_type: table
table_type_db: table
row_count: 200
columns:
  - name: actor_id
    position: 0
    primary_key: false
    base_type: INTEGER
    column_type: INTEGER
    kind: int
    nullable: true
  - name: first_name
    position: 1
    primary_key: false
    base_type: TEXT
    column_type: TEXT
    kind: text
    nullable: true
# [Truncated for brevity]
```

```yaml
# sq inspect --yaml @sakila/pg12.actor
name: actor
name_fq: sakila.public.actor
table_type: table
table_type_db: BASE TABLE
row_count: 200
size: 73728
columns:
- name: actor_id
  position: 1
  primary_key: true
  base_type: int4
  column_type: integer
  kind: int
  nullable: false
  default_value: nextval('actor_actor_id_seq'::regclass)
- name: first_name
  position: 2
  primary_key: false
  base_type: varchar
  column_type: character varying
  kind: text
  nullable: false
# [Truncated for brevity]
```

Note that although the `column_type` values differ, both sources have the same `kind`
for each column.

### Kinds

`sq` defines a kind for the most common data types.

| Kind       | Example                |
|------------|------------------------|
| `text`     | `seven`                |
| `int`      | `7`                    |
| `float`    | `7.12344556`           |
| `decimal`  | `7.12`                 |
| `bool`     | `true`                 |
| `datetime` | `1977-11-07T17:59:28Z` |
| `date`     | `1977-11-07`           |
| `time`     | `17:59:28`             |
| `bytes`    | `aHV6emFoCg==`         |

{{< alert icon="👉" >}}
In addition to the kinds in the table above, there are a handful of other internal
kinds, e.g. `null` and `unknown`, but the end-user
is typically not exposed to them.
{{< /alert >}}


### Kind detection

When `sq` ingests untyped data, it samples multiple values from each column.
As `sq` iterates over those values, it's able to eliminate various kinds. For
example, if the sampled value is `1977-11-07`, the kind cannot be `int`, or `float`,
or `bool`, etc. By a process of elimination, `sq` ends up with a column kind.
If there's ambiguity in the sample data, the determined kind will be `text`.

In practice, `sq` runs the sample data through a series of parsing functions. If
a parsing function fails, that kind is eliminated. For some kinds there are
multiple parsing functions to try. In particular, there are many date and time formats
that `sq` can parse.


### Date/time formats

Listed below are the various `datetime`, `date`, and `time` formats
that `sq` can detect. Note that some formats have unlisted variants
to accept timezones, or various degrees
of precision: e.g. RFC3339 can accept second precision (e.g. `2006-01-02T15:04:05`),
all the way down to nanosecond precision (`2006-01-02T15:04:05.999999999Z`).

| Kind       | Example                                                                 | Note                                                                                              |
|:-----------|-------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------|
| `datetime` | <code>2006-01-02T15:04:05Z</code>                                       | [RFC3339](https://datatracker.ietf.org/doc/html/rfc3339)                                          |
| `datetime` | <code>Mon&nbsp;Jan&nbsp;2&nbsp;15:04:05&nbsp;2006</code>                | ANSI C                                                                                            |
| `datetime` | <code>Mon&nbsp;Jan&nbsp;&nbsp;2&nbsp;15:04:05&nbsp;MST&nbsp;2006</code> | Unix date                                                                                         |
| `datetime` | <code>Mon&nbsp;Jan&nbsp;02&nbsp;15:04:05&nbsp;-0700&nbsp;2006</code>    | Ruby date                                                                                         |
| `datetime` | <code>02&nbsp;Jan&nbsp;06&nbsp;15:04&nbsp;MST</code>                    | [RFC822](https://datatracker.ietf.org/doc/html/rfc822)                                            |
| `datetime` | <code>Monday,&nbsp;02-Jan-06&nbsp;15:04:05&nbsp;MST</code>              | [RFC850](https://datatracker.ietf.org/doc/html/rfc850)                                            |
| `datetime` | <code>Mon,&nbsp;02&nbsp;Jan&nbsp;2006&nbsp;15:04:05&nbsp;MST</code>     | [RFC1123](https://datatracker.ietf.org/doc/html/rfc1123)                                          |
| `datetime` | <code>Monday,&nbsp;January&nbsp;2,&nbsp;2006</code>                     | Excel long date                                                                                   |
| `datetime` | <code>2006-01-02&nbsp;15:04:05</code>                                   |                                                                                                   |
| `datetime` | <code>01/2/06&nbsp;15:04:05</code>                                      |                                                                                                   |
| `date`     | <code>2006-01-02</code>                                                 |                                                                                                   |
| `date`     | <code>02&nbsp;Jan&nbsp;2006</code>                                      |                                                                                                   |
| `date`     | <code>2006/01/02</code>                                                 |                                                                                                   |
| `date`     | <code>01-02-06</code>                                                   | This is _month-day-year_. Try to avoid this format due to _month-first_ vs _day-first_ confusion. |
| `date`     | <code>01-02-2006</code>                                                 | Also _month-day-year_: try to avoid.                                                              |
| `date`     | <code>02-Jan-2006</code>                                                |                                                                                                   |
| `date`     | <code>2-Jan-2006</code>                                                 |                                                                                                   |
| `date`     | <code>2-Jan-06</code>                                                   |                                                                                                   |
| `date`     | <code>Jan&nbsp;2,&nbsp;2006</code>                                      |                                                                                                   |
| `date`     | <code>Monday,&nbsp;January&nbsp;2,&nbsp;2006</code>                     |                                                                                                   |
| `date`     | <code>Mon,&nbsp;January&nbsp;2,&nbsp;2006</code>                        |                                                                                                   |
| `date`     | <code>Mon,&nbsp;Jan&nbsp;2,&nbsp;2006</code>                            |                                                                                                   |
| `date`     | <code>January&nbsp;2,&nbsp;2006</code>                                  |                                                                                                   |
| `date`     | <code>2/Jan/06</code>                                                   |                                                                                                   |
| `time`     | <code>15:04:05</code>                                                   |                                                                                                   |
| `time`     | <code>15:04</code>&nbsp;                                                |                                                                                                   |
| `time`     | <code>3:04PM</code>                                                     |                                                                                                   |
| `time`     | <code>3:04&nbsp;PM</code>                                               |                                                                                                   |
| `time`     | <code>3:04pm</code>                                                     |                                                                                                   |
| `time`     | <code>3:04&nbsp;pm</code>                                               |                                                                                                   |

{{< alert icon="👉" >}}
If a `datetime` format does not specify a timezone/offset, the value is
ingested as [UTC](https://en.wikipedia.org/wiki/Coordinated_Universal_Time).
{{< /alert >}}

{{< alert icon="👉" >}}
Note that these date & time formats are for data ingestion. Date & time
output formats are controlled [by](/docs/config/#formatdatetime)
[config](/docs/config/#formatdate) [options](/docs/config/#formattime).
{{< /alert >}}
