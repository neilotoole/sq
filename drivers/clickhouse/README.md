# ClickHouse Driver for SQ

ClickHouse database driver implementation for SQ using [clickhouse-go](https://github.com/ClickHouse/clickhouse-go).

> [!WARNING]
> All testing has been performed on **ClickHouse v25+**.
> Behavior on versions below v25 is not guaranteed and is unsupported.

## Features Implemented

### Core Driver Features

- ✅ Provider and Driver registration
- ✅ Connection management via clickhouse-go v2
- ✅ ClickHouse-specific SQL dialect (? placeholders, backtick identifiers)
- ✅ Error handling and connection management

### Type System

- ✅ Bidirectional type mapping (ClickHouse types ↔ kind.Kind)
- ✅ Support for: Int8-64, UInt8-64, Float32/64, String, FixedString, Date, Date32, DateTime, DateTime64, Decimal, UUID, Bool
- ✅ Nullable type handling: Nullable(T)
- ✅ LowCardinality wrapper support: LowCardinality(T)

### Metadata Operations

- ✅ CurrentCatalog() / CurrentSchema() - Query current database via currentDatabase()
- ✅ ListCatalogs() / ListSchemas() - List databases from system.databases
- ✅ Schema inspection via ClickHouse system tables (system.tables, system.columns, system.databases)
- ✅ Table and column metadata extraction
- ✅ CatalogExists() / SchemaExists() - Database existence checks

### DDL Operations

- ✅ CreateTable() - Generate and execute CREATE TABLE with MergeTree engine
- ✅ DropTable() - DROP TABLE with IF EXISTS support
- ✅ Truncate() - TRUNCATE TABLE
- ✅ MergeTree engine with ORDER BY clause (required by ClickHouse)

### DML Operations

- ✅ PrepareInsertStmt() - Batch inserts with ? placeholders
- ✅ PrepareUpdateStmt() - ALTER TABLE UPDATE statements (ClickHouse syntax)
- ✅ CopyTable() - Create table copies with or without data

### Query Operations

- ✅ TableColumnTypes() - Extract column type information with optional column filtering
- ✅ RecordMeta() - Record metadata with proper scan types
- ✅ TableExists() - Check table existence
- ✅ ListTableNames() - List tables and views (distinguishes by engine type)

## Connection String Format

ClickHouse connection strings supported by clickhouse-go:

```bash
clickhouse://username:password@hostname:9000/database
clickhouse://username:password@hostname:9000/database?param=value
clickhouse://default:@localhost:9000/default
```

Default ports:

| Protocol | Non-Secure | Secure (TLS) |
|----------|------------|--------------|
| Native   | 9000       | 9440         |
| HTTP     | 8123       | 8443         |

> **Note**: The `clickhouse-go` driver does not apply default ports automatically
> (unlike `pgx` for Postgres, etc.). However, sq's ClickHouse driver handles this
> by applying the appropriate default port if not specified:
>
> - **9000** for non-secure connections (default)
> - **9440** for secure connections (when `secure=true` is in the connection string)

## Testing

**Quick Start:**

```bash
cd drivers/clickhouse

# Run unit tests only (no database required)
go test -v -short

# Run integration tests (requires Docker)
./testutils/test-integration.sh

# Run CLI end-to-end tests
./testutils/test-sq-cli.sh
```

For detailed testing instructions, including:

- Unit vs integration test organization
- `testutils/test-integration.sh` script usage
- Docker setup for ClickHouse
- Manual container management
- CLI testing workflow
- Troubleshooting

See **[testutils/Testing.md](./testutils/Testing.md)**

## ClickHouse-Specific Notes

### Key Differences from Other Databases

1. **No Traditional Transactions**: ClickHouse is OLAP-optimized and doesn't support traditional ACID transactions. Inserts are atomic at the batch level.

2. **MergeTree Engine Required**: All tables need an engine specification. SQ uses `ENGINE = MergeTree()` with `ORDER BY` on the first column.

3. **No UPDATE/DELETE by default**: Traditional row-level updates require `ALTER TABLE ... UPDATE` syntax. MergeTree doesn't support standard UPDATE.

4. **ORDER BY Required**: MergeTree engine requires an ORDER BY clause. SQ uses the first column by default.

5. **Type System**:
   - Separate signed (Int*) and unsigned (UInt*) integer types
   - No implicit type coercion
   - Fixed-length strings: FixedString(N)
   - Native Bool type (since ClickHouse 21.12)

6. **Schema = Database**: ClickHouse uses "database" terminology. SQ maps this to schema/catalog concepts.

7. **System Tables**: Metadata queried from `system.databases`, `system.tables`, `system.columns`

8. **Views**: Distinguish regular views (engine='View') from materialized views (engine='MaterializedView')

9. **Default Port Handling**: The `clickhouse-go` driver does not apply default
   ports automatically (unlike `pgx` for Postgres, etc.). SQ handles this by
   automatically applying the appropriate port if not specified: 9000 for
   non-secure connections, or 9440 for secure connections (`secure=true`).

## Implementation Files

| File                           | Lines | Purpose                                                |
| ------------------------------ | ----- | ------------------------------------------------------ |
| `clickhouse.go`                | ~680  | Main driver, SQLDriver implementation, DDL/DML ops     |
| `metadata.go`                  | ~320  | System table queries for schema/table/column metadata  |
| `render.go`                    | ~130  | Type mapping and SQL generation                        |
| `clickhouse_test.go`           | ~120  | Basic integration tests                                |
| `metadata_test.go`             | ~130  | Metadata extraction tests                              |
| `type_test.go`                 | ~220  | Type mapping and nullable tests                        |
| `testutils/docker-compose.yml` | ~30   | Docker setup for integration tests                     |
| `testutils/Testing.md`         | ~400  | Comprehensive testing documentation                    |

## What's Not Included (Post-MVP)

The following features were deferred for future implementation:

- Column compression codecs (LZ4, ZSTD, etc.)
- Table engine options (ReplicatedMergeTree, Distributed, etc.)
- Advanced ClickHouse features (dictionaries, materialized views)
- Partition management
- Sampling and sharding
- ClickHouse-specific optimizations
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

### 3. Batch Insert Argument Handling

`TestNewBatchInsert` fails with "expected 4 arguments, got 280" due to
differences in how ClickHouse handles batch operations compared to traditional
databases.

### 4. UPDATE Statement Syntax

ClickHouse uses `ALTER TABLE ... UPDATE` syntax instead of standard SQL UPDATE.
The `PrepareUpdateStmt` implementation generates this syntax, but it may not
integrate seamlessly with all test frameworks expecting standard prepared
statements.

## Usage Example

```go
import (
    "github.com/neilotoole/sq/drivers/clickhouse"
    "github.com/neilotoole/sq/libsq/driver"
    "github.com/neilotoole/sq/libsq/source/drivertype"
)

// Register the ClickHouse driver
registry.AddProvider(drivertype.ClickHouse, &clickhouse.Provider{Log: log})

// Use with SQ
// sq add clickhouse://default:@localhost:9000/default
// sq inspect @clickhouse_handle
// sq '.events | .timestamp, .user_id, .event_type' @clickhouse_handle
```

## Common Operations

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
# Copy table
sq tbl copy @ch.users @ch.users_backup

# Truncate table
sq tbl truncate @ch.staging_data

# Drop table
sq tbl drop @ch.old_table
```

## Requirements

- ClickHouse Server 25.0 or later (tested and supported version)
- Go 1.19 or later
- Docker (for integration tests)

## Dependencies

- `github.com/ClickHouse/clickhouse-go/v2` v2.42.0 - Official ClickHouse Go driver

## Docker Test Environment

The test environment includes:

- **ClickHouse 23.8**: Main test database
- **PostgreSQL 15**: For cross-database join tests

See `testutils/docker-compose.yml` and `testutils/Testing.md` for details.

## License

Same as main SQ project (MIT).

## Development Log

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
