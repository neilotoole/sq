# CHANGELOG

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Breaking changes are annotated with ☢️.

## Upcoming

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
- Column-only queries are now permissible. This has the neat side effect
  that `sq` can now be used as a calculator.

  ```shell
  $ sq 1+2
  1+2
  3
  ```
  You probably want to use `--no-header` (`-H`):

  ```shell
  $ sq -H 1+2
  3
  ```


### Fixed

- Literals can now be selected ([docs](https://sq.io/docs/query/#select-literals)).
  
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


## [v0.37.1] - 2023-06-15

### Fixed

- [#252]: Handle `*uint64` returned from DB.

## [v0.37.0] - 2023-06-13

### Added

- [#244]: Shell completion for `sq add LOCATION`. See [docs](https://sq.io/docs/source/#location-completion).

## [v0.36.2] - 2023-05-27

### Changed

- ☢️ [Proprietary database functions](https://sq.io/docs/query/#proprietary-functions) are now 
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
- [CSV](https://sq.io/docs/output/#csv-tsv) format now colorizes output.

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
  flag ([docs](https://sq.io/docs/query/#predefined-variables)):
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

- [#162]: `group_by` now accepts [function arguments](https://sq.io/docs/query/#group_by). 

### Changed

- Renamed `groupby` to `group_by` to match jq.
- Renamed `orderby` to `order_by` to match jq.

## [v0.28.0] - 2023-03-26

### Added

- [#160]: Use `groupby()` to group results. See [query guide](https://sq.io/docs/query/#group_by).

## [v0.27.0] - 2023-03-25

### Added

- [#158]: Use `orderby()` to order results. See [query guide](https://sq.io/docs/query/#order_by).

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
[#15]: https://github.com/neilotoole/sq/issues/15
[#89]: https://github.com/neilotoole/sq/pull/89
[#91]: https://github.com/neilotoole/sq/pull/91
[#95]: https://github.com/neilotoole/sq/issues/93
[#98]: https://github.com/neilotoole/sq/issues/98
[#123]: https://github.com/neilotoole/sq/issues/123
[#142]: https://github.com/neilotoole/sq/issues/142
[#144]: https://github.com/neilotoole/sq/issues/144
[#151]: https://github.com/neilotoole/sq/issues/151
[#153]: https://github.com/neilotoole/sq/issues/153
[#155]: https://github.com/neilotoole/sq/issues/155
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
