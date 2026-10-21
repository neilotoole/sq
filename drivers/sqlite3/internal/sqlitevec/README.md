# sqlitevec

Vendored [sqlite-vec](https://github.com/vlasky/sqlite-vec) C source, compiled into sq via cgo and
registered with SQLite using `sqlite3_auto_extension`. Do not edit these files by hand;
run `scripts/update-sqlite-vec.sh <commit>` instead.

- Repo: https://github.com/vlasky/sqlite-vec
- Version: v0.2.7-alpha
- Commit: 44e64405a62abe9bd06bdf69145150639bc89add
- mattn/go-sqlite3: v1.14.52 (SQLite 3.53.4)

`sqlite-vec.c` carries one local patch (applied by the script) in the NEON int8 L1 code:
explicit `vreinterpretq` casts replace implicit signed/unsigned vector conversions that gcc
rejects (clang accepts them). The sha256 below is for the patched file.

`sqlite3.h` is mattn's `sqlite3-binding.h`. When mattn is bumped, rerun the script so the
header matches the SQLite that is linked.

| File           | sha256                                                           |
| -------------- | ---------------------------------------------------------------- |
| sqlite-vec.c   | eb680d1265d57b22b888e658b0b11cfc5581d047f2cc448320a16fc9b29c04d6 |
| sqlite-vec.h   | e81178b2154bb81566f16dc6822fe9f2f8a4e50246bf65b2b93bea41600817bc |
| sqlite3.h      | 4e7d1523cf95991f7e4c08c576e2232e063da6f10067e5c03c9bf9f904b1cf5f |
| LICENSE-MIT    | 6ce72bbe12d975bd5286e5ab0a064c069693300c47bccbc57bec18485f1621ea |
| LICENSE-APACHE | a38070a94d4afd9cd710e3ce67bd1de78097cfe1784c1f0109ac95d3c196bfdc |
