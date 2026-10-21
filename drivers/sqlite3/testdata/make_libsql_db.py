"""Creates libsql.db, a SQLite database with libSQL vector columns and indexes.

Usage:
    uv run --python 3.12 --with libsql python -I make_libsql_db.py libsql.db

Python 3.12 is pinned because the libsql wheel fails to build on newer Pythons.
"""

import os
import sys

import libsql


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit("usage: make_libsql_db.py <out.db>")
    out = sys.argv[1]
    if os.path.exists(out):
        os.remove(out)

    db = libsql.connect(out)
    db.execute(
        "CREATE TABLE t(id INTEGER PRIMARY KEY, e32 F32_BLOB(4), e64 F64_BLOB(4), "
        "e16 F16_BLOB(4), eb16 FB16_BLOB(4), e8 F8_BLOB(4), e1 F1BIT_BLOB(12), "
        "f FLOAT32(3))"
    )
    db.execute(
        "INSERT INTO t(id, e32, e64, e16, eb16, e8, e1, f) VALUES (1, "
        "vector32('[1,2,-3.5,0.25]'), vector64('[1,2,-3.5,0.25]'), "
        "vector16('[1,2,-3.5,0.25]'), vectorb16('[1,2,-3.5,0.25]'), "
        "vector8('[1,2,-3.5,0.25]'), "
        "vector1bit('[1,-1,1,1,-1,-1,-1,-1,1,1,1,-1]'), "
        "vector32('[1,2,3]'))"
    )
    db.execute("INSERT INTO t(id, e32) VALUES (2, '[0.5,0.5,0.5,0.5]')")
    db.execute("CREATE INDEX t_idx ON t(libsql_vector_idx(e32, 'metric=cosine'))")

    db.execute("CREATE TABLE u(id INTEGER PRIMARY KEY, e F8_BLOB(4))")
    db.execute("INSERT INTO u(id, e) VALUES (1, vector8('[1,2,-3.5,0.25]'))")
    db.execute(
        "CREATE INDEX u_idx ON u(libsql_vector_idx(e, "
        "'compress_neighbors=float1bit', 'max_neighbors=8'))"
    )

    # libSQL accepts whitespace and comments between the function name and
    # its parenthesis. The note and x columns make the index the cheaper
    # covering index for stock SQLite's planner.
    db.execute(
        "CREATE TABLE w(id INTEGER PRIMARY KEY, e F32_BLOB(2), note TEXT, x TEXT)"
    )
    db.execute(
        "INSERT INTO w(id, e) VALUES (1, vector32('[1,2]')), (2, vector32('[3,4]'))"
    )
    db.execute("CREATE INDEX w_idx ON w(libsql_vector_idx /* spaced */ (e, 'metric=l2'))")
    db.commit()


if __name__ == "__main__":
    main()
