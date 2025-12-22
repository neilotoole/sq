# SQ Architecture Documentation

## Overview

**SQ** is a data wrangler CLI tool that provides jq-style access to structured data sources including SQL databases (PostgreSQL, MySQL, SQLite, SQL Server) and document formats (CSV, JSON, Excel, etc.). It supports cross-source joins and query execution.

This document focuses on the architecture with special attention to SQL dialects and data types, which are the primary extension points for adding new database support.

## Table of Contents

1. [Project Structure](#project-structure)
2. [SQL Dialects](#sql-dialects)
3. [Data Type System](#data-type-system)
4. [Driver Framework](#driver-framework)
5. [Query Building & Rendering](#query-building--rendering)
6. [Extension Guide](#extension-guide)

---

## Project Structure

```
sq/
├── cli/                          # Command-line interface & commands
│   ├── run.go                    # Bootstrap & driver initialization (lines 276-341)
│   ├── cmd_*.go                  # Individual command implementations
│   ├── config/                   # Configuration management
│   └── output/                   # Output formatting
│
├── libsq/                        # Core library (main logic)
│   ├── driver/                   # Driver framework & registry
│   │   ├── driver.go             # Core Driver & SQLDriver interfaces
│   │   ├── registry.go           # Driver provider registry
│   │   ├── dialect/              # SQL dialect definitions
│   │   │   └── dialect.go        # Dialect struct & operations
│   │   └── grip.go               # Connection wrapper interface
│   │
│   ├── ast/                      # Abstract Syntax Tree
│   │   ├── ast.go                # Query AST structure
│   │   ├── parser.go             # Query parser
│   │   └── render/               # SQL code generator
│   │       ├── render.go         # Core rendering logic
│   │       ├── function.go       # Function rendering
│   │       └── selectcols.go     # SELECT clause rendering
│   │
│   ├── core/                     # Core utilities
│   │   ├── kind/                 # Data type abstraction layer
│   │   │   └── kind.go           # Generic Kind enum (Text, Int, Bool, etc.)
│   │   ├── sqlz/                 # SQL utilities
│   │   └── schema/               # Schema definitions
│   │
│   └── source/                   # Source definitions
│       └── drivertype/           # Driver type constants
│           └── drivertype.go     # Defines: SQLite, Pg, MySQL, MSSQL, etc.
│
├── drivers/                      # Database driver implementations
│   ├── postgres/                 # PostgreSQL driver
│   │   ├── postgres.go           # Main driver & dialect (lines 91-102)
│   │   ├── metadata.go           # Schema extraction & type mapping (lines 29-91)
│   │   └── render.go             # Kind→DBType conversion (lines 22-47)
│   │
│   ├── mysql/                    # MySQL driver
│   │   ├── mysql.go              # Main driver & dialect (lines 111-123)
│   │   ├── metadata.go           # Type mapping
│   │   └── render.go             # Kind→DBType conversion (lines 16-39)
│   │
│   ├── sqlite3/                  # SQLite driver
│   │   ├── sqlite3.go            # Main driver & dialect
│   │   └── metadata.go           # Type mapping & conversion (lines 100-265)
│   │
│   ├── sqlserver/                # SQL Server driver
│   │   ├── sqlserver.go          # Main driver & dialect
│   │   ├── metadata.go           # Type mapping
│   │   └── render.go             # Kind→DBType conversion
│   │
│   ├── csv/                      # CSV/TSV driver (non-SQL)
│   ├── json/                     # JSON driver (non-SQL)
│   ├── xlsx/                     # Excel driver (non-SQL)
│   └── userdriver/               # User-defined driver framework
│       └── xmlud/                # XML user driver implementation
│
└── testh/                        # Test helpers
```

---

## SQL Dialects

### Dialect Definition

**Location:** `libsq/driver/dialect/dialect.go`

The `Dialect` struct defines SQL dialect-specific behavior for each database:

```go
type Dialect struct {
    // Type identifies the database (Pg, MySQL, SQLite, etc.)
    Type drivertype.Type

    // Placeholders generates SQL placeholder strings
    // e.g., PostgreSQL: ($1, $2, $3), MySQL: (?, ?, ?)
    Placeholders func(numCols, numRows int) string

    // Enquote quotes/escapes identifiers
    // PostgreSQL: double-quote, MySQL: backtick
    Enquote func(string) string

    // Ops maps SLQ operators to SQL equivalents
    // e.g., "==" -> "=", "&&" -> "AND"
    Ops map[string]string

    // Joins defines supported JOIN types
    Joins []jointype.Type

    // MaxBatchValues limits values in batch insert
    MaxBatchValues int

    // IntBool indicates if BOOLEAN is handled as INT
    IntBool bool

    // Catalog indicates if DB supports catalog concept
    Catalog bool
}
```

### Dialect Implementations

#### PostgreSQL
**Location:** `drivers/postgres/postgres.go:91-102`

```go
func (d *driveri) Dialect() dialect.Dialect {
    return dialect.Dialect{
        Type:           drivertype.Pg,
        Placeholders:   placeholders,        // Uses $1, $2, $3...
        Enquote:        stringz.DoubleQuote, // "identifier"
        MaxBatchValues: 1000,
        Ops:            dialect.DefaultOps(),
        Joins:          jointype.All(),      // All join types supported
        Catalog:        true,                // Supports catalog
    }
}

func placeholders(numCols, numRows int) string {
    // Generates: ($1, $2, $3), ($4, $5, $6), ...
    rows := make([]string, numRows)
    n := 1
    for i := 0; i < numRows; i++ {
        cols := make([]string, numCols)
        for j := 0; j < numCols; j++ {
            cols[j] = "$" + strconv.Itoa(n)
            n++
        }
        rows[i] = "(" + strings.Join(cols, ", ") + ")"
    }
    return strings.Join(rows, ", ")
}
```

#### MySQL
**Location:** `drivers/mysql/mysql.go:111-123`

```go
func (d *driveri) Dialect() dialect.Dialect {
    return dialect.Dialect{
        Type:           drivertype.MySQL,
        Placeholders:   placeholders,          // Uses ?
        Enquote:        stringz.BacktickQuote, // `identifier`
        IntBool:        true,                  // BOOLEAN as INT
        MaxBatchValues: 250,
        Ops:            dialect.DefaultOps(),
        Joins:          lo.Without(jointype.All(), jointype.FullOuter),
        Catalog:        false,                 // No catalog concept
    }
}

func placeholders(numCols, numRows int) string {
    // Generates: (?, ?, ?), (?, ?, ?), ...
    rows := make([]string, numRows)
    for i := 0; i < numRows; i++ {
        rows[i] = "(" + stringz.RepeatJoin("?", numCols, ",") + ")"
    }
    return strings.Join(rows, ",")
}
```

#### SQLite
**Location:** `drivers/sqlite3/sqlite3.go`

```go
func (d *driveri) Dialect() dialect.Dialect {
    return dialect.Dialect{
        Type:           drivertype.SQLite,
        Placeholders:   placeholders,          // Uses ?
        Enquote:        stringz.DoubleQuote,   // "identifier"
        MaxBatchValues: 500,
        Ops:            dialect.DefaultOps(),
        Joins:          lo.Without(jointype.All(), jointype.FullOuter),
        Catalog:        false,                 // No catalog support
    }
}
```

#### SQL Server
**Location:** `drivers/sqlserver/sqlserver.go`

```go
func (d *driveri) Dialect() dialect.Dialect {
    return dialect.Dialect{
        Type:           drivertype.MSSQL,
        Placeholders:   placeholders,          // Uses @p1, @p2...
        Enquote:        enquote,               // [identifier]
        MaxBatchValues: 1000,
        Ops:            dialect.DefaultOps(),
        Joins:          jointype.All(),
        Catalog:        true,
    }
}

func enquote(s string) string {
    return "[" + s + "]"
}
```

### Dialect Comparison Table

| Database | Location | Type Const | Placeholders | Quote | IntBool | Catalog | Full Outer Join |
|----------|----------|------------|--------------|-------|---------|---------|-----------------|
| PostgreSQL | `drivers/postgres/` | `drivertype.Pg` | `$1, $2...` | `"` | No | Yes | Yes |
| MySQL | `drivers/mysql/` | `drivertype.MySQL` | `?` | `` ` `` | Yes | No | No |
| SQLite | `drivers/sqlite3/` | `drivertype.SQLite` | `?` | `"` | No | No | No |
| SQL Server | `drivers/sqlserver/` | `drivertype.MSSQL` | `@p1, @p2...` | `[ ]` | No | Yes | Yes |

---

## Data Type System

### Generic Type Abstraction (Kind)

**Location:** `libsq/core/kind/kind.go`

The `Kind` type provides a generic abstraction over all data types. This is the canonical type system that all drivers map to/from:

```go
type Kind int

const (
    Unknown   Kind = 0  // Unknown type
    Null             1  // NULL
    Text             2  // Text/String
    Int              3  // Integer
    Float            4  // Floating point
    Decimal          5  // Decimal/BigDecimal
    Bool             6  // Boolean
    Datetime         7  // Date + Time
    Date             8  // Date only
    Time             9  // Time only
    Bytes           10  // Bytes/BLOB
)
```

### Type Mapping Architecture

Each driver implements **bidirectional type mapping**:

1. **DB Type → Kind**: Used during schema inspection
2. **Kind → DB Type**: Used during table creation

#### PostgreSQL Type Mapping

##### DB Type → Kind
**Location:** `drivers/postgres/metadata.go:29-91`

```go
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
    switch strings.ToUpper(dbTypeName) {
    case "INT", "INTEGER", "INT2", "INT4", "INT8", "SMALLINT", "BIGINT":
        return kind.Int
    case "VARCHAR", "TEXT", "CHAR", "CHARACTER", "CHARACTER VARYING", "BPCHAR":
        return kind.Text
    case "BOOLEAN", "BOOL":
        return kind.Bool
    case "TIMESTAMP", "TIMESTAMPTZ", "TIMESTAMP WITH TIME ZONE",
         "TIMESTAMP WITHOUT TIME ZONE":
        return kind.Datetime
    case "DATE":
        return kind.Date
    case "TIME", "TIMETZ", "TIME WITH TIME ZONE", "TIME WITHOUT TIME ZONE":
        return kind.Time
    case "BYTEA":
        return kind.Bytes
    case "DECIMAL", "NUMERIC", "MONEY":
        return kind.Decimal
    case "FLOAT", "FLOAT4", "FLOAT8", "DOUBLE PRECISION", "REAL":
        return kind.Float
    case "JSON", "JSONB":
        return kind.Text
    case "UUID":
        return kind.Text
    case "INTERVAL":
        return kind.Int
    default:
        log.Warn("unknown postgres data type", "type", dbTypeName, "column", colName)
        return kind.Unknown
    }
}
```

##### Kind → DB Type
**Location:** `drivers/postgres/render.go:22-47`

```go
func dbTypeNameFromKind(knd kind.Kind) string {
    switch knd {
    case kind.Text:
        return "TEXT"
    case kind.Int:
        return "BIGINT"
    case kind.Float:
        return "DOUBLE PRECISION"
    case kind.Decimal:
        return "DECIMAL"
    case kind.Bool:
        return "BOOLEAN"
    case kind.Datetime:
        return "TIMESTAMP"
    case kind.Date:
        return "DATE"
    case kind.Time:
        return "TIME"
    case kind.Bytes:
        return "BYTEA"
    case kind.Null, kind.Unknown:
        return "TEXT"
    default:
        return "TEXT"
    }
}
```

#### MySQL Type Mapping

##### DB Type → Kind
**Location:** `drivers/mysql/metadata.go`

```go
func kindFromDBTypeName(colName, dbTypeName string) kind.Kind {
    dbTypeName = strings.ToUpper(dbTypeName)

    switch {
    case strings.HasPrefix(dbTypeName, "TINYINT(1)"):
        return kind.Bool
    case strings.HasPrefix(dbTypeName, "TINYINT"),
         strings.HasPrefix(dbTypeName, "SMALLINT"),
         strings.HasPrefix(dbTypeName, "MEDIUMINT"),
         strings.HasPrefix(dbTypeName, "INT"),
         strings.HasPrefix(dbTypeName, "BIGINT"):
        return kind.Int
    case strings.HasPrefix(dbTypeName, "FLOAT"),
         strings.HasPrefix(dbTypeName, "DOUBLE"):
        return kind.Float
    case strings.HasPrefix(dbTypeName, "DECIMAL"),
         strings.HasPrefix(dbTypeName, "NUMERIC"):
        return kind.Decimal
    case strings.HasPrefix(dbTypeName, "VARCHAR"),
         strings.HasPrefix(dbTypeName, "TEXT"),
         strings.HasPrefix(dbTypeName, "CHAR"),
         strings.HasPrefix(dbTypeName, "JSON"):
        return kind.Text
    case strings.HasPrefix(dbTypeName, "DATETIME"),
         strings.HasPrefix(dbTypeName, "TIMESTAMP"):
        return kind.Datetime
    case strings.HasPrefix(dbTypeName, "DATE"):
        return kind.Date
    case strings.HasPrefix(dbTypeName, "TIME"):
        return kind.Time
    case strings.HasPrefix(dbTypeName, "BLOB"),
         strings.HasPrefix(dbTypeName, "BINARY"),
         strings.HasPrefix(dbTypeName, "VARBINARY"):
        return kind.Bytes
    default:
        return kind.Unknown
    }
}
```

##### Kind → DB Type
**Location:** `drivers/mysql/render.go:16-39`

```go
func dbTypeNameFromKind(knd kind.Kind) string {
    switch knd {
    case kind.Text:
        return "TEXT"
    case kind.Int:
        return "INT"
    case kind.Float:
        return "DOUBLE"
    case kind.Decimal:
        return "DECIMAL"
    case kind.Bool:
        return "TINYINT(1)"  // MySQL-specific: BOOLEAN as TINYINT
    case kind.Datetime:
        return "DATETIME"
    case kind.Date:
        return "DATE"
    case kind.Time:
        return "TIME"
    case kind.Bytes:
        return "BLOB"
    case kind.Null, kind.Unknown:
        return "TEXT"
    default:
        return "TEXT"
    }
}
```

#### SQLite Type Mapping

SQLite uses **type affinity** rules for flexible type handling.

##### DB Type → Kind
**Location:** `drivers/sqlite3/metadata.go:211-237`

```go
func determineKind(colName, typeName string, hasDefault bool) kind.Kind {
    typeName = strings.ToUpper(typeName)

    // SQLite type affinity rules
    if strings.Contains(typeName, "INT") {
        return kind.Int
    }
    if strings.Contains(typeName, "TEXT") ||
       strings.Contains(typeName, "CHAR") ||
       strings.Contains(typeName, "CLOB") {
        return kind.Text
    }
    if strings.Contains(typeName, "BLOB") {
        return kind.Bytes
    }
    if strings.Contains(typeName, "REAL") ||
       strings.Contains(typeName, "FLOA") ||
       strings.Contains(typeName, "DOUB") {
        return kind.Float
    }

    // Exact matches
    switch typeName {
    case "BOOLEAN":
        return kind.Bool
    case "DATETIME":
        return kind.Datetime
    case "DATE":
        return kind.Date
    case "TIME":
        return kind.Time
    case "NUMERIC", "DECIMAL":
        return kind.Decimal
    default:
        return kind.Text  // Default affinity
    }
}
```

##### Kind → DB Type
**Location:** `drivers/sqlite3/metadata.go:240-265`

```go
func DBTypeForKind(knd kind.Kind) string {
    switch knd {
    case kind.Text, kind.Null, kind.Unknown:
        return "TEXT"
    case kind.Int:
        return "INTEGER"
    case kind.Float:
        return "REAL"
    case kind.Bytes:
        return "BLOB"
    case kind.Decimal:
        return "NUMERIC"
    case kind.Bool:
        return "BOOLEAN"
    case kind.Datetime:
        return "DATETIME"
    case kind.Date:
        return "DATE"
    case kind.Time:
        return "TIME"
    default:
        return "TEXT"
    }
}
```

### Type System Integration Points

1. **Column Type Detection** (during schema inspection)
   - Driver queries database metadata
   - Converts DB-specific types to `Kind` using `kindFromDBTypeName()`
   - Stores in schema metadata

2. **Table Creation**
   - Input: `schema.Table` with `Col.Kind` values
   - Uses `dbTypeNameFromKind()` to generate CREATE TABLE DDL
   - Each driver generates its own SQL dialect

3. **Value Scanning**
   - `RecordMeta()` creates scanners based on column `Kind`
   - Handles database-specific quirks (e.g., MySQL BOOLEAN as INT)

---

## Driver Framework

### Core Interfaces

**Location:** `libsq/driver/driver.go`

#### Provider Interface (Factory Pattern)

```go
type Provider interface {
    // DriverFor creates a Driver instance for the given type
    DriverFor(typ drivertype.Type) (Driver, error)
}
```

Each driver package implements a `Provider` struct that acts as a factory.

#### Driver Interface (Base)

```go
type Driver interface {
    // Open opens a connection to the data source
    Open(ctx context.Context, src *source.Source) (Grip, error)

    // Ping verifies connectivity to the data source
    Ping(ctx context.Context, src *source.Source) error

    // DriverMetadata returns metadata about the driver
    DriverMetadata() Metadata

    // ValidateSource validates and normalizes a source
    ValidateSource(src *source.Source) (*source.Source, error)
}
```

#### SQLDriver Interface (SQL-specific)

```go
type SQLDriver interface {
    Driver

    // Dialect returns the SQL dialect for this driver
    Dialect() dialect.Dialect

    // Renderer returns the SQL renderer with any overrides
    Renderer() *render.Renderer

    // Schema operations
    CurrentSchema(ctx context.Context, db sqlz.DB) (string, error)
    ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error)

    // Metadata operations
    TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string) ([]*sql.ColumnType, error)
    RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, NewRecordFunc, error)

    // DDL operations
    CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error
    AlterTableAddColumn(ctx context.Context, db sqlz.DB, tblName, colName string, knd kind.Kind) error
    DropTable(ctx context.Context, db sqlz.DB, tbl string, ifExists bool) error

    // Insert operations
    PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string,
                     destCols []string, numRows int) (*StmtExecer, error)
    PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string,
                     destCols []string, where string) (*StmtExecer, error)

    // ... 10+ other methods for various SQL operations
}
```

### Driver Registration

**Location:** `cli/run.go:276-341`

Driver providers are registered during application bootstrap in `FinishRunInit()`:

```go
func FinishRunInit(cfg *config.Config, ru *Run, ...) error {
    dr := driver.NewRegistry(log)  // Create registry

    // Register SQL drivers
    dr.AddProvider(drivertype.SQLite, &sqlite3.Provider{Log: log})
    dr.AddProvider(drivertype.Pg, &postgres.Provider{Log: log})
    dr.AddProvider(drivertype.MSSQL, &sqlserver.Provider{Log: log})
    dr.AddProvider(drivertype.MySQL, &mysql.Provider{Log: log})

    // Register document drivers
    csvp := &csv.Provider{Log: log, Ingester: ru.Grips, Files: ru.Files}
    dr.AddProvider(drivertype.CSV, csvp)
    dr.AddProvider(drivertype.TSV, csvp)

    jsonp := &json.Provider{Log: log, Ingester: ru.Grips, Files: ru.Files}
    dr.AddProvider(drivertype.JSON, jsonp)
    dr.AddProvider(drivertype.JSONA, jsonp)
    dr.AddProvider(drivertype.JSONL, jsonp)

    dr.AddProvider(drivertype.XLSX, &xlsx.Provider{...})

    // Register user-defined drivers (from config)
    for _, udd := range cfg.Ext.UserDrivers {
        udp := &userdriver.Provider{...}
        dr.AddProvider(drivertype.Type(udd.Name), udp)
    }

    ru.DriverRegistry = dr
    return nil
}
```

### Example: PostgreSQL Driver Structure

**Location:** `drivers/postgres/postgres.go`

```go
// Provider is the factory
type Provider struct {
    Log *slog.Logger
}

func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
    if typ != drivertype.Pg {
        return nil, errz.Errorf("unsupported driver type {%s}", typ)
    }
    return &driveri{log: p.Log}, nil
}

// driveri implements SQLDriver interface
type driveri struct {
    log *slog.Logger
}

// SQLDriver method implementations
func (d *driveri) DriverMetadata() driver.Metadata {
    return driver.Metadata{
        Type:        drivertype.Pg,
        Description: "PostgreSQL",
        Doc:         "https://pkg.go.dev/github.com/lib/pq",
        IsSQL:       true,
    }
}

func (d *driveri) Dialect() dialect.Dialect {
    // Returns PostgreSQL-specific dialect (see SQL Dialects section)
}

func (d *driveri) Renderer() *render.Renderer {
    r := render.NewDefaultRenderer()
    // Customize function names
    r.FunctionNames[ast.FuncNameSchema] = "current_schema"
    r.FunctionNames[ast.FuncNameCatalog] = "current_database"
    return r
}

func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
    // Opens PostgreSQL connection
}

func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
    // Generates and executes CREATE TABLE statement
}

// ... 20+ other SQLDriver methods
```

---

## Query Building & Rendering

### AST (Abstract Syntax Tree)

**Location:** `libsq/ast/ast.go`

SQ parses queries into an AST structure:

```
SelectNode (root)
  ├─ TblSelectorNode (FROM table)
  ├─ Columns (SELECT columns)
  │   ├─ ColSelectorNode
  │   └─ FuncNode
  ├─ WhereNode (WHERE conditions)
  │   └─ OperatorNode
  ├─ GroupByNode (GROUP BY)
  ├─ HavingNode (HAVING)
  ├─ OrderByNode (ORDER BY)
  │   └─ OrderByTermNode
  ├─ JoinNode[] (JOINs)
  │   ├─ JoinConstraintNode
  │   └─ TblSelectorNode
  └─ RowRangeNode (LIMIT/OFFSET)

ExprNode (expressions)
  ├─ FuncNode (function calls)
  ├─ OperatorNode (operators)
  ├─ LiteralNode (values)
  └─ SelectorNode (columns)
```

### Renderer (AST → SQL)

**Location:** `libsq/ast/render/render.go`

The `Renderer` struct holds dialect-specific rendering functions:

```go
type Renderer struct {
    // Dialect provides DB-specific operations
    Dialect dialect.Dialect

    // FunctionNames maps AST function names to SQL function names
    // e.g., FuncNameSchema → "current_schema" (Postgres) or "DATABASE" (MySQL)
    FunctionNames map[string]string

    // FunctionOverrides provides custom function rendering
    FunctionOverrides map[string]FuncRenderer

    // Other rendering customizations
    // ...
}

type FuncRenderer func(ctx *Context, fn *ast.FuncNode) (string, error)
```

#### Rendering Process

1. **Parse Query** → AST (SLQ syntax → SelectNode tree)
2. **Build Context** → Includes dialect, args, fragments
3. **Render Fragments**:
   - Columns → `SelectCols()`
   - From → `FromTable()`
   - Where → `Where()`
   - Joins → `Join()`
   - Group By → `GroupBy()`
   - Order By → `OrderBy()`
4. **Assemble SQL** → Combine fragments into final SQL string

#### Custom Rendering per Driver

Each driver can customize the renderer:

**PostgreSQL** (`postgres.go:127-133`):
```go
func (d *driveri) Renderer() *render.Renderer {
    r := render.NewDefaultRenderer()
    r.FunctionNames[ast.FuncNameSchema] = "current_schema"
    r.FunctionNames[ast.FuncNameCatalog] = "current_database"
    return r
}
```

**MySQL** (`mysql.go:134-140`):
```go
func (d *driveri) Renderer() *render.Renderer {
    r := render.NewDefaultRenderer()
    r.FunctionNames[ast.FuncNameSchema] = "DATABASE"
    r.FunctionOverrides[ast.FuncNameCatalog] = doRenderFuncCatalog
    r.FunctionOverrides[ast.FuncNameRowNum] = renderFuncRowNum
    return r
}
```

This allows each database to have its own SQL generation logic while sharing the common AST structure.

---

## Extension Guide

### Adding a New SQL Database Driver

This guide uses Oracle as an example.

#### Step 1: Create Driver Package

Create a new directory: `drivers/oracle/`

Files to create:
- `oracle.go` - Provider, driver implementation, dialect
- `metadata.go` - Schema inspection, type mapping (DB Type → Kind)
- `render.go` - Type conversion (Kind → DB Type)
- `errors.go` - Error handling
- `oracle_test.go` - Tests

#### Step 2: Define Driver Type

**File:** `libsq/source/drivertype/drivertype.go`

```go
const (
    // ... existing types
    Oracle = Type("oracle")
)
```

#### Step 3: Implement Provider & Driver

**File:** `drivers/oracle/oracle.go`

```go
package oracle

import (
    "context"
    "database/sql"
    "github.com/neilotoole/sq/libsq/driver"
    "github.com/neilotoole/sq/libsq/driver/dialect"
    "github.com/neilotoole/sq/libsq/source/drivertype"
    "github.com/neilotoole/sq/libsq/core/stringz"
    "log/slog"

    _ "github.com/godror/godror" // Oracle driver
)

// Provider is the Oracle driver provider
type Provider struct {
    Log *slog.Logger
}

func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
    if typ != drivertype.Oracle {
        return nil, errz.Errorf("unsupported driver type {%s}", typ)
    }
    return &driveri{log: p.Log}, nil
}

type driveri struct {
    log *slog.Logger
}

func (d *driveri) DriverMetadata() driver.Metadata {
    return driver.Metadata{
        Type:        drivertype.Oracle,
        Description: "Oracle Database",
        Doc:         "https://github.com/godror/godror",
        IsSQL:       true,
    }
}

func (d *driveri) Dialect() dialect.Dialect {
    return dialect.Dialect{
        Type:           drivertype.Oracle,
        Placeholders:   placeholders,        // :1, :2, :3...
        Enquote:        stringz.DoubleQuote, // "identifier"
        Joins:          jointype.All(),
        MaxBatchValues: 1000,
        Ops:            dialect.DefaultOps(),
        Catalog:        false,  // Oracle uses schemas, not catalogs
    }
}

func placeholders(numCols, numRows int) string {
    // Oracle uses :1, :2, :3... style placeholders
    rows := make([]string, numRows)
    n := 1
    for i := 0; i < numRows; i++ {
        cols := make([]string, numCols)
        for j := 0; j < numCols; j++ {
            cols[j] = ":" + strconv.Itoa(n)
            n++
        }
        rows[i] = "(" + strings.Join(cols, ", ") + ")"
    }
    return strings.Join(rows, ", ")
}

func (d *driveri) Renderer() *render.Renderer {
    r := render.NewDefaultRenderer()
    // Customize for Oracle
    r.FunctionNames[ast.FuncNameSchema] = "USER"
    r.FunctionOverrides[ast.FuncNameRowNum] = renderRowNum
    return r
}

func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
    // Open Oracle connection
    db, err := sql.Open("godror", src.Location)
    if err != nil {
        return nil, errz.Wrap(err, "open oracle connection")
    }
    return &grip{log: d.log, db: db, src: src}, nil
}

// Implement all other SQLDriver methods...
```

#### Step 4: Define Type Mapping

**File:** `drivers/oracle/metadata.go`

```go
package oracle

import (
    "github.com/neilotoole/sq/libsq/core/kind"
    "log/slog"
    "strings"
)

// kindFromDBTypeName maps Oracle data types to generic Kind
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
    typeName := strings.ToUpper(dbTypeName)

    switch {
    // Numeric types
    case strings.HasPrefix(typeName, "NUMBER"):
        // Oracle NUMBER can be INT, DECIMAL, or FLOAT
        // Check precision/scale if available
        return kind.Decimal  // Conservative default

    case typeName == "BINARY_FLOAT", typeName == "BINARY_DOUBLE":
        return kind.Float

    case typeName == "INTEGER", typeName == "INT", typeName == "SMALLINT":
        return kind.Int

    // String types
    case strings.HasPrefix(typeName, "VARCHAR"),
         strings.HasPrefix(typeName, "CHAR"),
         strings.HasPrefix(typeName, "NVARCHAR"),
         strings.HasPrefix(typeName, "NCHAR"),
         typeName == "CLOB", typeName == "NCLOB":
        return kind.Text

    // Date/Time types
    case typeName == "DATE":
        return kind.Datetime  // Oracle DATE includes time

    case strings.HasPrefix(typeName, "TIMESTAMP"):
        return kind.Datetime

    // Binary types
    case typeName == "BLOB", typeName == "RAW", typeName == "LONG RAW":
        return kind.Bytes

    // Other types
    case typeName == "XMLTYPE":
        return kind.Text

    default:
        log.Warn("unknown oracle data type", "type", dbTypeName, "column", colName)
        return kind.Unknown
    }
}

// Implement schema inspection methods
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB,
                                     schemaPattern string) ([]*schema.Table, error) {
    // Query Oracle data dictionary (USER_TABLES, ALL_TABLES, etc.)
    // Use kindFromDBTypeName() to convert column types
}
```

**File:** `drivers/oracle/render.go`

```go
package oracle

import (
    "github.com/neilotoole/sq/libsq/core/kind"
)

// dbTypeNameFromKind maps generic Kind to Oracle data types
func dbTypeNameFromKind(knd kind.Kind) string {
    switch knd {
    case kind.Text:
        return "VARCHAR2(4000)"

    case kind.Int:
        return "NUMBER(18,0)"

    case kind.Float:
        return "BINARY_DOUBLE"

    case kind.Decimal:
        return "NUMBER"

    case kind.Bool:
        return "NUMBER(1,0)"  // 0 or 1

    case kind.Datetime:
        return "TIMESTAMP"

    case kind.Date:
        return "DATE"

    case kind.Time:
        return "TIMESTAMP"  // Oracle doesn't have TIME type

    case kind.Bytes:
        return "BLOB"

    case kind.Null, kind.Unknown:
        return "VARCHAR2(4000)"

    default:
        return "VARCHAR2(4000)"
    }
}

// Use in CreateTable implementation
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB,
                              tblDef *schema.Table) error {
    var sb strings.Builder
    sb.WriteString("CREATE TABLE ")
    sb.WriteString(d.Dialect().Enquote(tblDef.Name))
    sb.WriteString(" (")

    for i, col := range tblDef.Cols {
        if i > 0 {
            sb.WriteString(", ")
        }
        sb.WriteString(d.Dialect().Enquote(col.Name))
        sb.WriteString(" ")
        sb.WriteString(dbTypeNameFromKind(col.Kind))

        if !col.Nullable {
            sb.WriteString(" NOT NULL")
        }
    }

    sb.WriteString(")")

    _, err := db.ExecContext(ctx, sb.String())
    return err
}
```

#### Step 5: Register the Driver

**File:** `cli/run.go` (in `FinishRunInit()`)

```go
import (
    "github.com/neilotoole/sq/drivers/oracle"
)

func FinishRunInit(cfg *config.Config, ru *Run, ...) error {
    dr := driver.NewRegistry(log)

    // ... existing registrations

    // Add Oracle driver
    dr.AddProvider(drivertype.Oracle, &oracle.Provider{Log: log})

    // ... rest of function
}
```

#### Step 6: Testing

Create comprehensive tests:

**File:** `drivers/oracle/oracle_test.go`

```go
func TestOracle_TypeMapping(t *testing.T) {
    // Test DB Type → Kind conversion
    testCases := []struct {
        dbType   string
        expected kind.Kind
    }{
        {"NUMBER(18,0)", kind.Int},
        {"VARCHAR2(100)", kind.Text},
        {"TIMESTAMP", kind.Datetime},
        // ... more cases
    }

    for _, tc := range testCases {
        result := kindFromDBTypeName(log, "col", tc.dbType)
        assert.Equal(t, tc.expected, result)
    }
}

func TestOracle_KindToDBType(t *testing.T) {
    // Test Kind → DB Type conversion
    testCases := []struct {
        kind     kind.Kind
        expected string
    }{
        {kind.Int, "NUMBER(18,0)"},
        {kind.Text, "VARCHAR2(4000)"},
        {kind.Datetime, "TIMESTAMP"},
        // ... more cases
    }

    for _, tc := range testCases {
        result := dbTypeNameFromKind(tc.kind)
        assert.Equal(t, tc.expected, result)
    }
}
```

### Adding a New Data Type

To add support for a new data type (e.g., `UUID`, `JSON`, `Geometry`):

#### Step 1: Add to Kind Enum

**File:** `libsq/core/kind/kind.go`

```go
const (
    Unknown   Kind = 0
    Null           = 1
    Text           = 2
    // ... existing types
    UUID           = 11  // New type
    Geometry       = 12  // Another new type
)

func (k Kind) String() string {
    switch k {
    // ... existing cases
    case UUID:
        return "uuid"
    case Geometry:
        return "geometry"
    default:
        return "unknown"
    }
}

func (k Kind) MarshalText() ([]byte, error) {
    // Add marshaling for new types
    switch k {
    case UUID:
        return []byte("uuid"), nil
    case Geometry:
        return []byte("geometry"), nil
    // ... existing cases
    }
}
```

#### Step 2: Update Each Driver

For **each** SQL driver, update the type mapping functions:

**PostgreSQL** (`drivers/postgres/metadata.go`):
```go
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
    switch strings.ToUpper(dbTypeName) {
    // ... existing cases
    case "UUID":
        return kind.UUID
    case "GEOMETRY", "GEOGRAPHY":
        return kind.Geometry
    // ... rest of cases
    }
}
```

**PostgreSQL** (`drivers/postgres/render.go`):
```go
func dbTypeNameFromKind(knd kind.Kind) string {
    switch knd {
    // ... existing cases
    case kind.UUID:
        return "UUID"
    case kind.Geometry:
        return "GEOMETRY"
    // ... rest of cases
    }
}
```

**MySQL** (`drivers/mysql/metadata.go`):
```go
func kindFromDBTypeName(colName, dbTypeName string) kind.Kind {
    dbTypeName = strings.ToUpper(dbTypeName)
    switch {
    // ... existing cases
    case strings.HasPrefix(dbTypeName, "UUID"):
        return kind.UUID
    case strings.HasPrefix(dbTypeName, "GEOMETRY"),
         strings.HasPrefix(dbTypeName, "POINT"),
         strings.HasPrefix(dbTypeName, "POLYGON"):
        return kind.Geometry
    // ... rest of cases
    }
}
```

**MySQL** (`drivers/mysql/render.go`):
```go
func dbTypeNameFromKind(knd kind.Kind) string {
    switch knd {
    // ... existing cases
    case kind.UUID:
        return "VARCHAR(36)"  // MySQL doesn't have native UUID
    case kind.Geometry:
        return "GEOMETRY"
    // ... rest of cases
    }
}
```

Repeat for SQLite, SQL Server, and any other SQL drivers.

#### Step 3: Update Value Scanning

If the new type requires special scanning logic, update:

**File:** `drivers/postgres/metadata.go` (or respective driver)

```go
func (d *driveri) RecordMeta(ctx context.Context,
                             colTypes []*sql.ColumnType) (record.Meta, NewRecordFunc, error) {
    // ... existing code

    for i, colType := range colTypes {
        knd := kindFromDBTypeName(d.log, colType.Name(), colType.DatabaseTypeName())

        // Special handling for new types
        switch knd {
        case kind.UUID:
            scanDests[i] = &sql.NullString{}  // Scan as string
        case kind.Geometry:
            scanDests[i] = &[]byte{}  // Scan as bytes
        default:
            // ... existing logic
        }
    }
}
```

### Adding a Non-SQL Document Driver

To add support for a new document format (e.g., Parquet, Avro):

#### Step 1: Create Driver Package

```
drivers/parquet/
├── parquet.go       # Provider, driver implementation
├── ingest.go        # Data ingestion logic
├── detect.go        # File type detection
└── parquet_test.go  # Tests
```

#### Step 2: Implement Driver

**File:** `drivers/parquet/parquet.go`

```go
package parquet

type Provider struct {
    Log      *slog.Logger
    Ingester driver.GripOpenIngester  // For ingesting into scratch DB
    Files    *files.Files
}

func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
    if typ != drivertype.Parquet {
        return nil, errz.Errorf("unsupported type {%s}", typ)
    }
    return &driveri{
        log:      p.Log,
        ingester: p.Ingester,
        files:    p.Files,
    }, nil
}

type driveri struct {
    log      *slog.Logger
    ingester driver.GripOpenIngester
    files    *files.Files
}

func (d *driveri) DriverMetadata() driver.Metadata {
    return driver.Metadata{
        Type:        drivertype.Parquet,
        Description: "Apache Parquet",
        Doc:         "https://parquet.apache.org/",
        IsSQL:       false,  // Document driver
        Monotable:   true,   // Single table per file
    }
}

func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
    // 1. Open Parquet file
    // 2. Read schema
    // 3. Ingest data into scratch database (SQLite)
    // 4. Return Grip to scratch DB

    return d.ingester.OpenIngest(ctx, src, func(ctx context.Context, destGrip driver.Grip) error {
        // Read Parquet data and insert into destGrip
        return ingestParquet(ctx, src, destGrip)
    })
}
```

#### Step 3: Implement Detection

**File:** `drivers/parquet/detect.go`

```go
func DetectParquet(ctx context.Context, log *slog.Logger,
                   openFn files.FileOpenFunc) (drivertype.Type, error) {
    r, err := openFn()
    if err != nil {
        return drivertype.None, err
    }
    defer r.Close()

    // Check for Parquet magic bytes: "PAR1"
    magic := make([]byte, 4)
    if _, err := io.ReadFull(r, magic); err != nil {
        return drivertype.None, nil
    }

    if string(magic) == "PAR1" {
        return drivertype.Parquet, nil
    }

    return drivertype.None, nil
}
```

#### Step 4: Register Driver

**File:** `cli/run.go`

```go
import "github.com/neilotoole/sq/drivers/parquet"

func FinishRunInit(cfg *config.Config, ru *Run, ...) error {
    // ... existing code

    // Register Parquet driver
    parquetp := &parquet.Provider{
        Log:      log,
        Ingester: ru.Grips,
        Files:    ru.Files,
    }
    dr.AddProvider(drivertype.Parquet, parquetp)

    // Register file detector
    ru.Files.AddDriverDetectors(parquet.DetectParquet)

    // ... rest of function
}
```

---

## Key Design Patterns

### 1. Provider/Factory Pattern
- `Provider` interface creates `Driver` instances
- Enables lazy instantiation and polymorphism
- Supports dependency injection

### 2. Strategy Pattern (Dialect)
- `Dialect` encapsulates database-specific behavior
- Placeholders, quoting, operators vary by database
- Renderer uses Dialect for code generation

### 3. Adapter Pattern (Kind)
- `Kind` provides universal type abstraction
- Each driver adapts DB types ↔ Kind
- Enables cross-database operations

### 4. Template Method (Rendering)
- Base `Renderer` provides structure
- Drivers override specific methods
- Flexible for future extensions

### 5. Bridge Pattern (Grip)
- `Grip` decouples driver from connection
- Allows multiple connection implementations
- Supports caching, pooling, transformation

---

## Critical File Reference

### SQL Dialects

| Component | File Location | Lines |
|-----------|---------------|-------|
| Dialect struct | `libsq/driver/dialect/dialect.go` | - |
| PostgreSQL dialect | `drivers/postgres/postgres.go` | 91-102 |
| MySQL dialect | `drivers/mysql/mysql.go` | 111-123 |
| SQLite dialect | `drivers/sqlite3/sqlite3.go` | - |
| SQL Server dialect | `drivers/sqlserver/sqlserver.go` | - |

### Data Types

| Component | File Location | Lines |
|-----------|---------------|-------|
| Kind enum | `libsq/core/kind/kind.go` | - |
| PostgreSQL: DB→Kind | `drivers/postgres/metadata.go` | 29-91 |
| PostgreSQL: Kind→DB | `drivers/postgres/render.go` | 22-47 |
| MySQL: DB→Kind | `drivers/mysql/metadata.go` | - |
| MySQL: Kind→DB | `drivers/mysql/render.go` | 16-39 |
| SQLite: DB→Kind | `drivers/sqlite3/metadata.go` | 211-237 |
| SQLite: Kind→DB | `drivers/sqlite3/metadata.go` | 240-265 |

### Driver Framework

| Component | File Location | Lines |
|-----------|---------------|-------|
| Driver interfaces | `libsq/driver/driver.go` | - |
| Driver registry | `libsq/driver/registry.go` | - |
| Driver types enum | `libsq/source/drivertype/drivertype.go` | - |
| Driver registration | `cli/run.go` | 276-341 |

### Rendering

| Component | File Location | Lines |
|-----------|---------------|-------|
| Renderer struct | `libsq/ast/render/render.go` | - |
| AST definitions | `libsq/ast/ast.go` | - |
| Query parser | `libsq/ast/parser.go` | - |

---

## Summary

The SQ architecture is built on several key principles:

1. **Universal Type Abstraction**: The `Kind` enum provides a common type system that all drivers map to, enabling cross-database compatibility.

2. **Dialect-Aware Rendering**: Each database defines its `Dialect` with specific placeholder styles, quoting rules, and operator mappings. The renderer uses these to generate correct SQL.

3. **Pluggable Driver System**: The Provider/Factory pattern with centralized registration makes it easy to add new databases and document formats.

4. **Bidirectional Type Mapping**: Every SQL driver implements both DB Type → Kind and Kind → DB Type conversions, ensuring seamless data flow.

5. **Clean Separation of Concerns**:
   - **AST**: Query structure (database-agnostic)
   - **Dialect**: Database-specific syntax rules
   - **Driver**: Database-specific implementation
   - **Renderer**: SQL code generation

To extend SQ with new databases or types, follow the patterns established in existing drivers, focusing on the three critical components: **Dialect definition**, **Type mapping**, and **Driver implementation**.
