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


## Development

These steps are for Mac OS X (tested on El Capitan `10.11.16`). These steps assume
that your username is `ksoze`.

- Install [brew](http://brew.sh/), [Xcode](https://itunes.apple.com/us/app/xcode/id497799835?mt=12), and `git` (`brew install git`).
- Install and configure `go`:
	- `brew install go 1.7.1`
	- Set up your `$GOPATH`. Example: create a dir `~/go`, and in your `~/.bash_rc`, add the following line:
	
		```bash
		export GOPATH=/Users/ksoze/go
		```
	- Execute `src ~/.bash_rc`.
	- Verify your `go` setup:
	
		```
		> echo $GOPATH
		/Users/ksoze/nd/go
		> go version
		go version go1.7.1 darwin/amd64
		```
- `sq` is currently split into two repos: this repo, and a second repo that contains
 hacks of go's `database/sql` package and hacks of database drivers. The hacks are
 because go's SQL API doesn't expose all the data that `sq` needs. This is supposed
 to be fixed in the upcoming go `1.8` release. See this [issue](https://github.com/golang/go/issues/16652).
 Note that these repos are still private, so you need to be added as a collaborator
 on both repos.
- Login to [GitHub](https://github.com).
- Go to the https://github.com/neilotoole/sq-driver repo, and fork it.
	You'll now have your own fork, e.g. `https://github.com/ksoze/sq-driver`.
- Do the same for this repo, and you'll have another fork: `https://github.com/ksoze/sq`
- Now we'll need to clone those repos and set the `upstream` remotes:

	```
	> mkdir -p $GOPATH/src/github.com/neilotoole
	> cd $GOPATH/src/github.com/neilotoole
	> git clone https://github.com/ksoze/sq-driver.git
	> git clone https://github.com/ksoze/sq.git
	> cd sq-driver
	> git remote add upstream https://github.com/neilotoole/sq-driver.git
	> # verify that the remote was set
	> git remote -v
	> cd ../sq
	> git remote add upstream https://github.com/neilotoole/sq.git
	> # verify that the remote was set
	> git remote -v
	```
- Theoretically you should be good to go. From the `sq` directory:

	```
	> make install
	```
- That should be it.

