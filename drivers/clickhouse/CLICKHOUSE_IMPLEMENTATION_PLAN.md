# ClickHouse Support Implementation Plan

## Overview

Add comprehensive ClickHouse support to `sq` following the proven patterns established with the Oracle driver implementation. ClickHouse is a high-performance, column-oriented SQL database management system (DBMS) for online analytical processing (OLAP).

---

## Phase 1: Research & Setup (2-3 hours)

### 1.1 ClickHouse Fundamentals Research

**Key Characteristics to Understand:**
- Column-oriented storage architecture
- MergeTree table engine family (primary storage engine)
- No traditional transactions (eventual consistency)
- Specialized for analytical queries (OLAP vs OLTP)
- Unique type system (Arrays, Tuples, Maps, Nested, etc.)
- Two connection protocols: HTTP (port 8123) and Native (port 9000)
- Case-sensitive identifiers (unlike Oracle)
- Different SQL dialect quirks

**Research Tasks:**
- [ ] Review [ClickHouse documentation](https://clickhouse.com/docs/en/intro)
- [ ] Study ClickHouse data types and their mappings
- [ ] Understand MergeTree engines and table creation syntax
- [ ] Review ClickHouse SQL dialect differences (LIMIT vs LIMIT OFFSET, etc.)
- [ ] Investigate primary key and sorting key concepts
- [ ] Research ClickHouse-specific system tables (`system.*`)

### 1.2 Choose Go Driver

**Options:**
1. **clickhouse-go (recommended)**
   - Repository: https://github.com/ClickHouse/clickhouse-go
   - Official ClickHouse driver
   - Supports both HTTP and native protocol
   - Active maintenance by ClickHouse team
   - Version: v2.x (latest)

2. **ch-go** (alternative)
   - Lower-level, faster but more complex
   - May be overkill for sq's needs

**Decision:** Use `clickhouse-go` v2 (official, well-documented, sql/database compatible)

### 1.3 Create Development Branch

```bash
cd /Users/65720/Development/Projects/sq
git checkout master
git pull
git checkout -b feature/clickhouse-support
```

### 1.4 Set Up Test Infrastructure

**Docker Setup:**
- ClickHouse Server (latest or specific version like 23.8)
- Standard HTTP port: 8123
- Standard Native port: 9000
- Test database: `testdb`
- Test user: `testuser` / `testpass`

**Create Files:**
- `drivers/clickhouse/docker-compose.yml`
- `drivers/clickhouse/.env` (if needed for configs)

---

## Phase 2: Core Driver Implementation (4-6 hours)

### 2.1 Project Structure

```
drivers/clickhouse/
├── README.md                    # Installation & usage guide
├── docker-compose.yml           # Test infrastructure
├── test-integration.sh          # Integration test runner
├── test-sq-cli.sh              # CLI end-to-end tests
├── TEST_SQ_CLI.md              # CLI test documentation
├── clickhouse.go               # Main driver implementation
├── metadata.go                 # Schema introspection
├── render.go                   # SQL generation & type mapping
├── typeconv.go                 # Type conversions
└── clickhouse_test.go          # Integration tests
```

### 2.2 Driver Registration (`drivers/clickhouse/clickhouse.go`)

**Key Components:**

```go
package clickhouse

import (
    "context"
    "database/sql"

    "github.com/ClickHouse/clickhouse-go/v2"

    "github.com/neilotoole/sq/libsq/core/errz"
    "github.com/neilotoole/sq/libsq/driver"
    "github.com/neilotoole/sq/libsq/source"
    "github.com/neilotoole/sq/libsq/source/drivertype"
    "github.com/neilotoole/sq/libsq/source/metadata"
)

const (
    Type = drivertype.ClickHouse

    // Connection defaults
    defaultPort = 9000
)

func init() {
    // Register with database/sql
    driver.RegisterProvider(Type, &Provider{})
}
```

**Implementation Tasks:**
- [ ] Implement `Provider` interface
- [ ] Implement `Driver` interface
- [ ] Handle connection string parsing: `clickhouse://user:pass@host:port/database`
- [ ] Configure connection options (timeout, compression, etc.)
- [ ] Implement `Ping()` for connection verification
- [ ] Handle ClickHouse-specific errors and error wrapping
- [ ] Implement `Close()` for cleanup

**Connection String Examples:**
```
clickhouse://default:@localhost:9000/default
clickhouse://testuser:testpass@clickhouse.example.com:9000/analytics
clickhouse://user:pass@host:9000/db?dial_timeout=10s&compress=true
```

### 2.3 SQL Dialect (`clickhouse.go`)

**Dialect Methods to Implement:**
- [ ] `Dialect().Enquote()` - Backtick identifier quoting: `` `column_name` ``
- [ ] `Dialect().Placeholders()` - ClickHouse uses `?` for placeholders
- [ ] `SQLDriver()` - Return the sql.Driver instance
- [ ] `DBProperties()` - Database version, properties

**ClickHouse SQL Quirks:**
- Uses backticks for identifiers (not double quotes)
- `LIMIT n OFFSET m` syntax (standard SQL)
- `LIMIT m, n` also supported (MySQL-like)
- No BEGIN/COMMIT/ROLLBACK (but has batch inserts)
- `OPTIMIZE TABLE` instead of `VACUUM`
- Different `INSERT` syntax (supports multiple formats)

### 2.4 Metadata Extraction (`metadata.go`)

**System Tables to Query:**
- `system.databases` - List all databases
- `system.tables` - List all tables
- `system.columns` - Column information
- `system.parts` - Table parts and storage info

**Functions to Implement:**

```go
// getSourceMetadata returns metadata for the ClickHouse source
func getSourceMetadata(ctx context.Context, src *source.Source, db *sql.DB, noSchema bool) (*metadata.Source, error)

// getTablesMetadata returns metadata for all tables in database
func getTablesMetadata(ctx context.Context, db *sql.DB, dbName string) ([]*metadata.Table, error)

// getTableMetadata returns metadata for a specific table
func getTableMetadata(ctx context.Context, db *sql.DB, dbName, tblName string) (*metadata.Table, error)

// getColumnsMetadata returns metadata for all columns in a table
func getColumnsMetadata(ctx context.Context, db *sql.DB, dbName, tblName string) ([]*metadata.Column, error)

// getTableStats returns row count and size statistics
func getTableStats(ctx context.Context, db *sql.DB, dbName, tblName string) (rowCount int64, sizeBytes int64, error)
```

**Metadata Queries:**

```sql
-- Get database version
SELECT version()

-- Get current database
SELECT currentDatabase()

-- List tables
SELECT name, engine, total_rows, total_bytes
FROM system.tables
WHERE database = ?
ORDER BY name

-- Get columns
SELECT name, type, default_kind, default_expression, comment
FROM system.columns
WHERE database = ? AND table = ?
ORDER BY position

-- Get table size
SELECT sum(rows) as row_count, sum(bytes) as size_bytes
FROM system.parts
WHERE database = ? AND table = ? AND active = 1
```

### 2.5 Type Mapping (`render.go`, `typeconv.go`)

**ClickHouse Type System:**

| sq Kind | ClickHouse Type | Go Type | Notes |
|---------|-----------------|---------|-------|
| Int | Int64, Int32, Int16, Int8 | int64, int32, int16, int8 | Signed integers |
| Int | UInt64, UInt32, UInt16, UInt8 | uint64, uint32, uint16, uint8 | Unsigned integers |
| Float | Float64, Float32 | float64, float32 | IEEE floating point |
| Decimal | Decimal(P,S) | string/decimal | Fixed precision |
| Text | String, FixedString(N) | string | Variable or fixed length |
| Bool | Bool, UInt8 | bool, uint8 | Bool is alias for UInt8 |
| Datetime | DateTime, DateTime64 | time.Time | Unix timestamp based |
| Date | Date, Date32 | time.Time | Days since epoch |
| Bytes | String (for binary) | []byte | Binary data |
| UUID | UUID | string/uuid.UUID | UUID type |
| Array | Array(T) | []T | Nested arrays |
| JSON | String (JSON stored as string) | string | Or use JSONEachRow format |

**Special ClickHouse Types (Advanced):**
- `Tuple(T1, T2, ...)` - Multiple types in one column
- `Map(K, V)` - Key-value pairs
- `Nested(...)` - Nested structures
- `Enum8`, `Enum16` - Enumerated types
- `IPv4`, `IPv6` - IP address types
- `LowCardinality(T)` - Optimized for low cardinality data

**Functions to Implement:**

```go
// kindFromDBTypeName maps ClickHouse type to sq kind
func kindFromDBTypeName(colTypeInfo *sql.ColumnType, colName, dbTypeName string) kind.Kind

// dbTypeNameFromKind maps sq kind to ClickHouse type for CREATE TABLE
func dbTypeNameFromKind(knd kind.Kind, colName string) string

// buildCreateTableStmt generates CREATE TABLE statement
func buildCreateTableStmt(tblDef *schema.Table) string
```

**CREATE TABLE Considerations:**
- Must specify table engine (default: MergeTree)
- Must specify ORDER BY clause for MergeTree
- Primary key is optional (defaults to ORDER BY)
- Partition key is optional

**Example CREATE TABLE:**
```sql
CREATE TABLE my_table (
    id Int64,
    name String,
    created DateTime,
    value Float64
)
ENGINE = MergeTree()
ORDER BY id
```

### 2.6 DDL Operations (`render.go`)

**Operations to Implement:**

```go
// CreateTable creates a new table
func (d *driver) CreateTable(ctx context.Context, db *sql.DB, tblDef *schema.Table) error

// DropTable drops a table
func (d *driver) DropTable(ctx context.Context, db *sql.DB, tblName string, ifExists bool) error

// TruncateTable truncates a table (ClickHouse: TRUNCATE TABLE)
func (d *driver) TruncateTable(ctx context.Context, db *sql.DB, tblName string) error

// AlterTableAddColumn adds a column
func (d *driver) AlterTableAddColumn(ctx context.Context, db *sql.DB, tblName, colName, colType string) error

// AlterTableRenameColumn renames a column
func (d *driver) AlterTableRenameColumn(ctx context.Context, db *sql.DB, tblName, oldName, newName string) error

// CopyTable copies data from one table to another
func (d *driver) CopyTable(ctx context.Context, db *sql.DB, fromTable, toTable string, copyData bool) error
```

**ClickHouse DDL Syntax:**
```sql
-- Create table
CREATE TABLE IF NOT EXISTS table_name (...) ENGINE = MergeTree() ORDER BY ...

-- Drop table
DROP TABLE IF EXISTS table_name

-- Truncate table
TRUNCATE TABLE table_name

-- Add column
ALTER TABLE table_name ADD COLUMN column_name Type

-- Rename column
ALTER TABLE table_name RENAME COLUMN old_name TO new_name

-- Insert from select
INSERT INTO dest_table SELECT * FROM source_table
```

### 2.7 Query Operations

**Pagination:**
- ClickHouse uses standard `LIMIT n OFFSET m`
- Also supports MySQL-style `LIMIT m, n`

**Batch Inserts:**
- ClickHouse excels at batch operations
- Use prepared statements with batch execution
- Consider using `INSERT INTO ... FORMAT CSV` for bulk loads

---

## Phase 3: Testing Infrastructure (3-4 hours)

### 3.1 Docker Test Environment (`docker-compose.yml`)

```yaml
version: '3.8'

services:
  clickhouse:
    image: clickhouse/clickhouse-server:23.8
    container_name: clickhouse-clickhouse-1
    hostname: clickhouse
    ports:
      - "8123:8123"  # HTTP
      - "9000:9000"  # Native
    environment:
      CLICKHOUSE_DB: testdb
      CLICKHOUSE_USER: testuser
      CLICKHOUSE_PASSWORD: testpass
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
    volumes:
      - ./init-scripts:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD", "clickhouse-client", "--query", "SELECT 1"]
      interval: 5s
      timeout: 3s
      retries: 20
    networks:
      - clickhouse_network

  # Optional: Add Postgres for cross-database testing
  postgres:
    image: postgres:15-alpine
    container_name: clickhouse-postgres-1
    environment:
      POSTGRES_USER: testuser
      POSTGRES_PASSWORD: testpass
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U testuser"]
      interval: 5s
      timeout: 3s
      retries: 20
    networks:
      - clickhouse_network

networks:
  clickhouse_network:
    driver: bridge
```

### 3.2 Test Data Setup (`init-scripts/01-create-tables.sql`)

```sql
-- Create test tables with sample data
CREATE TABLE users (
    id UInt64,
    username String,
    email String,
    created DateTime,
    is_active UInt8
) ENGINE = MergeTree()
ORDER BY id;

INSERT INTO users VALUES
    (1, 'alice', 'alice@example.com', '2024-01-01 10:00:00', 1),
    (2, 'bob', 'bob@example.com', '2024-01-02 11:00:00', 1),
    (3, 'charlie', 'charlie@example.com', '2024-01-03 12:00:00', 0);

CREATE TABLE events (
    event_id UInt64,
    user_id UInt64,
    event_type String,
    event_time DateTime,
    properties String  -- JSON as string
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_time)
ORDER BY (user_id, event_time);

INSERT INTO events VALUES
    (1, 1, 'login', '2024-01-01 10:05:00', '{"ip":"192.168.1.1"}'),
    (2, 1, 'page_view', '2024-01-01 10:10:00', '{"page":"/home"}'),
    (3, 2, 'login', '2024-01-02 11:05:00', '{"ip":"192.168.1.2"}');
```

### 3.3 Integration Tests (`clickhouse_test.go`)

**Test Structure (following Oracle pattern):**

```go
package clickhouse_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/neilotoole/sq/drivers/clickhouse"
    "github.com/neilotoole/sq/libsq/source"
)

const (
    testDSN = "clickhouse://testuser:testpass@localhost:9000/testdb"
)

func TestConnection(t *testing.T) {
    // Test basic connectivity
}

func TestSourceMetadata(t *testing.T) {
    // Test schema introspection
}

func TestCreateAndDropTable(t *testing.T) {
    // Test table creation and deletion
}

func TestInsertAndQuery(t *testing.T) {
    // Test data insertion and querying
}

func TestTypeMappings(t *testing.T) {
    // Test all supported type conversions
}

func TestCrossDatabase(t *testing.T) {
    // Test data movement between ClickHouse and Postgres
}

func TestBatchInsert(t *testing.T) {
    // Test bulk insert performance
}

func TestArrayTypes(t *testing.T) {
    // Test ClickHouse-specific Array types
}
```

**Test Coverage Goals:**
- [ ] Connection and ping
- [ ] Source metadata extraction
- [ ] Database and table listing
- [ ] Column metadata
- [ ] Create/drop table
- [ ] Insert/select data
- [ ] All basic type mappings (Int, String, DateTime, etc.)
- [ ] ClickHouse-specific types (Array, UUID)
- [ ] Batch operations
- [ ] Cross-database with Postgres
- [ ] Error handling

### 3.4 Integration Test Runner (`test-integration.sh`)

```bash
#!/usr/bin/env bash
#
# test-integration.sh - Run ClickHouse driver integration tests
#

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WITH_POSTGRES=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --with-pg|--with-postgres)
            WITH_POSTGRES=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Start containers
echo "Starting ClickHouse container..."
cd "$SCRIPT_DIR"
if [ "$WITH_POSTGRES" = true ]; then
    docker-compose up -d clickhouse postgres
else
    docker-compose up -d clickhouse
fi

# Wait for health
echo "Waiting for ClickHouse to be ready..."
timeout 60s bash -c 'until docker-compose exec -T clickhouse clickhouse-client --query "SELECT 1" &>/dev/null; do sleep 2; done'

# Run tests
echo "Running integration tests..."
if [ "$WITH_POSTGRES" = true ]; then
    go test -v -tags=integration,crossdb -race -timeout=5m
else
    go test -v -tags=integration -race -timeout=5m
fi

# Cleanup
echo "Tests completed. To stop containers: docker-compose down"
```

### 3.5 CLI End-to-End Tests (`test-sq-cli.sh`)

**Test Scenarios:**
1. **Prerequisites Check**
   - Verify `sq` binary exists
   - Verify Docker is running

2. **ClickHouse Connectivity**
   - Add ClickHouse as data source
   - List data sources
   - Inspect ClickHouse schema
   - Execute simple query

3. **Table Operations**
   - Create table in ClickHouse
   - Insert data
   - Query with filters
   - Verify row counts

4. **Cross-Database Operations**
   - Query Postgres data
   - Copy data from Postgres to ClickHouse
   - Verify data integrity
   - Query ClickHouse with aggregations

5. **Type Mappings**
   - Create table with various types
   - Insert and verify data
   - Test type conversions

**Key Learnings from Oracle Implementation:**
- Use `/usr/bin/grep` to avoid alias issues
- Handle JSON output with proper spacing in grep patterns
- Use `TO_CHAR()` equivalents for consistent string output
- Test with `2>&1` redirection for error capture
- Make DSN configurable via environment variables

---

## Phase 4: Documentation (2-3 hours)

### 4.1 Driver README (`drivers/clickhouse/README.md`)

**Sections:**
- Overview of ClickHouse integration
- Installation requirements (none! Pure Go driver)
- Connection string format
- Usage examples
- Supported features
- Known limitations
- Type mapping table
- Troubleshooting guide

### 4.2 CLI Testing Documentation (`TEST_SQ_CLI.md`)

**Sections:**
- What the CLI tests cover
- Prerequisites
- Usage instructions
- Configuration options
- Troubleshooting
- Comparison with integration tests

### 4.3 Architecture Notes (`ARCHITECTURE.md` or inline comments)

**Document:**
- Why clickhouse-go v2 was chosen
- How MergeTree engine is used by default
- Type mapping decisions
- Identifier quoting strategy (backticks)
- Batch insert approach
- Array/Tuple type handling strategy

---

## Phase 5: Integration with sq Core (1-2 hours)

### 5.1 Register Driver Type

**File:** `libsq/source/drivertype/drivertype.go`

```go
const (
    // ... existing types
    ClickHouse Type = "clickhouse"
)
```

### 5.2 Add URL Scheme Parsing

**File:** `libsq/source/location/location.go`

```go
case "clickhouse":
    fields.DriverType = drivertype.ClickHouse
    fields.Name = strings.TrimPrefix(u.Path, "/")
```

### 5.3 Register in CLI

**File:** `cli/run.go`

```go
import (
    _ "github.com/neilotoole/sq/drivers/clickhouse"
)
```

### 5.4 Update Dependencies

```bash
go get github.com/ClickHouse/clickhouse-go/v2@latest
go mod tidy
```

---

## Phase 6: Advanced Features (Optional, 2-4 hours)

### 6.1 ClickHouse-Specific Optimizations

**Batch Insert Optimization:**
```go
// Use clickhouse-go batch API for bulk inserts
func (d *driver) InsertBatch(ctx context.Context, db *sql.DB, tblName string, rows [][]any) error {
    batch, err := conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s", tblName))
    // ... append rows
    return batch.Send()
}
```

**Compression:**
- Enable LZ4 or ZSTD compression in connection string
- `clickhouse://host:9000/db?compress=lz4`

### 6.2 Array Type Support

**Handle Array Types:**
```go
// Parse Array(T) types
func parseArrayType(clickhouseType string) (elementType string, isArray bool) {
    if strings.HasPrefix(clickhouseType, "Array(") {
        // Extract element type
        return elementType, true
    }
    return clickhouseType, false
}

// Convert []T to ClickHouse Array
func convertToArray(values []any, elementType string) any {
    // Type-specific conversion
}
```

### 6.3 JSON Support

**JSONEachRow Format:**
- ClickHouse supports JSON import/export natively
- Consider `INSERT INTO ... FORMAT JSONEachRow` for JSON data

### 6.4 Query Optimization Hints

**ClickHouse Settings:**
```go
// Add query-level settings
func (d *driver) Query(ctx context.Context, query string, settings map[string]any) (*sql.Rows, error) {
    // SETTINGS max_threads=4, max_memory_usage=10000000000
}
```

---

## Phase 7: Testing & Validation (2-3 hours)

### 7.1 Run All Tests

```bash
# Unit tests (if any)
go test ./drivers/clickhouse/... -v

# Integration tests
cd drivers/clickhouse
./test-integration.sh

# Cross-database tests
./test-integration.sh --with-pg

# CLI tests
./test-sq-cli.sh
```

### 7.2 Manual Testing Checklist

- [ ] Connect to local ClickHouse
- [ ] Connect to ClickHouse Cloud (if available)
- [ ] Inspect large databases (1000+ tables)
- [ ] Query tables with millions of rows
- [ ] Test all data types
- [ ] Copy data from Postgres → ClickHouse
- [ ] Copy data from ClickHouse → SQLite
- [ ] Test error scenarios (bad credentials, network issues)
- [ ] Test with different ClickHouse versions (21, 22, 23)

### 7.3 Performance Validation

**Benchmark Tests:**
- Insert 10K rows (batch)
- Insert 100K rows (batch)
- Query with aggregations
- Complex JOIN operations
- Compare with native clickhouse-client

---

## Phase 8: Pull Request (1 hour)

### 8.1 Pre-PR Checklist

- [ ] All tests passing
- [ ] Documentation complete
- [ ] Code follows project conventions
- [ ] No debug logging left in code
- [ ] Identifiers properly quoted
- [ ] Error messages clear and helpful
- [ ] Comments explain "why" not "what"
- [ ] No unused imports or variables

### 8.2 Commit Strategy

**Suggested Commits:**
1. `Add ClickHouse driver scaffolding and dependencies`
2. `Implement ClickHouse connection and metadata extraction`
3. `Add ClickHouse type mapping and SQL generation`
4. `Add ClickHouse integration tests`
5. `Add ClickHouse CLI end-to-end tests`
6. `Add ClickHouse documentation`
7. `Register ClickHouse driver in sq core`

### 8.3 PR Description

**Use template similar to Oracle PR:**
- Summary
- Motivation
- Changes (organized by file/component)
- Key Features
- Technical Details
- Testing
- Installation Requirements (should be none!)
- Usage Examples
- Known Limitations
- Future Enhancements
- Dependencies
- Testing Instructions

---

## Timeline Estimate

| Phase | Estimated Time | Description |
|-------|---------------|-------------|
| 1. Research & Setup | 2-3 hours | Understand ClickHouse, choose driver, setup branch |
| 2. Core Implementation | 4-6 hours | Driver, metadata, types, SQL generation |
| 3. Testing Infrastructure | 3-4 hours | Docker, integration tests, CLI tests |
| 4. Documentation | 2-3 hours | README, guides, comments |
| 5. Integration | 1-2 hours | Register with sq core |
| 6. Advanced Features | 2-4 hours | Optional optimizations |
| 7. Testing & Validation | 2-3 hours | Comprehensive testing |
| 8. Pull Request | 1 hour | Final review and PR |
| **Total** | **17-26 hours** | **~2-3 full days of work** |

---

## Key Differences from Oracle

### Advantages (ClickHouse is Simpler)

✅ **No External Dependencies**
- Pure Go driver (clickhouse-go)
- No need for Oracle Instant Client
- Works out of the box on all platforms

✅ **Standard Identifiers**
- Case-sensitive identifiers (no uppercasing needed)
- Simple backtick quoting: `` `column` ``
- More straightforward than Oracle's quoting rules

✅ **Better System Tables**
- Clean `system.*` tables for metadata
- No permission issues like Oracle's v$ views
- More intuitive schema introspection

✅ **Simpler SQL**
- Standard LIMIT/OFFSET
- No DUAL table needed
- Clearer syntax overall

### Challenges (ClickHouse is Different)

⚠️ **Table Engine Requirement**
- Must specify ENGINE in CREATE TABLE
- Need to understand MergeTree basics
- ORDER BY clause is required

⚠️ **No Transactions**
- Different consistency model
- Eventual consistency vs immediate
- Need to handle this in docs/tests

⚠️ **Unique Type System**
- Array, Tuple, Map types
- May need special handling
- Consider limiting initial support to basic types

⚠️ **OLAP vs OLTP**
- Optimized for analytics, not transactions
- Different performance characteristics
- May need to document use case differences

---

## Success Criteria

### Minimum Viable Product (MVP)

✅ **Must Have:**
- [ ] Connect to ClickHouse (native protocol)
- [ ] List databases and tables
- [ ] Inspect table schemas (columns, types)
- [ ] Query data with SELECT
- [ ] Insert data (row by row or batch)
- [ ] Create/drop tables (with MergeTree engine)
- [ ] Basic type support (Int, String, DateTime, Float)
- [ ] Cross-database copy (Postgres ↔ ClickHouse)
- [ ] Integration tests passing
- [ ] CLI tests passing
- [ ] Documentation complete

### Nice to Have (Post-MVP)

🎯 **Future Enhancements:**
- [ ] Array type support
- [ ] Tuple type support
- [ ] Map type support
- [ ] UUID type support
- [ ] Compression options
- [ ] HTTP protocol support (alternative to native)
- [ ] Query optimization settings
- [ ] Partition key support in CREATE TABLE
- [ ] Materialized view queries
- [ ] ClickHouse Cloud connection examples

---

## Risk Mitigation

### Potential Issues

1. **Array/Complex Types**
   - **Risk:** ClickHouse arrays may not map cleanly to sq's type system
   - **Mitigation:** Start with basic types only, add complex types in phase 2

2. **Table Engine Complexity**
   - **Risk:** Users may not understand MergeTree requirements
   - **Mitigation:** Use sensible defaults, document clearly

3. **No Transaction Support**
   - **Risk:** Users expect ACID transactions
   - **Mitigation:** Document clearly in README, explain OLAP vs OLTP

4. **Driver Compatibility**
   - **Risk:** clickhouse-go v2 API changes
   - **Mitigation:** Pin to specific version, test with multiple ClickHouse versions

---

## Resources

### Documentation
- [ClickHouse Official Docs](https://clickhouse.com/docs/en/intro)
- [clickhouse-go Driver](https://github.com/ClickHouse/clickhouse-go)
- [SQL Reference](https://clickhouse.com/docs/en/sql-reference)
- [Data Types](https://clickhouse.com/docs/en/sql-reference/data-types)

### Docker Images
- [clickhouse/clickhouse-server](https://hub.docker.com/r/clickhouse/clickhouse-server)

### Examples
- [clickhouse-go Examples](https://github.com/ClickHouse/clickhouse-go/tree/main/examples)

---

## Next Steps

Ready to start implementation? Begin with:

```bash
cd /Users/65720/Development/Projects/sq
git checkout -b feature/clickhouse-support
mkdir -p drivers/clickhouse
cd drivers/clickhouse
```

Then follow Phase 1 (Research & Setup) to understand ClickHouse fundamentals before writing any code.

---

**Questions? Issues? Track progress using TodoWrite tool throughout implementation!**
