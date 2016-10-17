# sq: simple queryer for structured data

`sq` is a command-line tool that provides uniform access to structured data sources.
This includes traditional SQL-style databases, or document formats such as JSON, XML, Excel etc.


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

> `sq` defines its own query language, seen above, formally known as `SLQ`.


For usage information or to download the binary, see the `sq` [manual](https://github.com/neilotoole/sq-manual/wiki).


## Development

These steps are for Mac OS X (tested on El Capitan `10.11.16`). The examples assume username  `ksoze`.


### Prerequisites
- [brew](http://brew.sh/)
- [Xcode](https://itunes.apple.com/us/app/xcode/id497799835?mt=12) dev tools.
- [jq](https://stedolan.github.io/jq/) `brew install jq 1.5`
- [Java](http://www.oracle.com/technetwork/java/javase/downloads/index.html) is required if you're working on the *SLQ* grammar.
- [Go](https://golang.org/doc/install) `brew install go 1.7.1`


### Fork
Fork this [repo](https://github.com/neilotoole/sq), e.g. to  `https://github.com/ksoze/sq`.

Clone the forked repo and set the `upstream` remote:

```
mkdir -p $GOPATH/src/github.com/ksoze
cd $GOPATH/src/github.com/ksoze
git clone https://github.com/ksoze/sq.git
cd ./sq
git remote add upstream https://github.com/neilotoole/sq.git
# verify that the remote was set
git remote -v
```
	
### Make
From  `$GOPATH/src/github.com/ksoze/sq`:

```
make test
make install
make smoke
```
	
That should be it. Try `sq ls`. Note that by default `sq` uses `~/.sq/sq.yml` as
its config store, and outputs debug logs to `~/.sq/sq.log`.

