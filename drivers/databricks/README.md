# Databricks driver: maintainer notes

Implementation notes for the Databricks driver, which wraps the official
[databricks-sql-go](https://github.com/databricks/databricks-sql-go) `database/sql` driver
(Thrift over HTTP, to a SQL warehouse). User documentation is on the
[sq.io Databricks page](https://sq.io/docs/drivers/databricks); cross-driver contributor guidance
is in [`docs/DRIVERS.md`](../../docs/DRIVERS.md). This file covers what fits neither, and says
plainly what has and has not been verified.

## Scope: read-only

The driver implements the read path: open, ping, query, `RecordMeta`, catalog and schema listing,
and metadata. Every `SQLDriver` method that would modify the source (`CreateTable`, `CopyTable`,
`PrepareInsertStmt`, `NewBatchInsert`, `Truncate`, `AlterTable*` and so on) returns an error built by
`errReadOnly`. Native SQL through `sq sql` is not restricted, because it goes straight to
`ExecContext` and `QueryContext`.

Writing needs decisions that need a real workspace to settle: the Delta `CREATE TABLE ... USING`
form, the practical limit on the number of bound parameters (`MaxBatchValues`, currently an
unused placeholder), insert throughput over the HTTP API, and `PrepareUpdateStmt` semantics on
tables that don't support `UPDATE`. A transaction is not an option: `BeginTx` returns "not
implemented", and `libsq.DBWriter` opens one first, so `--insert` into Databricks fails there,
before it sends any DDL or DML.

## Location to DSN

`dsn.go` turns the sq location into the DSN that databricks-sql-go parses. sq does not use dburl for
the `databricks://` scheme (see `location.parseDatabricks`): dburl's `databricks` scheme demands a
user and password, and rewrites the host. The username is the literal `token`, the PAT is the
password, and the HTTP path is the URL path (or the `http_path` param).

Two behaviors are worth knowing:

- **Loopback hosts use `http`.** databricks-sql-go can't be told to skip TLS verification from a
  DSN, and the test server is plain HTTP. A real workspace is never on a loopback address, so
  `localhost`, `127.0.0.1` and `::1` get `http` and everything else gets `https`.
- **Unknown query params are session parameters.** databricks-sql-go sends any param it doesn't
  recognize to the warehouse as a session configuration, so a typo such as `catlog=main` is not
  an error in sq or in the driver library. `sq` validates only the auth params.

The DSN contains the credential. It is never logged. Errors from `sql.Open` and `PingContext` pass
through `scrub`, which replaces the token and secret query values in the message, and
`location.Redact` and `location.IsSecretQueryParam` know about `clientSecret` and `accessToken`.

Logging from databricks-sql-go is process-global and defaults to warnings on stderr. `Provider`
redirects it to sq's debug log, unless `DATABRICKS_LOG_LEVEL` is set.

## Authentication

PAT is the supported method. OAuth M2M (`clientID` and `clientSecret`, no userinfo) is passed
through to databricks-sql-go, which does the token exchange and refresh. The tests check that the
DSN is built right and the secrets are redacted; the OAuth exchange is not exercised.
OAuth U2M is passed through but is an interactive browser flow, and is not a supported workflow.
Reading `~/.databrickscfg`, as asked for in
[#698](https://github.com/neilotoole/sq/issues/698), is not implemented.

## Metadata

Source and table metadata come from the `information_schema` of the current catalog
(`tables`, `columns`, `views`, `table_constraints` and `key_column_usage`), with the schema bound
as a parameter. The legacy `hive_metastore` catalog has no `information_schema`, so
`requireInfoSchema` returns a clear error for `hive_metastore` and `spark_catalog` instead of
failing in a confusing way. Queries against those catalogs work; inspecting them does not.

Row counts are a `SELECT COUNT(*)` per table or view, at most eight at once, like the MySQL and
Postgres drivers. A failure to count one table is logged and leaves its count at zero. Primary
keys and view definitions are also best-effort. Primary keys are informational in Databricks.
Foreign keys, unique constraints, indexes and sizes are not read.

## Type mapping and scanning

`kindFromDBTypeName` accepts both the names that databricks-sql-go reports for result columns
(`BIGINT`, `TIMESTAMP_NTZ`) and those in `information_schema` (`LONG`, `STRUCT`, or
`decimal(10,2)` from `full_data_type`). Nested and interval types come back as strings (JSON for
`ARRAY`, `MAP`, `STRUCT` and `VARIANT`), so they map to `kind.Text`. This flattens the structure,
like ClickHouse arrays ([#545](https://github.com/neilotoole/sq/issues/545)).

databricks-sql-go's `ColumnTypeNullable` always reports "unknown", so every result column is
scanned into a nullable type (`setScanType`).

## String literals

Spark SQL treats `\` as an escape character in string literals and reads `''` as two adjacent
literals, which it concatenates. So the default `stringz.SingleQuote` mis-renders `O'Brien` (as
`OBrien`), and lets a value such as `\' OR 1=1 --` end the literal early. This driver sets
`Dialect.QuoteString` to `quoteString`, which escapes `\` and `'` with a backslash. To make that
work, `libsq/ast/render` now quotes every string literal through `Dialect.QuoteLiteral` rather than
calling `stringz.SingleQuote`. Dialects that leave `QuoteString` nil are unchanged.

## Testing

There are three tiers, following the plan in [#698](https://github.com/neilotoole/sq/issues/698):

1. **Unit tests**: DSN building, type mapping, quoting, row range, error classification.
2. **Fake warehouse**: [`internal/thriftfake`](./internal/thriftfake) is an `httptest` server that
   speaks enough of the Thrift TCLIService protocol for the real databricks-sql-go client to open a
   session, run a statement, and read a column-based result. Handlers return canned results per
   statement. databricks-sql-go has a similar fake, but it and the Thrift bindings are in an
   `internal` package, so they can't be imported. Rather than vendor about 30k lines of generated
   code, `thriftfake` encodes and decodes the few messages involved with the apache/thrift
   protocol primitives. Field IDs come from `TCLIService.thrift`; the generated
   `internal/cli_service` package of databricks-sql-go is the reference. The fake returns results
   as "direct results" so the client doesn't poll, which it does at a one-second interval. The
   tests in [`databricks_test.go`](./databricks_test.go) and [`cli_test.go`](./cli_test.go) use it
   to drive the driver, the SLQ pipeline and the `sq` commands.
3. **Live test**: `TestLive` runs against `@databricks_test`, whose location comes from envar
   `SQ_TEST_SRC__DATABRICKS_TEST`. Without the envar it skips. It needs only a warehouse that can
   run a `SELECT`. A [free edition](https://docs.databricks.com/aws/en/getting-started/free-edition)
   workspace is enough for local runs.

   ```shell
   export SQ_TEST_SRC__DATABRICKS_TEST='databricks://token:dapi...@HOST/sql/1.0/warehouses/ID?catalog=main&schema=default'
   go test ./drivers/databricks -run TestLive -v
   ```

The fake is not a query engine, so it can't show that Databricks accepts the SQL that the driver
sends. These behaviors are written from the Spark and Databricks SQL documentation and have not
been run against a live warehouse, so `TestLive` is where to check them first:

- the `information_schema` queries and column names (`tables`, `columns`, `views`,
  `table_constraints`, `key_column_usage`)
- bound `?` parameters in those queries
- the `SHOW CATALOGS` and `SHOW SCHEMAS` result shapes, which are read by position
- `rownum()`, which uses the default `row_number() OVER (ORDER BY 1)`
- `OFFSET` without `LIMIT`
- the `sum()` cast to `DECIMAL(38, 6)`
- error classes `TABLE_OR_VIEW_NOT_FOUND`, `SCHEMA_NOT_FOUND` and `CATALOG_NOT_FOUND` mapping
  to `driver.NotExistError`
- how long `COUNT(*)` takes on a large schema
- whether rows-affected counts are populated (the dialect assumes not)
- OAuth M2M

There is no `@sakila_dbx` handle and the driver is not in `sakila.SQLAll`: the cross-driver tests
there create, fill and drop tables, which this driver doesn't do, and no Sakila dataset exists in
a workspace. Add them if the driver gains a write path.

## Dependencies

go.mod lists the `databricks-sql-kernel-bindings` modules because databricks-sql-go imports them
behind the `databricks_kernel` build tag. Without that tag (the default, and the only tested
build) they are not compiled and their archives are not downloaded by `go build`. `go mod download`
does fetch them: they are large static libraries, one module per platform. The kernel backend
(`useKernel=true`) is not supported by this driver.
