# sq drivers

This directory holds the concrete `sq` driver implementations: one package per
datasource type (Postgres, MySQL, CSV, JSON, etc.). In `sq` parlance, a "driver"
implements a datasource type. See the
[sq.io drivers section](https://sq.io/docs/drivers) for user-facing
documentation, and [ARCHITECTURE.md](../ARCHITECTURE.md) for a diagram of how
drivers fit into the overall `sq` architecture.

Some drivers carry their own README with maintainer notes (connection strings,
local Docker, test env vars), e.g. [`oracle/README.md`](oracle/README.md) or
[`clickhouse/README.md`](clickhouse/README.md).

## New driver implementations

There are two varieties of drivers: "SQL", and "non-SQL" (aka "Document") drivers.
These are defined by whether they implement just the
[`driver.Driver`](../libsq/driver/driver.go) interface, or also the
[`driver.SQLDriver`](../libsq/driver/driver.go) interface.

For the SQL drivers, it is expected that there exists a `sakiladb/DRIVER_NAME`
docker image, where `DRIVER_NAME` matches the driver type string (e.g.,
`sakiladb/postgres`, `sakiladb/mysql`, `sakiladb/clickhouse`). See the
[sakiladb images](https://hub.docker.com/u/sakiladb). These images contain the
Sakila dataset, enabling uniform integration tests across SQL drivers.

> Note that `SQLite` is a special case, because, although it is a SQL-based
> driver, it is also file-based. That is to say, SQLite implements the
> `driver.SQLDriver` interface, but it does not need a standalone docker
> container to serve up its SQL interface.

**Getting started:** Examine an existing driver implementation as a reference.

For SQL drivers, [`postgres`](postgres) or [`mysql`](mysql) are good templates.

> As mentioned above, for SQL drivers, you'll need a `sakiladb/DRIVER_NAME`
> docker image: [open a `sq` issue](https://github.com/neilotoole/sq/issues)
> when you need that docker image.

For document drivers, see [`csv`](csv) or [`json`](json).

### Driver ship checklist

When you **add a new driver type** (a new value in `sq driver ls`), ship **code,
site docs, and the end-user agent skill** in the same PR. Treat missing
documentation as incomplete work. This is what keeps [sq.io](https://sq.io),
`npx skills add`, and coding agents aligned with the binary.

1. **Driver package**: `drivers/{driver}/` (and registration in
   [`cli/run.go`](../cli/run.go); see [ARCHITECTURE.md](../ARCHITECTURE.md#extension-guide)).
2. **Driver type**: constant in
   [`libsq/source/drivertype/drivertype.go`](../libsq/source/drivertype/drivertype.go).
3. **Tests**: integration tests; for SQL drivers, a `sakiladb/{driver}` image
   and handle in [`testh/sakila/sakila.go`](../testh/sakila/sakila.go) when
   applicable.
4. **sq.io docs**: new page
   `site/content/en/docs/drivers/{driver}.md` (follow an existing driver page;
   link from [`site/content/en/docs/drivers/_index.md`](../site/content/en/docs/drivers/_index.md)).
5. **End-user agent skill**: required for every new driver:
   - Add `skills/sq/references/{driver}.md` (short CLI-focused summary;
     **canonical detail stays on sq.io**). Copy
     [`skills/sq/references/postgres.md`](../skills/sq/references/postgres.md)
     (SQL) or [`skills/sq/references/csv.md`](../skills/sq/references/csv.md)
     (document).
   - Update the driver table and `sq driver ls` examples in
     [`skills/sq/SKILL.md`](../skills/sq/SKILL.md).
   - Run `make fmt` to format the markdown (or `make fmt-check` to verify);
     `skills/**` is formatted by the repo-root [`dprint.json`](../dprint.json).
6. **CHANGELOG**: add an `## Unreleased` / `Added` entry when the driver is
   user-visible (maintainers may edit wording at release time).

Optional: `drivers/{driver}/README.md` for maintainers (connection strings,
local Docker, env vars for tests).

### All drivers

#### Driver type registration

Each driver defines a `Type` constant that corresponds to a value in
[`libsq/source/drivertype/drivertype.go`](../libsq/source/drivertype/drivertype.go).
For example:

```go
// In libsq/source/drivertype/drivertype.go
const ClickHouse = Type("clickhouse")

// In drivers/clickhouse/clickhouse.go
const Type = drivertype.ClickHouse
```

The driver type string (e.g., `"clickhouse"`) is used in:

- Connection URL schemes: `clickhouse://host:port/database`
- Source handles: `@my_clickhouse_db` (the handle itself is user-defined, but
  the driver type determines how the source is processed)
- The `sakiladb` docker image name: `sakiladb/clickhouse`

#### Package structure

A typical driver package contains:

- **`{driver}.go`**: Main driver implementation (`Provider`, `Driver`,
  connection handling).
- **`grip.go`**: Database handle wrapper (`Grip` implementation).
- **`metadata.go`**: Schema introspection and type mapping.
- **`render.go`**: SQL statement generation (for SQL drivers).
- **`errors.go`**: Driver-specific error handling and wrapping (optional).
- **`internal_test.go`**: Exports unexported functions for external test
  packages.
- **`{driver}_test.go`**: Integration tests using the external test package.

#### Test file organization

Driver tests use Go's external test package pattern (`package driver_test`).
To test unexported functions, create an `internal_test.go` file in the main
package that exports them as variables:

```go
// internal_test.go
package clickhouse

// Exported variables for testing unexported functions from external test
// packages. The naming convention is to capitalize the first letter of the
// unexported function name (e.g., buildCreateTableStmt becomes
// BuildCreateTableStmt).

var (
    KindFromDBTypeName   = kindFromDBTypeName
    BuildCreateTableStmt = buildCreateTableStmt
)
```

Then import and use these in your `*_test.go` files:

```go
// metadata_test.go
package clickhouse_test

import "github.com/neilotoole/sq/drivers/clickhouse"

func TestKindFromDBTypeName(t *testing.T) {
    got := clickhouse.KindFromDBTypeName("String")
    require.Equal(t, kind.Text, got)
}
```

#### Test handles

Test handles for sakila sources are defined in
[`testh/sakila/sakila.go`](../testh/sakila/sakila.go). Add your driver's handle
there:

```go
const (
    CH = "@sakila_ch"
)
```

Integration tests that require a running database should use `tu.SkipShort(t, true)`
to skip when running in short mode (`go test -short`):

```go
func TestSmoke(t *testing.T) {
    tu.SkipShort(t, true)

    th := testh.New(t)
    src := th.Source(sakila.CH)
    // ... test code
}
```

#### Oracle (optional)

Some packages (for example [`libsq/driver`](../libsq/driver)) run extra checks when
an Oracle Sakila source is configured. Set `SQ_TEST_SRC__SAKILA_OR` to the
full DSN for the source, as described in
[`testh/testdata/test.sq.yml`](../testh/testdata/test.sq.yml) and
[`oracle/README.md`](oracle/README.md).

For local Oracle with Sakila sample data, run
[`sakiladb/oracle`](https://github.com/sakiladb/oracle)
(`docker run -p 1521:1521 sakiladb/oracle:latest`); see
[`oracle/README.md`](oracle/README.md) for the full local setup.
Then set `SQ_TEST_SRC__SAKILA_OR` to the full DSN, for example
`oracle://sakila:p_ssW0rd@localhost:1521/SAKILA`. The Go driver is pure Go
([go-ora](https://github.com/sijms/go-ora));
no Instant Client is required. When Oracle is reachable, you can narrow
regression runs, for example:

```shell
go test ./libsq/driver/... -run 'Oracle|SourceMetadata_Oracle'
go test ./drivers/oracle/... -short
```

### SQL drivers

#### Type mapping

SQL drivers must map between the database's native types and sq's `kind.Kind`
type system. Key considerations:

- **Wrapper types**: Some databases use wrapper types like `Nullable(T)` or
  `LowCardinality(T)` (ClickHouse). Your type mapping function must unwrap
  these to determine the underlying kind.
- **Parameterized types**: Types like `Decimal(18,4)`, `FixedString(255)`, or
  `VARCHAR(100)` need prefix matching, not exact string comparison.
- **Default to `kind.Text`**: Unknown types should map to `kind.Text` as a safe
  fallback.

Example pattern for handling wrapped types:

```go
func kindFromDBTypeName(dbType string) kind.Kind {
    // Strip Nullable wrapper: Nullable(Int64) -> Int64
    if strings.HasPrefix(dbType, "Nullable(") {
        dbType = dbType[9 : len(dbType)-1]
    }

    // Strip LowCardinality wrapper
    if strings.HasPrefix(dbType, "LowCardinality(") {
        dbType = dbType[15 : len(dbType)-1]
        return kindFromDBTypeName(dbType) // Recurse for nested wrappers
    }

    switch {
    case dbType == "String", strings.HasPrefix(dbType, "FixedString"):
        return kind.Text
    case strings.HasPrefix(dbType, "Int"), strings.HasPrefix(dbType, "UInt"):
        return kind.Int
    // ... etc
    default:
        return kind.Text
    }
}
```

#### Database-specific quirks

Document any database-specific behaviors that affect driver implementation:

- **Transaction support**: Some databases (e.g., ClickHouse) don't support
  traditional ACID transactions.
- **DDL requirements**: ClickHouse's MergeTree engine requires an `ORDER BY`
  clause, and nullable columns cannot be used in the sorting key.
- **Update syntax**: Some databases use non-standard UPDATE syntax (e.g.,
  ClickHouse uses `ALTER TABLE ... UPDATE`).
- **Schema vs catalog**: Terminology varies between databases. Document how
  your driver maps "catalog" and "schema" concepts.

#### Nullable column handling

When creating tables, be aware of how nullable columns interact with other
database features. For example, in ClickHouse:

```go
// ClickHouse's MergeTree engine doesn't allow nullable columns in ORDER BY.
// Find the first NOT NULL column, or use tuple() if all are nullable.
orderByCol := ""
for _, col := range tblDef.Cols {
    if col.NotNull {
        orderByCol = col.Name
        break
    }
}
if orderByCol != "" {
    sb.WriteString("ORDER BY " + enquote(orderByCol))
} else {
    sb.WriteString("ORDER BY tuple()")
}
```

#### Dialect configuration

SQL drivers must return a properly configured `dialect.Dialect` from the
`Dialect()` method. Key settings include:

- **Enquote function**: How to quote identifiers (backticks, double quotes,
  brackets).
- **Placeholder style**: `?` for positional, `$1` for numbered.
- **IntBool**: Whether the database uses integers (0/1) for boolean values.

### Non-SQL drivers

Non-SQL (document) drivers handle file-based data sources like CSV, JSON, and
Excel files. These drivers implement only `driver.Driver`, not `driver.SQLDriver`.

Key considerations:

- **Ingest pattern**: Document drivers typically "ingest" data into a scratch
  SQLite database for query execution. See
  [`csv/ingest.go`](csv/ingest.go) for an example.
- **Type detection**: Implement heuristics to detect column types from data
  values. See [`csv/detect_field_kinds.go`](csv/detect_field_kinds.go).
- **Header detection**: For tabular formats, detect whether the first row
  contains headers. See [`csv/detect_header.go`](csv/detect_header.go).
