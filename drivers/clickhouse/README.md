# ClickHouse Driver for SQ

ClickHouse database driver implementation for SQ using
[clickhouse-go](https://github.com/ClickHouse/clickhouse-go) v2.

> [!WARNING]
> All testing has been performed on **ClickHouse v25+**.
> Behavior on versions below v25 is not guaranteed and is unsupported.

## Requirements

- **ClickHouse Server**: v25.0 or later (tested and supported)
- **Go**: 1.26 or later
- **Docker**: Required for integration tests
- **Dependency**: `github.com/ClickHouse/clickhouse-go/v2` v2.42.0

## Quick Start

### Adding a Source

```bash
# Add ClickHouse source
sq add clickhouse://default:@localhost:9000/default --handle @ch

# With authentication
sq add clickhouse://user:password@host:9000/database --handle @ch

# Inspect the source
sq inspect @ch
```

### Querying

```bash
# Run SQ query
sq '.users | .id, .name | .[0:10]' @ch

# Run native SQL
sq sql 'SELECT * FROM users LIMIT 10' @ch

# Join with other sources
sq '.users | join(.@pg.orders, .user_id) | .name, .order_total' @ch
```

### Table Operations

```bash
sq tbl copy @ch.users @ch.users_backup   # Copy table
sq tbl truncate @ch.staging_data          # Truncate table
sq tbl drop @ch.old_table                 # Drop table
```

## Connection String Format

```bash
clickhouse://username:password@hostname:9000/database
clickhouse://username:password@hostname:9000/database?param=value
clickhouse://default:@localhost:9000/default
```

**Default Ports:**

| Protocol | Non-Secure | Secure (TLS) |
|----------|------------|--------------|
| Native   | 9000       | 9440         |
| HTTP     | 8123       | 8443         |

> **Note**: The `clickhouse-go` driver does not apply default ports
> automatically (unlike `pgx` for Postgres). SQ handles this by
> applying the appropriate default port if not specified: **9000** for
> non-secure, **9440** for secure (`secure=true`).

## Features

### Core

- Provider and Driver registration
- Connection management via clickhouse-go v2
- ClickHouse-specific SQL dialect (`?` placeholders, backtick
  identifiers)

### Type System

- Bidirectional type mapping (ClickHouse types ↔ `kind.Kind`)
- Supported types: Int8-64, UInt8-64, Float32/64, String, FixedString,
  Date, Date32, DateTime, DateTime64, Decimal, UUID, Bool
- Nullable handling: `Nullable(T)`
- LowCardinality support: `LowCardinality(T)`

### Metadata

- `CurrentCatalog()` / `CurrentSchema()` via `currentDatabase()`
- `ListCatalogs()` / `ListSchemas()` from `system.databases`
- Schema inspection via system tables (`system.tables`,
  `system.columns`)
- `CatalogExists()` / `SchemaExists()` checks

### DDL/DML

- `CreateTable()` with MergeTree engine and ORDER BY
- `DropTable()` with IF EXISTS
- `Truncate()` table
- `NewBatchInsert()` using clickhouse-go's native Batch API
  (`PrepareBatch`/`Append`/`Send`)
- `PrepareInsertStmt()` for single-row prepared inserts
- `PrepareUpdateStmt()` using ALTER TABLE UPDATE syntax
- `CopyTable()` with or without data

### Query

- `TableColumnTypes()` with optional column filtering
- `RecordMeta()` with proper scan types
- `TableExists()` / `ListTableNames()`

## What Works

The following capabilities are fully functional with ClickHouse:

- **Reading/querying data**: Full support for SQ queries and native
  SQL against ClickHouse sources.
- **Batch inserts**: Full support for inserting data into ClickHouse
  using clickhouse-go's native Batch API. ClickHouse works as both
  a source (origin) and destination for data operations including
  `--insert` and cross-source copies.
- **Schema introspection and metadata**: Inspect databases, tables,
  columns, and views via system tables.
- **Cross-source joins**: ClickHouse as a source in multi-source
  joins (data copied to scratch SQLite DB).
- **Table operations**: Copy, truncate, and drop tables.
- **All output formats**: JSON, CSV, TSV, XLSX, text table, and all
  other sq output formats.

## ClickHouse-Specific Behavior

- **Transactions**: OLAP-optimized; no traditional ACID. Inserts are
  atomic at batch level.
- **Table Engine**: MergeTree required. SQ uses
  `ENGINE = MergeTree()` with `ORDER BY` on first column.
- **Updates**: Uses `ALTER TABLE ... UPDATE` syntax (not standard
  UPDATE). See [Synchronous vs Asynchronous
  Operations](#synchronous-vs-asynchronous-operations) below.
- **Schema/Catalog**: ClickHouse "database" maps to SQ
  schema/catalog concepts.
- **System Tables**: Metadata from `system.databases`,
  `system.tables`, `system.columns`.
- **Views**: Regular (`engine='View'`) vs materialized
  (`engine='MaterializedView'`).
- **Type System**: Separate signed/unsigned integers; no implicit
  coercion; `FixedString(N)`; native Bool.

### Synchronous vs Asynchronous Operations

ClickHouse differs fundamentally from traditional SQL databases in
how it handles write operations. Understanding which operations are
synchronous and which are asynchronous is critical, because an
asynchronous operation may return successfully while the data
remains unmodified — a subsequent `SELECT` can return stale
(pre-mutation) results.

#### Operations Overview

<!-- markdownlint-disable MD013 MD060 -->
| Operation | SQL Form | Sync by Default? | sq Behavior |
|-----------|----------|-------------------|-------------|
| Batch insert | `INSERT INTO ... VALUES` | Yes | Synchronous (native Batch API) |
| Single-row insert | `INSERT INTO ... VALUES` | Yes | Synchronous (prepared statement) |
| Copy table | `INSERT INTO ... SELECT` | Yes | Synchronous |
| Update | `ALTER TABLE ... UPDATE` | **No** | Forced synchronous (`mutations_sync = 1`) |
| Delete | `ALTER TABLE ... DELETE` | **No** | Not implemented in sq |
| DDL (CREATE, DROP, TRUNCATE) | Standard DDL | Yes | Synchronous |
<!-- markdownlint-enable MD013 MD060 -->

#### Inserts Are Synchronous

ClickHouse inserts are **synchronous by default**. When an `INSERT`
statement completes, the data is written and immediately visible to
subsequent queries. This applies to all insert paths used by sq:

- **Batch inserts** via clickhouse-go's native Batch API
  (`PrepareBatch`/`Append`/`Send`): the `Send()` call blocks until
  the server confirms the data is written.
- **Single-row prepared inserts** via `PrepareInsertStmt`: each
  `ExecContext()` call blocks until the row is written.
- **Copy table** via `INSERT INTO ... SELECT`: the statement blocks
  until all rows are copied.

> **Note on `async_insert`**: ClickHouse has a server-side setting
> called
> [`async_insert`](https://clickhouse.com/docs/en/operations/settings/settings#async-insert)
> that, when enabled, buffers inserts and commits them
> asynchronously for higher throughput. If a ClickHouse server has
> `async_insert = 1` enabled, inserts from sq would also be
> affected — the `INSERT` would return before data is visible. This
> is a server-level configuration choice; sq does not set or
> override this setting. By default, `async_insert` is disabled
> (`0`), and inserts are synchronous.

#### Mutations Are Asynchronous (By Default)

ClickHouse does not support standard SQL `UPDATE` or `DELETE`.
Instead, it uses "lightweight mutations":

```sql
ALTER TABLE t UPDATE col = value WHERE condition
ALTER TABLE t DELETE WHERE condition
```

**Mutations are asynchronous by default.** When the
`ALTER TABLE ... UPDATE` statement returns, it has only _submitted_
the mutation to ClickHouse's mutation queue. The actual data
modification happens in the background. This means:

1. The statement returns immediately, before any rows are modified.
2. A `SELECT` issued right after the `ALTER TABLE ... UPDATE` may
   return the **old (pre-mutation) data**.
3. `RowsAffected()` always returns `0`, regardless of how many rows
   were actually modified. ClickHouse does not track or report
   affected row counts for mutations.

This behavior is fundamental to ClickHouse's architecture.
Mutations rewrite entire data parts (analogous to an LSM
compaction), so they are intentionally deferred and batched.

##### How sq Forces Synchronous Mutations

sq's `PrepareUpdateStmt` appends `SETTINGS mutations_sync = 1` to
every `ALTER TABLE ... UPDATE` query:

```sql
ALTER TABLE `t` UPDATE `col` = ? WHERE condition
    SETTINGS mutations_sync = 1
```

The
[`mutations_sync`](https://clickhouse.com/docs/en/operations/settings/settings#mutations_sync)
setting controls whether the statement waits for the mutation to
complete:

<!-- markdownlint-disable MD013 MD060 -->
| Value | Behavior |
|-------|----------|
| `0` (default) | Asynchronous: returns immediately, mutation runs in background |
| `1` | Synchronous: waits for mutation to complete on the current replica |
| `2` | Synchronous + replicated: waits for mutation on all replicas |
<!-- markdownlint-enable MD013 MD060 -->

sq uses `mutations_sync = 1` to ensure that when
`PrepareUpdateStmt` returns, the data has been modified and is
visible to subsequent queries.

**Even with `mutations_sync = 1`, `RowsAffected()` still returns
`0`.** This is a ClickHouse limitation — the mutation completes
synchronously, but the server does not report how many rows were
affected. sq accounts for this by returning `0` from the
`StmtExecer` and the test suite asserts `affected == 0` for
ClickHouse.

##### Why PrepareContext Cannot Be Used

clickhouse-go v2's `PrepareContext()` internally classifies every
non-`SELECT` statement as an `INSERT` and validates it accordingly.
An `ALTER TABLE ... UPDATE` statement is rejected with the error
`"invalid INSERT query"`. This is an upstream limitation:
[clickhouse-go#1203](https://github.com/ClickHouse/clickhouse-go/issues/1203).

sq works around this by bypassing `PrepareContext()` entirely. The
`PrepareUpdateStmt` method constructs a `StmtExecFunc` closure that
calls `db.ExecContext()` directly, and passes `nil` for the
prepared statement to `NewStmtExecer`. The `StmtExecer.Close()`
method is nil-safe to support this pattern.

#### DDL Is Synchronous

DDL operations (`CREATE TABLE`, `DROP TABLE`, `TRUNCATE TABLE`,
`ALTER TABLE ... ADD COLUMN`) are synchronous in ClickHouse. The
statement blocks until the schema change is complete. sq calls
these via `ExecContext()` directly, with no special handling needed.

#### Comparison with Other Databases

<!-- markdownlint-disable MD013 MD060 -->
| Database | UPDATE Sync? | INSERT Sync? | RowsAffected for UPDATE? |
|----------|-------------|-------------|--------------------------|
| PostgreSQL | Yes | Yes | Yes (accurate count) |
| MySQL | Yes | Yes | Yes (accurate count) |
| SQLite | Yes | Yes | Yes (accurate count) |
| SQL Server | Yes | Yes | Yes (accurate count) |
| **ClickHouse** | **No** (forced via `mutations_sync = 1`) | Yes (unless `async_insert` is enabled) | **No** (always returns `0`) |
<!-- markdownlint-enable MD013 MD060 -->

## Testing

```bash
cd drivers/clickhouse

go test -v -short                    # Unit tests (no DB required)
./testutils/test-integration.sh      # Integration tests (requires Docker)
./testutils/test-sq-cli.sh           # CLI end-to-end tests
```

See **[testutils/Testing.md](./testutils/Testing.md)** for detailed
instructions.

## Deferred Features (Post-MVP)

- Column compression codecs (LZ4, ZSTD, etc.)
- Table engine options (ReplicatedMergeTree, Distributed, etc.)
- Advanced features (dictionaries, materialized views)
- Partition management, sampling, sharding
- Advanced data types (Array, Tuple, Map, Nested)
- Time series specific operations

## Known Limitations

The following are currently-understood limitations based on differences
between ClickHouse and traditional SQL databases. This understanding
may be incomplete or incorrect; contributions and corrections are
welcome.

### Limitations Overview

<!-- markdownlint-disable MD013 MD060 -->
| # | Limitation                                                                               | Category      | Severity | Workaround                           |
|---|------------------------------------------------------------------------------------------|---------------|----------|--------------------------------------|
| 1 | [~~Batch insert connection corruption~~](#1-batch-insert-connection-corruption-resolved) | Insert        | ~~High~~ | **Resolved**: native Batch API       |
| 2 | [~~PrepareUpdateStmt not supported~~](#2-prepareupdatestmt-resolved)                     | Update/Delete | ~~High~~ | **Resolved**: ExecContext workaround |
| 3 | [Standard UPDATE/DELETE not supported](#3-standard-updatedelete-not-supported)           | Update/Delete | Medium   | Tests skipped                        |
| 4 | [Type roundtrip issues](#4-type-roundtrip-issues)                                        | Types         | Low      | Tests skipped                        |
| 5 | [CopyTable row count unsupported](#5-copytable-row-count-unsupported)                    | Metadata      | Low      | Handled in CLI                       |
<!-- markdownlint-enable MD013 MD060 -->

### Insert Limitations

#### 1. Batch Insert: Connection Corruption (RESOLVED)

**Status**: Resolved. The ClickHouse driver now uses clickhouse-go's
native Batch API (`PrepareBatch`/`Append`/`Send`) via the
`SQLDriver.NewBatchInsert` method, bypassing the incompatible
multi-row `INSERT` approach.

Previously-skipped insert tests that are now enabled:

- `TestNewBatchInsert` — multi-row batch insert test
- `TestCreateTable_bytes` — creates table and inserts binary data
- `TestCmdSQL_Insert` — `sq sql --insert=@dest.tbl`
- `TestCmdSLQ_Insert` — `sq slq --insert=@dest.tbl`

Note: `TestOutputRaw` remains skipped for ClickHouse, but for a
different reason — the `kind.Bytes` type roundtrip limitation
(see Known Limitation #4), not the batch insert issue.

##### Historical Details

###### Root Cause

sq's generic `driver.PrepareInsertStmt` generates multi-row `INSERT`
statements of the form:

```sql
INSERT INTO t VALUES (?, ?), (?, ?), ...
```

with all argument values flattened into a single slice (e.g., 70 rows
x 4 columns = 280 args). PostgreSQL, MySQL, and SQLite all support
this multi-row parameter binding natively, so it works without
modification for those drivers.

However, clickhouse-go v2's prepared statement handler expects
arguments for exactly **one row** per `ExecContext` call. When it
receives 280 arguments but the column count is 4, it rejects with:

```text
expected 4 arguments, got 280
```

###### Why the Obvious Workaround Fails

Forcing single-row inserts (`numRows=1`) fixes the argument count
mismatch, but causes ClickHouse native protocol state corruption
after approximately 200 `Exec` calls on the same prepared statement.
The connection produces:

```text
code: 101, message: Unexpected packet Query received from client
```

The connection becomes invalid but is not closed by the driver, so
subsequent queries on the same connection also fail.

###### Solution

The `SQLDriver` interface was extended with a `NewBatchInsert`
method, allowing each driver to own its insert strategy. The
existing standalone `NewBatchInsert` function was renamed to
`DefaultNewBatchInsert` and serves as the default implementation that
most drivers (Postgres, MySQL, SQLite, SQL Server) delegate to.

The ClickHouse driver provides its own `NewBatchInsert`
implementation in `drivers/clickhouse/batch.go` that uses
clickhouse-go's native Batch API:

```go
batch, err := conn.PrepareBatch(ctx, "INSERT INTO t (c1, c2)")
for _, row := range rows {
    batch.Append(row...)
}
batch.Send()
```

This API handles protocol state correctly and is optimized for
ClickHouse's columnar batch semantics.

Key design decisions:

- **`*source.Source` in the signature**: ClickHouse needs
  `src.Location` (the DSN) to call `clickhouse.ParseDSN` and
  `clickhouse.Open` for a native connection. The `*sql.DB` /
  `*sql.Tx` don't expose the DSN.

- **Separate native connection**: clickhouse-go exposes two APIs:
  `sql.Open("clickhouse", dsn)` returns `*sql.DB` (no
  `PrepareBatch`); `clickhouse.Open(opts)` returns a native
  `driver.Conn` (has `PrepareBatch`). The standard
  `database/sql` connection can't access `PrepareBatch`, so a
  separate native connection is opened for batch inserts.

- **ClickHouse transactions are no-ops**: clickhouse-go's
  `BeginTx` returns `self` and `Commit` is nil/no-op. Bypassing
  the tx for inserts loses nothing.

- **Row count tracking**: ClickHouse returns 0 for
  `RowsAffected()`. The native Batch API goroutine tracks the
  count directly via an `atomic.Int64` counter.

- **`NewBatchInsert` constructor**: Added to
  `libsq/driver/batch.go` to allow the ClickHouse driver to
  build a `BatchInsert` with its own goroutine and channels,
  without going through `DefaultNewBatchInsert`.

###### Files Changed

<!-- markdownlint-disable MD013 MD060 -->
| File | Change |
|------|--------|
| `libsq/driver/driver.go` | Added `NewBatchInsert` to `SQLDriver` interface |
| `libsq/driver/batch.go` | `DefaultNewBatchInsert` (standard multi-row INSERT); `NewBatchInsert` (low-level constructor) |
| `drivers/clickhouse/batch.go` | New: native Batch API implementation |
| `drivers/postgres/postgres.go` | Added `NewBatchInsert` (delegates to `DefaultNewBatchInsert`) |
| `drivers/mysql/mysql.go` | Added `NewBatchInsert` (delegates) |
| `drivers/sqlite3/sqlite3.go` | Added `NewBatchInsert` (delegates) |
| `drivers/sqlserver/sqlserver.go` | Added `NewBatchInsert` (delegates) |
| `libsq/dbwriter.go` | Updated call site |
| `testh/testh.go` | Updated call site |
| `drivers/xlsx/ingest.go` | Updated call site |
| `libsq/driver/driver_test.go` | Updated call site, removed ClickHouse skip |
<!-- markdownlint-enable MD013 MD060 -->

### Update/Delete Limitations

#### 2. PrepareUpdateStmt (RESOLVED)

Reference: <https://github.com/ClickHouse/clickhouse-go/issues/1203>

**Status**: Resolved. The ClickHouse driver's `PrepareUpdateStmt`
now bypasses `PrepareContext()` and uses `ExecContext()` directly
via a custom `StmtExecFunc` closure. A `nil` stmt is passed to
`NewStmtExecer` (the `Close()` method is nil-safe).

Previously, `PrepareUpdateStmt` called `db.PrepareContext(ctx, query)`
which failed because clickhouse-go v2 internally classifies every
non-`SELECT` statement as an `INSERT` and rejects
`ALTER TABLE ... UPDATE` with `"invalid INSERT query"`.

The workaround follows the same pattern used by other ClickHouse
DDL/DML operations in the driver (`CreateTable`, `Truncate`,
`AlterTableAddColumn`), which all call `ExecContext()` directly.

##### Asynchronous Mutations and RowsAffected

ClickHouse mutations (`ALTER TABLE ... UPDATE`) are asynchronous
by default: the statement returns immediately, before the data is
actually modified. A subsequent `SELECT` may still return stale
(pre-mutation) data. To ensure the mutation completes before the
method returns, the query appends `SETTINGS mutations_sync = 1`,
which forces synchronous execution.

Even with `mutations_sync = 1`, ClickHouse does not report the
number of rows affected by a mutation. The
`sql.Result.RowsAffected()` value returned by the driver is always
0, regardless of how many rows were actually modified. The
`StmtExecer` therefore always returns 0 for affected rows. The
test `TestSQLDriver_PrepareUpdateStmt` accounts for this by
asserting `affected == 0` for ClickHouse (vs `affected == 1` for
other databases).

Previously-skipped test that is now enabled:

- `TestSQLDriver_PrepareUpdateStmt` — shared update via prepared
  statement (covers single-row, 2-column update across all SQL
  drivers)

Dedicated ClickHouse integration tests in
`drivers/clickhouse/clickhouse_test.go`:

- `TestDriver_PrepareUpdateStmt/multi_row` — update 5 rows via
  WHERE `actor_id <= 5`
- `TestDriver_PrepareUpdateStmt/no_match` — WHERE matches no rows
- `TestDriver_PrepareUpdateStmt/update_all_rows` — empty WHERE
  updates all 200 rows
- `TestDriver_PrepareUpdateStmt/null_value` — update column to NULL

#### 3. Standard UPDATE/DELETE Not Supported

ClickHouse does not support standard SQL `UPDATE` and `DELETE`
syntax. Executing these statements produces an error:

```sql
-- Standard SQL (fails on ClickHouse):
UPDATE test_table SET name = 'Charlie' WHERE id = 1;
DELETE FROM test_table WHERE id = 2;

-- ClickHouse equivalent (lightweight mutations):
ALTER TABLE test_table UPDATE name = 'Charlie' WHERE id = 1;
ALTER TABLE test_table DELETE WHERE id = 2;
```

##### How sq Handles Updates

sq's `PrepareUpdateStmt` (Known Limitation #2, now resolved)
generates `ALTER TABLE ... UPDATE` syntax via `buildUpdateStmt()`
in `drivers/clickhouse/render.go`. This means that sq operations
that go through the driver abstraction layer (e.g., `sq tbl`
with `--update`) can update ClickHouse tables successfully. See
Known Limitation #2 for details on the `ExecContext()` workaround
and asynchronous mutation handling.

##### What Remains Unsupported

Standard `DELETE` has no sq driver workaround yet — there is no
`PrepareDeleteStmt` equivalent. The two skipped tests below
exercise standard `UPDATE`/`DELETE` SQL directly via `sq sql`,
which sends the SQL text to the database as-is. Because these
tests bypass sq's driver abstraction, the `ALTER TABLE` rewriting
that `PrepareUpdateStmt` performs is not available.

##### Skipped Tests

Both tests are skipped for ClickHouse via:

```go
tu.SkipIf(t, src.Type == drivertype.ClickHouse,
    "ClickHouse: doesn't support standard UPDATE/DELETE statements")
```

**`TestCmdSQL_ExecMode`** (`cli/cmd_sql_test.go`): Full CRUD
lifecycle test — CREATE, INSERT, UPDATE, DELETE, DROP — that
verifies sq correctly distinguishes between queries (which return
result sets) and statements (which return rows-affected counts).
Skipped because the test executes standard SQL directly:

```sql
-- Line 98:
UPDATE test_exec_type SET name = 'Charlie' WHERE id = 1
-- Line 109:
DELETE FROM test_exec_type WHERE id = 2
```

**`TestCmdSQL_ExecTypeEdgeCases`** (`cli/cmd_sql_test.go`): SQL
statement type detection with formatting variations (case
sensitivity, block comments, CTEs). Verifies that sq identifies
queries vs statements regardless of SQL formatting conventions.
Skipped because the test executes standard SQL directly:

```sql
-- Line 315:
update test_edge_cases set name = 'Updated' where id = 10
-- Line 321:
delete from test_edge_cases where id = 10
```

### Type Limitations

#### 4. Type Roundtrip Issues

Some `kind.Kind` types cannot roundtrip through ClickHouse because
it lacks native equivalents:

| sq Kind      | Created As | Read Back As    | Notes               |
|--------------|------------|-----------------|---------------------|
| `kind.Time`  | `DateTime` | `kind.Datetime` | No time-only type   |
| `kind.Bytes` | `String`   | `kind.Text`     | Binary as String    |

The `kind.Bytes` roundtrip issue means binary data inserted as
`[]byte` is read back as `string` (`kind.Text`). ClickHouse's
`String` type is binary-safe, so the data is preserved, but the
Go type and sq kind change on readback.

**Status**: The following tests are **skipped** for ClickHouse:

- `TestDriver_CreateTable_Minimal` — asserts kind roundtrip fidelity
- `TestOutputRaw` — asserts `kind.Bytes` and `[]byte` type assertion
  on readback (data is actually preserved, but typed as `string`)

### Metadata Limitations

#### 5. CopyTable Row Count Unsupported

`CopyTable` returns `dialect.RowsAffectedUnsupported` (-1) because
ClickHouse's `INSERT ... SELECT` doesn't report affected rows. The
CLI handles this gracefully by displaying
"(rows copied: unsupported)".

## Array Type Architecture

This section documents how sq handles array types from ClickHouse,
and the broader architectural constraints that inform this design.

### The Core Constraint: No `kind.Array`

The sq type system (`kind.Kind`) defines only 11 scalar types:

- `Unknown`, `Null`, `Text`, `Int`, `Float`, `Decimal`, `Bool`,
  `Bytes`, `Datetime`, `Date`, `Time`

There is intentionally **no `kind.Array` type**. The record system
(`libsq/core/record/record.go`) restricts valid values to 8
primitive Go types:

```go
// Valid types: nil, int64, float64, decimal.Decimal, bool,
//   string, []byte, time.Time
```

The `record.Valid()` function enforces this at runtime, rejecting
any record containing types outside this set.

### Why This Design?

1. **Cross-database abstraction**: sq supports PostgreSQL, MySQL,
   SQLite, SQL Server, ClickHouse, plus non-SQL sources (CSV, JSON,
   XLSX). A minimal common type set enables uniform querying across
   all sources.

2. **Output format compatibility**: CSV, TSV, and text-table outputs
   have no native array representation. JSON and YAML can represent
   arrays, but keeping a unified record format simplifies the
   architecture.

3. **Simplicity over completeness**: The 11 kinds cover the vast
   majority of real-world use cases. Complex types (arrays, JSON,
   user-defined) are relatively rare in typical queries.

### How ClickHouse Arrays Are Handled

ClickHouse has native `Array(T)` types (e.g., `Array(String)`,
`Array(Int32)`). The sq ClickHouse driver handles these through a
two-step process:

1. **Kind mapping**: Array types are mapped to `kind.Text` via
   `kindFromClickHouseType()` in `metadata.go`.

2. **Scan type override**: After the standard `setScanType()` call,
   Array columns have their scan type overridden to `sqlz.RTypeAny`
   (accepting Go slices):

   ```go
   if strings.HasPrefix(dbTypeName, "Array") {
       colTypeData.ScanType = sqlz.RTypeAny
   }
   ```

3. **String conversion**: The `getNewRecordFunc()` detects slice
   values and converts them to comma-separated strings via
   `convertArrayToString()`:

   ```go
   // Input:  []string{"Action", "Drama", "Comedy"}
   // Output: "Action,Drama,Comedy"
   ```

   Supported array element types: `string`, `int*`, `uint*`,
   `float*`, `bool`.

### Example Transformation

When a ClickHouse query returns `Array(String)` data:

| Stage              | Value                         | Type        |
|--------------------|-------------------------------|-------------|
| ClickHouse returns | `["Action", "Drama"]`         | `[]string`  |
| After scan         | `[]string{"Action", "Drama"}` | Go slice    |
| After conversion   | `"Action,Drama"`              | `string`    |
| In sq record       | `"Action,Drama"`              | `kind.Text` |

### Array Support Across Databases

| Database       | Array Support           | sq Handling             |
|----------------|-------------------------|-------------------------|
| **ClickHouse** | Native `Array(T)`       | Converted to CSV string |
| **PostgreSQL** | Native (`text[]`, etc.) | Mapped to `kind.Text`   |
| **MySQL**      | None (JSON has arrays)  | JSON stored as-is       |
| **SQLite**     | None                    | N/A                     |
| **SQL Server** | None (XML/JSON as text) | Stored as text          |

### Known Limitations and Trade-offs

1. **Information loss**: `["Action", "Drama"]` becomes
   `"Action,Drama"`. The original array structure cannot be
   reconstructed.

2. **No round-trip fidelity**: Cannot distinguish between
   `"Action,Drama"` (a string containing a comma) and
   `["Action", "Drama"]` (an array of two strings).

3. **Nested arrays flattened**: `[[1,2],[3,4]]` becomes `"1,2,3,4"`
   (structure is completely lost).

4. **Delimiter ambiguity**: If array elements contain commas, the CSV
   representation may be ambiguous.

5. **Multi-source joins**: When joining ClickHouse with other sources,
   data is copied to a scratch SQLite database. Arrays must be
   serialized to text before this copy operation, hence the conversion
   happens early in the pipeline.

### Potential Future Directions

1. **JSON serialization**: Instead of CSV, serialize arrays as JSON
   strings (e.g., `["Action","Drama"]`). This preserves structure and
   enables potential reconstruction, but still stores as `kind.Text`.

2. **Add `kind.Array`**: A major architectural change that would
   require updating the record validation, all drivers, and all output
   writers. This would enable richer array support but at significant
   complexity cost.

3. **Driver-specific handling**: The current approach — each driver
   converts arrays to text at scan time. This is pragmatic but
   requires per-driver implementation.

### References

- Record validation: `libsq/core/record/record.go:Valid()`
- Kind types: `libsq/core/kind/kind.go`
- ClickHouse array handling:
  `drivers/clickhouse/metadata.go:getNewRecordFunc()`
- Array conversion:
  `drivers/clickhouse/metadata.go:convertArrayToString()`

## Development Log

Condensed summaries of significant investigations during driver
development. Full details are preserved in the
[development log memo](../../.claude/memos/gh503-clickhouse-dev-log.memo.md).

### Query Test Overrides (2026-01-19)

Added `drivertype.ClickHouse` entries to all libsq query test cases
with `drivertype.MySQL` overrides (both use backtick quoting).
ClickHouse needed proper rownum SQL overrides using
`row_number() OVER (ORDER BY 1)` syntax.

**Status**: Resolved.

### JOIN Column Naming (2026-01-19)

ClickHouse returns qualified column names in `table.column` format
for JOIN queries, unlike other databases which return bare column
names. This prevented sq's duplicate column munging from triggering.
Fixed by stripping the table prefix in
`recordMetaFromColumnTypes()` before passing names to the munging
mechanism.

**Status**: Resolved.

### Array Type Handling (2026-01-19)

ClickHouse `Array(T)` types caused scan errors in multi-source joins
because `[]string` values couldn't be scanned into `*string` fields.
Fixed by overriding scan types for Array columns to `sqlz.RTypeAny`
and converting slice values to comma-separated strings in
`getNewRecordFunc()`.

**Status**: Resolved.

### Batch Insert Failure and Resolution (2026-01-19, resolved 2026-02-14)

clickhouse-go does not support multi-row parameter binding.
Single-row workaround causes connection state corruption after many
`Exec` calls. Resolved by adding `NewBatchInsert` to the `SQLDriver`
interface and implementing a ClickHouse-specific version in
`drivers/clickhouse/batch.go` using clickhouse-go's native Batch API
(`conn.PrepareBatch()`/`batch.Append()`/`batch.Send()`). The
existing `NewBatchInsert` function was renamed to `DefaultNewBatchInsert`;
other drivers delegate to it. See Known Limitation #1 for full
architectural details.

**Status**: Resolved.

### PrepareUpdateStmt Failure (2026-01-19, resolved 2026-02-14)

clickhouse-go's `PrepareContext()` only supports INSERT and SELECT.
`ALTER TABLE ... UPDATE` is rejected as "invalid INSERT query".
Resolved by bypassing `PrepareContext()` and using `ExecContext()`
directly via a custom `StmtExecFunc` closure, passing `nil` for the
stmt parameter. `StmtExecer.Close()` was made nil-safe to support
this.

ClickHouse mutations are asynchronous by default: the statement
returns immediately, before data is modified. Without
`mutations_sync = 1`, a subsequent `SELECT` returns stale data.
The fix appends `SETTINGS mutations_sync = 1` to the query so the
mutation completes synchronously. Even so, `RowsAffected()` always
returns 0 for ClickHouse mutations — the driver does not track
affected row counts. The test asserts `affected == 0` for
ClickHouse accordingly.

See Known Limitation #2 for full details.

Dedicated ClickHouse integration tests were added in
`drivers/clickhouse/clickhouse_test.go:TestDriver_PrepareUpdateStmt`
covering multi-row updates, no-match WHERE, update-all-rows (empty
WHERE), and NULL value updates. A nil-safety unit test for
`StmtExecer.Close()` was added in
`libsq/driver/record_test.go:TestStmtExecer_Close_NilStmt`.

**Status**: Resolved.
