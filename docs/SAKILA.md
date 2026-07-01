# Sakila test data

[Sakila](https://dev.mysql.com/doc/sakila/en/) is a well-known sample database
(a DVD-rental store: `actor`, `film`, `customer`, `payment`, …) originally
published for MySQL. `sq` uses Sakila as its **canonical test dataset**: the
same logical schema and data, materialized once per datasource type, so a
single query can be exercised uniformly across every driver: SQLite, DuckDB,
Postgres, MySQL, SQL Server, ClickHouse, Oracle, rqlite, and the document
formats (CSV, TSV, Excel).

## The `sakiladb` project

The external database engines are served by the pre-built Docker images under
the [`github.com/sakiladb`](https://github.com/sakiladb) organization (published
to [`hub.docker.com/u/sakiladb`](https://hub.docker.com/u/sakiladb)). There is
one image per engine (`sakiladb/postgres`, `sakiladb/mysql`,
`sakiladb/sqlserver`, `sakiladb/clickhouse`, `sakiladb/oracle`,
`sakiladb/rqlite`), each preloaded with the Sakila dataset and shipping a Docker
`HEALTHCHECK` so callers can wait for readiness. The image name always matches
the `sq` driver type string (see
[`docs/DRIVERS.md`](./DRIVERS.md#driver-type-registration)).

`sq` does **not** vendor these servers; the images are the single source of a
ready-to-query Sakila instance for each engine.

## Two flavors: embedded vs external

Sakila reaches `sq`'s tests two ways:

- **Embedded fixtures**: checked into the repo under `drivers/*/testdata/`, so
  they need no container and run under `go test -short`:
  - SQLite: [`drivers/sqlite3/testdata/sakila.db`](../drivers/sqlite3/testdata)
  - DuckDB: [`drivers/duckdb/testdata/sakila.duckdb`](../drivers/duckdb/testdata)
  - Excel: [`drivers/xlsx/testdata/sakila.xlsx`](../drivers/xlsx/testdata)
  - CSV / TSV: [`drivers/csv/testdata/sakila-csv/`](../drivers/csv/testdata)
    (and `sakila-tsv/`, plus `*-noheader/` variants)
- **External engines**: Postgres, MySQL, SQL Server, ClickHouse, Oracle, and
  rqlite, each requiring a running `sakiladb/*` container. Tests that touch
  these skip automatically under `-short` (mark them with `tu.SkipShort`).

## Test handles (`testh/sakila`)

The [`testh/sakila`](../testh/sakila) package
([`sakila.go`](../testh/sakila/sakila.go)) is the Go-side source of truth for
test code. It defines:

- **Handle constants** are the `@sakila_*` source handles: `@sakila_sl3`,
  `@sakila_duck`, `@sakila_pg`, `@sakila_my`, `@sakila_ms`, `@sakila_ch`,
  `@sakila_or`, `@sakila_rq`, plus the document handles `@sakila_xlsx`,
  `@sakila_csv_actor`, `@sakila_tsv_actor`, and their variants. The engine
  **version** is a property of the image the source points at, not of the
  handle (see gh #958).
- **Handle sets** are helpers that select the right group for a test:
  `SQLEmbedded()` (SQLite, DuckDB), `SQLAllExternal()`, `SQLAll()`,
  `SQLLatest()` (one handle per engine, sans rqlite), `AllHandles()`, and
  `CrossSourceDests()` for cross-source (origin × dest) matrices.
- **Dataset facts**: table names and row counts (`TblActor` / `TblActorCount`
  = 200, `TblFilmCount` = 1000, `TblPaymentCount` = 16049, …) plus column
  name/kind helpers, so assertions don't hard-code magic numbers.

## Source configuration

The test harness registers all Sakila sources from
[`testh/testdata/test.sq.yml`](../testh/testdata/test.sq.yml), which maps each
handle to a location:

- **Embedded** handles resolve to a fixture path under the repo (e.g.
  `sqlite3://.../drivers/sqlite3/testdata/sakila.db`).
- **External** handles resolve to an environment variable that carries the DSN:
  `SQ_TEST_SRC__SAKILA_PG`, `_MY`, `_MS`, `_CH`, `_OR`, `_RQ`. When the variable
  is unset, that source is simply unavailable and its tests skip.

## The engine matrix

[`.github/sakila-db.json`](../.github/sakila-db.json) is the **single source of
truth** for the external engines: for each engine it records the container
port, the DSN, the `SQ_TEST_SRC__SAKILA_*` env-var name, the test packages, and
the image `tags` (versions) to exercise. It is shared by both CI and the local
scripts, so they never drift.

| Engine     | Env var                  | Port | Image tags     |
| ---------- | ------------------------ | ---- | -------------- |
| postgres   | `SQ_TEST_SRC__SAKILA_PG` | 5432 | latest, 17, 12 |
| mysql      | `SQ_TEST_SRC__SAKILA_MY` | 3306 | latest, 9, 8   |
| sqlserver  | `SQ_TEST_SRC__SAKILA_MS` | 1433 | latest, 2019   |
| clickhouse | `SQ_TEST_SRC__SAKILA_CH` | 9000 | latest         |
| oracle     | `SQ_TEST_SRC__SAKILA_OR` | 1521 | latest         |
| rqlite     | `SQ_TEST_SRC__SAKILA_RQ` | 4001 | latest         |

## Running external engines locally

[`sakila-start-local.sh`](../sakila-start-local.sh) starts every engine from the
matrix above (`docker run --pull always` on each `sakiladb/*` image at its first
tag), waits for the `HEALTHCHECK` to report healthy, and prints the
`export SQ_TEST_SRC__SAKILA_*` lines to enable those sources. Source it to set
the vars in the current shell:

```bash
source ./sakila-start-local.sh   # start containers + export DSN env vars
make test                        # now the external-engine tests run
./sakila-stop-local.sh           # tear the containers down
```

`make test-short` skips everything that needs a container.

## CI

In CI, the same matrix drives the reusable **DB integration** workflow
(nightly at `:latest`, a weekly full version sweep, or on demand). See
[`docs/WORKFLOW.md`](./WORKFLOW.md#database-integration-tests) for how
`db-integration.yml` and `db-scheduled.yml` consume `.github/sakila-db.json`.

## Regenerating embedded fixtures

The in-repo fixtures are generated, not hand-authored; e.g.
`drivers/sqlite3/testdata/recreate_sakila_sqlite.sh`, the
`drivers/duckdb/testdata/duckdb-sakila-*.sql` scripts, and
`drivers/csv/testdata/generate-sakila.sh`. Regenerate with those when the schema
or data needs to change, rather than editing the binary fixtures directly.

## User-facing Sakila

Sakila also underpins the end-user docs: the [sq.io](https://sq.io) tutorial and
command examples query `@sakila` sources, and downloadable Sakila datasets are
served from the site (`site/static/testdata/`, e.g.
[`sq.io/testdata/sakila.db`](https://sq.io/testdata/sakila.db)) so readers can
follow along.

## See also

- [`testh/sakila`](../testh/sakila): the Go test-constants package.
- [`.github/sakila-db.json`](../.github/sakila-db.json): the engine/version
  matrix.
- [`docs/DRIVERS.md`](./DRIVERS.md): driver development, including the
  `sakiladb/{driver}` image requirement for new SQL drivers.
- [`docs/WORKFLOW.md`](./WORKFLOW.md): CI workflows that run the integration
  suites.
