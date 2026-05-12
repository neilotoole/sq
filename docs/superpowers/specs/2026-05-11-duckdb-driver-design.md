# DuckDB Driver — Design

- **Issue:** [#437](https://github.com/neilotoole/sq/issues/437)
- **Branch:** `gh437-duckdb-support`
- **Status:** Approved (2026-05-11)
- **Author:** neilotoole

## Goal

Add a first-class DuckDB driver to `sq` so users can `sq add ./foo.duckdb`,
`sq inspect`, `sq sql`, `sq slq`, and use every other `sq` command against
DuckDB databases — with the same ergonomics that the SQLite driver provides
today.

The driver must:

1. Work out of the box on macOS (amd64, arm64), Linux (amd64, arm64), and
   Windows (amd64). Windows arm64 is excluded, matching the existing
   goreleaser policy.
2. Statically bundle the standard set of in-tree DuckDB extensions, so users
   never need network access at runtime to load `parquet`, `httpfs`, `json`,
   etc.
3. Expose the bulk of DuckDB's type system through `sq`'s existing `kind.Kind`
   model, with composite types (LIST/STRUCT/MAP/ENUM) projected to JSON text.
4. Behave consistently with how SQLite is plumbed into the CLI: file-extension
   detection, stdin support, scheme tab-completion, etc.

## Non-goals (tracked as follow-ups)

- **DuckDB as an internal SQL engine** for CSV/JSON/Parquet/Excel sources
  (replacing the bespoke readers in `drivers/csv/`, `drivers/json/`,
  `drivers/xlsx/`). Large performance win available; out of scope for this PR.
- **DuckDB as the scratch/cache DB.** SQLite stays. DuckDB's single-writer
  model and heavier startup don't fit the ephemeral, multi-process cache
  use-case.
- **First-class `kind.Array` / `kind.Struct` / `kind.Map`.** Cross-driver
  change; needs its own design pass. v1 ships composite columns as JSON text.
- **Community / third-party DuckDB extensions** (motherduck, vss, spatial).
  Loadable at runtime via `INSTALL`/`LOAD`; not bundled.

## Architecture

### Package layout

Mirror `drivers/sqlite3/`:

```
drivers/duckdb/
  duckdb.go             Provider, driveri, ConnParams, NewScratchSource (parallel API; unused)
  grip.go               driver.Grip impl
  metadata.go           SourceMetadata + TableMetadata via DuckDB system tables
  render.go             ast/render dialect overrides
  alter.go              ALTER TABLE wrappers
  errors.go             error normalization to errz
  pragma.go             SET / PRAGMA helpers
  doc.go                package doc; documents single-writer caveat
  internal/
    duckparser/         location & DSN parsing helpers
    portsakila/         one-off Go program that ports SQLite sakila SQL → DuckDB SQL
  testdata/             see "Test data" section
  *_test.go             see "Tests" section
```

Backing SQL driver: `github.com/duckdb/duckdb-go/v2` imported for side
effect, registered with `database/sql` as `duckdb`.

### Hook points in the rest of the codebase

- `libsq/source/drivertype/drivertype.go` — add `DuckDB = Type("duckdb")`.
- `libsq/source/location/location.go` — add `"duckdb"` to the scheme list and
  duplicate the SQLite file-based handling for `duckdb://path/to/file`.
- `cli/cmd_add.go` — add a `MungeLocation` branch for DuckDB so
  `sq add ./foo.duckdb` resolves to `duckdb:///abs/path/foo.duckdb`.
- `cli/run.go` — register `&duckdb.Provider{Log: log}` against
  `drivertype.DuckDB`. Do **not** swap the scratch source.
- `cli/source.go` — replicate the stdin-detection special case so
  `cat foo.duckdb | sq` works (write to temp file, open from there).
- `cli/complete_location.go` — add `"duckdb://"` to the scheme completion list.
- `libsq/files/` — add a sniffer that recognizes the DuckDB magic header
  (`DUCK` at file offset 8) plus the `.duckdb` and `.ddb` extensions.

### Cache DB

Stays SQLite. Rationale: cache files may be opened concurrently by multiple
`sq` invocations; DuckDB is single-writer per file. Cache also benefits from
fast cold-start, and SQLite is unbeatable there.

## Cross-platform build

### Library

`duckdb/duckdb-go/v2` ships prebuilt static `libduckdb` for:
`darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`,
`windows/amd64`. ARM Windows is unsupported upstream, matching the existing
`goreleaser` `ignore` rule for `windows/arm64`.

### Static linking + extensions

v2 exposes a `duckdb_use_static_lib` build tag. The bundled static lib
includes the in-tree extension set. Required bundled extensions:

`json`, `parquet`, `icu`, `fts`, `httpfs`, `excel`, `inet`, `autocomplete`,
`tpch`, `tpcds`.

A package-init hook in `duckdb.go` runs `INSTALL <ext>; LOAD <ext>;` once per
process for the statically-bundled extensions that require explicit `LOAD`
even when present in the static binary. The exact set is determined during
implementation against v2's documented behavior; some extensions auto-load.

### CI changes (`.github/workflows/main.yml`)

- Append `duckdb_use_static_lib duckdb_arrow` (final list TBD against v2 docs)
  to `BUILD_TAGS`.
- macOS / Linux: no toolchain change. Existing CGo via clang/gcc is fine.
- Windows: currently builds SQLite via the default Go toolchain. DuckDB v2's
  Windows static lib is MSVC-built; linking against MinGW (Go's default on
  Windows CGo) needs verification at implementation time.
  - **Risk mitigation:** if MinGW link fails, switch the windows job to
    MSYS2/MinGW64 (the commented-out block in `main.yml` shows the intended
    setup) or to MSVC via `zig cc`. Plan budgets a half-day for Windows
    link debugging.
- Add a build smoke step on every OS: `./dist/sq drivers | grep duckdb` and
  `./dist/sq sql --src @duckdb_smoke "SELECT current_setting('extensions')"`
  to fail fast if any platform regressed.

### Goreleaser

No config change. `prebuilt` mode assembles whatever each OS job built.
Compressed binary grows by ~40–60 MB. CHANGELOG flags this.

### `go.mod`

Adds `duckdb/duckdb-go/v2` and its transitive deps
(`apache/arrow`, `google/flatbuffers`). Mark `duckdb/duckdb-go/v2` as
`// BRITTLE` because version bumps may pull a different bundled libduckdb
version with extension-API breakage.

### Makefile

- Replicate the macOS `CGO_LDFLAGS := -Wl,-no_warn_duplicate_libraries`
  workaround if DuckDB triggers similar warnings.
- Add `make test-duckdb` shortcut: `go test -tags '$(BUILD_TAGS)' ./drivers/duckdb/...`.

### Local dev

Document in `CONTRIBUTING.md` that the first build downloads the bundled
`libduckdb` (~80 MB) into the Go module cache; subsequent builds are fast.
No system DuckDB install required.

### Implementation-time assumptions to verify first

These are load-bearing for the cross-platform / "all optional flags" story
and must be confirmed in the first hour of implementation, before any other
work proceeds. If any are wrong, revisit this design.

1. `duckdb/duckdb-go/v2` exposes a static-linking build mode (working
   name `duckdb_use_static_lib`) that produces a self-contained binary on
   macOS, Linux, and Windows without requiring a system `libduckdb`.
2. The static bundle includes the in-tree extensions listed above (or they
   are obtainable via per-extension build tags). If some are not bundled,
   either add per-extension tags or accept runtime `INSTALL`/`LOAD` for
   that subset and update the CHANGELOG accordingly.
3. The Windows static lib links against MinGW (Go's default Windows CGo
   toolchain). If not, switch the windows CI job to MSYS2/MinGW64 or
   MSVC-via-`zig cc` per the risk-mitigation note above.

## Driver semantics

### Dialect (`render.go`)

DuckDB SQL is largely Postgres-compatible, **not** SQLite-compatible. Start
from the Postgres render dialect.

| Aspect | DuckDB | Notes |
|---|---|---|
| Identifier quoting | `"name"` | Postgres-style. |
| String quoting | `'value'` | Standard. |
| Placeholders | `$1, $2, …` | Numbered, Postgres-style. |
| LIMIT / OFFSET | `LIMIT n OFFSET m` | Standard. |
| Joins | All standard joins | Reuse Postgres `jointype` defaultSet. |
| String concat | `\|\|` and `concat()` | Both work. |
| Cast | `CAST(x AS TYPE)` and `x::TYPE` | Both work. Match Postgres approach. |
| Group concat | `string_agg(col, ',')` | Postgres-style; not SQLite's `group_concat`. |
| Random | `random()` returns DOUBLE in [0,1) | Note for function-mapping tests. |
| Date functions | `date_trunc`, `extract`, … | Postgres-flavored. Inherit mappings. |

A new `duckdb` entry is added to `libsq/driver/dialect/` so cross-driver
renderer code can dispatch.

### Type mapping

`kind.Kind` ↔ DuckDB:

| DuckDB type | sq kind | Notes |
|---|---|---|
| `BOOLEAN` | `kind.Bool` | |
| `TINYINT` … `BIGINT`, `UTINYINT` … `UBIGINT` | `kind.Int` | |
| `HUGEINT`, `UHUGEINT` | `kind.Int` | May overflow `int64`; fall back to string when out of range. |
| `FLOAT`, `REAL`, `DOUBLE` | `kind.Float` | |
| `DECIMAL(p,s)` | `kind.Decimal` | `shopspring/decimal` (already a dep). |
| `VARCHAR`, `TEXT`, `STRING` | `kind.Text` | |
| `BLOB`, `BYTEA` | `kind.Bytes` | |
| `DATE` | `kind.Date` | |
| `TIME`, `TIME WITH TIME ZONE` | `kind.Time` | |
| `TIMESTAMP`, `TIMESTAMP_S/MS/NS`, `TIMESTAMPTZ` | `kind.Datetime` | |
| `INTERVAL` | `kind.Text` | ISO 8601 duration string. |
| `UUID` | `kind.Text` | Hex-dash form. |
| `JSON` | `kind.Text` | Already JSON. |
| `LIST` / `ARRAY` | `kind.Text` | Projected via `to_json(col)` at scan time. |
| `STRUCT` | `kind.Text` | Same. |
| `MAP` | `kind.Text` | Same. |
| `ENUM` | `kind.Text` | Underlying value. |
| `BIT` | `kind.Text` | Bit-string. |

**Composite-type projection:** rather than scan native ARRAY/STRUCT through
`database/sql`, `metadata.go` detects composite columns and the renderer
rewrites the projection to `to_json(col) AS col`. Implementation lives in
`metadata.go` plus a single hook in `render.go`.

### Metadata

Populate `SourceMetadata` / `TableMetadata` via DuckDB-specific system
functions (`duckdb_tables()`, `duckdb_columns()`, `duckdb_schemas()`),
falling back to `information_schema` where richer info isn't needed.

### Error normalization (`errors.go`)

DuckDB error prefixes (`Catalog Error:`, `Binder Error:`,
`Conversion Error:`, `IO Error:`) map to `errz.NotExist`,
`errz.AlreadyExists`, `errz.NotSupported`, etc., so CLI exit codes are
consistent across drivers.

### ALTER TABLE (`alter.go`)

DuckDB supports `RENAME TO`, `ADD COLUMN`, `DROP COLUMN`, `RENAME COLUMN`,
`ALTER COLUMN ... SET DATA TYPE`. Trivial wrappers; mirror SQLite signatures.

### Settings (`pragma.go`)

DuckDB uses `SET` and `PRAGMA` interchangeably for many options. Helpers
expose `memory_limit`, `threads`, `default_order`, and force
`enable_progress_bar=false` to silence the interactive bar when sq runs from
a TTY.

### Concurrency caveat

DuckDB is single-process / single-writer per file. Opening the same
`.duckdb` file from two `sq` processes will fail with a lock error, unlike
SQLite WAL mode which permits concurrent readers. Documented in `doc.go`
and the user docs. Acceptable for sq's typical single-shot CLI usage.

## Test data

### Sakila

Mirror `drivers/sqlite3/testdata/`. Source of truth is the **SQLite** sakila
SQL files, not the MySQL/Postgres ones, so row counts and IDs match SQLite
tests exactly. Existing `cli/` table-row-count assertions parameterized over
driver handles work without per-driver expected values.

```
drivers/duckdb/testdata/
  sakila.duckdb
  sakila-whitespace.duckdb
  empty.duckdb
  misc.duckdb
  blob.duckdb
  sakila_diff.duckdb
  recreate_sakila_duckdb.sh
  duckdb-sakila-schema.sql
  duckdb-sakila-insert-data.sql
  duckdb-sakila-drop-objects.sql
  duckdb-sakila-delete-data.sql
  type_test.ddl
  README.md
```

### Porting issues from SQLite → DuckDB

| SQLite | DuckDB | Action |
|---|---|---|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | No `AUTOINCREMENT` keyword | Rewrite as `INTEGER PRIMARY KEY` with explicit IDs in INSERTs (the existing data file already supplies them). |
| ~30 `CREATE TRIGGER ... AFTER INSERT/UPDATE` maintaining `last_update` | **DuckDB has no TRIGGER support** | Drop them. `last_update` is set at insert time via `DEFAULT current_timestamp`. The triggers only mattered for in-flight UPDATEs, which sq's read-mostly test suite doesn't exercise. Document the divergence in `testdata/README.md`. |
| `DATETIME('NOW')` | `current_timestamp` / `now()` | Only present inside trigger bodies; removed with triggers. |
| `VARCHAR(n)` | DuckDB stores as VARCHAR with length ignored | Leave as-is. |
| `CREATE VIEW ... AS ...` with potential SQLite-specific functions (e.g. `IIF`, `printf`) | Compatible only after rewriting | Inspect view bodies; replace with `CASE WHEN` / `format()` if found. |
| FKs, indexes, `BEGIN/COMMIT` | All work | No change. |

### Generation pipeline

A small Go program at `drivers/duckdb/internal/portsakila/` (~150 LOC) reads
each `sqlite-sakila-*.sql` from `drivers/sqlite3/testdata/`, strips
`CREATE TRIGGER ... END;` blocks via SQL-aware tokenization (not regex —
trigger bodies contain `;`), strips `AUTOINCREMENT`, and writes the result
to `drivers/duckdb/testdata/duckdb-sakila-*.sql`.

Future SQLite-sakila edits propagate via `go run ./drivers/duckdb/internal/portsakila`,
keeping the two DBs in lockstep without manual diff-merging. The generated
`.sql` files are still committed so contributors don't *need* to run the
port tool — it's only needed when the SQLite source changes.

`recreate_sakila_duckdb.sh`:

```bash
#!/bin/bash
set -euo pipefail
rm -f sakila.duckdb
duckdb sakila.duckdb < ./duckdb-sakila-schema.sql
duckdb sakila.duckdb < ./duckdb-sakila-insert-data.sql
```

### Variants

- `sakila-whitespace.duckdb` — same port logic via `--whitespace` flag.
- `sakila_diff.duckdb` — for `sq diff` tests.
- `empty.duckdb`, `misc.duckdb`, `blob.duckdb` — handcrafted, small.

### Reproducibility

`testdata/README.md` notes that contributors regenerating sakila need a
`duckdb` CLI matching the version of `libduckdb` that
`duckdb/duckdb-go/v2` bundles (file-format compatibility within minor
versions is stable but not guaranteed across majors). CI doesn't regenerate;
it consumes the committed `.duckdb` files.

## Tests

### Test files

- `duckdb_test.go` — top-level driver tests (open/close, basic SELECT,
  ConnParams).
- `metadata_test.go` — tables/columns/schemas/comments enumeration.
- `db_type_test.go` — exhaustive type mapping driven by `type_test.ddl`,
  which exercises every kind in the type-mapping table plus one each of
  LIST/STRUCT/MAP/ENUM/INTERVAL/UUID/HUGEINT.
- `extension_test.go` — verifies each statically-bundled extension loads
  and is callable: `read_json_auto`, `read_parquet`, `read_csv`,
  `excel_open`, `httpfs` LOAD success (no network call), `icu` collation,
  `fts` macro existence. **Critical** for "all optional flags" validation.
  Doubles as the cross-platform smoke test referenced in the build section.
- `functions_test.go` — date/string/agg builtins behave as the renderer
  expects.
- `internal_test.go` — package-private helpers.

### Test handles

`testh/sakila/sakila.go`:

```go
const (
    DuckDB    = "@sakila_duck"
    DuckDBWS  = "@sakila_duck_ws"
)
```

These point at the on-disk files — no Docker needed, mirroring how SQLite
handles work today. `tu.SkipShort(t, true)` is **not** used; DuckDB tests run
under `make test-short` like SQLite tests, keeping DuckDB in the fast
feedback loop.

### CLI integration coverage

Add `@sakila_duck` to the parameterized SQL/SLQ tests in `cli/` that already
iterate over driver handles — typically a one-line addition to existing
`[]string{sakila.SL3, sakila.PG, ...}` slices found via
`grep -r "sakila.SL3" cli/`. This gives free coverage of `sq sql`, `sq slq`,
`sq inspect`, `sq diff`, `sq tbl copy`, output formats, etc.

## CLI surface

No new commands. DuckDB plugs into existing flows:

- `sq add ./foo.duckdb` — auto-detects type via magic header or extension.
  Munged to `duckdb:///abs/path/foo.duckdb`.
- `sq add 'duckdb:///abs/path/foo.duckdb?memory_limit=4GB'` — explicit URI
  with conn params.
- `sq add 'duckdb://:memory:'` — in-memory DB. Useful with
  `sq sql @mem "SELECT * FROM read_parquet('s3://...')"`.
- `sq inspect`, `sq sql`, `sq slq`, `sq diff`, `sq tbl copy`, `sq drivers`
  — all work via the standard driver interface; no per-command code changes.
- `sq drivers` lists `duckdb` with version string from `select version()`.
- Tab-completion for `sq add` knows the `duckdb://` scheme.
- Stdin: `cat foo.duckdb | sq sql 'SELECT ...'` works via the same temp-file
  trick SQLite uses (`cli/source.go`).

### `ConnParams` (whitelisted, drives shell completion)

`access_mode` (`READ_WRITE`, `READ_ONLY`), `memory_limit`, `threads`,
`default_order`, `default_null_order`, `enable_external_access`
(`true`/`false`), `enable_object_cache`, `temp_directory`,
`wal_autocheckpoint`. Not exhaustive — DuckDB has 100+ settings — covers
the ones a CLI user actually touches.

## Documentation

- New `site/content/en/docs/drivers/duckdb.md` modeled on `sqlite.md`:
  install/build notes, supported types, conn params table, examples
  (persistent file, in-memory, querying parquet/csv via DuckDB extensions).
- Update `site/content/en/docs/drivers/_index.md` to list DuckDB.
- Update top-level `README.md` driver matrix.

Site-only changes don't need CHANGELOG entries.

## CHANGELOG

Under `## Unreleased`:

```markdown
### Added
- DuckDB driver ([#437](https://github.com/neilotoole/sq/issues/437)):
  `sq` can now read and write DuckDB databases via the `duckdb://` scheme.
  The driver supports the full DuckDB type system, statically links the
  standard set of in-tree extensions (json, parquet, icu, fts, httpfs,
  excel, inet, autocomplete, tpch, tpcds), and works on macOS
  (amd64/arm64), Linux (amd64/arm64), and Windows (amd64). See
  [DuckDB driver docs](https://sq.io/docs/drivers/duckdb).
```

## Rollout

1. **Branch:** `gh437-duckdb-support` (current worktree).
2. **PR strategy:** Single PR. Driver code, sakila DB generation, CI
   changes, and docs land together. Splitting hurts because each piece is
   unverifiable without the others.
3. **Pre-merge gates:**
   - `make all` green on macOS local.
   - All three CI OS jobs green (linux, macOS, windows-2022).
   - `extension_test.go` passes on all three OSes — proves all bundled
     extensions linked.
   - Snapshot release built locally via `goreleaser` to confirm bundled
     binary works end-to-end.
4. **Post-merge:** No release-tag bump immediately. Wait at least one
   release cycle of dogfooding via `master` builds before announcing.
