package dialect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsQueryString validates SQL type detection.
func TestIsQueryString(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		// === Query statements (should return true) ===
		{"simple select", "SELECT * FROM users", true},
		{"select with whitespace", "  SELECT 1", true},
		{"select with tab", "\tSELECT 1", true},
		{"lowercase select", "select * from users", true},
		{"with clause", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"show tables", "SHOW TABLES", true},
		{"show databases", "SHOW DATABASES", true},
		{"describe", "DESCRIBE users", true},
		{"desc", "DESC users", true},
		{"explain", "EXPLAIN SELECT * FROM users", true},
		{"explain analyze", "EXPLAIN ANALYZE SELECT * FROM users", true},
		{"select only", "SELECT", true},
		{"show only", "SHOW", true},

		// With comments
		{"single line comment then select", "-- comment\nSELECT 1", true},
		{"block comment then select", "/* block comment */ SELECT 1", true},
		{"multiple comments then select", "-- line 1\n-- line 2\nSELECT 1", true},
		{"comment with mixed case", "/* Comment */ select 1", true},

		// === DDL/DML statements (should return false) ===
		{"create table", "CREATE TABLE users (id INT)", false},
		{"create table if not exists", "CREATE TABLE IF NOT EXISTS users (id INT)", false},
		{"insert", "INSERT INTO users VALUES (1)", false},
		{"insert select", "INSERT INTO users SELECT * FROM temp", false},
		{"update", "UPDATE users SET name = 'test'", false},
		{"delete", "DELETE FROM users WHERE id = 1", false},
		{"drop table", "DROP TABLE users", false},
		{"drop table if exists", "DROP TABLE IF EXISTS users", false},
		{"alter table", "ALTER TABLE users ADD COLUMN age INT", false},
		{"truncate", "TRUNCATE TABLE users", false},
		{"create index", "CREATE INDEX idx ON users(id)", false},
		{"drop index", "DROP INDEX idx", false},
		{"create database", "CREATE DATABASE testdb", false},
		{"drop database", "DROP DATABASE testdb", false},

		// With whitespace
		{"create with leading space", "  CREATE TABLE users (id INT)", false},
		{"insert with leading tab", "\tINSERT INTO users VALUES (1)", false},

		// With comments
		{"create with comment", "-- comment\nCREATE TABLE users (id INT)", false},
		{"insert with comment", "/* comment */ INSERT INTO users VALUES (1)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := isQueryString(tt.sql)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result, "isQueryString(%q)", tt.sql)
		})
	}
}

// TestIsQueryString_Errors tests error cases.
func TestIsQueryString_Errors(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr string
	}{
		{"empty string", "", "empty SQL string"},
		{"only whitespace", "   ", "empty SQL string"},
		{"only line comment", "-- just a comment", "SQL string contains only comments"},
		{"only block comment", "/* just a comment */", "SQL string contains only comments"},
		{"multiple comments only", "-- line 1\n-- line 2\n/* block */", "SQL string contains only comments"},
		{"unclosed block comment", "/* unclosed comment", "SQL string contains unclosed block comment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := isQueryString(tt.sql)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestIsQueryString_RealWorld tests with real-world SQL examples.
func TestIsQueryString_RealWorld(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected bool
		reason   string
	}{
		{
			name:     "complex select",
			sql:      "SELECT u.id, u.name, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id",
			expected: true,
			reason:   "Complex SELECT with JOINs should be detected as query",
		},
		{
			name: "multi-line insert",
			sql: "INSERT INTO users (id, name, email)\nVALUES \n  " +
				"(1, 'Alice', 'alice@example.com'),\n  (2, 'Bob', 'bob@example.com')",
			expected: false,
			reason:   "Multi-line INSERT should be detected as statement",
		},
		{
			name: "create table with constraints",
			sql: "CREATE TABLE users (id SERIAL PRIMARY KEY, " +
				"name VARCHAR(100) NOT NULL, created_at TIMESTAMP DEFAULT NOW())",
			expected: false,
			reason:   "CREATE TABLE with constraints should be detected as statement",
		},
		{
			name: "update with subquery",
			sql: "UPDATE users SET status = 'active' " +
				"WHERE id IN (SELECT user_id FROM logins WHERE login_date > '2024-01-01')",
			expected: false,
			reason:   "UPDATE (even with subquery) should be detected as statement",
		},
		{
			name:     "delete with join",
			sql:      "DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE status = 'deleted')",
			expected: false,
			reason:   "DELETE (even with subquery) should be detected as statement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := isQueryString(tt.sql)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result, "Reason: %s", tt.reason)
		})
	}
}

// TestDefaultExecModeFor validates DefaultExecModeFor returns the correct ExecMode.
func TestDefaultExecModeFor(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected ExecMode
	}{
		{"select", "SELECT * FROM users", ExecModeQuery},
		{"with", "WITH cte AS (SELECT 1) SELECT * FROM cte", ExecModeQuery},
		{"show", "SHOW TABLES", ExecModeQuery},
		{"create", "CREATE TABLE users (id INT)", ExecModeExec},
		{"insert", "INSERT INTO users VALUES (1)", ExecModeExec},
		{"update", "UPDATE users SET name = 'test'", ExecModeExec},
		{"delete", "DELETE FROM users", ExecModeExec},
		{"drop", "DROP TABLE users", ExecModeExec},
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
		{"only comment", "-- just a comment", "SQL string contains only comments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DefaultExecModeFor(tt.sql)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
