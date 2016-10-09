# sq: simple queryer for structured data

`sq` is a command-line tool that provides uniform access to your datasources, whether
they be traditional SQL-style databases, or document formats such as JSON, XML, Excel.


One line explanation: use `sq` to query a datasource and output JSON or other formats.

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

For usage information or to download the binary, see the `sq` [manual](https://github.com/neilotoole/sq-manual/wiki).


## building sq
