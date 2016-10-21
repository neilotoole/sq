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
- [Go](https://golang.org/doc/install) `brew install go 1.7.1`
- [Docker](https://docs.docker.com/docker-for-mac/)
- [Java](http://www.oracle.com/technetwork/java/javase/downloads/index.html) is required if you're working on the *SLQ* grammar.



### Fork
Fork this [repo](https://github.com/neilotoole/sq), e.g. to  `https://github.com/ksoze/sq`.

Clone the forked repo and set the `upstream` remote:

```
mkdir -p $GOPATH/src/github.com/neilotoole
cd $GOPATH/src/github.com/neilotoole
git clone https://github.com/ksoze/sq.git
cd ./sq
git remote add upstream https://github.com/neilotoole/sq.git
# verify that the remote was set
git remote -v
```
	
### Make
From  `$GOPATH/src/github.com/neilotoole/sq`, run `./test.sh`. This will run `make test`
inside a docker container.

For developing locally, this sequence should get you started:

```
make install-go-tools
make start-test-containers
make test
make install
make smoke
make dist
```
	
Note that running these steps may take some time (in particular due the use of
Cgo and cross-compiling distributables). Try `sq ls`. Note that by default `sq` uses `~/.sq/sq.yml` as
its config store, and outputs debug logs to `~/.sq/sq.log`.


Assuming the test containers are running (`make start-test-containers`), this workflow is suggested:

- Make your changes
- Run `make test && make install && make smoke`


## Contributing

When your changes are ready and tested, run `make dist` and a final `make smoke`.
Push the changes to your own fork, and then open a PR against the upstream repo. The PR should include a link
to the GitHub issue(s) that it addresses, and it must include the output of `make smoke`.