---
title: "Oracle"
description: "Oracle driver"
draft: false
images: []
weight: 4037
toc: true
url: /docs/drivers/oracle
---
The `sq` Oracle driver implements connectivity for
[Oracle Database](https://www.oracle.com/database/).

{{< alert icon="🧪" >}}
The Oracle driver is experimental. Behavior may change as test coverage and
edge-case support improve.
{{< /alert >}}

## Add source

Use [`sq add`](/docs/cmd/add) to add an Oracle source.

```shell
sq add 'oracle://user:password@host:1521/service_name'
```

## Connection string format

The backing driver is pure Go ([go-ora](https://github.com/sijms/go-ora)).
Use URL-style locations only:

```text
oracle://username:password@hostname:1521/service_name
oracle://username:password@hostname/service_name
```

`TNSNAMES.ora` aliases, Oracle Wallet, and Kerberos are not handled here.
Those setups typically require Oracle client tooling or another SQL CLI.

## Notes

### Schema and catalog

Oracle does not implement catalogs in the same sense as Postgres or SQL
Server. In `sq`, `catalog()` returns `NULL` for Oracle sources, and
`schema()` returns the current Oracle schema (which maps to the connected
user).

### Metadata visibility

`sq inspect` loads **base tables**, **views**, and **materialized views** from
`USER_*` dictionary views for the connected schema. View rows use
`TableType` `view`; materialized views use `TableType` `table` with
`DBTableType` `MATERIALIZED VIEW` (so they contribute to `TableCount`).

`ListTableNames(schema=...)` reads `ALL_TABLES`, `ALL_MVIEWS`, and `ALL_VIEWS`
filtered by owner. `TableExists` checks `USER_OBJECTS` for `TABLE`, `VIEW`, or
`MATERIALIZED VIEW`.

**Synonyms** (resolving through `ALL_SYNONYMS` to base objects, including DB
links) are not implemented yet.

`DBProperties` always returns `db_name` and `current_schema` from
`SYS_CONTEXT`. The `version` field prefers `v$instance` and falls back to
`v$version` when `v$instance` is not readable.

### Database-specific quirks

- **Transactions**: Same defaults as other SQL drivers via `database/sql`; DDL
  commits open transactions.
- **TRUNCATE** (`sq tbl` truncate): Oracle ignores identity sequence reset in
  the sense of other databases; the driver's `reset` flag maps to
  `DROP STORAGE` vs `REUSE STORAGE` on `TRUNCATE TABLE`.
- **Empty strings**: Oracle treats empty string as `NULL`.
- **`CREATE TABLE`**: Defaults avoid unsupported constructs (for example,
  `EMPTY_BLOB()` cannot be used as a literal default); Oracle rejects defaults
  some drivers accept elsewhere.

### Requirements

No Oracle Instant Client is required. The driver speaks Oracle Net in pure Go.

## Related

- [Oracle driver README](https://github.com/neilotoole/sq/blob/master/drivers/oracle/README.md)
