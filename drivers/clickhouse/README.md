# ClickHouse Driver for SQ

ClickHouse database driver implementation for SQ using
[clickhouse-go](https://github.com/ClickHouse/clickhouse-go) v2.

> [!WARNING]
> All testing has been performed on **ClickHouse v25+**.
> Behavior on versions below v25 is not guaranteed and is unsupported.

## Requirements

- **ClickHouse Server**: v25.0 or later (tested and supported)
- **Go**: 1.19 or later
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

> **Note**: The `clickhouse-go` driver does not apply default ports automatically
> (unlike `pgx` for Postgres). SQ handles this by applying the appropriate
> default port if not specified: **9000** for non-secure, **9440** for secure
> (`secure=true`).

## Features

### Core

- Provider and Driver registration
- Connection management via clickhouse-go v2
- ClickHouse-specific SQL dialect (`?` placeholders, backtick identifiers)

### Type System

- Bidirectional type mapping (ClickHouse types ↔ `kind.Kind`)
- Supported types: Int8-64, UInt8-64, Float32/64, String, FixedString, Date,
  Date32, DateTime, DateTime64, Decimal, UUID, Bool
- Nullable handling: `Nullable(T)`
- LowCardinality support: `LowCardinality(T)`

### Metadata

- `CurrentCatalog()` / `CurrentSchema()` via `currentDatabase()`
- `ListCatalogs()` / `ListSchemas()` from `system.databases`
- Schema inspection via system tables (`system.tables`, `system.columns`)
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

## ClickHouse-Specific Behavior

- **Transactions**: OLAP-optimized; no traditional ACID. Inserts are atomic at
  batch level.
- **Table Engine**: MergeTree required. SQ uses `ENGINE = MergeTree()` with
  `ORDER BY` on first column.
- **Updates**: Uses `ALTER TABLE ... UPDATE` syntax (not standard UPDATE).
- **Schema/Catalog**: ClickHouse "database" maps to SQ schema/catalog concepts.
- **System Tables**: Metadata from `system.databases`, `system.tables`,
  `system.columns`.
- **Views**: Regular (`engine='View'`) vs materialized
  (`engine='MaterializedView'`).
- **Type System**: Separate signed/unsigned integers; no implicit coercion;
  `FixedString(N)`; native Bool.

## Testing

```bash
cd drivers/clickhouse

go test -v -short                    # Unit tests (no DB required)
./testutils/test-integration.sh      # Integration tests (requires Docker)
./testutils/test-sq-cli.sh           # CLI end-to-end tests
```

See **[testutils/Testing.md](./testutils/Testing.md)** for detailed instructions.

## Deferred Features (Post-MVP)

- Column compression codecs (LZ4, ZSTD, etc.)
- Table engine options (ReplicatedMergeTree, Distributed, etc.)
- Advanced features (dictionaries, materialized views)
- Partition management, sampling, sharding
- Advanced data types (Array, Tuple, Map, Nested)
- Time series specific operations

## Known Limitations

The following are currently-understood limitations based on differences between
ClickHouse and traditional SQL databases. This understanding may be incomplete
or incorrect; contributions and corrections are welcome.

### 1. Type Roundtrip Limitations

Based on our current understanding, some `kind.Kind` types cannot roundtrip
through ClickHouse because it lacks native equivalents:

| sq Kind      | Created As | Read Back As    | Notes                    |
|--------------|------------|-----------------|--------------------------|
| `kind.Time`  | `DateTime` | `kind.Datetime` | No time-only type        |
| `kind.Bytes` | `String`   | `kind.Text`     | Binary stored as String  |

This causes `TestDriver_CreateTable_Minimal` to be skipped for ClickHouse.

### 2. CopyTable Row Count Unsupported

`CopyTable` returns `dialect.RowsAffectedUnsupported` (-1) because ClickHouse's
`INSERT ... SELECT` doesn't support reporting affected rows. The operation
succeeds, but the count is unsupported. The CLI handles this gracefully by
displaying "(rows copied: unsupported)" instead of an incorrect count.

### 3. Batch Insert: Multi-Row Parameter Binding Not Supported

The clickhouse-go driver does not support multi-row parameter binding. When sq
generates `INSERT INTO t VALUES (?,?), (?,?)` with flattened arguments,
clickhouse-go expects arguments for a **single row only**. This causes
`TestNewBatchInsert` to fail with "expected 4 arguments, got 280".

Forcing single-row inserts (`numRows=1`) fixes the argument count error, but
causes connection state corruption after many `Exec` calls, resulting in
"Unexpected packet Query received from client" errors. A proper fix requires
using clickhouse-go's native Batch API.

**Status**: `TestNewBatchInsert` is **skipped** for ClickHouse.
See Development Log for full investigation details.

### 4. UPDATE Statement: PrepareContext Limitation

ClickHouse uses `ALTER TABLE ... UPDATE` syntax instead of standard SQL UPDATE.
While sq's `PrepareUpdateStmt` correctly generates this syntax, clickhouse-go's
`PrepareContext()` only supports **INSERT and SELECT statements**. Any other
statement type (including ALTER TABLE) is rejected with "invalid INSERT query"
because the driver incorrectly categorizes it as an INSERT and fails validation.

Direct execution via `ExecContext()` works, but `PrepareUpdateStmt` requires
`PrepareContext()` for parameter binding.

**Status**: `TestSQLDriver_PrepareUpdateStmt` is **skipped** for ClickHouse.
See Development Log for full investigation details.

## Development Log

### 2026-01-19: ClickHouse Query Test Overrides

Added `drivertype.ClickHouse` entries to all libsq query test cases that had
`drivertype.MySQL` overrides. Both databases use backticks for identifier
quoting, so the SQL strings are identical.

#### Rownum Tests: Proper SQL Overrides Required

Contrary to the original plan (which suggested excluding rownum tests like
MySQL), ClickHouse needed proper SQL overrides with actual SQL strings.
ClickHouse supports `row_number() OVER (ORDER BY 1)` syntax with backtick
quoting, unlike MySQL which uses a different variable-based implementation.

Example ClickHouse rownum override:

```go
drivertype.ClickHouse: "SELECT (row_number() OVER (ORDER BY 1)) AS `rownum()` FROM `actor`",
```

#### Join Tests: Pre-existing Column Naming Issue

Some ClickHouse join tests fail due to a pre-existing issue with column naming
behavior in JOIN query results. This is a fundamental difference in how
ClickHouse reports column metadata compared to other SQL databases.

##### The Problem

When executing JOIN queries, databases return column metadata via Go's
`sql.ColumnType.Name()` method. Most databases (PostgreSQL, MySQL, SQLite,
SQL Server) return just the column name:

```text
Query: SELECT * FROM actor JOIN film_actor ON actor.actor_id = film_actor.actor_id

PostgreSQL/MySQL/SQLite column names:
  actor_id, first_name, last_name, last_update, actor_id, film_id, last_update
  |<-------- from actor -------->| |<----- from film_actor ----->|
```

ClickHouse, however, returns **qualified column names** in `table.column` format:

```text
ClickHouse column names:
  actor.actor_id, actor.first_name, actor.last_name, actor.last_update,
  film_actor.actor_id, film_actor.film_id, film_actor.last_update
```

##### How sq Handles Duplicate Column Names

sq has a column munging mechanism (`driver.MungeResultColNames`) that handles
duplicate column names in result sets. When columns have identical names, the
default template renames them:

```go
// Default template: "{{.Name}}{{with .Recurrence}}_{{.}}{{end}}"
// Input:  [actor_id, first_name, last_name, last_update, actor_id, film_id, last_update]
// Output: [actor_id, first_name, last_name, last_update, actor_id_1, film_id, last_update_1]
```

This mechanism is defined in `libsq/driver/record.go` via `OptResultColRename`.

##### Why ClickHouse Fails

Because ClickHouse returns `actor.actor_id` and `film_actor.actor_id` as
distinct names (not duplicates), sq's column munging doesn't trigger:

```go
// ClickHouse input (no duplicates detected):
//   [actor.actor_id, actor.first_name, ..., film_actor.actor_id, film_actor.film_id, ...]
// ClickHouse output (unchanged):
//   [actor.actor_id, actor.first_name, ..., film_actor.actor_id, film_actor.film_id, ...]
```

The join tests use `assertSinkColMungedNames` to verify expected column names:

```go
// From libsq/query_join_test.go
colsJoinActorFilmActor = []string{
    "actor_id",
    "first_name",
    "last_name",
    "last_update",
    "actor_id_1",    // Expected: munged duplicate
    "film_id",
    "last_update_1", // Expected: munged duplicate
}
```

But ClickHouse returns:

```go
// Actual ClickHouse result:
[]string{
    "actor.actor_id",
    "actor.first_name",
    "actor.last_name",
    "actor.last_update",
    "film_actor.actor_id",  // Not munged - different name
    "film_actor.film_id",
    "film_actor.last_update", // Not munged - different name
}
```

##### Affected Tests

The following tests in `libsq/query_join_test.go` fail for ClickHouse:

| Test Function | Test Case | Reason |
|--------------|-----------|--------|
| `TestQuery_join_others` | `left_join` | `sinkFns` column name assertion |
| `TestQuery_join_others` | `left_outer_join` | `sinkFns` column name assertion |
| `TestQuery_join_others` | `right_join` | `sinkFns` column name assertion |
| `TestQuery_join_others` | `right_outer_join` | `sinkFns` column name assertion |
| `TestQuery_join_others` | `cross/actor-film_actor/no-constraint` | `sinkFns` column name assertion |

Tests without `sinkFns` assertions pass because they only verify:

- SQL string generation (via `wantSQL` and `override`)
- Record count (via `wantRecCount`)

##### Code Path Analysis

1. Query executed via `libsq/engine.go`
2. Results processed in `drivers/clickhouse/clickhouse.go:RecordMeta()`
3. Column types passed to `drivers/clickhouse/metadata.go:recordMetaFromColumnTypes()`
4. Column names extracted: `ogColNames[i] = colTypeData.Name` (line 443)
5. Names passed to `driver.MungeResultColNames(ctx, ogColNames)` (line 446)
6. Munging doesn't trigger because ClickHouse names are already unique

##### Potential Solutions

1. **ClickHouse-specific column name normalization**: Strip table prefix from
   column names in `recordMetaFromColumnTypes` before passing to munging:

   ```go
   // Strip "table." prefix if present
   name := colTypeData.Name
   if idx := strings.LastIndex(name, "."); idx != -1 {
       name = name[idx+1:]
   }
   ogColNames[i] = name
   ```

   **Trade-off**: Loses table context, may cause issues if user explicitly
   wants qualified names.

2. **Enhanced munging template**: Modify `OptResultColRename` to handle
   qualified names by extracting the base column name for duplicate detection.

   **Trade-off**: More complex template logic, may affect other databases.

3. **ClickHouse setting**: Investigate if ClickHouse has a setting to return
   unqualified column names in JOIN results. The `output_format_pretty_row_numbers`
   and similar settings exist but may not apply here.

4. **Test-level workaround**: Skip `sinkFns` assertions for ClickHouse in join
   tests, or provide ClickHouse-specific expected column names.

   **Trade-off**: Doesn't fix the underlying behavior difference.

5. **Driver-level override**: Implement custom `RecordMeta` that normalizes
   column names specifically for ClickHouse JOIN queries.

##### Root Cause

This is a fundamental behavior difference in the clickhouse-go driver and/or
ClickHouse server itself. The `sql.ColumnType.Name()` method returns what
ClickHouse reports, and ClickHouse chooses to return qualified names for
JOIN queries to disambiguate columns from different tables.

This behavior may be intentional from ClickHouse's perspective (providing
explicit column provenance) but differs from the behavior of other databases
that sq supports.

