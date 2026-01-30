package dialect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultExecModeFor validates DefaultExecModeFor returns the correct ExecMode.
func TestDefaultExecModeFor(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected ExecMode
	}{
		// === Query statements (should return ExecModeQuery) ===
		{"simple select", "SELECT * FROM users", ExecModeQuery},
		{"select with whitespace", "  SELECT 1", ExecModeQuery},
		{"select with tab", "\tSELECT 1", ExecModeQuery},
		{"lowercase select", "select * from users", ExecModeQuery},
		{"with clause", "WITH cte AS (SELECT 1) SELECT * FROM cte", ExecModeQuery},
		{"show tables", "SHOW TABLES", ExecModeQuery},
		{"show databases", "SHOW DATABASES", ExecModeQuery},
		{"describe", "DESCRIBE users", ExecModeQuery},
		{"desc", "DESC users", ExecModeQuery},
		{"explain", "EXPLAIN SELECT * FROM users", ExecModeQuery},
		{"explain analyze", "EXPLAIN ANALYZE SELECT * FROM users", ExecModeQuery},
		{"table shorthand", "TABLE users", ExecModeQuery},

		// Bare keywords (edge cases)
		{"select only", "SELECT", ExecModeQuery},
		{"show only", "SHOW", ExecModeQuery},
		{"table only", "TABLE", ExecModeQuery},
		{"values only", "VALUES", ExecModeQuery},

		// SELECT with expression-starting characters (no whitespace)
		// See: https://github.com/neilotoole/sq/issues/532
		{"select star no space", "SELECT*FROM users", ExecModeQuery},
		{"select paren", "SELECT(1)", ExecModeQuery},
		{"select paren col", "SELECT(col)FROM t", ExecModeQuery},
		{"select negative", "SELECT-1", ExecModeQuery},
		{"select negative float", "SELECT-5.0", ExecModeQuery},
		{"select positive", "SELECT+1", ExecModeQuery},
		{"select string literal", "SELECT'hello'", ExecModeQuery},
		{"select double quote", `SELECT"col"FROM t`, ExecModeQuery},
		{"select backtick", "SELECT`col`FROM t", ExecModeQuery},
		{"select bracket", "SELECT[col]FROM t", ExecModeQuery},
		{"select variable", "SELECT@var", ExecModeQuery},
		{"select system var", "SELECT@@version", ExecModeQuery},

		// VALUES constructor (PostgreSQL, MySQL 8+, SQLite)
		{"values constructor", "VALUES (1),(2),(3)", ExecModeQuery},
		{"values with spaces", "VALUES   (1, 'a'), (2, 'b')", ExecModeQuery},
		{"values newline paren", "VALUES\n(1)", ExecModeQuery},

		// With comments
		{"single line comment then select", "-- comment\nSELECT 1", ExecModeQuery},
		{"block comment then select", "/* block comment */ SELECT 1", ExecModeQuery},
		{"multiple comments then select", "-- line 1\n-- line 2\nSELECT 1", ExecModeQuery},
		{"comment with mixed case", "/* Comment */ select 1", ExecModeQuery},

		// === DDL/DML statements (should return ExecModeExec) ===
		{"create table", "CREATE TABLE users (id INT)", ExecModeExec},
		{"create table if not exists", "CREATE TABLE IF NOT EXISTS users (id INT)", ExecModeExec},
		{"insert", "INSERT INTO users VALUES (1)", ExecModeExec},
		{"insert select", "INSERT INTO users SELECT * FROM temp", ExecModeExec},
		{"update", "UPDATE users SET name = 'test'", ExecModeExec},
		{"delete", "DELETE FROM users WHERE id = 1", ExecModeExec},
		{"drop table", "DROP TABLE users", ExecModeExec},
		{"drop table if exists", "DROP TABLE IF EXISTS users", ExecModeExec},
		{"alter table", "ALTER TABLE users ADD COLUMN age INT", ExecModeExec},
		{"truncate", "TRUNCATE TABLE users", ExecModeExec},
		{"create index", "CREATE INDEX idx ON users(id)", ExecModeExec},
		{"drop index", "DROP INDEX idx", ExecModeExec},
		{"create database", "CREATE DATABASE testdb", ExecModeExec},
		{"drop database", "DROP DATABASE testdb", ExecModeExec},

		// With whitespace
		{"create with leading space", "  CREATE TABLE users (id INT)", ExecModeExec},
		{"insert with leading tab", "\tINSERT INTO users VALUES (1)", ExecModeExec},

		// With comments
		{"create with comment", "-- comment\nCREATE TABLE users (id INT)", ExecModeExec},
		{"insert with comment", "/* comment */ INSERT INTO users VALUES (1)", ExecModeExec},

		// Real-world examples
		{
			"complex select",
			"SELECT u.id, u.name, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id",
			ExecModeQuery,
		},
		{
			"multi-line insert",
			"INSERT INTO users (id, name, email)\nVALUES \n  " +
				"(1, 'Alice', 'alice@example.com'),\n  (2, 'Bob', 'bob@example.com')",
			ExecModeExec,
		},
		{
			"create table with constraints",
			"CREATE TABLE users (id SERIAL PRIMARY KEY, " +
				"name VARCHAR(100) NOT NULL, created_at TIMESTAMP DEFAULT NOW())",
			ExecModeExec,
		},
		{
			"update with subquery",
			"UPDATE users SET status = 'active' " +
				"WHERE id IN (SELECT user_id FROM logins WHERE login_date > '2024-01-01')",
			ExecModeExec,
		},
		{
			"delete with join",
			"DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE status = 'deleted')",
			ExecModeExec,
		},
		{
			// See: https://github.com/neilotoole/sq/issues/532
			// Note that "select*from orders" is confirmed to work on SQLite, Pg, and
			// MySQL. Thus, "sq sql" should support it.
			"select star without whitespace",
			"select*from orders",
			ExecModeQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DefaultExecModeFor(tt.sql)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestDefaultExecModeFor_Errors tests error cases.
func TestDefaultExecModeFor_Errors(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr string
	}{
		{"empty string", "", "empty SQL string"},
		{"only whitespace", "   ", "empty SQL string"},
		{"only line comment", "-- just a comment", "only comments"},
		{"only block comment", "/* just a comment */", "only comments"},
		{"multiple comments only", "-- line 1\n-- line 2\n/* block */", "only comments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DefaultExecModeFor(tt.sql)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
