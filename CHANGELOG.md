# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased - 3000-01-01

### Added


### Changed


### Fixed

- Shell completion is better behaved when a source is offline [#123](issues/123).

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

- [#91](https://github.com/neilotoole/sq/pull/91): MySQL driver options no longer stripped

## [v0.15.2] - 2021-03-08

### Changed

- [#89](https://github.com/neilotoole/sq/pull/89): Bug with SQL generated for joins.


[v0.16.0]: https://github.com/neilotoole/sq/compare/v0.15.11...v0.16.0
[v0.15.11]: https://github.com/neilotoole/sq/compare/v0.15.4...v0.15.11
[v0.15.4]: https://github.com/neilotoole/sq/compare/v0.15.3...v0.15.4
[v0.15.3]: https://github.com/neilotoole/sq/compare/v0.15.2...v0.15.3
[v0.15.2]: https://github.com/neilotoole/sq/releases/tag/v0.15.2
