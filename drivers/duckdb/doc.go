// Package duckdb implements the sq driver for DuckDB.
// The backing SQL driver is github.com/duckdb/duckdb-go/v2.
//
// Concurrency caveat: DuckDB is single-writer per file. Opening the same
// .duckdb file from two sq processes will fail with a lock error, unlike
// SQLite WAL mode which permits concurrent readers.
package duckdb
