# sq: swiss army knife for data

`sq` provides uniform access to
structured data sources like traditional SQL-style databases,
or document formats such as CSV or Excel. `sq` can perform cross-source joins, 
execute database-native SQL, and output to a multitude of formats including JSON,
Excel, CSV, HTML, Markdown and XML, or output directly to a SQL database.
`sq` can inspect sources to see metadata about the source structure (tables,
columns, size) and has commands for common database operations such as copying
or dropping tables.

## Usage

See the [wiki](https://github.com/neilotoole/sq/wiki). 

## Installation


### From source

From the `sq` project dir:

```shell script
$ go install
```

The simple go install does not populate the binary with build info that
is output via the `sq version` command. To do so, use [mage](https://magefile.org/).

```shell script
$ brew install mage
$ mage install
```

### Other installation options

For homebrew, scoop, rpm etc, see the [wiki](https://github.com/neilotoole/sq/wiki).


## Acknowledgements

- Much inspiration is owed to [jq](https://stedolan.github.io/jq/).
- See [`go.mod`](https://github.com/neilotoole/sq/blob/master/go.mod) for a list of third-party packages.
- Additionally, `sq` incorporates modified versions of:
    - [`olekukonko/tablewriter`](https://github.com/olekukonko/tablewriter)
    - [`segmentio/encoding`](https://github.com/segmentio/encoding) for JSON encoding.
- The [_Sakila_](https://dev.mysql.com/doc/sakila/en/) example databases were lifted from [jOOQ](https://github.com/jooq/jooq), which
  in turn owe their heritage to earlier work on Sakila.


