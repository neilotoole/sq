# sqlitevec

Vendored [sqlite-vec](https://github.com/vlasky/sqlite-vec) C source, compiled into sq via cgo and
registered with SQLite using `sqlite3_auto_extension`. Do not edit these files by hand;
run `scripts/update-sqlite-vec.sh <commit>` instead.

- Repo: https://github.com/vlasky/sqlite-vec
- Version: v0.2.7-alpha
- Commit: 44e64405a62abe9bd06bdf69145150639bc89add
- mattn/go-sqlite3: v1.14.52 (SQLite 3.53.4)

NEON is enabled only on darwin/arm64 (see `sqlitevec.go`): gcc rejects the fork's NEON int8
code at this commit (implicit signed/unsigned vector conversions), so linux/arm64 uses the
scalar path. The vendored files are verbatim.

`sqlite3.h` is mattn's `sqlite3-binding.h`. When mattn is bumped, rerun the script so the
header matches the SQLite that is linked.

The script fetches each file by full commit SHA and records the sha256 sums below. It doesn't
verify them against a published list. Run `make fmt` after the script, to format this file.

| File           | sha256                                                           |
| -------------- | ---------------------------------------------------------------- |
| sqlite-vec.c   | 30a3958c1859dba90db41d2f860ac68dad71cf3d37325669669e2903af1232a8 |
| sqlite-vec.h   | e81178b2154bb81566f16dc6822fe9f2f8a4e50246bf65b2b93bea41600817bc |
| sqlite3.h      | 4e7d1523cf95991f7e4c08c576e2232e063da6f10067e5c03c9bf9f904b1cf5f |
| LICENSE-MIT    | 6ce72bbe12d975bd5286e5ab0a064c069693300c47bccbc57bec18485f1621ea |
| LICENSE-APACHE | a38070a94d4afd9cd710e3ce67bd1de78097cfe1784c1f0109ac95d3c196bfdc |