##### References

- Column munging logic: `libsq/driver/record.go:MungeResultColNames()` (line 662)
- Rename template option: `libsq/driver/record.go:OptResultColRename` (line 605)
- ClickHouse metadata: `drivers/clickhouse/metadata.go:recordMetaFromColumnTypes()`
- Test expectations: `libsq/query_join_test.go:colsJoinActorFilmActor` (line 450)
- Test helper: `libsq/query_test.go:assertSinkColMungedNames()` (line 252)

**Status**: **Resolved**. Implemented Solution 1 (ClickHouse-specific column name
normalization) in `recordMetaFromColumnTypes()`. The table prefix is stripped
from column names before passing to the munging mechanism, enabling consistent
duplicate detection and renaming across all databases.

### 2026-01-19: Investigation of TestNewBatchInsert Failure

#### Issue

`TestNewBatchInsert/@sakila_ch25` fails with error:
`clickhouse [Append]: clickhouse: expected 4 arguments, got 280`

#### Root Cause Analysis

The clickhouse-go driver does not support multi-row parameter binding like
traditional SQL drivers. When sq generates a batch INSERT statement like:

```sql
INSERT INTO t VALUES (?,?,?,?), (?,?,?,?), ... -- 70 rows × 4 columns
```

And calls `Exec(vals...)` with all 280 values flattened, clickhouse-go rejects
this because it expects arguments for a **single row only** (4 arguments), not
multiple rows.

