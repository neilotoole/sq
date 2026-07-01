# Oracle driver: maintainer notes

Internals and test setup for the Oracle driver. User-facing documentation
(connection strings, type mapping, SQL rendering, quirks) lives on the
[sq.io Oracle page](https://sq.io/docs/drivers/oracle); cross-driver
contributor guidance is in [`docs/DRIVERS.md`](../../docs/DRIVERS.md). This
file covers only the Oracle-specific details that fit neither.

## Testing

Oracle tests use the standard `testh` harness like every other SQL driver: the
`@sakila_or` handle reads its DSN from `SQ_TEST_SRC__SAKILA_OR`, and tests skip
cleanly when it is unset. To run the integration tests locally:

```bash
docker run -d -p 1521:1521 sakiladb/oracle:latest
export SQ_TEST_SRC__SAKILA_OR='oracle://sakila:p_ssW0rd@localhost:1521/SAKILA'
go test ./drivers/oracle/...          # or repo-wide: go test ./...
```

[`sakiladb/oracle`](https://github.com/sakiladb/oracle) is slow to start, so
wait until it accepts connections before running tests. See
[`docs/DRIVERS.md`](../../docs/DRIVERS.md) and
[`docs/SAKILA.md`](../../docs/SAKILA.md) for the cross-driver test-handle setup.

## Test package layout

Most integration tests live in `package oracle_test`
([`oracle_test.go`](./oracle_test.go)) and go through the harness. A few tests
need unexported symbols and so stay in `package oracle`
([`internal_test.go`](./internal_test.go) for pure helpers like `placeholders`
and `kindFromOracleNumber`; [`metadata_internal_test.go`](./metadata_internal_test.go)
for metadata error paths). Because `testh` imports this driver, an internal
test cannot import `testh` (that would be an import cycle), so the one internal
behavior that needs a live DB is reached from the external package via the
[`export_test.go`](./export_test.go) seam instead.

## Skipped cross-driver test

`TestDriver_CreateTable_Minimal`, which asserts exact `kind` round-trip
fidelity, is skipped for Oracle. Oracle `DATE` carries time-of-day and Oracle
has no time-only type, so `kind.Date` and `kind.Time` both inspect back as
`kind.Datetime`.
