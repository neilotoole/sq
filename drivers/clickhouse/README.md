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

The following are known limitations due to fundamental differences between
ClickHouse and traditional SQL databases.

### 1. Type Roundtrip Limitations

Some `kind.Kind` types cannot roundtrip through ClickHouse because it lacks
native equivalents:

| sq Kind      | Created As | Read Back As    | Notes                    |
|--------------|------------|-----------------|--------------------------|
| `kind.Time`  | `DateTime` | `kind.Datetime` | No time-only type        |
| `kind.Bytes` | `String`   | `kind.Text`     | Binary stored as String  |

This causes `TestDriver_CreateTable_Minimal` to fail for ClickHouse. These are
inherent database limitations, not driver bugs.

### 2. CopyTable Returns Zero Rows

`CopyTable` returns 0 for the copied row count because ClickHouse's
`INSERT ... SELECT` doesn't report affected rows. The operation succeeds, but
the count is unavailable. This causes `TestDriver_CopyTable` to fail the row
count assertion.

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
