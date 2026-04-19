---
title: "SQL Server"
description: "SQL Server"
draft: false
images: []
weight: 4030
toc: false
url: /docs/drivers/sqlserver
aliases:
- /docs/driver/sql-server
- /docs/drivers/sql-server
---
The `sq` SQL Server driver implements connectivity for
the Microsoft [SQL Server](https://www.microsoft.com/en-us/sql-server) and
[Azure SQL Edge](https://azure.microsoft.com/en-us/products/azure-sql/edge/) databases.
The driver implements all optional driver features.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source.  The location argument should
start with `sqlserver://`. For example:

```shell
sq add 'sqlserver://sakila:p_ssW0rd@localhost?database=sakila'
```

## Notes

### Active schema & catalog

SQL Server supports the concepts of [schema and catalog](/docs/concepts/#schema--catalog).

When executing a `sq` query, you can use `--src.schema` to specify the active schema
(or catalog.schema).

```shell
$ sq src
@sakila/ms19

$ sq --src.schema=INFORMATION_SCHEMA '.TABLES'
TABLE_CATALOG  TABLE_SCHEMA          TABLE_NAME              TABLE_TYPE
sakila         dbo                   rental                  BASE TABLE
sakila         dbo                   customer_list           VIEW
sakila         dbo                   film_list               VIEW
# output truncated ^^
```

When `--src.schema` is provided, `sq` explicitly renders the schema name in the generated SQL query.
So, the query above would be rendered to:

```sql
SELECT * FROM "INFORMATION_SCHEMA"."TABLES"
```

When `--src.schema` is not provided, `sq` doesn't explicitly render a schema
name in the SQL query. The following two queries return the same results,
but are rendered differently:

```sql
-- $ sq '.actor'
SELECT * FROM "actor"

-- $ sq --src.schema=dbo '.actor'
SELECT * FROM "dbo"."actor"
```

#### `schema()` builtin caveat

For other database implementations, such as [Postgres](/docs/drivers/postgres),
`sq` implements `--src.schema` by setting the default schema
when opening the DB connection, in addition to explicitly
rendering the schema name in the SQL query.
However, SQL Server [does not](https://stackoverflow.com/questions/48506918/is-it-possible-to-change-the-default-schema) support setting a default schema on a per-connection
basis; the default schema is a property of the DB user. Most of the time this different behavior
is moot. However, one consequence is that the [SLQ](/docs/concepts#slq) builtin `schema()` function always returns the
user's default schema, regardless of the value of `--src.schema`.

```shell
$ sq -H 'schema()'
dbo

$ sq -H --src.schema=INFORMATION_SCHEMA 'schema()'
dbo
```

