# CHANGELOG

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Breaking changes are annotated with ☢️, and alpha/beta features with 🐥.

## Upcoming

### Added

- New [SLQ](https://sq.io/docs/concepts#slq) function [`rownum()`](https://sq.io/docs/query#rownum) that returns the one-indexed
  row number of the current record.
  
  ```shell
  $ sq '.actor | rownum(), .actor_id, .first_name | order_by(.first_name)'
  rownum()  actor_id  first_name
  1         71        ADAM
  2         132       ADAM
  3         165       AL
  ```

### Fixed

- [`sq version`](https://sq.io/docs/cmd/version) now honors option
  [`format.datetime`](https://sq.io/docs/config#formatdatetime) when outputting build timestamp.
- Fixed a fairly nasty bug that prevented correct rendering of SLQ functions nested inside
  an expression.

## [v0.43.1] - 2023-11-19

### Added

- Related to [#270], the output of [`sq inspect`](https://sq.io/docs/inspect) now includes the
  source's catalog (in JSON and YAML output formats).

### Fixed

- MySQL driver didn't populate all expected values for [`sq inspect --overview`](https://sq.io/docs/inspect#source-overview).

### Changed

- ☢️ Removed unused `--exec` and `--query` flags from [`sq sql`](https://sq.io/docs/cmd/sql) command.

## [v0.43.0] - 2023-11-18

### Added

- [#270]: Flag [`--src.schema`](https://sq.io/docs/cmd/sq/#override-active-schema) permits switching the source's schema (and catalog)
  for the duration of the command. The flag is supported for the
  [`sq`](https://sq.io/docs/cmd/sq#override-active-schema), [`sql`](https://sq.io/docs/cmd/sql/#active-source--schema)
  and [`inspect`](https://sq.io/docs/inspect#override-active-schema) commands.
- New SLQ functions [`catalog()`](https://sq.io/docs/query#catalog) and
  [`schema()`](https://sq.io/docs/query#schema) return the catalog and schema of the DB connection.
- The SLQ [`unique`](https://sq.io/docs/query#unique) function now has a synonym `uniq`.

### Changed

- `sq src --text` now outputs only the handle of the active source. Previously it
  also printed the driver type and short location. Instead use `sq src --text --verbose` to
  see those details.

## [v0.42.1] - 2023-09-10

### Fixed

- [#308]: Fix to allow build on [32-bit systems](https://github.com/void-linux/void-packages/pull/45023). 
  Thanks [@icp](https://github.com/icp1994).


## [v0.42.0] - 2023-08-22

### Added

- 🐥 [#279]: The SQLite [driver](https://sq.io/docs/drivers/sqlite) now has initial support
  for several SQLite [extensions](https://sq.io/docs/drivers/sqlite#extensions)
  baked in, including [Virtual Table](https://www.sqlite.org/vtab.html) and [FTS5](https://www.sqlite.org/fts5.html).
  Note that this is an early access release of extensions support. Please [open an issue](https://github.com/neilotoole/sq/issues/new/choose) if
  you discover something bad.

## [v0.41.1] - 2023-08-20

### Fixed

- `sq version` was missing a newline in its output.

## [v0.41.0] - 2023-08-20

This release is heavily focused on improvements to Microsoft Excel support.
The underlying Excel library has been changed from [`tealeg/xlsx`](https://github.com/tealeg/xlsx)
to [`qax-os/excelize`](https://github.com/qax-os/excelize), largely because
`tealeg/xlsx` is no longer actively maintained. Thus, both the [XLSX output writer](https://sq.io/docs/output/#xlsx)
and the [XLSX driver](https://sq.io/docs/drivers/xlsx) have been rewritten. There should be some performance
improvements, but it's also possible that the rewrite introduced bugs. If you
discover anything strange, please [open an issue](https://github.com/neilotoole/sq/issues/new/choose).

### Added

- [#99]: The [CSV](https://sq.io/docs/drivers/csv) and [XLSX](https://sq.io/docs/drivers/xlsx)
  drivers can now handle duplicate header column names in the ingest data.
  For example, given a CSV file:
  
  ```csv
  actor_id,first_name,actor_id
  1,PENELOPE,1
  2,NICK,2
  ```
  
  The columns will be renamed to:
  
  ```csv
  actor_id,first_name,actor_id_1
  ```
  
  The renaming behavior is controlled by a new option [`ingest.column.rename`](https://sq.io/docs/config#ingestcolumnrename)
  This new option is effectively the ingest counterpart of the existing output option
  [`result.column.rename`](https://sq.io/docs/config#resultcolumnrename).

- [#191]: The [XLSX](https://sq.io/docs/drivers/xlsx) driver now [detects](https://sq.io/docs/drivers/xlsx#header-row) header rows, like
  the CSV driver already does. Thus, you now typically don't need to specify
  the `--ingest.header` flag for Excel files. However, the option remains available
  in case `sq` can't figure it out for a particular file.

- The Excel writer has three new config options for controlling date/time output.
  Note that these format strings are distinct from [`format.datetime`](https://sq.io/docs/config#formatdatetime)
  and friends, because Excel has its own format string mechanism.
  - [`format.excel.datetime`](https://sq.io/docs/config#formatexceldatetime): Controls datetime format, e.g. `2023-08-03 16:07:01`.
  - [`format.excel.date`](https://sq.io/docs/config#formatexceldatetime): Controls date-only format, e.g. `2023-08-03`.
  - [`format.excel.time`](https://sq.io/docs/config#formatexceldatetime): Controls time-only format, e.g. `4:07 pm`.

- The ingest [kind detectors](https://sq.io/docs/detect#kinds) (e.g. for `CSV` or `XLSX`)
  now detect more [date & time formats](https://sq.io/docs/detect#datetime-formats) as `kind.Datetime`, `kind.Date`, and `kind.Time`.

- If an error occurs when the output format is `text`, a stack trace is printed
  to `stderr` when the command is executed with `--verbose` (`-v`).

- There's a new option [`error.format`](https://sq.io/docs/config#errorformat) that controls error output format independent
  of the main [`format`](https://sq.io/docs/config#format) option . The `error.format` value must be one of `text` or `json`.

### Changed

- ☢️ The default Excel date format has changed. Previously
  the format was `11/9/89`, and now it is `1989-11-09`. The same applies
  to datetimes, e.g. `11/9/1989  00:00:00` becomes `1989-11-09 00:00`.
  
  This change is made to reduce ambiguity and confusion.
  `sq` uses a [library](https://github.com/qax-os/excelize)
  to interact with Excel files, and it seems that the library chooses a particular format
  by default (`11/9/89`). There are several paths we could take here:
  
  1. Interrogate the OS, and use the OS locale date format.
  2. Stick with the library default `11/9/89`.
  3. Pick a default other than `11/9/89`.
  
  We pick the third option. The first option (locale-dependent)
  is excluded because, as a general rule, we want `sq` to produce the same 
  output regardless of locale/system settings. We exclude the second option
  because month/day confuses most of the world. Thus, we're left with picking a
  default, and `1989-11-09` is the format used in
  [RFC3339](https://datatracker.ietf.org/doc/html/rfc3339) and friends.
  
  Whether this is the correct (standard?) approach is still unclear, and
  feedback is welcome. However, the user can make use of the new config options
  ([`format.excel.datetime`](https://sq.io/docs/config#formatexceldatetime) etc.)
  to customize the format as they see fit.

- The XLSX writer now outputs header rows in **bold text**.

- ☢️ The XLSX writer now outputs blob (`bytes`) cell data as a base64-encoded string,
  instead of raw bytes.

### Fixed

- Fixed bug where source-specific config wasn't being propagated.


## [v0.40.0] - 2023-07-03

This release features a complete overhaul of the [`join`](https://sq.io/docs/query#joins)
mechanism.

### Added

- [#277]: A table selector can now have an alias. This in and of itself is not
  particularly useful, but it's a building block for [multiple joins](https://github.com/neilotoole/sq/issues/12).

  ```shell
  $ sq '@sakila | .actor:a | .a.first_name'
  ```

- New option `result.column.rename` that exposes a template used to rename
  result set column names before display. The primary use case is to de-duplicate
  columns names on a `SELECT * FROM tbl1 JOIN tbl2`, where `tbl1` and `tbl2`
  have clashing column names ([docs](https://sq.io/docs/config#resultcolumnrename)).

- [#157]: Previously only `join` (`INNER JOIN`) was available: now the rest of
  the join types such as `left_outer_join`, `cross_join`, etc. are
  implemented ([docs](https://sq.io/docs/query#join-types)).

  
### Changed

-  ☢️ [#12]: The table [join](https://sq.io/docs/query#joins) mechanism has been
   completely overhauled. Now there's support for multiple joins. See [docs](https://sq.io/docs/query#joins).

   ```shell
   # Previously, only a single join was possible
   $ sq '.actor, .film_actor | join(.actor_id)'
   
   # Now, an arbitrary number of joins
   $ sq '.actor | join(.film_actor, .actor_id) | join(.film, .film_id)'
   ```
- ☢️ The alias for `--jsonl` (JSON Lines) has been changed to `-J`.

### Fixed

- Config options weren't being propagated correctly to all parts of the code.

## [v0.39.1] - 2023-06-22

### Fixed

- Bug with `sq version` output on Windows.

## [v0.39.0] - 2023-06-22

### Added

- [#263]: `sq version` now supports `--yaml` output.
- [#263]: `sq version` now outputs host OS details with `--verbose`, `--json`
  and `--yaml` flags. The motivation behind this is bug submission: we want
  to know which OS/arch the user is on. E.g. for `sq version -j`:
```json
{
  "version": "v0.38.1",
  "commit": "eedc11ec46d1f0e78628158cc6fd58850601d701",
  "timestamp": "2023-06-21T11:41:34Z",
  "latest_version": "v0.39.0",
  "host": {
    "platform": "darwin",
    "arch": "arm64",
    "kernel": "Darwin",
    "kernel_version": "22.5.0",
    "variant": "macOS",
    "variant_version": "13.4"
  }
}
```
- [#263]: The output of `sq inspect` and `sq inspect -v` has been refactored
  significantly, and should now be easier to work with ([docs](https://sq.io/docs/inspect)).

## [v0.38.1] - 2023-06-19

### Fixed

- [#261]: The JSON writer (`--json`) could get deadlocked when a record contained
  a large amount of data, triggering an internal `Flush()` (which is mutex-guarded)
  from within the mutex-guarded `WriteRecords()` method.

## [v0.38.0] - 2023-06-18

This release has significant improvements (and breaking changes)
to SLQ (`sq`'s query language).

### Changed

- ☢️ [#254]: The formerly-implicit "WHERE" mechanism now requires an explicit `where()` function.
  This, alas, is a fairly big breaking change. But it's necessary to remove an ambiguity roadblock.
  See discussion in the [issue](https://github.com/neilotoole/sq/issues/254).

  ```shell
  # Previously
  $ sq '.actor | .actor_id <= 2'
  
  # Now
  $ sq '.actor | where(.actor_id <= 2)'
  ```
- [#256]: Column-only queries are now possible. This has the neat side effect
  that `sq` can now be used as a calculator.

  ```shell
  $ sq 1+2
  1+2
  3
  ```
  You may want to use `--no-header` (`-H`) when using `sq` as a calculator.

  ```shell
  $ sq -H 1+2
  3
  $ sq -H '(1+2)*3'
  9
  ```

### Fixed

- Literals can now be selected ([docs](https://sq.io/docs/query#select-literal)).
  
  ```shell
  $ sq '.actor | .first_name, "X":middle_name, .last_name | .[0:2]'
  first_name  middle_name  last_name
  PENELOPE    X            GUINESS
  NICK        X            WAHLBERG
  ```
- Lots of expressions that previously failed badly, now work.
  
  ```shell
  $ sq '.actor | .first_name, (1+2):addition | .[0:2]'
  first_name  addition
  PENELOPE    3
  NICK        3
  ```
- [#258]: Column aliases can now be arbitrary strings, instead of only a
  valid identifier.

  ```shell
  # Previously only valid identifier allowed
  $ sq '.actor | .first_name:given_name | .[0:2]'
  given_name
  PENELOPE
  NICK
  
  # Now, any arbitrary string can be used
  $ sq '.actor | .first_name:"Given Name" | .[0:2]'
  Given Name
  PENELOPE
  NICK
  ```

## [v0.37.1] - 2023-06-15

### Fixed

- [#252]: Handle `*uint64` returned from DB.

## [v0.37.0] - 2023-06-13

### Added

- [#244]: Shell completion for `sq add LOCATION`. See [docs](https://sq.io/docs/source#location-completion).

## [v0.36.2] - 2023-05-27

### Changed

- ☢️ [Proprietary database functions](https://sq.io/docs/query#proprietary-functions) are now 
  invoked by prefixing the function name with an underscore. For example:
  ```shell
  # mysql "date_format" func
  $ sq '@sakila/mysql | .payment | _date_format(.payment_date, "%m")'
  
  # Postgres "date_trunc" func
  $ sq '@sakila/postgres | .payment | _date_trunc("month", .payment_date)'
  ```

## [v0.36.1] - 2023-05-26

### Fixed

- `sq diff`: Renamed `--count` flag to `--counts` as intended.

## [v0.36.0] - 2023-05-25

The major feature is the long-gestating [`sq diff`](https://sq.io/docs/diff).

## Added

- [#229]: `sq diff` compares two sources, or tables.
- `sq inspect --dbprops` is a new mode that returns only the DB properties.
  Relatedly, the properties mechanism is now implemented for all four supported
  DB types (previously, it was only implemented for Postgres and MySQL).
- [CSV](https://sq.io/docs/output#csv-tsv) format now colorizes output.

## Changed

- `sq inspect -v` previously returned DB properties in a field named `db_variables`.
  This field has been renamed to `db_properties`. The renaming reflects the fact
  that some of those properties aren't really variables in the sense that they
  can be modified (e.g. DB server version or such).
- The structure of the former `db_variables` (now `db_properties`) field has
  changed. Previously it was an array of `{"name": "XX", "value": "YY"}` values,
  but now is a map, where the keys are strings, and the values can be either
  a scalar (`bool`, `int`, `string`, etc.), or a nested value such as an array
  or map. This change is made because some databases (e.g. SQLite) feature
  complex data in some property values.
- CSV format now renders byte sequences as `[777 bytes]` instead of dumping
  the raw bytes.
- ☢️ TSV format (`--tsv`) no longer has a shorthand form `-T`. Apparently that
  shorthand wasn't used much, and `-T` is needed elsewhere.
- ☢️ Likewise, `--xml` no longer has shorthand `-X`. And `--markdown` has lost alias `--md`.
- In addition to the format flags `--text`, `--json`, etc., there is now
  a `--format=FORMAT` flag, e.g. `--format=json`. This will allow `sq` to
  continue to expand the number of output formats, without needing to have
  a dedicated flag for each format.

## Fixed

- `sq config edit @source` was failing to save any edits.

## [v0.35.0] - 2023-05-10

### Added

- [#8]: Results can now be output in [YAML](https://sq.io/docs/output#yaml).

### Fixed

- `sq config get OPT --text` now prints only the value, not `KEY VALUE`.
  If you want to see key and value, consider using `--yaml`, or `--text --verbose`.


## [v0.34.2] - 2023-05-08

### Fixed

- Both `--markdown` and the alias `--md` are now supported.

## [v0.34.1] - 2023-05-07

### Fixed

- Fixed a minor issue where `sq ls -jv` and `sq ls -yv` produced no output
  if config contained no explicitly set options.

## [v0.34.0] - 2023-05-07

This release significantly overhauls `sq`'s config mechanism ([#199]).
For an overview, see the new [config docs](https://sq.io/docs/config).

Alas, this release has several minor breaking changes ☢️. 

### Added

- `sq config ls` shows config.
- `sq config get` gets individual config option.
- `sq config set` sets config values.
- `sq config edit` edits config.
  - Editor can be specified via `$EDITOR` or `$SQ_EDITOR`.
- `sq config location` prints the location of the config dir.
- `--config` flag is now honored globally.
- Many more knobs are exposed in config.
- Logging is much more configurable. There are new knobs:
  ```shell
  $ sq config set log true
  $ sq config set log.level INFO
  $ sq config set log.file /var/log/sq.log
  ```
  There are also equivalent flags  (`--log`, `--log.file` and `--log.level`) and
  envars (`SQ_LOG`, `SQ_LOG_FILE` and `SQ_LOG_LEVEL`).
- Several more commands support YAML output:
  - [`sq group`](https://sq.io/docs/cmd/group)
  - [`sq ls`](https://sq.io/docs/cmd/ls)
  - [`sq mv`](https://sq.io/docs/cmd/mv)
  - [`sq rm`](https://sq.io/docs/cmd/rm)
  - [`sq src`](https://sq.io/docs/cmd/src)


### Changed

- The structure of `sq`'s config file (`sq.yml`) has changed. The config
  file is automatically upgraded when using the new version.
- The default location of the `sq` log file has changed. The new location
  is platform-dependent. Use `sq config get log.file -v` to view the location,
  or `sq config set log.file /path/to/sq.log` to set it.
- ☢️ Envar `SQ_CONFIG` replaces `SQ_CONFIGDIR`. 
- ☢️ Envar `SQ_LOG_FILE` replaces `SQ_LOGFILE`.
- ☢️ Format flag `--table` is renamed to `--text`. This is changed because while the
  output is mostly in table format, sometimes it's just plain text. Thus
  `table` was not quite accurate.
- ☢️ The flag to explicitly specify a driver when piping input to `sq` has been
  renamed from `--driver` to `--ingest.driver`. This change aligns
  the naming of the ingest options and reduces ambiguity.
  ```shell
  # previously
  $ cat mystery.data | sq --driver=csv '.data'
  
  # now
  $ cat mystery.data | sq --ingest.driver=csv '.data'
  ```
- ☢️ `sq add` no longer has the generic `--opts x=y` mechanism. This flag was
  ambiguous and confusing. Instead, use explicit option flags.
  ```shell
  # previously
  $ sq add ./actor.csv --opts=header=false
  
  # now
  $ sq add ./actor.csv --ingest.header=false
   ```
- ☢️ The short form of the `sq add --handle` flag has been changed from `-h` to
  `-n`. While this is not ideal, the `-h` shorthand is already in use everywhere
  else as the short form of `--header`.
    ```shell
  # previously
  $ sq add ./actor.csv -h @actor
  
  # now
  $ sq add ./actor.csv -n @actor
   ```
- ☢️ The `--pretty` flag has been removed. Its only previous use was with the
  `json` format, where if `--pretty=false` would output the JSON in compact form.
  To better align with jq, there is now a `--compact` / `-c` flag that behaves
  identically to jq.
- ☢️ Because of the above `--compact` / `-c` flag, the short form of the `--csv`
  flag is changing from `-c` to `-C`. It's an unfortunate situation, but alignment
  with jq's behavior is an overarching principle that justifies the change.

## [v0.33.0] - 2023-04-15

The headline feature is [source groups](https://sq.io/docs/source#groups).
This is the biggest change to the `sq` CLI in some time, and should
make working with lots of sources much easier.

### Added

- [#192]: `sq` now has a mechanism to group sources. A source handle can
  now be scoped. For example, instead of `@sakila_prod`, `@sakila_staging`, etc,
  you can use `@prod/sakila`, `@staging/sakila`. Use `sq group prod` to
  set the active group (which `sq ls` respects). See [docs](https://sq.io/docs/source#groups).
- `sq group GROUP` sets the active group to `GROUP`.
- `sq group` returns the active group (default is `/`, the root group).
- `sq ls GROUP` lists the sources in `GROUP`.
- `sq ls --group` (or `sq ls -g`) lists all groups.
- `sq mv` moves/renames sources and groups.

### Changed

- `sq ls` now shows the active item in a distinct color. It no longer adds
  an asterisk to the active item.
- `sq ls` now sorts alphabetically when using `--table` format.
- `sq ls` now shows the sources in the active group only. But note that
  the default active group is `/` (the root group), so the default behavior
  of `sq ls` is the same as before.
- `sq add hello.csv` will now generate the handle `@hello` instead of `@hello_csv`.
  On a second invocation, it will return `@hello1` instead of `@hello_csv_1`. Why
  this change? Well, with the availability of the source group mechanism, the `_` character
  in the handle somehow looked ugly. And more importantly, `_` is a relative pain to type.
- `sq ping` has changed to support groups. Instead of `sq ping --all`, you can
  do `sq ping GROUP`, e.g. `sq ping /`.

## [v0.32.0] - 2023-04-09

### Added

- [#187]: For `csv` sources, `sq` will now try to auto-detect if the CSV file
  has a header row or not. Previously, this needed to be explicitly specified
  via an awkward syntax:

  ```shell
  $ sq add ./actor.csv --opts=header=true
  ````
  This change makes working with CSV files significantly lower friction.
  A command like the below now almost always works as expected:

  ```shell
  $ cat ./actor.csv | sq .data
  ```
  Support for Excel/XLSX header detection is in [#191].

### Fixed

- `sq` is now better at detecting the (data) kind of CSV fields. It now more
  accurately distinguishes between `Decimal` and `Int`, and knows how to
  handle `Datetime`.

- [#189]: `sq` now treats CSV empty fields as `NULL`.

## [v0.31.0] - 2023-03-08

### Added

- [#173]: Predefined variables via `--arg` 
  flag ([docs](https://sq.io/docs/query#predefined-variables)):
  ```shell
  $ sq --arg first TOM '.actor | .first_name == $first'
  ```

### Changes
- Use `--md` instead of `--markdown` for outputting Markdown.

### Fixed

- [#185]: `sq inspect` now better handles "too many connections" situations.
- `go.mod`: Moved to `jackc/pgx` `v5`.
- Refactor: switched to [`slog`](https://pkg.go.dev/golang.org/x/exp/slog) logging library.

## [v0.30.0] - 2023-03-27

### Added

- [#164]: Implemented `unique` function ([docs](https://sq.io/docs/query#unique)):
  ```shell
  $ sq '.actor | .first_name | unique'
  ```
  This is equivalent to:
  ```sql
  SELECT DISTINCT first_name FROM actor
  ```
- Implemented `count_unique` function ([docs](https://sq.io/docs/query#count_unique)).
  ```shell
  $ sq '.actor | count_unique(.first_name)'
  ```

### Changed

- The `count` function has been changed ([docs](https://sq.io/docs/query#count))
  - Added no-args version: `.actor | count` equivalent to `SELECT COUNT(*) AS "count" FROM "actor"`.
  - ☢️ The "star" version (`.actor | count(*)`) is no longer supported; use the
    naked version instead.
- Function columns are now named according to the `sq` token, not the SQL token.
  ```shell
  # previous behavior
  $ sq '.actor | max(.actor_id)'
  max("actor_id")
  200
  
  # now
  $ sq '.actor | max(.actor_id)'
  max(.actor_id)
  200
  ```

## [v0.29.0] - 2023-03-26

### Added

- [#162]: `group_by` now accepts [function arguments](https://sq.io/docs/query#group_by). 

### Changed

- Renamed `groupby` to `group_by` to match jq.
- Renamed `orderby` to `order_by` to match jq.

## [v0.28.0] - 2023-03-26

### Added

- [#160]: Use `groupby()` to group results. See [query guide](https://sq.io/docs/query#group_by).

## [v0.27.0] - 2023-03-25

### Added

- [#158]: Use `orderby()` to order results. See [query guide](https://sq.io/docs/query#order_by).

## [v0.26.0] - 2023-03-22

### Added

- [#98]: Whitespace is now allowed in SLQ selector names. You can
  do `@sakila | ."film actor" | ."actor id"`.

### Fixed

- [#155]: `sq inspect` now populates `schema` field in JSON for MySQL,
  SQLite, and SQL Server (Postgres already worked).

## [v0.25.1] - 2023-03-19

### Fixed

- [#153]: Improved formatting of text table with long lines.

## [v0.25.0] - 2023-03-19

### Added

- [#15]: Column Aliases. You can now change specify an alias for a column (or column expression
  such as a function). For example: `sq '.actor | .first_name:given_name`, or `sq .actor | count(*):quantity`.
- [#151]: `sq add` now has a `--active` flag, which immediately sets the new source
  as the active source.

## [v0.24.4] - 2023-03-15

### Fixed

- Fixed typos in `sq sql` command help.

## [v0.24.3] - 2023-03-14

### Added

- When a CSV source has explicit column names (via `--opts cols=A,B,C`), `sq` now verifies
  that the CSV data record field count matches the number of explicit columns.

## [v0.24.2] - 2023-03-13

### Fixed

- [#142]: Improved error handling when Postgres `current_schema()` is unavailable.

## [v0.24.1] - 2023-03-11

### Fixed

- [#144]: Handle corrupted config active source.

## [v0.24.0] - 2022-12-31

### Added

- `sq ping` now respects `--json` flag.

### Fixed

- Improved handling of file paths on Windows.

## [v0.23.0] - 2022-12-31

### Added

- `sq ls` now respects `--json` flag.
- `sq rm` now respects `--json` flag.
- `sq add` now respects `--json` flag.\`
- CI pipeline now verifies install packages after publish.

### Changed

- `sq rm` can delete multiple sources.
- `sq rm` doesn't print output unless `--verbose`.
- Redacted snipped is now `xxxxx` instead of `****`, to match stdlib `url.URL.Redacted()`.

### Fixed

- Fixed crash on Fedora systems (needed `--tags=netgo`).

## [v0.21.3] - 2022-12-30

### Added

- `sq version` respects `--json` flag.
- `sq version` respects `--verbose` flag.
- `sq version` shows `latest_version` info when `--verbose` and there's a newer version available.

### Changed

- `sq version` shows less info when `--verbose` is not set.

## [v0.20.0] - 2022-12-29

### Added

- `sq` now generates manpages (and installs them).

## [v0.19.0] - 2022-12-29

### Added

- Installer for [Arch Linux](https://archlinux.org),
  via [Arch User Repository](https://aur.archlinux.org).

## [v0.18.2] - 2022-12-25

### Added

- The build pipeline now produces `.apk` packages for [Alpine Linux](https://www.alpinelinux.org),
  and `install.sh` has been updated accordingly. However, the `.apk` files
  are not yet published to a repository, so it's necessary to run `apk` against
  the downloaded `.apk` file (`install.sh` does this for you).

## [v0.18.0] - 2022-12-24

### Added

- [#95]: `sq add` now has a `--password` (`-p`) flag that prompts the user for the data source
  password, instead of putting it in the location string. It will also read from stdin
  if there's input there.

## [v0.17.0] - 2022-12-23

### Changed

- More or less every `go.mod` dependency has been updated to latest. This includes
  drivers for `sqlite` and `sqlserver`. The driver updates led to some broken
  things, which have been fixed.

## [v0.16.1] - 2022-12-23

### Fixed

- [#123]: Shell completion is better behaved when a source is offline.

## [v0.16.0] - 2022-12-16

### Added

- `--verbose` flag is now global
- `install.sh` install script.

### Changed

- Improved GH workflow
- `sq inspect` shows less output by default (use `-v` to restore previous behavior)

### Fixed

- `sq inspect` can now deal with Postgres sources that have null values for constraint fields

## [v0.15.11] - 2022-11-06

### Changed

- Yet more changes to GitHub workflow.

## [v0.15.4] - 2021-09-18

### Changed

- Bug fixes

## [v0.15.3] - 2021-03-13

### Changed

- [#91]: MySQL driver options no longer stripped

## [v0.15.2] - 2021-03-08

### Changed

- [#89]: Bug with SQL generated for joins.

[#8]: https://github.com/neilotoole/sq/issues/8
[#12]: https://github.com/neilotoole/sq/issues/12
[#15]: https://github.com/neilotoole/sq/issues/15
[#89]: https://github.com/neilotoole/sq/pull/89
[#91]: https://github.com/neilotoole/sq/pull/91
[#95]: https://github.com/neilotoole/sq/issues/93
[#98]: https://github.com/neilotoole/sq/issues/98
[#99]: https://github.com/neilotoole/sq/issues/99
[#123]: https://github.com/neilotoole/sq/issues/123
[#142]: https://github.com/neilotoole/sq/issues/142
[#144]: https://github.com/neilotoole/sq/issues/144
[#151]: https://github.com/neilotoole/sq/issues/151
[#153]: https://github.com/neilotoole/sq/issues/153
[#155]: https://github.com/neilotoole/sq/issues/155
[#157]: https://github.com/neilotoole/sq/issues/157
[#158]: https://github.com/neilotoole/sq/issues/158
[#160]: https://github.com/neilotoole/sq/issues/160
[#162]: https://github.com/neilotoole/sq/issues/162
[#164]: https://github.com/neilotoole/sq/issues/164
[#173]: https://github.com/neilotoole/sq/issues/173
[#185]: https://github.com/neilotoole/sq/issues/185
[#187]: https://github.com/neilotoole/sq/issues/187
[#189]: https://github.com/neilotoole/sq/issues/189
[#191]: https://github.com/neilotoole/sq/issues/191
[#192]: https://github.com/neilotoole/sq/issues/192
[#199]: https://github.com/neilotoole/sq/issues/199
[#229]: https://github.com/neilotoole/sq/issues/229
[#244]: https://github.com/neilotoole/sq/issues/244
[#252]: https://github.com/neilotoole/sq/issues/252
[#254]: https://github.com/neilotoole/sq/issues/254
[#256]: https://github.com/neilotoole/sq/issues/256
[#258]: https://github.com/neilotoole/sq/issues/258
[#261]: https://github.com/neilotoole/sq/issues/261
[#263]: https://github.com/neilotoole/sq/issues/263
[#270]: https://github.com/neilotoole/sq/issues/270
[#277]: https://github.com/neilotoole/sq/issues/277
[#279]: https://github.com/neilotoole/sq/issues/279
[#308]: https://github.com/neilotoole/sq/pull/308


[v0.15.2]: https://github.com/neilotoole/sq/releases/tag/v0.15.2
[v0.15.3]: https://github.com/neilotoole/sq/compare/v0.15.2...v0.15.3
[v0.15.4]: https://github.com/neilotoole/sq/compare/v0.15.3...v0.15.4
[v0.15.11]: https://github.com/neilotoole/sq/compare/v0.15.4...v0.15.11
[v0.16.0]: https://github.com/neilotoole/sq/compare/v0.15.11...v0.16.0
[v0.16.1]: https://github.com/neilotoole/sq/compare/v0.16.0...v0.16.1
[v0.17.0]: https://github.com/neilotoole/sq/compare/v0.16.1...v0.17.0
[v0.18.0]: https://github.com/neilotoole/sq/compare/v0.17.0...v0.18.0
[v0.18.2]: https://github.com/neilotoole/sq/compare/v0.18.0...v0.18.2
[v0.19.0]: https://github.com/neilotoole/sq/compare/v0.18.2...v0.19.0
[v0.20.0]: https://github.com/neilotoole/sq/compare/v0.19.0...v0.20.0
[v0.21.3]: https://github.com/neilotoole/sq/compare/v0.20.0...v0.21.3
[v0.23.0]: https://github.com/neilotoole/sq/compare/v0.21.3...v0.23.0
[v0.24.0]: https://github.com/neilotoole/sq/compare/v0.23.0...v0.24.0
[v0.24.1]: https://github.com/neilotoole/sq/compare/v0.24.0...v0.24.1
[v0.24.2]: https://github.com/neilotoole/sq/compare/v0.24.1...v0.24.2
[v0.24.3]: https://github.com/neilotoole/sq/compare/v0.24.2...v0.24.3
[v0.24.4]: https://github.com/neilotoole/sq/compare/v0.24.3...v0.24.4
[v0.25.0]: https://github.com/neilotoole/sq/compare/v0.24.4...v0.25.0
[v0.25.1]: https://github.com/neilotoole/sq/compare/v0.25.0...v0.25.1
[v0.26.0]: https://github.com/neilotoole/sq/compare/v0.25.1...v0.26.0
[v0.27.0]: https://github.com/neilotoole/sq/compare/v0.26.0...v0.27.0
[v0.28.0]: https://github.com/neilotoole/sq/compare/v0.27.0...v0.28.0
[v0.29.0]: https://github.com/neilotoole/sq/compare/v0.28.0...v0.29.0
[v0.30.0]: https://github.com/neilotoole/sq/compare/v0.29.0...v0.30.0
[v0.31.0]: https://github.com/neilotoole/sq/compare/v0.30.0...v0.31.0
[v0.32.0]: https://github.com/neilotoole/sq/compare/v0.31.0...v0.32.0
[v0.33.0]: https://github.com/neilotoole/sq/compare/v0.32.0...v0.33.0
[v0.34.0]: https://github.com/neilotoole/sq/compare/v0.33.0...v0.34.0
[v0.34.1]: https://github.com/neilotoole/sq/compare/v0.34.0...v0.34.1
[v0.34.2]: https://github.com/neilotoole/sq/compare/v0.34.1...v0.34.2
[v0.35.0]: https://github.com/neilotoole/sq/compare/v0.34.2...v0.35.0
[v0.36.0]: https://github.com/neilotoole/sq/compare/v0.35.0...v0.36.0
[v0.36.1]: https://github.com/neilotoole/sq/compare/v0.36.0...v0.36.1
[v0.36.2]: https://github.com/neilotoole/sq/compare/v0.36.1...v0.36.2
[v0.37.0]: https://github.com/neilotoole/sq/compare/v0.36.2...v0.37.0
[v0.37.1]: https://github.com/neilotoole/sq/compare/v0.37.0...v0.37.1
[v0.38.0]: https://github.com/neilotoole/sq/compare/v0.37.1...v0.38.0
[v0.38.1]: https://github.com/neilotoole/sq/compare/v0.38.0...v0.38.1
[v0.39.0]: https://github.com/neilotoole/sq/compare/v0.38.1...v0.39.0
[v0.39.1]: https://github.com/neilotoole/sq/compare/v0.39.0...v0.39.1
[v0.40.0]: https://github.com/neilotoole/sq/compare/v0.39.1...v0.40.0
[v0.41.0]: https://github.com/neilotoole/sq/compare/v0.40.0...v0.41.0
[v0.41.1]: https://github.com/neilotoole/sq/compare/v0.41.0...v0.41.1
[v0.42.0]: https://github.com/neilotoole/sq/compare/v0.41.1...v0.42.0
[v0.42.1]: https://github.com/neilotoole/sq/compare/v0.42.0...v0.42.1
[v0.43.0]: https://github.com/neilotoole/sq/compare/v0.42.1...v0.43.0
[v0.43.1]: https://github.com/neilotoole/sq/compare/v0.43.0...v0.43.1
