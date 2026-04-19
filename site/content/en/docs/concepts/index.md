---
title: "Concepts"
description: "Concepts & Terminology"
lead: ""
draft: false
images: []
weight: 1030
toc: true
url: /docs/concepts
aliases:
- /docs/terminology

---
## sq

`sq` is the command-line utility itself. It is free/libre open-source software, available
under the [MIT License](https://github.com/neilotoole/sq/blob/master/LICENSE). The code is
available on [GitHub](https://github.com/neilotoole/sq). `sq` was created
by [Neil O'Toole](https://github.com/neilotoole).

## SLQ

`SLQ` is the formal name of `sq`'s query language, similar to [jq](https://jqlang.github.io/jq/)'s syntax.
The [Antlr](https://www.antlr.org) grammar
is available on [GitHub](https://github.com/neilotoole/sq/tree/master/grammar).

## Source

A _source_ is a data source such as a database instance ([SQL source](#sql-source)),
or an Excel or CSV file ([document source](#document-source)).
A source has a [driver type](#driver-type), [location](#location) and [handle](#handle).

Learn more in the [sources](/docs/source) section.

### Driver type

This is the [driver](#driver) type used to connect to the source,
e.g. `postgres`, `sqlserver`, `clickhouse`, `csv`, etc. You can specify the type explicitly
when invoking [`sq add`](/docs/cmd/add), but usually `sq` can [detect](/docs/detect/#driver-type)
the driver automatically.

### Location

The source _location_ is the URI or file path of the source, such
as `postgres://sakila:****@localhost/sakila`
or `/Users/neilotoole/sq/xl_demo.xlsx`. You specify the source location when
invoking [`sq add`](/docs/cmd/add).

### Handle

The _handle_ is how `sq` refers to a data source, such as `@sakila` or `@customer_csv`.
The handle must begin with `@`. You specify the handle when adding a source with [`sq add`](/docs/cmd/add).
The handle can also be used to specify a source [group](/docs/source#groups), e.g. `@prod/sales`, `@dev/sales`.

## Active source

The _active source_ is the _source_ upon which `sq` acts if no other source is specified.

By default, `sq` requires that the first element of a query is the source handle:

```shell
$ sq '@sakila | .actor | .first_name, last_name'

```

But if an active source is set, you can omit the handle:

```shell
$ sq '.actor | .first_name, .last_name'
```

You can use [`sq src`](/docs/cmd/src) to get or set the active source.

## SQL source

A _SQL source_ is a source backed by a "real" DB, such as Postgres. Contrast
with [document source](#document-source).

## Document source

A [document source](/docs/source#document-source) is a source backed by a document or file such as [CSV](/docs/drivers/csv) or
[XLSX](/docs/drivers/xlsx). Some functionality
is not available for document sources. For example, `sq` doesn't provide a mechanism to insert query
results into a CSV file. Contrast with [SQL source](#sql-source). A document source's data
is automatically [ingested](/docs/source#ingest) by `sq`. The document source's location
can be a local filepath or an [HTTP URL](/docs/source#download).

## Group

The _group_ mechanism organizes sources into groups, based on path-like names.
Given handles `@prod/sales`, `@dev/sales` and `@dev/test`, we have three
sources, but two groups, `prod` and `dev`. See the [groups](/docs/source#groups) docs.

## Active group

Like [active source](#active-source), there is an active [group](/docs/source#groups). Use
the [`sq group`](/docs/cmd/group) command to get or set the active group.

## Driver

A `driver` is a software component implemented by `sq` for each data source type. For
example, [Postgres](/docs/drivers/postgres) or [CSV](/docs/drivers/csv).

Use [`sq driver ls`](/docs/cmd/driver-ls) to view the available drivers.

## Monotable

If a source is `monotable`, it means that the source type is really only a single table, such
as a [CSV](/docs/drivers/csv) file. `sq` always names that single table `data`. You access that
table
like this: `@actor_csv | .data`.

Note that not all document sources are _monotable_. For example, [XLSX](/docs/drivers/xlsx) sources
have multiple tables, where each worksheet is effectively equivalent to a DB table.

## Metadata

[`sq inspect`](/docs/cmd/inspect) returns metadata about a source. At a minimum, `sq inspect`
is useful for a quick reminder of table and column names:

![sq inspect source text](/images/sq_inspect_source_text.png)

`sq inspect` comes into its own when used with the `--json` flag, which outputs voluminous info
on the data source. It is a frequent practice to combine `sq inspect`
with [jq ](https://jqlang.github.io/jq/).
For example, to list the tables of the active source:

```shell
$ sq inspect -j | jq -r '.tables[] | .name'
actor
address
category
[...]
```

See more examples in the [cookbook](/docs/cookbook).

<a id="scratch-db"></a>
## Ingest DB

[Ingest DB](/docs/source#ingest) refers to the temporary ("_ingest_") database that `sq` uses for under-the-hood
activity such as converting a [document source](#document-source) like [CSV](/docs/drivers/csv) to relational
format. By default, `sq` uses an embedded [SQLite](/docs/drivers/sqlite) instance for the ingest DB.

## Join DB

_Join DB_ is similar to [Ingest DB](#ingest-db), but is used for cross-source joins. By default, `sq`
uses an embedded [SQLite](/docs/drivers/sqlite) instance for the Join DB.

## Schema & catalog

Database implementations have the concept of a _schema_, which
you can think of as a namespace for tables. Some databases go further and
support the concept of a _catalog_, which is a collection of schemas.
Often the terms _catalog_ and _database_ are used interchangeably, and, in practice,
the various terms are used inconsistently and confusedly.

{{< alert icon="👉" >}}
Above _catalog_, there's also the concept of a _cluster_,
which is the database server (logical or physical) that hosts catalogs. `sq` doesn't concern itself
with clusters.
{{< /alert >}}

Here's what the hierarchy looks like for Postgres ([credit](https://stackoverflow.com/a/17943883)):

![Hierarchy](db-hierarchy.png)

Each of the `sq` DB driver implementations supports the concept of a schema in some way,
but some drivers don't support the catalog mechanism. Here's a summary:

<a name="catalog-schema-support"></a>

| Driver                                | Default schema       | Catalog support?                                                                                              |
|---------------------------------------|----------------------|---------------------------------------------------------------------------------------------------------------|
| [Postgres](/docs/drivers/postgres)    | `public`             | Yes                                                                                                           |
| [SQLite](/docs/drivers/sqlite)        | `main`               | No                                                                                                            |
| [MySQL](/docs/drivers/mysql)          | Connection-dependent | [No](https://dev.mysql.com/doc/connector-odbc/en/connector-odbc-usagenotes-functionality-catalog-schema.html) |
| [SQL server](/docs/drivers/sqlserver) | `dbo`                | Yes                                                                                                           |
| [ClickHouse](/docs/drivers/clickhouse) | `default`            | Yes                                                                                                           |

The SLQ functions [`schema()`](/docs/query#schema) and [`catalog()`](/docs/query#catalog) return
the schema and catalog of the active source. See the docs for details of how each driver implements
these functions.

{{< alert icon="👉" >}}
You can override the active schema (and catalog) using the `--src.schema` flag
for the [`sq`](/docs/cmd/sq#override-active-schema), [`sql`](/docs/cmd/sql/#active-source--schema)
and [`inspect`](/docs/inspect#override-active-schema) commands.
{{< /alert >}}