This is a fundamental difference from MySQL, PostgreSQL, SQLite, and SQL Server,
which all accept flattened multi-row arguments.

#### Attempted Solution: Force numRows=1

The initial approach was to override `PrepareInsertStmt` in the ClickHouse
driver to always use `numRows=1`, regardless of the requested batch size:

```go
// In PrepareInsertStmt:
const clickHouseNumRows = 1
_ = numRows // Explicitly ignore the requested batch size
stmt, err := driver.PrepareInsertStmt(
    ctx, d, db, destTbl, destColsMeta.Names(), clickHouseNumRows)
```

This required broader changes to the `StmtExecer` type to track the actual
batch size used by the prepared statement (vs. the requested batch size), so
that `NewBatchInsert` could flush after the correct number of rows:

1. Added `batchSize int` field to `StmtExecer` struct
2. Added `BatchSize()` method to retrieve actual batch size
3. Updated `NewBatchInsert` to use `inserter.BatchSize()` instead of the
   passed-in `batchSize`
4. Updated all 10 callers of `NewStmtExecer` across 5 driver files

#### Result: Partial Success, New Problem

The argument count error was **fixed** - the batch insert no longer fails with
"expected 4 arguments, got 280". The inserts themselves succeed (no error from
`bi.ErrCh`).

