package typetest

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// This file generates the testdata/coltypes.ddl file described in the
// package doc's Generated DDL section.

// EnvUpdateDDL is the envar that makes CheckDDL rewrite the DDL file instead
// of comparing with it. An envar rather than a -update flag, since a flag
// registered here would collide with a driver package's own.
const EnvUpdateDDL = "SQ_TYPETEST_UPDATE_DDL"

// ddlServerVer is the server version the DDL file is rendered for: newer
// than any real one, so every MinVer-gated case is included.
const ddlServerVer = "v999.0.0"

// LiteralFunc renders a Go input value v, for column col, as a SQL literal in
// the driver's dialect. It returns ok false for a value it leaves to
// DefaultLiteral, so a driver only handles what its dialect writes
// differently: typically bytes, and sometimes booleans or times. The column
// is there for dialects whose literal depends on the target type, such as a
// timestamp with time zone needing an explicit offset.
type LiteralFunc func(col Column, v any) (lit string, ok bool)

// DefaultLiteral renders v as a SQL literal in the form most dialects
// accept: NULL for nil, a SQL input as is, a quoted string with quotes
// doubled, integers, floats and decimals as numbers, TRUE and FALSE, and a
// time as a quoted UTC date ('2017-07-07') if it's midnight, or date and time
// ('2017-07-07 07:07:07.123') otherwise. Bytes have no portable literal, so
// non-nil bytes are an error: the driver's LiteralFunc must render them.
func DefaultLiteral(v any) (string, error) {
	switch v := v.(type) {
	case nil:
		return "NULL", nil
	case []byte:
		if v == nil {
			return "NULL", nil
		}
		return "", errz.Errorf("typetest: no default SQL literal for bytes %#v: the driver's Config.Literal must "+
			"render them", v)
	case SQL:
		return string(v), nil
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'", nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", errz.Errorf("typetest: no default SQL literal for %v: the driver's Config.Literal must "+
				"render it", v)
		}
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case bool:
		if v {
			return "TRUE", nil
		}
		return "FALSE", nil
	case decimal.Decimal:
		return v.String(), nil
	case time.Time:
		v = v.UTC()
		if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
			return "'" + v.Format(time.DateOnly) + "'", nil
		}
		return "'" + v.Format("2006-01-02 15:04:05.999999999") + "'", nil
	default:
		return "", errz.Errorf("typetest: no default SQL literal for %T: the driver's Config.Literal must render it",
			v)
	}
}

// HasTimeZone reports whether the column type decl stores a time zone or
// offset: TIMESTAMPTZ, TIMETZ, ... WITH TIME ZONE, ... WITH LOCAL TIME ZONE,
// or DATETIMEOFFSET, in any case and with or without a precision. A literal
// for such a column needs an explicit offset: without one, the database reads
// it in the session's time zone. "... WITHOUT TIME ZONE" doesn't match.
func HasTimeZone(decl string) bool {
	decl = strings.ToUpper(decl)
	switch {
	case strings.Contains(decl, "WITHOUT TIME ZONE"):
		return false
	case strings.Contains(decl, "TIMESTAMPTZ"),
		strings.Contains(decl, "TIMETZ"),
		strings.Contains(decl, "TIME ZONE"),
		strings.Contains(decl, "DATETIMEOFFSET"):
		return true
	default:
		return false
	}
}

// CheckTimeZone is the time zone for Config.DDLCheckSession to set: one
// far from UTC, at a fractional offset (+05:45) with no daylight saving
// time, so a literal read in it rather than in UTC is plainly wrong.
// CheckTimeZoneOffset is the same zone as an offset, for a database without
// named zones.
const (
	CheckTimeZone       = "Asia/Kathmandu"
	CheckTimeZoneOffset = "+05:45"
)

// TimestampTZLiteral renders t as a quoted timestamp literal with an
// explicit UTC offset, e.g. '2017-07-07 07:07:07.123+00:00', for a column
// that stores a time zone: without the offset, the database would read the
// literal in the session's time zone.
func TimestampTZLiteral(t time.Time) string {
	return "'" + t.UTC().Format("2006-01-02 15:04:05.999999999") + "+00:00'"
}

// RenderLiteral renders v, for col, as the generated DDL file writes it:
// with cfg.Literal, falling back to DefaultLiteral.
func RenderLiteral(cfg Config, col Column, v any) (string, error) {
	if cfg.Literal != nil {
		if lit, ok := cfg.Literal(col, v); ok {
			return lit, nil
		}
	}
	return DefaultLiteral(v)
}

