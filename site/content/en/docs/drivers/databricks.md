---
title: "Databricks"
description: "Databricks driver"
draft: false
images: []
weight: 4038
toc: true
url: /docs/drivers/databricks
---

The `sq` Databricks driver implements connectivity for
[Databricks SQL warehouses](https://docs.databricks.com/aws/en/compute/sql-warehouse/).
It uses the official [databricks-sql-go](https://github.com/databricks/databricks-sql-go)
driver. You can query, inspect and join Databricks (Unity Catalog) data
alongside `sq`'s other sources.

{{< alert icon="🧪" >}}
The Databricks driver is experimental, and it is **read-only**. It queries,
inspects and joins, but `sq` does not create, alter, copy, truncate or drop
tables, or insert rows. You can still run native SQL for those, via
[`sq sql`](/docs/cmd/sql). See [Read-only](#read-only).

The driver is new, and has not yet been tested against a wide range of
workspaces. Behavior may change as that improves. If you find a bug, please open an
[issue](https://github.com/neilotoole/sq/issues/new/choose).
{{< /alert >}}

The driver connects to the HTTP endpoint of a SQL warehouse, not to a
self-hosted server. You need the warehouse's server hostname and HTTP path,
which are on the **Connection details** tab of the warehouse in the Databricks UI,
and a credential.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source. The location starts with
`databricks://`. The username is always the literal `token`, and the password is
a Databricks [personal access token](https://docs.databricks.com/aws/en/dev-tools/auth/pat)
(PAT). The path is the warehouse's HTTP path.

```shell
sq add 'databricks://token:dapi0123456789abcdef@dbc-a1b2c3d4-e5f6.cloud.databricks.com/sql/1.0/warehouses/abc123def456' \
  --handle @dbx
```

Use `-p` to be prompted for the token, instead of putting it in the command
line or your shell history:

```shell
sq add -p 'databricks://token@dbc-a1b2c3d4-e5f6.cloud.databricks.com/sql/1.0/warehouses/abc123def456' --handle @dbx
Password: ****
```

To keep the token out of the config file, store it in the OS keyring with
`--store keyring`. See [`sq add`](/docs/cmd/add#storing-secrets-in-the-os-keyring).

## Connection string format

```text
databricks://token:PAT@HOST[:PORT]/HTTP_PATH[?param=value&...]
```

| Part        | Notes                                                                                                       |
| ----------- | ----------------------------------------------------------------------------------------------------------- |
| `token:PAT` | The username must be `token`. The password is the personal access token.                                    |
| `HOST`      | The workspace server hostname. The default port is `443`.                                                   |
| `HTTP_PATH` | The warehouse path, such as `/sql/1.0/warehouses/abc123def456`.                                             |
| `?param`    | Optional. See below. Params that `sq` doesn't know are passed to the driver as Databricks session settings. |

The HTTP path can instead be given as the `http_path` param, which is handy if
you copy it from the Databricks UI:

```shell
sq add 'databricks://token:PAT@HOST?http_path=/sql/1.0/warehouses/abc123def456'
```

If you give both, they must be the same.

The connection is always TLS (`https`), with one exception: a loopback host
(`localhost`, `127.0.0.1` or `::1`) uses plain `http`. A real workspace is never
on a loopback address, so that rule only matters for local proxies and test
servers.

### Parameters

| Param                | Notes                                                                                                |
| -------------------- | ---------------------------------------------------------------------------------------------------- |
| `catalog`            | The initial catalog. Defaults to the workspace default, often `main`.                                |
| `schema`             | The initial schema. Defaults to the workspace default, usually `default`.                            |
| `http_path`          | The warehouse HTTP path, as an alternative to the URL path.                                          |
| `maxRows`            | Rows fetched per request. Default `100000`.                                                          |
| `timeout`            | Server-side query timeout in seconds. Default: none.                                                 |
| `useCloudFetch`      | Fetch large results in parallel from cloud storage. Default `true`.                                  |
| `maxDownloadThreads` | Concurrent Cloud Fetch downloads. Default `10`.                                                      |
| `enableTelemetry`    | `false` turns off the Databricks driver's own telemetry. If unset, a Databricks server flag decides. |
| `timezone`           | The session time zone, such as `America/Los_Angeles`. `TIMESTAMP` values are shown in this zone.     |
| `authType`           | `Pat`, `OauthM2M` or `OauthU2M`. Usually inferred. See [Authentication](#authentication).            |
| `clientID`           | OAuth M2M client ID.                                                                                 |
| `clientSecret`       | OAuth M2M client secret.                                                                             |
| `accessToken`        | An access token, as an alternative to the `token:PAT@` userinfo.                                     |

The [databricks-sql-go documentation](https://github.com/databricks/databricks-sql-go/blob/main/CONNECTION_PARAMETERS.md)
lists every parameter the underlying driver accepts. Any other param becomes a
[session configuration](https://docs.databricks.com/aws/en/sql/language-manual/sql-ref-parameters),
so a misspelled param is not reported by `sq`.

## Authentication

The driver supports these methods:

- **Personal access token:** `databricks://token:PAT@HOST/HTTP_PATH`. This is the
  method the driver is built around.
- **OAuth machine-to-machine (M2M),** for service principals. Leave out the
  userinfo, and give the principal's client ID and secret:

  ```shell
  sq add 'databricks://HOST/sql/1.0/warehouses/abc123def456?clientID=ID&clientSecret=SECRET'
  ```

  `sq` hands these to databricks-sql-go, which fetches and refreshes the OAuth
  token. The secret is redacted in `sq`'s output and logs, like a password.

OAuth user-to-machine (`authType=OauthU2M`, an interactive browser login) is
passed through to the underlying driver, but is not a documented or tested
`sq` workflow. Reading credentials from `~/.databrickscfg` is not supported.

## Warehouse start-up

A stopped warehouse has to start before it can run a query, which can take
minutes, depending on the warehouse type. When `sq` opens a source to run a command,
it gives up after [`conn.open-timeout`](/docs/config#connopen-timeout), which
defaults to 10 seconds. If you get a timeout because the warehouse was stopped,
start the warehouse first, or raise the timeout for the source:

```shell
sq config set --src @dbx conn.open-timeout 3m
```

## Notes

### Catalog and schema

Databricks has a three-level namespace, `catalog.schema.table`. That maps onto
`sq`'s [catalog and schema](/docs/concepts/#schema--catalog) concepts. The
active schema and catalog are those of the connection: the `catalog` and
`schema` params of the location, or else the workspace defaults.

Use `--src.schema` to override them for one command. The value can be `schema`
or `catalog.schema`:

```shell
sq --src.schema=samples.nyctaxi '.trips | .[0:3]'
```

To name a table in another schema without changing the active one, use SQL:

```shell
sq sql 'SELECT * FROM samples.nyctaxi.trips LIMIT 3'
```

Identifiers are case-insensitive in Databricks, and `sq` quotes them with backticks.

### Read-only

The driver implements the read path: `sq` queries, [`sq inspect`](/docs/inspect),
[`sq ping`](/docs/cmd/ping), [`sq diff`](/docs/diff), and joins that use a
Databricks source as an input. It does not write to the source, so these fail
with a "read-only" error:

- `sq tbl copy`, `sq tbl truncate`, `sq tbl drop`
- `--insert`, when the destination is a Databricks source. This fails before any
  table is created, because opening a transaction on the destination is not
  supported by Databricks.

[`sq sql`](/docs/cmd/sql) is not restricted: it sends your SQL to the warehouse,
so you can run DDL and DML yourself, one statement at a time:

```shell
sq sql --src=@dbx 'CREATE TABLE main.sakila.t (id BIGINT, name STRING)'
sq sql --src=@dbx "$(cat statement.sql)"
```

Writing is not supported as a first-class feature because it needs more
design for Databricks: tables are Delta tables created with an explicit
format, inserts are slow over the HTTP API, and a warehouse has no
multi-statement transactions.

### Rows affected

`sq` reports the number of rows affected by `INSERT`, `UPDATE` and `DELETE` as
unsupported, which is `-1` in JSON and YAML output. The underlying driver has a
rows-affected count, but it has not been confirmed to be populated by Databricks
SQL warehouses, so `sq` does not rely on it.

### Type mapping

The Databricks type system is richer than `sq`'s [kind system](/docs/concepts),
so some coarsening occurs. Types are matched without their parameters, so
`DECIMAL(10,2)` and `VARCHAR(20)` map as `DECIMAL` and `VARCHAR`.

| Databricks type                        | sq kind         | Notes                                              |
| -------------------------------------- | --------------- | -------------------------------------------------- |
| `BOOLEAN`                              | `kind.Bool`     |                                                    |
| `TINYINT`, `SMALLINT`, `INT`, `BIGINT` | `kind.Int`      |                                                    |
| `FLOAT`, `DOUBLE`                      | `kind.Float`    |                                                    |
| `DECIMAL(P,S)`                         | `kind.Decimal`  | Exact, as the driver returns decimals as strings.  |
| `STRING`, `VARCHAR(N)`, `CHAR(N)`      | `kind.Text`     |                                                    |
| `BINARY`                               | `kind.Bytes`    |                                                    |
| `DATE`                                 | `kind.Date`     |                                                    |
| `TIMESTAMP`, `TIMESTAMP_NTZ`           | `kind.Datetime` | `TIMESTAMP` is shown in the session time zone.     |
| `ARRAY`, `MAP`, `STRUCT`, `VARIANT`    | `kind.Text`     | Rendered as JSON text. The structure is flattened. |
| `INTERVAL`                             | `kind.Text`     | For example `1-2` or `1 02:03:04.000000000`.       |
| `GEOGRAPHY`, `GEOMETRY`                | `kind.Text`     | Rendered as WKT.                                   |
| `VOID`                                 | `kind.Null`     |                                                    |

`sq` has no array or struct kind, so `ARRAY`, `MAP`, `STRUCT` and `VARIANT` values
become text and cannot be rebuilt from it. This is the same limit as
ClickHouse arrays ([#545](https://github.com/neilotoole/sq/issues/545)). Any other
type is reported as `kind.Unknown`, and a warning is logged.

The driver does not report column nullability for query results, so every
result column is treated as nullable. [`sq inspect`](/docs/inspect) does report
nullability, from the catalog.

### Inspect

[`sq inspect`](/docs/inspect) reads the catalog's `information_schema`, which
exists for Unity Catalog. For a source whose current catalog is the legacy
`hive_metastore`, `sq inspect` and the table listing commands report an
error: Hive metastore has no `information_schema`. Queries against
`hive_metastore` tables still work. Set the `catalog` param to a Unity Catalog
catalog to inspect it.

| Field                     | Source                                                                    |
| ------------------------- | ------------------------------------------------------------------------- |
| `catalog`, `schema`       | `current_catalog()`, `current_schema()`                                   |
| `user`                    | `current_user()`                                                          |
| `db_version`, `db_semver` | `version()`, the Spark version of the warehouse, not a Databricks release |
| tables, views, `comment`  | `information_schema.tables`                                               |
| columns                   | `information_schema.columns`                                              |
| `view_definition`         | `information_schema.views`                                                |
| `primary_key`             | `information_schema.table_constraints` and `key_column_usage`             |
| `row_count`               | `SELECT COUNT(*)` on each table or view                                   |

{{< alert icon="👉" >}}
`sq inspect` runs a `SELECT COUNT(*)` for each table and view in the schema, up
to eight at a time. On a large schema that takes a while, and uses warehouse
compute. To inspect only the tables you need, use
`sq inspect @dbx.actor`.
{{< /alert >}}

Primary keys are shown, but Databricks does not enforce primary or foreign
keys: they are informational. Foreign keys, unique constraints, indexes and
table sizes are not reported.

### Query rendering

`sq` renders [SLQ](/docs/query) to Spark SQL, with these differences from the
other drivers:

- String literals escape `\` and `'` with a backslash, because Spark SQL does not
  read a doubled quote (`''`) as a quote.
- `sum()` is cast to `DECIMAL(38, 6)`, so it always returns a decimal. See
  [`sum`](/docs/query#sum).
- A row range with an offset but no limit renders as `OFFSET n`. A limit above
  2,147,483,647, which Spark can't take as a `LIMIT`, is dropped.

## Related

- [#698](https://github.com/neilotoole/sq/issues/698): the Databricks driver issue.
