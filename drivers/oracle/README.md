# Oracle Database Driver for SQ

Oracle database driver implementation for SQ using pure Go
[go-ora](https://github.com/sijms/go-ora) (`database/sql` driver name `oracle`).
No Oracle Instant Client or CGO is required for Oracle connectivity.

## Status

Core implementation complete (inspect, sql, SLQ, table operations, Sakila test
integration via [`sakiladb/oracle`](https://github.com/sakiladb/oracle)).

## Features Implemented

### Core Driver Features

- Provider and driver registration
- Connection management via [go-ora](https://github.com/sijms/go-ora)
- Oracle-specific SQL dialect (`:1, :2, :3` placeholders, double-quote
  identifiers)
- Error handling for selected Oracle error codes (ORA-xxxxx)

### Type System

- Bidirectional type mapping (Oracle types ↔ `kind.Kind`)
- Support for: NUMBER, VARCHAR2, CHAR, CLOB, BLOB, DATE, TIMESTAMP,
  BINARY_FLOAT, BINARY_DOUBLE
- BOOLEAN emulation using NUMBER(1,0); dialect sets `IntBool` because values
  scan as integers

### Metadata Operations

- `CurrentSchema()` via `SYS_CONTEXT`
- `ListSchemas()` via `all_users`
- Schema inspection via `USER_TABLES`, `USER_VIEWS`, `USER_MVIEWS`,
  `USER_TAB_COLUMNS`, `USER_CONSTRAINTS`
- Primary key detection
- Schema-scoped `ListTableNames()` via `ALL_TABLES` / `ALL_MVIEWS` /
  `ALL_VIEWS` (`owner = :schema`)

### DDL / DML / Query

- Create/drop/alter/truncate/copy patterns aligned with other SQL drivers
- Batch insert / update preparation
- `TableColumnTypes` / `RecordMeta`

## Connection string format

Use URL locations only (same style as `sq add`):

```bash
oracle://username:password@hostname:1521/service_name
oracle://username:password@hostname/service_name
```

Optional query parameters follow go-ora URL rules (SSL, traces, timeouts); see
[go-ora](https://github.com/sijms/go-ora). `TNSNAMES.ora`, Oracle Wallet, and
Kerberos are out of scope for this driver.

## Testing

**Quick start:**

```bash
cd drivers/oracle

# Unit tests only (no database)
go test -v -short

# Integration tests (Docker; pulls sakiladb/oracle via compose)
./testutils/test-integration.sh

# Include Postgres for cross-database tests
./testutils/test-integration.sh --with-pg
```

### `testh` / repo-wide tests

Set `SQ_TEST_SRC__SAKILA_ORA` to the part of the DSN after
`oracle://sakila:p_ssW0rd@` (for example `localhost:1521/FREEPDB1`), matching
[`testh/testdata/sources.sq.yml`](../../testh/testdata/sources.sq.yml) handle
`@sakila_ora`. Recommended database image:
[`sakiladb/oracle`](https://github.com/sakiladb/oracle) (`docker run -p
1521:1521 sakiladb/oracle:latest`).

Details: **[Testing.md](./testutils/Testing.md)**

### Test package layout

[`internal_test.go`](./internal_test.go) stays in `package oracle` to cover
unexported helpers (`placeholders`, `kindFromOracleNumber`, …).
[`oracle_test.go`](./oracle_test.go) uses `package oracle_test` for integration
tests. This differs from some other drivers; it keeps helper coverage without
exporting test-only symbols.

## Oracle-specific notes

### Quirks (transactions, DDL, TRUNCATE, defaults)

- **Catalog**: Oracle has schemas, not catalogs; `catalog()` renders `NULL`.
- **Transactions**: Ordinary `database/sql` semantics; DDL commits an open
  transaction.
- **`sq tbl ... --truncate`**: Oracle does not reset sequences via `TRUNCATE`;
  the driver's `reset` option maps to `TRUNCATE TABLE ... DROP STORAGE` vs
  `REUSE STORAGE`.
- **Empty strings**: treated as `NULL`.
- **`CREATE TABLE`**: Avoid unsupported defaults (Oracle rejects some literals
  other databases allow).

### Key differences (summary)

1. **Schema = user** — no separate `CREATE SCHEMA`.
2. **DATE** includes time.
3. **`FROM DUAL`** for scalar selects (handled in rendered SQL where needed).
4. **Quoted identifiers** — uppercase quoting matches Oracle conventions.
5. **NUMBER** mapping uses dictionary precision/scale and `DecimalSize()` for
   result sets.
6. **Synonyms** — not resolved yet.

## Implementation files

| File | Purpose |
| ---- | ------- |
| `oracle.go` | `SQLDriver`, connection, DDL/DML |
| `metadata.go` | Data dictionary queries |
| `render.go` | Type mapping and rendering |
| `grip.go` | Grip |
| `errors.go` | Delegates to `orshared` |
| `orshared/wrap.go` | Shared Oracle error-code wrapping |
| `internal_test.go` | Short/unit tests |
| `testutils/docker-compose.yml` | Local Oracle + Postgres |

## Common Oracle error codes

| Code | Description | SQ handling |
| ---- | ------------- | ----------- |
| ORA-00942 | Table/view not found | `NotExistError` |
| ORA-00904 | Invalid identifier | `NotExistError` |

Other errors pass through with standard wrapping.

## Requirements

- Oracle Database (12c+; CI/examples use Oracle Database 23 Free via
  `sakiladb/oracle`)
- Go toolchain matching the main module

## Dependencies

- [`github.com/sijms/go-ora/v2`](https://github.com/sijms/go-ora)

## License

Same as the main SQ project.