However, a **new error** appeared when querying the data afterward:

```text
code: 101, message: Unexpected packet Query received from client
```

#### Connection State Corruption

After performing 200 individual `Exec` calls (one per row) on the same prepared
statement, the ClickHouse connection enters an invalid protocol state. When the
connection is returned to the pool and reused for a subsequent SELECT query,
the ClickHouse server rejects it with "Unexpected packet Query received".

This appears to be a limitation of the clickhouse-go driver's native protocol
implementation when using prepared statements with many repeated Exec calls.
The connection is corrupted but not closed, so it gets reused incorrectly.

#### Key Learnings

1. **clickhouse-go's INSERT semantics differ fundamentally**: Unlike standard
   database/sql drivers, clickhouse-go's prepared INSERT statements expect
   arguments for exactly one row per Exec call.

2. **Multi-row binding is not supported**: The clickhouse-go driver does not
   support the common pattern of `INSERT INTO t VALUES (?,?), (?,?)` with
   flattened arguments.

3. **Single-row workaround has side effects**: While forcing `numRows=1` fixes
   the argument count error, it causes connection state corruption after many
   Exec calls on the same prepared statement.

4. **Protocol state is fragile**: The ClickHouse native protocol apparently
   maintains connection state that isn't properly reset after many statement
   executions, making the connection unusable for subsequent operations.

5. **Proper solution requires native Batch API**: The clickhouse-go driver
   provides a dedicated Batch API (`conn.PrepareBatch()`) designed for bulk
   inserts. This API handles the protocol correctly but requires significant
   architectural changes to integrate with sq's driver abstraction.

#### Potential Future Solutions

1. **Use clickhouse-go's native Batch API**: Implement `PrepareBatch()` instead
   of prepared statements for INSERT operations. This would require:
   - New interface method in `driver.SQLDriver` for batch operations
   - ClickHouse-specific implementation using `conn.PrepareBatch()`
   - Integration with `NewBatchInsert` or a new batch insert path

2. **Connection isolation**: Use dedicated connections for batch operations
   that are not returned to the pool after completion.

3. **Transaction wrapping**: Wrap batch inserts in transactions (though
   ClickHouse has limited transaction support).

4. **Periodic statement recreation**: Close and recreate the prepared statement
   periodically during large batch inserts to reset connection state.

#### References

