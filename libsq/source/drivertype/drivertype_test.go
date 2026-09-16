package drivertype_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// typeCases is the single source of truth for the drivertype tests below. It
// must have an entry for every constant declared in drivertype.go;
// TestType_Coverage enforces that.
//
// The string values are load-bearing: they're what sq driver ls prints, the
// connection URL scheme, and the sakiladb/{driver} image name. A rename is a
// breaking change, so these assertions exist to make one deliberate.
var typeCases = []struct {
	// name is the Go identifier, e.g. "SQLite".
	name string

	// typ is the constant itself.
	typ drivertype.Type

	// want is the constant's underlying string value, e.g. "sqlite3".
	want string
}{
	{"None", drivertype.None, ""},
	{"SQLite", drivertype.SQLite, "sqlite3"},
	{"Rqlite", drivertype.Rqlite, "rqlite"},
	{"DuckDB", drivertype.DuckDB, "duckdb"},
	{"Pg", drivertype.Pg, "postgres"},
	{"MSSQL", drivertype.MSSQL, "sqlserver"},
	{"MySQL", drivertype.MySQL, "mysql"},
	{"ClickHouse", drivertype.ClickHouse, "clickhouse"},
	{"Oracle", drivertype.Oracle, "oracle"},
	{"CSV", drivertype.CSV, "csv"},
	{"TSV", drivertype.TSV, "tsv"},
	{"JSON", drivertype.JSON, "json"},
	{"JSONA", drivertype.JSONA, "jsona"},
	{"JSONL", drivertype.JSONL, "jsonl"},
	{"XLSX", drivertype.XLSX, "xlsx"},
}

func TestType_String(t *testing.T) {
	for _, tc := range typeCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.typ.String())
		})
	}
}

func TestType_Constants(t *testing.T) {
	// Verify that each constant has the expected underlying string value.
	// This ensures the constants don't accidentally change.
	for _, tc := range typeCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, drivertype.Type(tc.want), tc.typ)
		})
	}
}

// TestType_Coverage verifies that typeCases matches the constants declared in
// drivertype.go, name for name and value for value. Without it, a new driver
// type can be added and silently go unasserted, which is what happened to
// DuckDB and Oracle (see #1201).
func TestType_Coverage(t *testing.T) {
	declared := parseTypeConsts(t, srcFilename)
	require.NotEmpty(t, declared, "found no Type constants in %s", srcFilename)

	tabled := make(map[string]string, len(typeCases))
	for _, tc := range typeCases {
		require.NotContains(t, tabled, tc.name, "typeCases has a duplicate entry for %s", tc.name)
		tabled[tc.name] = tc.want
	}

	for name, value := range declared {
		want, ok := tabled[name]
		require.True(t, ok, "drivertype.%s is declared in %s but has no typeCases entry", name, srcFilename)
		require.Equal(t, value, want, "typeCases entry for drivertype.%s doesn't match its declared value", name)
	}

	for name := range tabled {
		require.Contains(t, declared, name,
			"typeCases has an entry for drivertype.%s, which is no longer declared in %s", name, srcFilename)
	}
}

func TestType_Equality(t *testing.T) {
	// Verify that Type can be compared for equality.
	typ := drivertype.Pg
	require.True(t, typ == drivertype.Pg)
	require.False(t, typ == drivertype.MySQL)
	require.True(t, drivertype.None == drivertype.Type(""))
}

func TestType_ZeroValue(t *testing.T) {
	var typ drivertype.Type
	require.Equal(t, drivertype.None, typ)
	require.Equal(t, "", typ.String())
}

// srcFilename is the file that declares the Type constants. Tests run with the
// package dir as their working dir, so the bare filename resolves.
const srcFilename = "drivertype.go"

// parseTypeConsts returns the identifier and underlying string value of every
// Type constant declared in filename, e.g. {"SQLite": "sqlite3"}. It reads the
// source because Go constants have no runtime representation to reflect over.
func parseTypeConsts(t *testing.T, filename string) map[string]string {
	t.Helper()

	f, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	require.NoError(t, err)

	consts := map[string]string{}
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			require.True(t, ok, "%s: unexpected const spec %T", filename, spec)
			require.Len(t, valueSpec.Values, len(valueSpec.Names),
				"%s: const decl without an explicit value; teach parseTypeConsts about it", filename)

			for i, ident := range valueSpec.Names {
				value, ok := typeConstValue(valueSpec, valueSpec.Values[i])
				require.True(t, ok,
					"%s: can't extract a Type value from const %s; teach parseTypeConsts about its declaration form",
					filename, ident.Name)
				consts[ident.Name] = value
			}
		}
	}

	return consts
}

// typeConstValue returns the string value of a Type constant declaration,
// accepting both the conversion form (X = Type("x")) and the typed form
// (X Type = "x"). It returns false for any other form.
func typeConstValue(spec *ast.ValueSpec, value ast.Expr) (string, bool) {
	switch value := value.(type) {
	case *ast.CallExpr:
		// X = Type("x")
		fn, ok := value.Fun.(*ast.Ident)
		if !ok || fn.Name != "Type" || len(value.Args) != 1 {
			return "", false
		}
		return stringLit(value.Args[0])
	case *ast.BasicLit:
		// X Type = "x"
		ident, ok := spec.Type.(*ast.Ident)
		if !ok || ident.Name != "Type" {
			return "", false
		}
		return stringLit(value)
	default:
		return "", false
	}
}

// stringLit returns the unquoted value of a string literal expression.
func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	return s, true
}
