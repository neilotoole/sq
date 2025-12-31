# CHANGELOG

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Breaking changes are annotated with ☢️, and alpha/beta features with 🐥.

> [!NOTE]
> Sometimes this `CHANGELOG.md` has gaps between versions, e.g. `v0.18.0` to `v0.18.2`.
> This typically means that there was some CI/tooling mishap. Ignore those gaps.

## Unreleased

### Fixed

- [#469]: Column widths were too wide when using `--no-header` flag. Header text
  is now excluded from column width calculation when headers are disabled.
  Thanks to [@majiayu000](https://github.com/majiayu000) for the fix.

### Changed

### Added

## [v0.48.10] - 2025-12-28

### Fixed

- [#506]: Fixed two XLSX-related issues (sadly, both are regression fixes):
  - **Stdin detection**: Fixed type detection failing for XLSX files created by
    various tools (e.g., the [`excelize`](https://github.com/qax-os/excelize)
    library). These files have varying internal ZIP structures that the previous
    detection couldn't handle. Detection now scans ZIP local file headers
    instead of relying on fragile magic number heuristics.
  - **Output colorization**: Fixed XLSX binary output
    ([`--xlsx`](https://sq.io/docs/output#xlsx)) being corrupted
    when written to `stdout`. The colorization decorator was modifying the binary
    data. XLSX format now bypasses colorization, like
    [`--raw`](https://sq.io/docs/output#raw) output already does.

### Changed

- [#504]: Updated `golangci-lint` to `v2.7.2`, along with Go dependencies
  and GitHub Actions workflow versions. Other tool versions have been updated too.
  
  Note that Go tool dependencies are now located in the [`tools/`](./tools)
  directory, each with its own `go.mod`. Tools are invoked via
  `go tool -modfile`, e.g. `go tool -modfile=tools/golangci-lint/go.mod golangci-lint`.
  See the [Makefile](Makefile) and [`tools/README.md`](./tools/README.md) for more detail.

## [v0.48.5] - 2025-01-19

### Fixed

- [#446]: A [`bufio.ErrTooLong`](https://pkg.go.dev/bufio#ErrTooLong) was being returned
  by [`bufio.Scanner`](https://pkg.go.dev/bufio#Scanner), when splitting
  lines from input that was too long (larger than
  [`bufio.MaxScanTokenSize`](https://pkg.go.dev/bufio#MaxScanTokenSize), i.e. `64KB`). This meant that
  `sq` wasn't able to parse large JSON files, amongst other problems. The maximum buffer size is 
  now configurable via the new [`tuning.scan-buffer-limit`](https://sq.io/docs/config/#tuningscan-buffer-limit)
  option. Note that the buffer will start small and grow as needed, up to the limit.

  ```plaintext
  $ sq config set tuning.scan-buffer-limit 64MB   # or 1024B, 64KB, 1GB, etc.
  ```
  A more useful error message is also now returned when the buffer limit is exceeded
  (the error suggests adjusting `tuning.scan-buffer-limit`).

### Changed

- Renamed config option `tuning.buffer-mem-limit` to [`tuning.buffer-spill-limit`](https://sq.io/docs/config/#tuningbuffer-spill-limit).
  The new name better reflects the purpose of the option.

  
## [v0.48.4] - 2024-11-24

### Changed

- Updated Go dependencies (was failing some security vulnerability scans).

## [v0.48.3] - 2024-03-11

Small bugfix release.

### Fixed

- [#415]: The JSON ingester could fail due to a bug when a JSON blob landed on the edge
  of a buffer.
- The JSON ingester wasn't able to handle the case where a post-[sampling](https://sq.io/docs/config#ingestsample-size)
  JSON field had a different kind from the kind determined by the sampling process. For example, let's say
  the sample size was 1000, and the field `zip` was determined to be of kind `int`, because
  values 0-1000 were all parseable as integers. But then the 1001st value was `BX123`, which
  obviously is not an integer. `sq` will now see the non-integer value, and alter the ingest DB schema
  to a compatible kind, e.g. `text`. This flexibility is powerful, but it does come at the cost of slower
  ingest speed. But that's a topic for another release.

## [v0.48.1] - 2024-03-07

This release features significant improvements to [`sq diff`](https://sq.io/docs/diff).

### Added

- Previously `sq diff --data` diffed every row, which could get crazy
  with a large table. Now the command stops after N differences, where N is controlled by
  the `--stop` (`-n`) flag, or the new config option [`diff.stop`](https://sq.io/docs/config#diffstop).
  The default stop-after value is `3`; set to `0` to show all differences.

  ```shell
  # Stop on first difference
  $ sq diff @prod.actor @staging.actor --data --stop 1
  
  # Stop after 5 differences, using the -n shorthand flag
  $ sq diff @prod.actor @staging.actor --data -n5
  ```

- [#353]: The performance of `sq diff` has been significantly improved. There's still more to do.

- Previously, `sq diff --data` compared the rendered (text) representation of each value. This could
  lead to inaccurate results, for example with two timestamp values in different time zones, but the text
  rendering omitted the time zone. Now, `sq diff --data` compares the raw values, not the rendered text.
  Note in particular with time values that both time and location components are compared.

- `sq` can now handle a SQLite DB on `stdin`. This is useful for testing, or for
  working with SQLite DBs in a pipeline.

  ```shell
  $ cat sakila.db | sq '.actor | .first_name, .last_name'
  ```

  It's also surprisingly handy in daily life, because there are sneaky SQLite DBs
  all around us. Let's see how many text messages I've sent and received over the years:

  ```shell
  $ cat ~/Library/Messages/chat.db | sq '.message | count'
  count
  215439
  ```
  I'm sure that number makes me an amateur with these millenials 👴🏻. 
  
  > Note that you'll need to enable macOS [Full Disk Access](https://spin.atomicobject.com/search-imessage-sql/) to read the `chat.db` file.

- `sq` now allows you to use `true` and `false` literals in queries. Which, in hindsight, does seem like a bit of
  an oversight 😳. (Although previously you could usually get away with using `1` and `0`).

  ```shell
  $ sq '.people | where(.is_alive == false)'
  name        is_alive
  Kubla Khan  false
  
  $ sq '.people | where(.is_alive == true)'
  name         is_alive
  Kaiser Soze  true
  ```

### Changed

- ☢️ Previously, `sq diff` only exited non-zero on an error. Now, `sq diff` exits `0` when no differences,
  exits `1` if differences are found, and exits `2` on any error.
  This aligns with the behavior of [GNU diff](https://www.gnu.org/software/diffutils/manual/):

  ```text
  Exit status is 0 if inputs are the same, 1 if different, 2 if trouble.
  ```

- Minor fiddling with the color scheme for some command output.


## [v0.47.4] - 2024-02-09

Patch release with changes to flags.
See the earlier [`v0.47.0`](https://github.com/neilotoole/sq/releases/tag/v0.47.0)
release for recent headline features.

### Added

- By default, `sq` prints source locations with the password redacted. This is a sensible default, but
  there are legitimate reasons to access the unredacted connection string. Thus a new
  global flag `--no-redact` (and a corresponding [`redact`](https://sq.io/docs/config#redact) config option).

  ```shell
  # Default behavior: password is redacted
  $ sq src -v
  @sakila/pg12  postgres  postgres://sakila:xxxxx@192.168.50.132/sakila
  
  # Unredacted
  $ sq src -v --no-redact
  @sakila/pg12  postgres  postgres://sakila:p_ssW0rd@192.168.50.132/sakila
  ```

- Previously, if an error occurred when [`verbose`](https://sq.io/docs/config#verbose) was true,
  and [`error.format`](https://sq.io/docs/config#errorformat) was `text`, `sq` would print a stack trace 
  to `stderr`. This was poor default behavior, flooding the user terminal, so the default is now no stack trace.
  To restore the previous behavior, use the new `-E` (`--error.stack`) flag, or set the [`error.stack`](https://sq.io/docs/config#errorstack) config option.


### Changed

- The [`--src.schema`](https://sq.io/docs/source#source-override) flag (as used in [`sq inspect`](https://sq.io/docs/cmd/inspect),
  [`sq sql`](https://sq.io/docs/cmd/sql), and the root [`sq`](https://sq.io/docs/cmd/sq#override-active-schema) cmd)
  now accepts `--src.schema=CATALOG.`. Note the `.` suffix on `CATALOG.`. This is in addition to the existing allowed forms `SCHEMA`
  and `CATALOG.SCHEMA`. This new `CATALOG.` form is effectively equivalent to `CATALOG.CURRENT_SCHEMA`.
  
  ```shell
  # Inspect using the default schema in the "sales" catalog
  $ sq inspect --src.schema=sales. 
  ```

- The [`--src.schema`](https://sq.io/docs/source#source-override) flag is now validated. Previously, if you provided a non-existing catalog or schema
  value, `sq` would silently ignore it and use the defaults. This could mislead the user into thinking that
  they were getting valid results from the non-existent catalog or schema. Now an error is returned.


## [v0.47.3] - 2024-02-03

Minor bug fix release. See the earlier [`v0.47.0`](https://github.com/neilotoole/sq/releases/tag/v0.47.0)
release for recent headline features.

### Fixed

- Shell completion for `bash` only worked for top-level commands, not for subcommands, flags,
  args, etc. This bug was due to an unnoticed behavior change in an imported library 🤦‍♂️. It's now fixed,
  and tests have been added.

### Changed

- Shell completion now initially suggests only sources within the
  [active group](https://sq.io/docs/source#groups). Previously, all sources were suggested,
  potentially flooding the user with irrelevant suggestions. However, if the user
  continues to input a source handle that is outside the active group, completion will
  suggest all matching sources. This behavior is controlled
  via the new config option [`shell-completion.group-filter`](https://sq.io/docs/config#shell-completiongroup-filter).

## [v0.47.2] - 2024-01-29

Yet another morning-after-the-big-release issue, a nasty little one this time. 
See the earlier [`v0.47.0`](https://github.com/neilotoole/sq/releases/tag/v0.47.0) release
for recent headline features.

### Fixed

- `sq` was failing to write config when there was no pre-existing config file. This was due to
  a bug in the newly-introduced (as of `v0.47.0`) config locking mechanism. Fixed.


## [v0.47.1] - 2024-01-29

This is a tiny bugfix release for a runtime issue on some Linux distros. See
the previous [`v0.47.0`](https://github.com/neilotoole/sq/releases/tag/v0.47.0) release
for recent headline features.

### Fixed

- `sq` [panicked](https://github.com/neilotoole/sq/actions/runs/7701355729/job/20987599862#step:3:383) on some Linux distros that don't include timezone data (`tzdata`). It's now
  explicitly [imported](https://wawand.co/blog/posts/go-timezonedata-go115/).


## [v0.47.0] - 2024-01-29

This is a significant release, focused on improving i/o, responsiveness,
and performance. The headline features are [caching](https://sq.io/docs/source#cache)
of [ingested](https://sq.io/docs/source#ingest) data for [document sources](https://sq.io/docs/source#document-source)
such as CSV or Excel, and [download](https://sq.io/docs/source#download) caching for remote document sources.
There are a lot of under-the-hood changes, so please [open an issue](https://github.com/neilotoole/sq/issues/new/choose) if
you encounter any weirdness.

### Added

- Long-running operations (such as data [ingestion](https://sq.io/docs/source#ingest),
  or file [download](https://sq.io/docs/source#download)) now result
  in a progress bar being displayed. Display of the progress bar is controlled
  by the new config options [`progress`](https://sq.io/docs/config#progress)
  and [`progress.delay`](https://sq.io/docs/config#progressdelay). You can also use
  the `--no-progress` flag to disable the progress bar.
  - 👉 The progress bar is rendered on `stderr` and is always zapped from the terminal when command output begins.
    It won't corrupt the output.
- [#307]: Ingested [document sources](https://sq.io/docs/source#document-source) (such as
  [CSV](https://sq.io/docs/drivers/csv) or [Excel](https://sq.io/docs/drivers/xlsx))
  now make use of an [ingest](https://sq.io/docs/source#ingest) cache DB. Previously, ingestion
  of document source data occurred  on each `sq` command. It is now a one-time cost; subsequent
  use of the document source utilizes
  the cache DB. Until, that is, the source document changes: then the ingest cache DB is invalidated and
  ingested again. This is a significantly improved experience for large document sources.
- There are several new commands to interact with the cache (although you shouldn't need to):
  - [`sq cache enable`](https://sq.io/docs/cmd/cache-enable) and
    [`sq cache disable`](https://sq.io/docs/cmd/cache-disable) control cache usage.
    You can also instead use the new [`ingest.cache`](https://sq.io/docs/config#ingestcache)
    config option.
  - [`sq cache clear`](https://sq.io/docs/cmd/cache-clear) clears the cache.
  - [`sq cache location`](https://sq.io/docs/cmd/cache-location) prints the cache location on disk.
  - [`sq cache stat`](https://sq.io/docs/cmd/cache-stat) shows stats about the cache.
  - [`sq cache tree`](https://sq.io/docs/cmd/cache-location) shows a tree view of the cache.
- [#24]: The [download](https://sq.io/docs/source#download) mechanism for remote document sources (e.g. a CSV file at
  [`https://sq.io/testdata/actor.csv`](https://sq.io/testdata/actor.csv)) has been completely
  overhauled. Previously, `sq` would re-download the remote file on every command. Now, the
  remote file is downloaded and [cached](https://sq.io/docs/source#cache) locally.
  Subsequent `sq` invocations check for staleness of the cached download, and re-download if necessary.
- As part of the download revamp, new config options have been introduced:
  - [`http.request.timeout`](https://sq.io/docs/config#httprequesttimeout) is the timeout for the initial response from the server, and
    [`http.response.timeout`](https://sq.io/docs/config#httpresponsetimeout) is the timeout for reading the entire response body. We separate
    these two timeouts because it's possible that the server responds quickly, but then
    for a large file, the download takes too long.
  - [`https.insecure-skip-verify`](https://sq.io/docs/config#httpsinsecure-skip-verify) controls
    whether HTTPS connections verify the server's certificate. This is useful for remote files served
    with a self-signed certificate.
  - [`download.cache`](https://sq.io/docs/config#downloadcache) controls whether remote files are
    cached locally.
  - [`download.refresh.ok-on-err`](https://sq.io/docs/config#downloadrefreshok-on-err)
    controls whether `sq` should continue with a stale cached download if an error
    occurred while trying to refresh the download. This is a sort
    of "Airplane Mode" for remote document sources: `sq` continues with the cached download when
    the network is unavailable.
- There are two more new config options introduced as part of the above work.
  - [`cache.lock.timeout`](https://sq.io/docs/config#cachelocktimeout) controls the time that
    `sq` will wait for a lock on the cache DB. The cache lock is introduced for when you have
    multiple `sq` commands running concurrently, and you want to avoid them stepping on each other.
  - Similarly, [`config.lock.timeout`](https://sq.io/docs/config#configlocktimeout) controls the
    timeout for acquiring the (newly-introduced) lock on `sq`'s config file. This helps prevent
    issues with multiple `sq` processes mutating the config concurrently.
- `sq`'s own [logs](https://sq.io/docs/config#logging) previously outputted in JSON
  format. Now there's a new [`log.format`](https://sq.io/docs/config#logformat) config option
  that permits setting the log format to `json` or `text`. The `text` format is more human-friendly, and
  is now the default.

### Changed

- Ingestion performance for [`json`](https://sq.io/docs/drivers/json#json) and
  [`jsonl`](https://sq.io/docs/drivers/json#jsonl) sources has been significantly improved.

### Fixed

- Opening a DB connection now correctly honors [`conn.open-timeout`](https://sq.io/docs/config#connopen-timeout).

## [v0.46.1] - 2023-12-06

### Fixed

- `sq` sometimes failed to read from stdin if piped input was slow
  to arrive. This is now fixed.

## [v0.46.0] - 2023-11-22

### Added

- [#338]: While `sq` has had [`group_by`](https://sq.io/docs/query#group_by) for some time,
  somehow the [`having`](https://sq.io/docs/query#having) mechanism was never implemented. That's fixed.

  ```shell
  $ sq '.payment | .customer_id, sum(.amount) | group_by(.customer_id) | having(sum(.amount) > 200)'
  customer_id  sum(.amount)
  526          221.55
  148          216.54
  ```

- [#340]: The [`group_by`](https://sq.io/docs/query#group_by) function
  now has a synonym `gb`, and [`order_by`](https://sq.io/docs/query#order_by) now has synonym `ob`.
  These synonyms are experimental 🧪. The motivation is to reduce typing, especially the underscore (`_`)
  in both function names, but it's not clear that the loss of clarity is worth it. Maybe synonyms
  `group` and `order` might be better? Feedback welcome.

  ```shell
  # Previously
  $ sq '.payment | .customer_id, sum(.amount) | group_by(.customer_id) | order_by(.customer_id)'
  
  # Now
  $ sq '.payment | .customer_id, sum(.amount) | gb(.customer_id) | ob(.customer_id)'
  ```
- [#340]: [`sq inspect`](https://sq.io/docs/cmd/inspect): added flag shorthand `-C`
  for `--catalogs` and `-S` for `--schemata`. These were the only `inspect`
  flags without shorthand.

## [v0.45.0] - 2023-11-21

### Changed

- [#335]: Previously, `sq` didn't handle decimal values correctly. It basically
  shoved a decimal value into a `float` or `string` and hoped for the best.
  [As is known](https://medium.com/@mayuribudake999/difference-between-decimal-and-float-eede050f6c9a),
  floats are imprecise, and so we saw [unwanted behavior](https://github.com/neilotoole/sq/actions/runs/6932116521/job/18855333269#step:6:2345), e.g.

  ```shell
  db_type_test.go:194: 
        Error Trace:	D:/a/sq/sq/drivers/sqlite3/db_type_test.go:194
        Error:      	Not equal: 
                      expected: "77.77"
                      actual  : "77.77000000000001"
  ```

  Now, `sq` uses a dedicated [`Decimal`](https://github.com/shopspring/decimal) type end-to-end.
  No precision is lost, and at the output end, the value is rendered with the correct precision.

  There is a [proposal](https://github.com/golang/go/issues/30870) to add decimal support to
  the Go [`database/sql`](https://pkg.go.dev/database/sql) package. If that happens, `sq` will happily
  switch to that mechanism.
  - 👉 A side effect of decimal support is that some output formats may now render decimal values
    differently (i.e. correctly). In particular, Excel output should now render decimals as a number
    (as opposed to a string), and with the precision defined in the database. Previously,
    a database `NUMERIC(10,5)` value might have been rendered as `100.00`,
    but will now accurately render `100.00000`.

## [v0.44.0] - 2023-11-20

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

- [`sq inspect`](https://sq.io/docs/inspect) has two new flags:
  - [`--schemata`](https://sq.io/docs/inspect#schemata): list the source's schemas
    ```shell
    $ sq inspect @sakila/pg12 --schemata -y
    - schema: information_schema
      catalog: sakila
      owner: sakila
    - schema: public
      catalog: sakila
      owner: sakila
      active: true
    ```
  - [`--catalogs`](https://sq.io/docs/inspect#catalogs): list the source's catalogs
    ```shell
    $ sq inspect @sakila/pg12 --catalogs
    CATALOG
    postgres
    sakila
    ```

### Fixed

- [`sq version`](https://sq.io/docs/cmd/version) now honors option
  [`format.datetime`](https://sq.io/docs/config#formatdatetime) when outputting build timestamp.
- Fixed a fairly nasty bug that prevented correct rendering of SLQ functions nested inside
  an expression.

### Changed

- The  `--exec` and `--query` flags for [`sq sql`](https://sq.io/docs/cmd/sql) were removed in
  the preceding release ([v0.43.1]).
  That was probably a bit hasty, especially because it's possible those flags _could_ be reintroduced
  when the _query vs exec_ situation is figured out. So, those two flags are now restored, in
  that their use won't cause an error, but they've been hidden from command help, and remain no-op.

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

### Added

- [#229]: `sq diff` compares two sources, or tables.
- `sq inspect --dbprops` is a new mode that returns only the DB properties.
  Relatedly, the properties mechanism is now implemented for all four supported
  DB types (previously, it was only implemented for Postgres and MySQL).
- [CSV](https://sq.io/docs/output#csv-tsv) format now colorizes output.

### Changed

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

### Fixed

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

- [#15]: Column Aliases. You can now specify an alias for a column (or column expression
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
[#24]: https://github.com/neilotoole/sq/issues/24
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
[#307]: https://github.com/neilotoole/sq/issues/307
[#308]: https://github.com/neilotoole/sq/pull/308
[#335]: https://github.com/neilotoole/sq/issues/335
[#338]: https://github.com/neilotoole/sq/issues/338
[#340]: https://github.com/neilotoole/sq/pull/340
[#353]: https://github.com/neilotoole/sq/issues/353
[#415]: https://github.com/neilotoole/sq/issues/415
[#446]: https://github.com/neilotoole/sq/issues/446
[#469]: https://github.com/neilotoole/sq/issues/469
[#504]: https://github.com/neilotoole/sq/issues/504
[#506]: https://github.com/neilotoole/sq/issues/506


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
[v0.44.0]: https://github.com/neilotoole/sq/compare/v0.43.1...v0.44.0
[v0.45.0]: https://github.com/neilotoole/sq/compare/v0.44.0...v0.45.0
[v0.46.0]: https://github.com/neilotoole/sq/compare/v0.45.0...v0.46.0
[v0.46.1]: https://github.com/neilotoole/sq/compare/v0.46.0...v0.46.1
[v0.47.0]: https://github.com/neilotoole/sq/compare/v0.46.1...v0.47.0
[v0.47.1]: https://github.com/neilotoole/sq/compare/v0.47.0...v0.47.1
[v0.47.2]: https://github.com/neilotoole/sq/compare/v0.47.1...v0.47.2
[v0.47.3]: https://github.com/neilotoole/sq/compare/v0.47.2...v0.47.3
[v0.47.4]: https://github.com/neilotoole/sq/compare/v0.47.3...v0.47.4
[v0.48.1]: https://github.com/neilotoole/sq/compare/v0.47.4...v0.48.1
[v0.48.3]: https://github.com/neilotoole/sq/compare/v0.48.1...v0.48.3
[v0.48.4]: https://github.com/neilotoole/sq/compare/v0.48.3...v0.48.4
[v0.48.5]: https://github.com/neilotoole/sq/compare/v0.48.4...v0.48.5
[v0.48.10]: https://github.com/neilotoole/sq/compare/v0.48.5...v0.48.10