- [clickhouse-go Batch API](https://clickhouse.com/docs/en/integrations/go#batch)
- Issue context: TestNewBatchInsert expects multi-row parameter binding that
  clickhouse-go doesn't support
- Error codes: ClickHouse protocol error 101 indicates unexpected packet type

### 2026-01-19: Investigation of TestSQLDriver_PrepareUpdateStmt Failure

#### Issue

`TestSQLDriver_PrepareUpdateStmt/@sakila_ch25` fails with error:

```text
invalid INSERT query: ALTER TABLE `actor__g9wtwk8r` UPDATE `first_name` = ?,
`last_name` = ? WHERE actor_id = ?
```

#### Background: ClickHouse UPDATE Syntax

ClickHouse does not support standard SQL `UPDATE` statements. Instead, it uses
`ALTER TABLE ... UPDATE` syntax for row-level modifications:

```sql
-- Standard SQL (not supported by ClickHouse):
UPDATE actor SET first_name = 'John' WHERE actor_id = 1;

-- ClickHouse syntax:
ALTER TABLE actor UPDATE first_name = 'John' WHERE actor_id = 1;
```

The sq ClickHouse driver's `PrepareUpdateStmt` implementation correctly
generates the `ALTER TABLE ... UPDATE` syntax via `buildUpdateStmt()`.

#### Root Cause: clickhouse-go PrepareContext Limitations

The clickhouse-go driver's `db.PrepareContext()` function has strict
limitations on what SQL statements can be prepared. It only supports:

1. **INSERT statements** - For batch data insertion via the native protocol
2. **SELECT statements** - For query execution

When `PrepareContext()` receives any other statement type, it attempts to
parse it as one of these two categories. The parsing logic appears to be:

1. Check if statement starts with SELECT-like keywords → treat as query
2. Otherwise → assume it's an INSERT and validate INSERT syntax

When `ALTER TABLE ... UPDATE` is passed to `PrepareContext()`:

1. It's not recognized as a SELECT
2. clickhouse-go assumes it must be an INSERT
3. INSERT syntax validation fails
4. Error returned: "invalid INSERT query: ALTER TABLE ..."

#### Code Path Analysis

```go
// In clickhouse.go PrepareUpdateStmt:
query := buildUpdateStmt(destTbl, destColNames, where)
// query = "ALTER TABLE `tbl` UPDATE `col1` = ?, `col2` = ? WHERE id = ?"

stmt, err := db.PrepareContext(ctx, query)  // ← Fails here
// Error: "invalid INSERT query: ALTER TABLE ..."
```

The `buildUpdateStmt` function correctly generates ClickHouse-compatible
syntax, but `db.PrepareContext()` cannot handle it.

#### Why This Differs from Direct Execution

Interestingly, `ALTER TABLE ... UPDATE` works fine when executed directly:

```go
// This works:
_, err := db.ExecContext(ctx, "ALTER TABLE t UPDATE col = 'value' WHERE id = 1")

// This fails:
stmt, err := db.PrepareContext(ctx, "ALTER TABLE t UPDATE col = ? WHERE id = ?")
```

The difference is that `ExecContext()` sends the query directly to ClickHouse
server, while `PrepareContext()` goes through clickhouse-go's statement
preparation logic which has the INSERT/SELECT restriction.

#### clickhouse-go's Prepared Statement Architecture

The clickhouse-go driver uses prepared statements primarily for:

1. **Batch INSERTs**: Efficiently streaming rows via the native protocol
2. **Parameterized SELECTs**: Safe query execution with bound parameters

For other statement types (DDL, ALTER, etc.), the driver expects direct
execution via `ExecContext()`. This is a design choice in clickhouse-go,
not a ClickHouse server limitation.

#### Key Learnings

1. **clickhouse-go has a limited PrepareContext scope**: Only INSERT and
   SELECT statements can be prepared. All other statement types must use
   direct execution.

2. **Error message is misleading**: "invalid INSERT query" doesn't mean the
   query is malformed—it means clickhouse-go incorrectly categorized a
   non-INSERT statement as an INSERT and then failed validation.

3. **ALTER TABLE UPDATE cannot be parameterized via PrepareContext**: Even
   though ClickHouse server supports parameterized ALTER TABLE UPDATE, the
   Go driver doesn't support preparing such statements.

4. **Workaround requires architectural change**: To support parameterized
   UPDATE in ClickHouse, sq would need to either:
   - Use string interpolation (security risk for user-provided values)
   - Use `ExecContext` with positional args (if clickhouse-go supports it)
   - Implement a custom parameter binding layer

5. **This is separate from the batch insert issue**: While both involve
   `PrepareContext()` limitations, they fail for different reasons:
   - Batch INSERT fails due to multi-row parameter binding expectations
   - UPDATE fails because ALTER TABLE syntax isn't recognized at all

#### Potential Future Solutions

1. **Direct ExecContext with parameters**: Investigate if clickhouse-go's
   `ExecContext()` supports parameter binding for ALTER TABLE statements.
   If so, `PrepareUpdateStmt` could be reimplemented without `PrepareContext`.

2. **Query builder approach**: Build the complete query string with properly
   escaped values, bypassing prepared statements entirely. This requires
   careful escaping to prevent SQL injection.

3. **clickhouse-go feature request**: Request support for preparing ALTER
   TABLE statements in the clickhouse-go driver.

4. **Alternative driver**: Evaluate if other ClickHouse Go drivers (e.g.,
   `mailru/go-clickhouse`) have fewer PrepareContext limitations.

#### Relationship to Known Limitations

This issue is related to but distinct from the existing "Known Limitations"
section item #4 (UPDATE Statement Syntax). That section documents the syntax
difference; this investigation reveals that even with correct syntax, the
prepared statement mechanism doesn't work.

#### References

- clickhouse-go source: Statement preparation logic in `conn.go`
- ClickHouse ALTER TABLE UPDATE docs:
  <https://clickhouse.com/docs/en/sql-reference/statements/alter/update>
- Test: `TestSQLDriver_PrepareUpdateStmt/@sakila_ch25`
- Error location: `drivers/clickhouse/clickhouse.go:828`
