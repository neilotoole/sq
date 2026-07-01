# ClickHouse Driver for SQ

ClickHouse database driver implementation for SQ using
[clickhouse-go](https://github.com/ClickHouse/clickhouse-go) v2.

> [!WARNING]
> All testing has been performed on **ClickHouse v25+**.
> Behavior on versions below v25 is not guaranteed and is unsupported.

## Requirements

- **ClickHouse Server**: v25.0 or later (tested and supported)
- **Go**: matching the main module (see [`go.mod`](../../go.mod))
- **Docker**: required for integration tests
- **Dependency**: `github.com/ClickHouse/clickhouse-go/v2` (pinned in
  [`go.mod`](../../go.mod))

## Quick Start

```bash
# Add a source (with or without auth)
sq add clickhouse://default:@localhost:9000/default --handle @ch
sq add clickhouse://user:password@host:9000/database --handle @ch

sq inspect @ch                                    # Inspect
sq '.users | .id, .name | .[0:10]' @ch            # SLQ query
sq sql 'SELECT * FROM users LIMIT 10' @ch         # Native SQL
sq tbl copy @ch.users @ch.users_backup            # Table ops
```

## Connection String Format

```bash
clickhouse://username:password@hostname:9000/database
clickhouse://username:password@hostname:9000/database?param=value
clickhouse://default:@localhost:9000/default
```

**Default Ports:**

| Protocol | Non-Secure | Secure (TLS) |
| -------- | ---------- | ------------ |
| Native   | 9000       | 9440         |
| HTTP     | 8123       | 8443         |

> **Note**: The `clickhouse-go` driver does not apply default ports
> automatically (unlike `pgx` for Postgres). SQ applies the default port if
> not specified: **9000** for non-secure, **9440** for secure (`secure=true`).

## Type System

ClickHouse types are mapped to sq's `kind` type system. Because sq's kind
system is coarser than ClickHouse's native types, the mapping is not a perfect
round-trip: e.g. `Int8` and `Int64` both become `kind.Int`, and `kind.Int`
always maps back to `Int64`. Wrapper types are stripped before mapping, so
`LowCardinality(Nullable(String))` unwraps to `String` → `kind.Text`.

<!-- markdownlint-disable MD013 MD060 -->

### ClickHouse → sq (reading)

| ClickHouse Type                       | sq Kind         | Notes                              |
| ------------------------------------- | --------------- | ---------------------------------- |
| `Int8`/`Int16`/`Int32`/`Int64`        | `kind.Int`      | All signed integers                |
| `UInt8`/`UInt16`/`UInt32`/`UInt64`    | `kind.Int`      | All unsigned integers              |
| `Float32`, `Float64`                  | `kind.Float`    |                                    |
| `Decimal(P,S)`, `Decimal128(S)`, etc. | `kind.Decimal`  | All Decimal variants               |
| `Bool`                                | `kind.Bool`     |                                    |
| `String`, `FixedString(N)`, `UUID`    | `kind.Text`     |                                    |
| `Date`, `Date32`                      | `kind.Date`     |                                    |
| `DateTime`, `DateTime64`              | `kind.Datetime` | Including parameterized variants   |
| `Array(T)`                            | `kind.Text`     | Serialized as comma-separated text |
| `Enum8`/`Enum16`/`Map`/`Tuple`        | `kind.Text`     | Fallback to text                   |
| Unknown types                         | `kind.Text`     | Safe fallback                      |

### sq → ClickHouse (writing)

| sq Kind                     | ClickHouse Type | Notes                            |
| --------------------------- | --------------- | -------------------------------- |
| `kind.Text`                 | `String`        |                                  |
| `kind.Int`                  | `Int64`         |                                  |
| `kind.Float`                | `Float64`       |                                  |
| `kind.Decimal`              | `Decimal(18,4)` |                                  |
| `kind.Bool`                 | `Bool`          |                                  |
| `kind.Date`                 | `Date`          |                                  |
| `kind.Datetime`             | `DateTime`      |                                  |
| `kind.Time`                 | `DateTime`      | ClickHouse has no time-only type |
| `kind.Bytes`                | `String`        | Binary data stored as String     |
| `kind.Unknown`, `kind.Null` | `String`        |                                  |

<!-- markdownlint-enable MD013 MD060 -->

Nullable columns are wrapped with `Nullable(T)`; ClickHouse columns are
non-nullable by default. ClickHouse has no dedicated binary/`BLOB` type:
`String` holds arbitrary bytes, so `String`/`FixedString` map to `kind.Text`
and a `kind.Bytes` value reads back as `kind.Text`. This is intentional
upstream
([ClickHouse/ClickHouse#53482](https://github.com/ClickHouse/ClickHouse/issues/53482)),
not a gap awaiting a fix. See [#544](https://github.com/neilotoole/sq/issues/544).

## ClickHouse-Specific Behavior

- **Table engine**: `CreateTable` uses `ENGINE = MergeTree()` with `ORDER BY`
  on the first column.
- **Schema/catalog**: a ClickHouse "database" maps to sq's schema/catalog;
  metadata comes from `system.databases`, `system.tables`, `system.columns`.
- **Transactions**: OLAP-optimized, no traditional ACID. clickhouse-go's
  `BeginTx`/`Commit` are effectively no-ops; inserts are atomic at batch level.
- **Batch insert**: uses clickhouse-go's native Batch API
  (`PrepareBatch`/`Append`/`Send`) in [`batch.go`](./batch.go), not the generic
  multi-row `INSERT` path (which clickhouse-go rejects). The native connection
  is opened separately because `*sql.DB` doesn't expose `PrepareBatch`.
- **Rows affected**: ClickHouse reports `0` for all DML. sq surfaces this as
  `dialect.RowsAffectedUnsupported` (-1); text output shows
  "rows affected: unsupported".

### Mutations are asynchronous by default

ClickHouse has no standard `UPDATE`/`DELETE`; it uses "lightweight mutations"
via `ALTER TABLE ... UPDATE/DELETE`, which are **asynchronous by default**:
the statement returns before rows change, so a subsequent `SELECT` may see
stale data.

To keep sq correct, `PrepareUpdateStmt` appends `SETTINGS mutations_sync = 1`
so the mutation completes before the call returns. It also bypasses
`PrepareContext()` (clickhouse-go misclassifies non-`SELECT` as `INSERT` and
rejects `ALTER TABLE ... UPDATE`; see
[clickhouse-go#1203](https://github.com/ClickHouse/clickhouse-go/issues/1203))
by calling `ExecContext()` directly via a `StmtExecFunc` closure with a `nil`
stmt. When no `WHERE` is given, `buildUpdateStmt` emits `WHERE 1` (ClickHouse
requires a `WHERE`, else the `SETTINGS` clause lands in an invalid position).

Raw `UPDATE`/`DELETE` via `sq sql` work on tables created with lightweight
mutation settings (`enable_block_number_column = 1`,
`enable_block_offset_column = 1`).

## Testing

```bash
cd drivers/clickhouse

# Unit tests only (no database)
go test -v -short

# Integration tests: start sakiladb/clickhouse, then point the harness at it
docker run -d -p 9000:9000 sakiladb/clickhouse:latest
export SQ_TEST_SRC__SAKILA_CH='clickhouse://sakila:p_ssW0rd@localhost:9000/sakila'
go test -v
```

The integration tests run against the shared `@sakila_ch` handle; see
[`../README.md`](../README.md) for the cross-driver test-handle setup.

## Known Limitations

<!-- markdownlint-disable MD013 -->

| # | Limitation                                   | Follow-up                                           |
| - | -------------------------------------------- | --------------------------------------------------- |
| 1 | `kind.Time` reads back as `kind.Datetime`    | [#544](https://github.com/neilotoole/sq/issues/544) |
| 2 | `kind.Bytes` reads back as `kind.Text`       | [#544](https://github.com/neilotoole/sq/issues/544) |
| 3 | `Array(T)` flattened to comma-separated text | [#545](https://github.com/neilotoole/sq/issues/545) |

<!-- markdownlint-enable MD013 -->

For #1/#2, `TestDriver_CreateTable_Minimal` and `TestOutputRaw` are skipped for
ClickHouse (the `kind.Bytes` data is preserved on disk, since `String` is
binary-safe, but the Go type and sq kind change on readback).

### Array handling (#3)

sq's kind system has no `kind.Array`, and `record.Valid()` restricts values to
8 primitive Go types (`nil`, `int64`, `float64`, `decimal.Decimal`, `bool`,
`string`, `[]byte`, `time.Time`). ClickHouse `Array(T)` columns are therefore
mapped to `kind.Text` in `metadata.go`: their scan type is overridden to
`sqlz.RTypeAny`, and `getNewRecordFunc()` converts the resulting Go slice to a
comma-separated string (`convertArrayToString()`). This loses structure:
`["Action","Drama"]` becomes `"Action,Drama"`, nested arrays flatten, and
elements containing commas are ambiguous. Serialization happens early because
multi-source joins copy ClickHouse data into a scratch SQLite database.
