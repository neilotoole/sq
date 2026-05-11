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
edge-case support improve. So far the driver has been tested against a single
Oracle version (Oracle Database 23ai Free, via the
[`sakiladb/oracle`](https://github.com/sakiladb/oracle) image); broader
version coverage may come over time.
{{< /alert >}}

The driver uses the pure-Go [go-ora](https://github.com/sijms/go-ora) driver
and does not require
[Oracle Instant Client](https://www.oracle.com/database/technologies/instant-client.html),
[OCI](https://en.wikipedia.org/wiki/Oracle_Call_Interface), or [CGo](https://pkg.go.dev/cmd/cgo).

## Add source

Use [`sq add`](/docs/cmd/add) to add an Oracle source.

```shell
sq add 'oracle://user:password@host:1521/service_name'
```

For the [Sakila Oracle test image](https://hub.docker.com/repository/docker/sakiladb/oracle/general):

```shell
sq add 'oracle://sakila:p_ssW0rd@localhost:1521/SAKILA' --handle @sakila_ora
```

## Connection string format

Use URL-style locations:

```text
oracle://username:password@hostname:1521/service_name
oracle://username:password@hostname/service_name
```

Query parameters are passed through to `go-ora`. Useful examples include SSL,
wallet, trace, and timeout settings:

```shell
sq add 'oracle://user:password@host:1521/service_name?SSL=true'
sq add 'oracle://user:password@host:1521/service_name?CONNECTION%20TIMEOUT=30'
```

Quote the location string when it contains `?`, `&`, spaces, or shell-special
characters.

`TNSNAMES.ora` aliases and Kerberos are not handled by `sq` directly. Use an
Oracle URL location for `sq add`.

## Notes

### Schema and catalog

Oracle's catalog and schema concepts differ from other databases:

- **Catalog**: in 12c and later, Oracle is multitenant — a Container
  Database (CDB) hosts one or more Pluggable Databases (PDBs), and each
  PDB is functionally an independent database with its own schemas, users,
  and tables. The PDB is the catalog-equivalent. In a non-CDB or pre-12c
  deployment there is just one database, which serves the same role.
  In `sq`, `catalog()` returns `SYS_CONTEXT('USERENV', 'DB_NAME')`, which
  yields the PDB name in multitenant deployments and the database name
  otherwise.
- **Schema**: Oracle schemas are users. In `sq`, `schema()` returns the
  current Oracle schema (the connected user). `sq` can list schemas from
  `ALL_USERS`, but `CREATE SCHEMA` and `DROP SCHEMA` are not Oracle
  operations; create or drop Oracle users instead.

The connection URL's *service name* is a connection-time routing identifier
the listener uses to pick which database/PDB to attach you to. It is not
itself the catalog — once the session is established, `catalog()` reflects
the database/PDB the service routed you into.

Unquoted Oracle identifiers are stored uppercase. `sq` follows that convention
when rendering quoted identifiers, so table and column names created through
`sq` are typically visible in uppercase in Oracle metadata.

Cross-source operations such as `--insert=@dest.tbl` from an Oracle source
to a case-sensitive destination (Postgres, ClickHouse) translate column
names case-insensitively against the destination table's actual columns
before quoting them, so Oracle's UPPERCASE column names match the
destination's stored case (typically lowercase) transparently.

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

### Inspect field provenance

`sq inspect` populates the fields below from the Oracle data dictionary.
Every column listed is readable by an ordinary user — no DBA privileges
required.

#### Source-level fields

| Field | Source |
| --- | --- |
| `name`, `schema` | `SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')` |
| `catalog` | `SYS_CONTEXT('USERENV', 'DB_NAME')` (PDB name in multitenant; database name otherwise) |
| `user` | `SYS_CONTEXT('USERENV', 'SESSION_USER')` |
| `db_product` | `BANNER` from `V$VERSION` (the full descriptive string, e.g. `Oracle Database 23ai Free Release …`) |
| `db_version` | `VERSION_FULL` from `PRODUCT_COMPONENT_VERSION` (e.g. `23.26.1.0.0`); falls back to `V$INSTANCE.VERSION` (DBA-only) and finally to the banner |
| `size` | `SUM(bytes)` over `USER_SEGMENTS` — bytes occupied by segments owned by the connected user. The PDB- or database-wide equivalents (`DBA_DATA_FILES`) require DBA privileges and are not used. |

#### Per-table fields

| Field | Source |
| --- | --- |
| `row_count` (tables, materialized views) | `NUM_ROWS` from `USER_TABLES` / `USER_MVIEWS`, with a live `SELECT COUNT(*)` fallback when the dictionary value is NULL (it stays NULL until `DBMS_STATS` / `ANALYZE` has run) |
| `row_count` (views) | always live `SELECT COUNT(*)` — `USER_VIEWS` carries no row count |
| `size` (tables, materialized views) | `SUM(bytes)` from `USER_SEGMENTS` for the matching segment name |
| `size` (views) | not reported — views have no underlying segment |

### SQL rendering

Oracle SQL rendering differs from several other SQL drivers:

- Bind placeholders use Oracle's numbered form: `:1`, `:2`, `:3`, and so on.
- `rownum()` renders as the portable `row_number() OVER (ORDER BY ...)`
  window function, threading the query's `ORDER BY` through the window
  definition. Oracle's `ROWNUM` pseudo-column is intentionally not used
  because it is assigned at fetch time, *before* `ORDER BY` is applied,
  which silently produces wrong row numbers when the query also sorts.
- `catalog()` renders as `SYS_CONTEXT('USERENV', 'DB_NAME')`, which yields
  the PDB name in multitenant deployments and the database name in non-CDB
  deployments. `schema()` renders as `SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')`.
- `avg()` and `sum()` are wrapped in `CAST(... AS BINARY_DOUBLE)`. Oracle
  returns these aggregates as `NUMBER(38, 255)` regardless of operand type,
  which `sq` would otherwise classify as `int`; the cast pins the result
  to a float so fractional values scan cleanly. Tradeoff: integer-valued
  sums lose precision past ~15-17 significant digits — use raw SQL for
  lossless big-integer aggregation.
- Row ranges render using `OFFSET ... FETCH NEXT ... ROWS ONLY` for Oracle
  12c and newer. When a row range has no explicit sort, `sq` adds an
  Oracle-compatible `ORDER BY` expression before the row range.
- Scalar selections that need a table source use `FROM DUAL` where required.
- The `AS` keyword is stripped from table-alias positions in `FROM`/`JOIN`
  clauses (e.g. `FROM "tbl" AS "alias"` becomes `FROM "tbl" "alias"`).
  Oracle accepts `FROM tbl alias` but rejects `FROM tbl AS alias`. Column
  aliases (e.g. `SELECT col AS alias`) are unaffected.

### Type mapping

Common Oracle types map to `sq` kinds as follows:

| Oracle type | `sq` kind |
| --- | --- |
| `VARCHAR2`, `NVARCHAR2`, `CHAR`, `NCHAR`, `CLOB`, `NCLOB`, `ROWID` | `text` |
| `NUMBER(p,0)` where `p` is 1-19 | `int` |
| Other `NUMBER` values | `decimal` |
| `BINARY_FLOAT`, `BINARY_DOUBLE`, `FLOAT` | `float` |
| `DATE`, `TIMESTAMP`, `TIMESTAMP WITH TIME ZONE` | `datetime` |
| `BLOB`, `RAW`, `LONG RAW` | `bytes` |
| Interval types | `text` |

When `sq` creates Oracle tables, it uses Oracle-native equivalents such as
`NUMBER(19,0)` for `int`, `NUMBER(1,0)` for `bool`, `TIMESTAMP` for
`datetime`, and `BLOB` for `bytes`.

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
- **Boolean values**: Oracle has no database-wide boolean column type in the
  same sense as other SQL engines, so `sq` stores boolean columns as
  `NUMBER(1,0)`.
- **DATE and TIME round-tripping**: Oracle `DATE` includes time-of-day, and
  Oracle has no standalone time-only column type. A `date` or `time` column
  created by `sq` can inspect back as `datetime`.
- **Column type changes**: `sq tbl` column-kind alteration is not implemented
  for Oracle yet.

## Local Sakila database

For local development and integration tests, use
[`sakiladb/oracle`](https://github.com/sakiladb/oracle):

```shell
docker run -d -p 1521:1521 sakiladb/oracle:latest
sq add 'oracle://sakila:p_ssW0rd@localhost:1521/SAKILA' --handle @sakila_ora
```

The image uses Oracle Database Free with the Sakila sample schema. Startup can
take several minutes; wait until the database is accepting connections before
running `sq ping @sakila_ora` or integration tests.

### Requirements

No Oracle Instant Client is required. The driver speaks Oracle Net in pure Go.

## Related

- [Oracle driver README](https://github.com/neilotoole/sq/blob/master/drivers/oracle/README.md)
- [Sakila test databases](/docs/develop/sakila)
