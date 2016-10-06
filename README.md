# sq: simple queryer for structured data

`sq` is a command-line tool that provides uniform access to your datasources, whether
they be traditional SQL-style databases, or document formats such as JSON, XML, Excel.

A typical session might look like:

![sq session](http://neilotoole.io/sq/assets/sq-example-overview.png)

In the simplest narrative, you use `sq` to query a datasource and output JSON.

```
> sq '.user | .uid, .username, .email'
```
```json
[
  {
    "uid": 1,
    "username": "neilotoole",
    "email": "neilotoole@apache.org"
  },
  {
    "uid": 2,
    "username": "ksoze",
    "email": "kaiser@soze.org"
  },
  {
    "uid": 3,
    "username": "kubla",
    "email": "kubla@khan.mn"
  }
]
```

`sq` has its own query language (also know as `sq`), which takes much of its inspiration
from the excellent [jq](https://stedolan.github.io/jq/) utility. However, for SQL-based
datasources, you can also revert to DB-native SQL if you prefer. And JSON is not your
only output option.

```
> sq --native --table --header 'SELECT uid, username, email FROM user'
uid  username    email
1    neilotoole  neilotoole@apache.org
2    ksoze       kaiser@soze.org
3    kubla       kubla@khan.mn
```

`sq` aims for terseness; in typical usage, that command would use shorthand flags:

```
> sq -nth 'select uid, username, email from user'
uid  username    email
1    neilotoole  neilotoole@apache.org
2    ksoze       kaiser@soze.org
3    kubla       kubla@khan.mn
```

Use `sq inspect` to get schema information.

```
 > sq inspect -th @pq1
REF   NAME   FQ NAME       SIZE   TABLES  LOCATION
@pq1  pqdb1  pqdb1.public  7.1MB  4       postgres://sq:sq@localhost/pqdb1?sslmode=disable

TABLE       ROWS  SIZE     NUM COLS  COL NAMES                                                                     COL TYPES
tblall      2     104.0KB  7         col_id, col_int, col_int_n, col_varchar, col_varchar_n, col_blob, col_blob_n  integer, integer, integer, varchar(255), varchar(255), bytea, bytea
tbladdress  2     48.0KB   7         address_id, street, city, state, zip, country, uid                            integer, varchar(255), varchar(255), varchar(255), varchar(255), varchar(255), integer
tbluser     6     64.0KB   3         uid, username, email                                                          integer, varchar(255), varchar(255)
tblorder    0     16.0KB   6         order_id, uid, item_id, address_id, quantity, description                     integer, integer, integer, integer, integer, varchar(255)
```

Omit the `-th` (i.e. default JSON output) to get fuller schema information.


Use `sq --help` to see the available commands.


## Usage

Basics:

```
# Add a datasource... yeah, the mysql driver URL could be prettier
> sq add 'mysql://root:root@tcp(localhost:33306)/sq_my1' @my1

# Set the active datasource
> sq use @my1

# List datasources
> sq ls
REF       DRIVER    LOCATION
@pq1       postgres  postgres://pqgotest:password@localhost/sq_pq1
@my1       mysql     mysql://root:root@tcp(localhost:3306)/sq_my1


# Execute a query
> sq '.user'                  # get all rows and cols from table "user"
> sq '.user | .uid, .email'   # get cols "uid" and "email" from table "user"
```

### Ping

Use `sq ping` to check on the health of your datasources.

```
> sq ping         # ping active datasource
> sq ping @my1    # ping @my1 datsource
> sq ping --all   # ping all datasources
```

Output of `sq ping --all`:

![sq ping](http://neilotoole.io/sq/assets/sq-ping-all.png)



### Adding datasources

Note that the format of the database URL/DSN is driver-dependent. These examples should work tho.

```
> sq add 'mysql://root:root@tcp(localhost:3306)/sq_my1' @my1
> sq add 'postgres://pqgotest:password@localhost/sq_pq1' @pq1
> sq add 'sqlite3:///Users/neilotoole/testdata/sqlite1.db' @sl1
> sq add /Users/neilotoole/testdata/test.xlsx @excel1
> sq add http://neilotoole.io/sq/test/test1.xlsx @excelRemote1
```

### Remove datasource

```
> sq rm @my1    # remove datasource @my1
```

### join

Currently (`sq 0.30.0`) only one join per query. To be fixed soon.

```
> sq '.user, .address | join(.uid) | .username, .city, .zip'
> sq '.user, .order | join(.uid == .order_uid) | .email, .order_id'
```


### cross-datasource join

You can join across datasources. Note that the current implementation is very
naive (slow and resource-intensive), basically it results in full-table copy of
each joined table. This can and will be improved.

Given some datasources like this:

```
> sq ls
REF               DRIVER    LOCATION
@my1              mysql     mysql://root:root@tcp(localhost:3306)/sq_my1
@pq1              postgres  postgres://sq:sq@localhost/sq_pq1?sslmode=disable
@excel1           xlsx      /Users/neilotoole/testdata/test.xlsx
```

You can do joins like this:

```
> sq '@my1.user, @pq1.tbladdress | join(.uid) | .username, .email, .city'
```

```json
[
  {
    "username": "ksoze",
    "email": "kaiser@soze.org",
    "city": "Washington"
  },
  {
    "username": "kubla",
    "email": "kubla@khan.mn",
    "city": "Ulan Bator"
  }
]
```

Or join an Excel spreadsheet with a database:

```
> sq '@excel1.user_sheet, @pq1.tbladdress | join(.A == .uid)'
```

```json
[
  {
    "A": 2,
    "B": "ksoze",
    "C": "kaiser@soze.org",
    "address_id": 1,
    "street": "1600 Penn",
    "city": "Washington",
    "state": "DC",
    "zip": "12345",
    "country": "US",
    "uid": 2
  },
  {
    "A": 3,
    "B": "kubla",
    "C": "kubla@khan.mn",
    "address_id": 2,
    "street": "999 Coleridge St",
    "city": "Ulan Bator",
    "state": "UB",
    "zip": "888",
    "country": "MN",
    "uid": 3
  }
]
```


### range / row select

Select specific rows from the result set (similar to SQL's `LIMIT X OFFSET Y`).

```
> sq '.user | .[0]'      # get first row
> sq '.user | .[3:7]     # get rows 3 thru 7
> sq '.user | .[5:]      # get all rows from 5 onwards
```


## Coming soon

These features not implemented yet, but next on the agenda.

### multiple joins
```
> sq '@my1.user, @pq1.tbladdress | join(.uid), @excel1.user_sheet | join(.uid == .A) | .username, .email, .city, .B'
```

And/or this mechanism:

```
> sq '@my1.user, @pq1.tbladdress, @excel1.user_sheet | join(.uid), . | join(.uid == .A) | .username, .email, .city, .B'
```

### where (conditional select)
```
> sq '.user | .uid > 100 && .username != "ksoze"'
```

### table / column aliasing
```
> sq '.user | .uid:user_id, .username, .email:mail'  # rename "uid" to "user_id", and "email" to "mail"
```

### column selection by ordinal (index)
```
> sq '.user | .2, .uid, .5:9`    # select the second col, the "uid" col, and cols 5 thru 9
```

### cross-datasource copy / backup

```
> sq '@my1.user | > | @pq1.users'                                         # overwrite @pq1.users with @my1.user
> sq '@my1.user | >> | @pq1.users'                                        # append @my1.user to @pq1.users
> sq '@my1.user | .username, .email | >> | @pq1.users | .uname, .email'   # append certain fields
```

### Additional datasource types

- Oracle
- MS SQL Server
- Teradata
- ODBC
- JSON
- XML

### Additional output formats

- JSON grid (just an array of arrays `row[cell]`)
- CSV
- TSV
- Excel