# Microsoft SQL Server (`sqlserver` driver)

[SQL Server](https://www.microsoft.com/sql-server) and [Azure SQL Edge](https://azure.microsoft.com/products/azure-sql-edge/). Driver implements all optional `sq` features.

**Canonical docs:** [SQL Server driver](https://sq.io/docs/drivers/sqlserver/)

## Add a source

Location string should start with **`sqlserver://`**. Use [`sq add`](https://sq.io/docs/cmd/add):

```shell
sq add 'sqlserver://user:password@localhost?database=dbname'
```

## Schema and catalog

SQL Server uses [schema and catalog](https://sq.io/docs/concepts/#schema--catalog) concepts. Use **`--src.schema`** when querying to target a schema (e.g. `INFORMATION_SCHEMA`, `dbo`).

The SLQ **`schema()`** builtin reflects the **database user’s default schema**, not necessarily `--src.schema` (differs from Postgres). Details: [SQL Server driver notes](https://sq.io/docs/drivers/sqlserver/).