// RenderStatements returns the statements that create table tbl for cfg's
// cases on a server at serverVer, and insert its three rows: the same table
// Run creates, and one INSERT per row with each input written as a SQL
// literal (see Config.Literal). They're formatted for reading, one column or
// value per line, each value preceded by a comment naming its column, and
// have no terminating semicolon.
func RenderStatements(cfg Config, tbl, serverVer string) ([]string, error) {
	cols, err := Resolve(cfg.Cases, serverVer)
	if err != nil {
		return nil, err
	}

	colDecl := cfg.ColumnDecl
	if colDecl == nil {
		colDecl = DefaultColumnDecl
	}

	var sb strings.Builder
	sb.WriteString("CREATE TABLE " + tbl + " (\n  " + rowIDCol + " " + cfg.RowIDDecl)
	for _, col := range cols {
		sb.WriteString(",\n  " + col.Name + " " + colDecl(col.Case.Decl, col.Variant))
	}
	sb.WriteString("\n)")
	if cfg.TableSuffix != "" {
		sb.WriteString(" " + cfg.TableSuffix)
	}
	stmts := append(slices.Clone(cfg.DDLPrelude), sb.String())

	// Each value's comment goes on the line before it, not after it: on
	// ClickHouse 26.5, an unquoted Float32 literal followed by a comment
	// read back as 0.
	for row := range 3 {
		sb.Reset()
		sb.WriteString("INSERT INTO " + tbl + " VALUES (\n  -- " + rowIDCol + "\n  " + strconv.Itoa(row+1))
		for _, col := range cols {
			lit, err := RenderLiteral(cfg, col, col.In[row])
			if err != nil {
				return nil, errz.Wrapf(err, "typetest: column %s (%s), row %d", col.Name, col.Case.Decl, row+1)
			}
			sb.WriteString(",\n  -- " + col.Name + "\n  " + lit)
		}
		sb.WriteString("\n)")
		stmts = append(stmts, sb.String())
	}
	return stmts, nil
}

// DDLTable is one table of a DDL file: a name and the Config that defines it.
type DDLTable struct {
	Name   string
	Config Config
}

// RenderDDL renders the DDL file for tables, rendered for the newest server
// version so that every case is included: header, then each table's
// statements, each terminated by a semicolon.
func RenderDDL(header string, tables ...DDLTable) (string, error) {
	var sb strings.Builder
	sb.WriteString(header)
	for _, tbl := range tables {
		stmts, err := RenderStatements(tbl.Config, tbl.Name, ddlServerVer)
		if err != nil {
			return "", err
		}
		for _, stmt := range stmts {
			sb.WriteString("\n" + stmt + ";\n")
		}
	}
	return sb.String(), nil
}

// CompareDDL compares the file at path with want. With update true, it
// writes want to the file instead.
func CompareDDL(path, want string, update bool) error {
	if update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return errz.Wrapf(err, "typetest: create directory for %s", path)
		}
		if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
			return errz.Wrapf(err, "typetest: write %s", path)
		}
		return nil
	}

	got, err := os.ReadFile(path)
	if err != nil {
		return errz.Wrapf(err, "typetest: read %s: regenerate it with %s=1", path, EnvUpdateDDL)
	}
	if string(got) == want {
		return nil
	}

	gotLines, wantLines := strings.Split(string(got), "\n"), strings.Split(want, "\n")
	i := 0
	for i < len(gotLines) && i < len(wantLines) && gotLines[i] == wantLines[i] {
		i++
	}
	var gotLine, wantLine string
	if i < len(gotLines) {
		gotLine = gotLines[i]
	}
	if i < len(wantLines) {
		wantLine = wantLines[i]
	}
	return errz.Errorf("typetest: %s is out of date with the case table: line %d is %q in the file, %q from "+
		"the case table: regenerate it with %s=1 and review the diff", path, i+1, gotLine, wantLine, EnvUpdateDDL)
}

// CheckDDL renders the DDL file for tables and compares it with the file at
// path, failing t if they differ. With EnvUpdateDDL set to true, it rewrites
// the file. server names the server version the file is for, e.g. "MySQL 9",
// for the file's header, which also names t's test and package as the way to
// regenerate the file.
func CheckDDL(t *testing.T, path, server string, tables ...DDLTable) {
	t.Helper()

	update, err := envBool(EnvUpdateDDL)
	require.NoError(t, err)

	wd, err := os.Getwd()
	require.NoError(t, err)
	pkgDir, err := moduleRelDir(wd)
	require.NoError(t, err)
	testName, _, _ := strings.Cut(t.Name(), "/")

	ddl, err := RenderDDL(ddlHeader(testName, pkgDir, server), tables...)
	require.NoError(t, err)
	require.NoError(t, CompareDDL(path, ddl, update))
}

// ddlHeader returns the DDL file's header comment: what the file is, for
// the server version named by server, and how to regenerate it, by running
// testName in the package at pkgDir, relative to the module root.
func ddlHeader(testName, pkgDir, server string) string {
	return fmt.Sprintf(`-- Code generated by %s; DO NOT EDIT.
--
-- The tables the column-type tests create for %s, with the three
-- rows each inserts. Load it into a database to query the tables or
-- inspect them with sq. See the testh/typetest package doc.
--
-- Regenerate it after changing a case table:
--
--   %s=1 go test -run '^%s$' ./%s/
`, testName, server, EnvUpdateDDL, testName, pkgDir)
}

// moduleRelDir returns dir relative to the root of the Go module that
// contains it, the nearest directory at or above dir with a go.mod, as a
// slash-separated path.
func moduleRelDir(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", errz.Err(err)
	}
	for root := dir; ; {
		if _, err = os.Stat(filepath.Join(root, "go.mod")); err == nil {
			rel, err := filepath.Rel(root, dir)
			if err != nil {
				return "", errz.Err(err)
			}
			return filepath.ToSlash(rel), nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", errz.Errorf("typetest: no go.mod at or above %s", dir)
		}
		root = parent
	}
}

// envBool reports whether the envar name is set to true. Unset or empty is
// false; any other value must parse as a boolean (1, true, 0, false, ...).
func envBool(name string) (bool, error) {
	val := os.Getenv(name)
	if val == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false, errz.Errorf("typetest: envar %s=%q: must be a boolean such as 1 or 0", name, val)
	}
	return b, nil
}
