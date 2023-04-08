# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
  - **BREAKING CHANGE**: The "star" version (`.actor | count(*)`) is no longer supported; use the
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

[#185]: https://github.com/neilotoole/sq/issues/185
[#173]: https://github.com/neilotoole/sq/issues/173
[#164]: https://github.com/neilotoole/sq/issues/164
[#162]: https://github.com/neilotoole/sq/issues/162
[#123]: https://github.com/neilotoole/sq/issues/123
[#142]: https://github.com/neilotoole/sq/issues/142
[#144]: https://github.com/neilotoole/sq/issues/144
[#151]: https://github.com/neilotoole/sq/issues/151
[#153]: https://github.com/neilotoole/sq/issues/153
[#155]: https://github.com/neilotoole/sq/issues/155
[#158]: https://github.com/neilotoole/sq/issues/158
[#15]: https://github.com/neilotoole/sq/issues/15
[#160]: https://github.com/neilotoole/sq/issues/160
[#89]: https://github.com/neilotoole/sq/pull/89
[#91]: https://github.com/neilotoole/sq/pull/91
[#95]: https://github.com/neilotoole/sq/issues/93
[#98]: https://github.com/neilotoole/sq/issues/98
[v0.15.11]: https://github.com/neilotoole/sq/compare/v0.15.4...v0.15.11
[v0.15.2]: https://github.com/neilotoole/sq/releases/tag/v0.15.2
[v0.15.3]: https://github.com/neilotoole/sq/compare/v0.15.2...v0.15.3
[v0.15.4]: https://github.com/neilotoole/sq/compare/v0.15.3...v0.15.4
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
