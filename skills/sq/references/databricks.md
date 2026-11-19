# Databricks (`databricks` driver)

[Databricks](https://www.databricks.com/) SQL warehouses (Unity Catalog), through the official
[databricks-sql-go](https://github.com/databricks/databricks-sql-go) driver.

**Canonical docs:** [Databricks driver](https://sq.io/docs/drivers/databricks/)

## Experimental and read-only

The Databricks driver is **experimental**, and **read-only**: it queries, inspects and joins, but
`sq` does not create, alter, copy, truncate or drop tables, or insert rows. `sq tbl copy`,
`sq tbl truncate`, `sq tbl drop` and `--insert` into a Databricks source fail. Native SQL via
[`sq sql`](https://sq.io/docs/cmd/sql) is not restricted, so run DDL and DML with it, one statement
at a time. Prefer reporting issues via [sq issues](https://github.com/neilotoole/sq/issues/new/choose).

## Add a source

Location string should start with **`databricks://`**. The username is always the literal `token`,
the password is a personal access token (PAT), and the path is the warehouse HTTP path (see the
warehouse's **Connection details** in the Databricks UI). Use [`sq add`](https://sq.io/docs/cmd/add)
with **`-p`** so the token is prompted rather than embedded in the URL:

```shell
sq add -p 'databricks://token@dbc-a1b2c3d4-e5f6.cloud.databricks.com/sql/1.0/warehouses/abc123' --handle @dbx
sq add -p 'databricks://token@HOST/sql/1.0/warehouses/abc123?catalog=main&schema=sakila' --handle @dbx
```

**Connection pattern:** `databricks://token:PAT@HOST[:PORT]/HTTP_PATH` with optional
`?catalog=...&schema=...`. The HTTP path may instead be the `http_path` query param. The default
port is 443, and the connection is TLS.

For a service principal, OAuth M2M is passed through to the driver: omit the userinfo, and give
`?clientID=ID&clientSecret=SECRET`.

## Behavior notes (summary)

- **Catalog and schema:** Databricks has `catalog.schema.table`, which maps onto `sq`'s
  [schema/catalog](https://sq.io/docs/concepts/#schema--catalog). Use `--src.schema=CATALOG.SCHEMA`
  or the `catalog` and `schema` params.
- **Warehouse start-up:** a stopped warehouse can take longer than the default 10s
  `conn.open-timeout` to start. Start it first, or `sq config set --src @dbx conn.open-timeout 3m`.
- **Inspect:** reads `information_schema`, so it needs a Unity Catalog catalog. The legacy
  `hive_metastore` catalog can be queried but not inspected. Primary keys are informational in
  Databricks, and foreign keys are not reported. Row counts come from `SELECT COUNT(*)` per table.
- **Types:** `ARRAY`, `MAP`, `STRUCT` and `VARIANT` become JSON text (`kind.Text`). See the mapping
  table on [sq.io](https://sq.io/docs/drivers/databricks/).
- **Strings:** literals are escaped with backslash, as Spark SQL requires.
- **Rows affected:** reported as unsupported (`-1`).

Results can be printed as text, JSON, CSV, HTML, Markdown, XML, XLSX, etc. See
[Output formats](https://sq.io/docs/output#formats).

Common flags: `-j`/`--json`, `-t`/`--text`, `-o FILE`; details in **`sq --help`** and the docs above.
