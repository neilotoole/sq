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
- `PrepareInsertStmt()` for batch inserts
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
  UPDATE).
- **Schema/Catalog**: ClickHouse "database" maps to SQ
  schema/catalog concepts.
- **System Tables**: Metadata from `system.databases`,
  `system.tables`, `system.columns`.
- **Views**: Regular (`engine='View'`) vs materialized
  (`engine='MaterializedView'`).
- **Type System**: Separate signed/unsigned integers; no implicit
  coercion; `FixedString(N)`; native Bool.

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

### Summary

<!-- markdownlint-disable MD013 MD060 -->
| # | Limitation                          | Category      | Severity | Workaround                          |
|---|-------------------------------------|---------------|----------|-------------------------------------|
| 1 | ~~Batch insert connection corruption~~ | Insert      | ~~High~~ | **Resolved**: native Batch API      |
| 2 | PrepareUpdateStmt not supported     | Update/Delete | High     | Tests skipped                       |
| 3 | Standard UPDATE/DELETE not supported | Update/Delete | Medium   | Tests skipped                       |
| 4 | Type roundtrip issues               | Types         | Low      | Tests skipped                       |
| 5 | CopyTable row count unsupported     | Metadata      | Low      | Handled in CLI                      |
<!-- markdownlint-enable MD013 MD060 -->

### Insert Limitations

#### 1. Batch Insert: Connection Corruption (RESOLVED)

**Status**: Resolved. The ClickHouse driver now uses clickhouse-go's
native Batch API (`PrepareBatch`/`Append`/`Send`) via the
`SQLDriver.NewBatchInsert` method, bypassing the incompatible
multi-row `INSERT` approach. All previously-skipped insert tests
are now enabled.

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

The `SQLDriver` interface was extended with a `NewBatchInsert` method,
allowing each driver to own its insert strategy. The ClickHouse
implementation uses clickhouse-go's native Batch API:

```go
batch, err := conn.PrepareBatch(ctx, "INSERT INTO t (c1, c2)")
for _, row := range rows {
    batch.Append(row...)
}
batch.Send()
```

This API handles protocol state correctly and is optimized for
ClickHouse's columnar batch semantics.

### Update/Delete Limitations

#### 2. PrepareUpdateStmt Not Supported

ClickHouse uses `ALTER TABLE ... UPDATE` syntax instead of standard
SQL UPDATE. While sq's `PrepareUpdateStmt` correctly generates this
syntax, clickhouse-go's `PrepareContext()` only supports **INSERT and
SELECT statements**. Any other statement type is rejected with
"invalid INSERT query". Direct execution via `ExecContext()` works,
but `PrepareUpdateStmt` requires `PrepareContext()` for parameter
binding.

**Status**: `TestSQLDriver_PrepareUpdateStmt` is **skipped**.

#### 3. Standard UPDATE/DELETE Not Supported

ClickHouse does not support standard SQL `UPDATE` and `DELETE`
statements. Instead, it requires `ALTER TABLE ... UPDATE` or
`ALTER TABLE ... DELETE` syntax (lightweight mutations). Tests that
execute standard CRUD operations are skipped:

- `TestCmdSQL_ExecMode` — full CRUD lifecycle (CREATE, INSERT,
  UPDATE, DELETE, DROP)
- `TestCmdSQL_ExecTypeEdgeCases` — SQL type detection with
  UPDATE/DELETE

### Type Limitations

#### 4. Type Roundtrip Issues

Some `kind.Kind` types cannot roundtrip through ClickHouse because
it lacks native equivalents:

| sq Kind      | Created As | Read Back As    | Notes               |
|--------------|------------|-----------------|---------------------|
| `kind.Time`  | `DateTime` | `kind.Datetime` | No time-only type   |
| `kind.Bytes` | `String`   | `kind.Text`     | Binary as String    |

**Status**: `TestDriver_CreateTable_Minimal` is **skipped**.

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

### Comparison with Other Databases

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

### Batch Insert Failure (2026-01-19)

clickhouse-go does not support multi-row parameter binding.
Single-row workaround causes connection state corruption after many
`Exec` calls. Fixed by adding `NewBatchInsert` to the `SQLDriver`
interface and implementing it with clickhouse-go's native Batch API
(`conn.PrepareBatch()`/`batch.Append()`/`batch.Send()`).

**Status**: Resolved. See Known Limitation #1.

### PrepareUpdateStmt Failure (2026-01-19)

clickhouse-go's `PrepareContext()` only supports INSERT and SELECT.
`ALTER TABLE ... UPDATE` is rejected as "invalid INSERT query". Direct
`ExecContext()` works but `PrepareUpdateStmt` requires
`PrepareContext()` for parameter binding.

**Status**: Tests skipped. See Known Limitation #4.
