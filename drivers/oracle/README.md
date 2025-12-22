# Oracle Database Driver for SQ

Oracle database driver implementation for SQ using godror.

## Status

✅ **Core implementation complete** - All MVP features implemented and building successfully.

## Features Implemented

### Core Driver Features

- ✅ Provider and Driver registration
- ✅ Connection management via godror
- ✅ Oracle-specific SQL dialect (`:1, :2, :3` placeholders, double-quote identifiers)
- ✅ Error handling with Oracle error codes (ORA-xxxxx)

### Type System

- ✅ Bidirectional type mapping (Oracle types ↔ kind.Kind)
- ✅ Support for: NUMBER, VARCHAR2, CHAR, CLOB, BLOB, DATE, TIMESTAMP, BINARY_FLOAT, BINARY_DOUBLE
- ✅ BOOLEAN emulation using NUMBER(1,0)

### Metadata Operations

- ✅ CurrentSchema() - Query current schema via SYS_CONTEXT
- ✅ ListSchemas() - List user schemas from all_users
- ✅ Schema inspection via Oracle data dictionary (USER_TABLES, USER_TAB_COLUMNS, USER_CONSTRAINTS)
- ✅ Table and column metadata extraction
- ✅ Primary key detection

### DDL Operations

- ✅ CreateTable() - Generate and execute CREATE TABLE statements
- ✅ DropTable() - DROP TABLE with CASCADE CONSTRAINTS
- ✅ AlterTableAddColumn() - Add columns to existing tables
- ✅ AlterTableRename() - Rename tables
- ✅ AlterTableRenameColumn() - Rename columns
- ✅ Truncate() - TRUNCATE TABLE with optional storage reset

### DML Operations

- ✅ PrepareInsertStmt() - Batch inserts with proper placeholders
- ✅ PrepareUpdateStmt() - UPDATE statements with WHERE clauses
- ✅ CopyTable() - Create table copies with or without data

### Query Operations

- ✅ TableColumnTypes() - Extract column type information
- ✅ RecordMeta() - Record metadata with proper scan types
- ✅ TableExists() - Check table existence
- ✅ ListTableNames() - List tables and views

## Connection String Format

Oracle connection strings supported by godror:

```
oracle://username:password@hostname:1521/service_name
oracle://username:password@hostname/service_name
oracle://username:password@tns_alias
```

## Testing

**Quick Start:**

```bash
cd drivers/oracle

# Run unit tests only (no database required)
go test -v -short

# Run integration tests (requires Oracle Instant Client + Docker)
./test-integration.sh

# Run all tests including cross-database (requires Postgres too)
./test-integration.sh --with-pg
```

For detailed testing instructions, including:

- Unit vs integration test organization
- `test-integration.sh` script usage
- Oracle Instant Client installation
- Manual Docker setup
- Cross-database testing (Postgres → Oracle)
- Troubleshooting

See **[Testing.md](./Testing.md)**

## Oracle-Specific Notes

### Key Differences from Other Databases

1. **Schema = User**: In Oracle, schemas are tied to users. There's no separate `CREATE SCHEMA` command.

2. **DATE Type**: Oracle's DATE type includes time (equivalent to DATETIME in other databases).

3. **DUAL Table**: Oracle requires `FROM DUAL` for scalar queries.

4. **Identifiers**: Oracle folds unquoted identifiers to uppercase. SQ uses double quotes consistently.

5. **NUMBER Type Handling**:
   - NUMBER(p,0) where p ≤ 19 → treated as Int
   - NUMBER(p,s) where s > 0 → treated as Decimal
   - NUMBER with no precision → treated as Decimal

6. **BOOLEAN**: Oracle has no native BOOLEAN type. Uses NUMBER(1,0) instead.

7. **Empty Strings**: Oracle treats empty strings as NULL.

8. **LIMIT/OFFSET**: Oracle 12c+ syntax:

   ```sql
   SELECT * FROM table
   OFFSET 10 ROWS FETCH NEXT 20 ROWS ONLY
   ```

## Implementation Files

| File | Lines | Purpose |
|------|-------|---------|
| `oracle.go` | ~650 | Main driver, SQLDriver implementation, DDL/DML operations |
| `metadata.go` | ~230 | Data dictionary queries for schema/table/column metadata |
| `render.go` | ~120 | Type mapping and SQL generation |
| `grip.go` | ~70 | Connection grip implementation |
| `errors.go` | ~60 | Oracle error code handling |
| `internal_test.go` | ~80 | Unit tests |
| `docker-compose.yml` | ~20 | Docker setup for integration tests |

## What's Not Included (Post-MVP)

The following features were deferred for future implementation:

- External tools (exp/imp, expdp/impdp wrappers)
- Advanced Oracle features (partitioning, materialized views)
- Oracle-specific optimizations (hints, parallel query)
- Full DBMS_METADATA integration
- Sequence management helpers
- IDENTITY column support details
- Advanced constraint handling

## Usage Example

```go
import (
    "github.com/neilotoole/sq/drivers/oracle"
    "github.com/neilotoole/sq/libsq/driver"
    "github.com/neilotoole/sq/libsq/source/drivertype"
)

// Register the Oracle driver
registry.AddProvider(drivertype.Oracle, &oracle.Provider{Log: log})

// Use with SQ
// sq add oracle://user:pass@localhost:1521/service_name
// sq inspect @oracle_handle
// sq '.actor | .first_name, .last_name' @oracle_handle
```

## Common Oracle Error Codes

| Code | Description | SQ Handling |
|------|-------------|-------------|
| ORA-00942 | Table/view not found | Converted to NotExistError |
| ORA-00955 | Object name exists | Converted to AlreadyExistsError |
| ORA-00904 | Invalid identifier | Converted to NotExistError |
| ORA-01017 | Invalid credentials | Authentication error |
| ORA-12516 | No available handler | Connection pooling issue |
| ORA-12541 | No listener | Connection refused |

## Requirements

- Oracle 12c or later
- Oracle Instant Client (for godror)
- Go 1.19 or later

## Dependencies

- `github.com/godror/godror` v0.40.3 - Oracle driver for Go

## License

Same as main SQ project.
