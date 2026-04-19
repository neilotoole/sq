---
title: "sq add"
description: "Add data source"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2020
toc: false
url: /docs/cmd/add
---
Use `sq add` to add a data source. The source can be a SQL database, or a document
such as a CSV or Excel file. This action will add an entry to `sq`'s
[config file](/docs/overview/#config).

If you later want to change the source, generally the easiest solution is to
`sq rm @handle` and then `sq add` again. However, you can also directly edit
the config file (e.g. `vi ~/.config/sq/sq.yml`).

A data source has three main elements:

- `driver type`: such as `postgres`, or `csv`. You may also see this referred to as the _source type_
  or simply _type_.
- `handle`: such as `@sakila_pg`. A handle always starts with `@`. The handle is used to refer
  to the data source.
- `location`: such as `postgres://user:p_ssW0rd@localhost/sakila`. For
  a document source, _location_ may just be a file path, e.g. `/Users/neilotoole/sakila.csv`.

The format of the command is:

```shell
sq add [--handle HANDLE] [--driver DRIVER] [--active] LOCATION
```

For example, to add a postgres data source:

```shell
$ sq add postgres://sakila:p_ssW0rd@localhost/sakila
@sakila_pg  postgres  sakila@localhost/sakila
```

Note that flags can generally be omitted. If `--handle` is omitted,
`sq` will generate a handle. In the example above, the generated handle
is `@sakila_pg`. Usually `--driver` can also be omitted, and `sq`
will determine the driver type. The `--active` flag immediately sets
the newly-added source as the active source (this also happens regardless if there is
not currently an active source).

To add a document source, you can generally just add the file path:

```shell
sq add ~/customers.csv
```

## Password visibility

In the Postgres example above, the _location_ string includes the database password. This is a
security hazard, as the password value is visible on the command line, and in
shell history etc. You can use the `--password` / `-p` flag to be prompted
for the password.

```shell
$ sq add 'postgres://user@localhost/sakila' -p
Password: ****
```

You can also read the password from a file or a shell variable. For example:

```shell
# Add a source, but read password from an environment variable
$ export PASSWORD='open:;"_Ses@me'
$ sq add 'postgres://user@localhost/sakila' -p <<< $PASSWORD

# Same as above, but instead read password from file
$ echo 'open:;"_Ses@me' > password.txt
$ sq add 'postgres://user@localhost/sakila' -p < password.txt
```

### Location completion

It can be difficult to remember the format of database URLs (i.e. the source **location**).
To make life easier, `sq` provides shell completion for the `sq add LOCATION` field. To
use it, just press `TAB` after `$ sq add`.

For location completion to work, do not enclose the location in single quotes. However,
this does mean that the inputted location string must escape special shell characters
such as `?` and `&`.

```shell
# Location completion not available, because location is in quotes.
$ sq add 'postgres://sakila@192.168.50.132/sakila?sslmode=disable'

# Location completion available: note the escaped ?.
$ sq add postgres://sakila@192.168.50.132/sakila\?sslmode=disable
```

The location completion mechanism suggests usernames, hostnames (from history),
database names, and even values for query params (e.g. `?sslmode=disable`) for
each supported database. It never suggests passwords.

{{< asciicast src="/casts/src-add-location-completion-pg.cast" poster="npt:0:8" idleTimeLimit=0.5 rows=6 speed=1.5 >}}

## Header row

File formats like CSV/TSV or Excel often have a header row. `sq` can usually auto-detect
if a header row is present. But depending on the nature of the data file,
it may be necessary to explicitly tell `sq` to use a header row (or not).

```shell
$ sq add ./actor.csv --ingest.header
```

## Reference

{{< readfile file="add.help.txt" code="true" lang="text" >}}
