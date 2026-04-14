---
title: "CSV & friends"
description: "CSV & friends"
draft: false
images: []
weight: 4050
toc: true
url: /docs/drivers/csv
---
The `sq` CSV driver implements connectivity for [CSV](https://en.wikipedia.org/wiki/Comma-separated_values)
and variants, such as [TSV](https://en.wikipedia.org/wiki/Tab-separated_values), pipe-delimited, etc..

Note that the CSV data sources are read-only. That is to say, while you can query the CSV
source as if it were a SQL table, you cannot insert values into the CSV source.

## Add source

When adding a CSV source via [`sq add`](/docs/cmd/add), the location string is simply the filepath.
For example:

```shell
$ sq add ./actor.csv
@actor_csv  csv  actor.csv
```

You can also pass an absolute filepath (and, in fact, any relative path is expanded to
an absolute path when saved to `sq`'s config).

Usually you can omit the `--driver=csv` flag, because `sq` will inspect the file contents
and [detect](/docs/detect/#driver-type) that it's a CSV file. However, it's safer to explicitly specify the flag.

```shell
sq add --driver=csv ./actor.csv
```

The same is true for TSV files. You can specify the driver explicitly:

```shell
$ sq add ./actor.tsv --driver=tsv
@actor_tsv  tsv  actor.tsv
```

But, if you omit the driver, `sq` can generally figure out that it's a TSV file.

```shell
sq add ./actor.tsv
```

{{< alert icon="👉" >}}
CSV is a [document source](/docs/source#document-source) and thus its data
is [ingested](/docs/source#ingest) and [cached](/docs/source#cache).

Note also that a CSV source is read-only; you can't [insert](/docs/output#insert)
values into the source.
{{< /alert >}}

## Monotable

`sq` considers CSV to be a _monotable_ data source (unlike, say, a Postgres data source, which
obviously can have many tables). Like all other `sq` monotable sources,
the source's data is accessed via the synthetic `.data` table. For example:

```shell
$ sq @actor_csv.data
actor_id  first_name   last_name     last_update
1         PENELOPE     GUINESS       2020-02-15T06:59:28Z
2         NICK         WAHLBERG      2020-02-15T06:59:28Z
```

## Delimiters

It's common to encounter delimiters other than comma. TSV (tab) is the most common, but other
variants exist, e.g. pipe (`a|b|c`). Use the `--driver.csv.delim` flag to specify
the delimiter. Because the delimiter is often a shell token (e.g. `|`), the `delim` option
requires text aliases. For example:

```shell
sq add ./actor.csv --driver.csv.delim=pipe
```

The accepted values are:

| Delim    | Value                     |
|----------|---------------------------|
| `comma`  | `,`                       |
| `space`  | <code>&nbsp;</code>       |
| `pipe`   | <code>&vert;</code>       |
| `tab`    | <code>&nbsp;&nbsp;</code> |
| `colon`  | `:`                       |
| `semi`   | `;`                       |
| `period` | `.`                       |

Note:

- `comma` is the default. You generally never need to specify this.
- `tab` is the delimiter for TSV files. Because this is such a common variant, `sq` allows
  you to specify `--driver=tsv` instead. But usually `sq` will [detect](/docs/detect/#driver-type) that it's a TSV file.
  The following are equivalent:

  ```shell
  $ sq add --driver=tsv ./actor.tsv
  $ sq add --driver=csv --driver.csv.delim=tab ./actor.tsv
  $ sq add ./actor.tsv
  ```

## Header row

CSV files will often have a header row containing column names. If the sheet
doesn't have a header row, by default `sq` will name the columns `A`, `B`, `C`, etc.
(Note that the column naming behavior is [configurable](/docs/config/#ingestcolumnrename).

Generally, `sq` will automatically [detect](/docs/detect)
whether or not the first row of a CSV file is a header row. If the header row detection
is having difficulty with your CSV file, you can explicitly specify that a
header row is present (or not) via [`--ingest.header`](/docs/config/#ingestheader).

```shell
# Explicitly specify that a header row exists
$ sq add --ingest.header ./actor_header.csv

# Explicitly specify no header row
$ sq add --ingest.header=false ./actor_no_header.csv
```

### Duplicate columns

If the header row has duplicate column names, the later columns are renamed.
For example, these columns:

```text
actor_id, first_name, actor_id
```

become:

```text
actor_id, first_name, actor_id_1
```

The renaming behavior is configurable via [`ingest.column.rename`](/docs/config#ingestcolumnrename).


## Column kind

When ingesting a CSV file, `sq` attempts to detect the data ["kind"](/docs/detect/#column-kind)
of each column (`int`, `float`, `text`, etc.). Thus a CSV date string such as `1989-11-09`
becomes a date in the backing DB, a number string becomes an int or float,
and various date & time values are parsed
into an appropriate DB type. See the [column kind section](/docs/detect/#column-kind)
for more on this mechanism.
