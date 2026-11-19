# ClickHouse driver: maintainer notes

Implementation rationale for the non-obvious parts of the ClickHouse driver,
which are mostly workarounds for upstream
[clickhouse-go](https://github.com/ClickHouse/clickhouse-go) behavior.
User-facing documentation (connection strings, type mapping, quirks) lives on
the [sq.io ClickHouse page](https://sq.io/docs/drivers/clickhouse); cross-driver
contributor guidance is in [`docs/DRIVERS.md`](../../docs/DRIVERS.md). This file
covers only the driver-internal details that fit neither.

## Batch insert

sq uses clickhouse-go's native Batch API (`PrepareBatch`/`Append`/`Send`) in
[`batch.go`](./batch.go), not the generic multi-row `INSERT` path, which
clickhouse-go rejects. A separate native connection is opened for batches
because the `database/sql` `*sql.DB` does not expose `PrepareBatch`.

ClickHouse transactions are no-ops (clickhouse-go's `BeginTx` returns self and
`Commit` does nothing; inserts are atomic at the batch level), so bypassing the
transaction for batch inserts loses nothing.

## Mutations are asynchronous by default

ClickHouse has no standard `UPDATE`/`DELETE`; it uses "lightweight mutations"
via `ALTER TABLE ... UPDATE/DELETE`, which are asynchronous by default: the
statement returns before rows change, so a subsequent `SELECT` may see stale
data.

`PrepareUpdateStmt` therefore appends `SETTINGS mutations_sync = 1` so the
mutation completes before the call returns. It also bypasses `PrepareContext()`,
which clickhouse-go implements by misclassifying non-`SELECT` as `INSERT` and
rejecting `ALTER TABLE ... UPDATE` (see
[clickhouse-go#1203](https://github.com/ClickHouse/clickhouse-go/issues/1203));
instead it calls `ExecContext()` directly via a `StmtExecFunc` closure with a
`nil` stmt. When no `WHERE` is given, `buildUpdateStmt` emits `WHERE 1`, because
ClickHouse requires a `WHERE` or the appended `SETTINGS` clause lands in an
invalid position.

ClickHouse reports `0` rows affected for every DML operation; sq surfaces this
as `dialect.RowsAffectedUnsupported` (-1).

## Skipped cross-driver tests

`TestDriver_CreateTable_Minimal` and `TestOutputRaw` are skipped for ClickHouse
because of the `kind.Time`/`kind.Bytes` round-trip limitations
([#544](https://github.com/neilotoole/sq/issues/544)): the `kind.Bytes` data is
preserved on disk (ClickHouse `String` is binary-safe), but the Go type and sq
kind change on readback.

## Array handling

sq's kind system has no `kind.Array`, and `record.Valid()` restricts values to
8 primitive Go types (`nil`, `int64`, `float64`, `decimal.Decimal`, `bool`,
`string`, `[]byte`, `time.Time`). ClickHouse `Array(T)` columns are therefore
mapped to `kind.Text` in `metadata.go`: their scan type is overridden to
`sqlz.RTypeAny`, and `getNewRecordFunc()` converts the resulting Go slice to a
comma-separated string (`convertArrayToString()`). This loses structure, so the
original array cannot be reconstructed. Serialization happens early because
multi-source joins copy ClickHouse data into a scratch SQLite database. See
[#545](https://github.com/neilotoole/sq/issues/545).
