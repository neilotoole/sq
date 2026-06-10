package rqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3" // For TestWriteAtomic_DBTypeCheck.
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

// Exported for external_test consumers in drivers/rqlite/*_test.go.
var (
	KindFromDBTypeName = kindFromDBTypeName
	RTypeNullTime      = rtypeNullTime
)

func TestPlaceholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "(?)"},
		{numCols: 2, numRows: 1, want: "(?, ?)"},
		{numCols: 1, numRows: 2, want: "(?), (?)"},
		{numCols: 2, numRows: 2, want: "(?, ?), (?, ?)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}

func TestDsnFromLocation(t *testing.T) {
	testCases := []struct {
		loc     string
		want    string
		wantErr bool
	}{
		{loc: "", wantErr: true},
		{loc: "sqlite3://foo.db", wantErr: true},
		{loc: "http://host:4001", wantErr: true},
		{loc: Prefix + "host:4001", want: "http://host:4001"},
		{loc: Prefix + "user:pass@host:4001", want: "http://user:pass@host:4001"},
		{loc: Prefix + "host:4001?level=strong", want: "http://host:4001?level=strong"},
		{loc: PrefixSecure + "host:4001", want: "https://host:4001"},
		{loc: PrefixSecure + "user:pass@host:4001?level=none", want: "https://user:pass@host:4001?level=none"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			got, _, err := dsnFromLocation(tc.loc)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDBTypeForKind(t *testing.T) {
	testCases := map[kind.Kind]string{
		kind.Text:     "TEXT",
		kind.Int:      "INTEGER",
		kind.Float:    "REAL",
		kind.Bytes:    "BLOB",
		kind.Decimal:  "NUMERIC",
		kind.Bool:     "BOOLEAN",
		kind.Datetime: "DATETIME",
		kind.Date:     "DATE",
		kind.Time:     "TIME",
		kind.Unknown:  "TEXT",
		kind.Null:     "TEXT",
	}

	for knd, want := range testCases {
		t.Run(knd.String(), func(t *testing.T) {
			require.Equal(t, want, DBTypeForKind(knd))
		})
	}
}

func TestKindFromDBTypeName(t *testing.T) {
	ctx := context.Background()
	// The kind mapping mirrors SQLite affinity rules. These cases cover
	// the common direct matches, the parameterized-suffix stripping, and
	// the fallback affinity branches.
	testCases := map[string]kind.Kind{
		"INTEGER":      kind.Int,
		"BIGINT":       kind.Int,
		"TEXT":         kind.Text,
		"VARCHAR(45)":  kind.Text,
		"BLOB":         kind.Bytes,
		"DATETIME":     kind.Datetime,
		"TIMESTAMP":    kind.Datetime,
		"DATE":         kind.Date,
		"TIME":         kind.Time,
		"BOOLEAN":      kind.Bool,
		"NUMERIC":      kind.Decimal,
		"DECIMAL":      kind.Decimal,
		"REAL":         kind.Float,
		"FLOAT":        kind.Float,
		"INT2":         kind.Int,
		"MEDIUMINT":    kind.Int,
		"NCHAR":        kind.Text,
		"DOUBLE":       kind.Float,
		"someInteger":  kind.Int,  // affinity rule: contains "INT"
		"someText":     kind.Text, // affinity rule: contains "TEXT"
		"longCLOB":     kind.Text, // affinity rule: contains "CLOB"
		"weirdBLOBish": kind.Bytes,
	}
	for dbType, want := range testCases {
		t.Run(dbType, func(t *testing.T) {
			require.Equal(t, want, kindFromDBTypeName(ctx, "col", dbType, nil))
		})
	}
}

func TestBuildCreateTableStmt(t *testing.T) {
	tblDef := &schema.Table{
		Name:          "actor",
		PKColName:     "actor_id",
		AutoIncrement: true,
		Cols: []*schema.Column{
			{Name: "actor_id", Kind: kind.Int, NotNull: true},
			{Name: "first_name", Kind: kind.Text, NotNull: true, HasDefault: true},
			{Name: "last_name", Kind: kind.Text},
			{Name: "last_update", Kind: kind.Datetime, NotNull: true, HasDefault: true},
		},
	}

	got := buildCreateTableStmt(tblDef)

	require.Contains(t, got, `CREATE TABLE "actor"`)
	require.Contains(t, got, `"actor_id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL`)
	require.Contains(t, got, `"first_name" TEXT DEFAULT '' NOT NULL`)
	require.Contains(t, got, `"last_name" TEXT`)
	require.Contains(t, got, `"last_update" DATETIME DEFAULT '1970-01-01T00:00:00' NOT NULL`)
}

func TestBuildUpdateStmt(t *testing.T) {
	t.Run("with where", func(t *testing.T) {
		got, err := buildUpdateStmt("actor", []string{"first_name", "last_name"}, "actor_id = ?")
		require.NoError(t, err)
		require.Equal(t, `UPDATE "actor" SET "first_name" = ?, "last_name" = ? WHERE actor_id = ?`, got)
	})
	t.Run("no where", func(t *testing.T) {
		got, err := buildUpdateStmt("actor", []string{"first_name"}, "")
		require.NoError(t, err)
		require.Equal(t, `UPDATE "actor" SET "first_name" = ?`, got)
	})
	t.Run("no cols errors", func(t *testing.T) {
		_, err := buildUpdateStmt("actor", nil, "")
		require.ErrorContains(t, err, "no columns")
	})
}

func TestWriteAtomic_DBTypeCheck(t *testing.T) {
	ctx := context.Background()

	// Open an in-memory sqlite3 db just to obtain a real *sql.Tx for the
	// type-switch check. No network involved.
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	t.Run("rejects *sql.Tx", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback() })

		_, err = writeAtomic(ctx, tx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "*sql.Tx")
	})
}

func TestLocationWithDefaultPort(t *testing.T) {
	testCases := []struct {
		loc       string
		wantLoc   string
		wantAdded bool
		wantErr   bool
	}{
		{loc: "", wantErr: true},
		{loc: "://bad", wantErr: true},
		{loc: "rqlite://host:4001", wantLoc: "rqlite://host:4001", wantAdded: false},
		{loc: "rqlite://host", wantLoc: "rqlite://host:4001", wantAdded: true},
		{loc: "rqlites://host", wantLoc: "rqlites://host:4001", wantAdded: true},
		{loc: "rqlite://user:pass@host", wantLoc: "rqlite://user:pass@host:4001", wantAdded: true},
		{loc: "rqlite://host:9999", wantLoc: "rqlite://host:9999", wantAdded: false},
		{loc: "rqlite://host?level=strong", wantLoc: "rqlite://host:4001?level=strong", wantAdded: true},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			got, added, err := locationWithDefaultPort(tc.loc)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantLoc, got)
			require.Equal(t, tc.wantAdded, added)
		})
	}
}

// TestCoerceFloat64 covers the per-kind reshaping that
// newRecordFromScanRow applies to gorqlite's JSON-number float64
// returns. The Sakila-driven cross-driver tests exercise the kind.Int
// branch end-to-end, but they would still pass if the upstream Sakila
// image switched actor_id from NUMERIC to INTEGER. This table-driven
// case keeps direct coverage on the helper regardless of the upstream
// schema choice.
func TestCoerceFloat64(t *testing.T) {
	mkMeta := func(k kind.Kind) record.Meta {
		return record.Meta{record.NewFieldMeta(&record.ColumnTypeData{Name: "c", Kind: k}, "c")}
	}

	testCases := []struct {
		name string
		knd  kind.Kind
		in   float64
		want any
	}{
		{name: "int_whole", knd: kind.Int, in: 42, want: int64(42)},
		{name: "int_truncates_fraction", knd: kind.Int, in: 42.9, want: int64(42)},
		{name: "decimal_integer_demoted", knd: kind.Decimal, in: 42, want: int64(42)},
		{name: "decimal_fractional_preserved", knd: kind.Decimal, in: 19.99, want: decimal.NewFromFloat(19.99)},
		{name: "bool_zero_false", knd: kind.Bool, in: 0, want: false},
		{name: "bool_nonzero_true", knd: kind.Bool, in: 1, want: true},
		{name: "float_passthrough", knd: kind.Float, in: 3.14, want: 3.14},
		{name: "unknown_promotes_to_float", knd: kind.Unknown, in: 7.5, want: 7.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := coerceFloat64(mkMeta(tc.knd), 0, tc.in)
			if want, ok := tc.want.(decimal.Decimal); ok {
				gotDec, gotOK := got.(decimal.Decimal)
				require.True(t, gotOK, "expected decimal.Decimal, got %T", got)
				require.True(t, want.Equal(gotDec), "want %s, got %s", want, gotDec)
				return
			}
			require.Equal(t, tc.want, got)
		})
	}
}

// TestCoerceDecimal covers the whole-number demotion that pairs with
// coerceFloat64's kind.Decimal branch and the *decimal.NullDecimal /
// *decimal.Decimal scan cases in newRecordFromScanRow.
func TestCoerceDecimal(t *testing.T) {
	t.Run("integer_demoted_to_int64", func(t *testing.T) {
		got := coerceDecimal(decimal.NewFromInt(42))
		require.Equal(t, int64(42), got)
	})
	t.Run("fractional_passthrough", func(t *testing.T) {
		want := decimal.NewFromFloat(19.99)
		got := coerceDecimal(want)
		gotDec, ok := got.(decimal.Decimal)
		require.True(t, ok, "expected decimal.Decimal, got %T", got)
		require.True(t, want.Equal(gotDec))
	})
	t.Run("negative_integer_demoted", func(t *testing.T) {
		got := coerceDecimal(decimal.NewFromInt(-7))
		require.Equal(t, int64(-7), got)
	})
}

func TestBuildCreateTableStmt_ForeignKey(t *testing.T) {
	tblDef := &schema.Table{
		Name: "film_actor",
		Cols: []*schema.Column{
			{Name: "actor_id", Kind: kind.Int, ForeignKey: &schema.FKConstraint{
				RefTable: "actor", RefCol: "actor_id",
				// Empty OnDelete/OnUpdate to exercise the CASCADE default.
			}},
			{Name: "film_id", Kind: kind.Int, Unique: true, ForeignKey: &schema.FKConstraint{
				RefTable: "film", RefCol: "film_id",
				OnDelete: "RESTRICT", OnUpdate: "SET NULL",
			}},
		},
	}
	got := buildCreateTableStmt(tblDef)
	require.Contains(t, got, `CONSTRAINT "film_actor_actor_id_actor_actor_id_fk"`)
	require.Contains(t, got, `ON DELETE CASCADE ON UPDATE CASCADE`)
	require.Contains(t, got, `ON DELETE RESTRICT ON UPDATE SET NULL`)
	require.Contains(t, got, `"film_id" INTEGER UNIQUE`)
}

func Test_maybeWarnLocalhostDiscovery(t *testing.T) {
	testCases := []struct {
		name    string
		loc     string
		wantLog bool
	}{
		{name: "localhost", loc: "rqlite://localhost:4001", wantLog: true},
		{name: "localhost upper", loc: "rqlite://LOCALHOST:4001", wantLog: true},
		{name: "127.0.0.1", loc: "rqlite://127.0.0.1:4001", wantLog: true},
		{name: "ipv6 loopback", loc: "rqlite://[::1]:4001", wantLog: true},
		{name: "127.0.0.5", loc: "rqlite://127.0.0.5:4001", wantLog: true},
		{name: "remote host", loc: "rqlite://example.com:4001", wantLog: false},
		{name: "discovery off explicit", loc: "rqlite://localhost:4001?disableClusterDiscovery=true", wantLog: false},
		{name: "discovery on explicit", loc: "rqlite://localhost:4001?disableClusterDiscovery=false", wantLog: false},
		{name: "localhost other params", loc: "rqlite://localhost:4001?level=strong", wantLog: true},
		{name: "https loopback", loc: "rqlites://localhost:4001", wantLog: true},
		{name: "malformed", loc: "rqlite://%zz", wantLog: false},
		{name: "discovery empty value", loc: "rqlite://localhost:4001?disableClusterDiscovery=", wantLog: true},
		{name: "discovery bare key", loc: "rqlite://localhost:4001?disableClusterDiscovery", wantLog: true},
		{name: "discovery upper TRUE", loc: "rqlite://localhost:4001?disableClusterDiscovery=TRUE", wantLog: false},
		{name: "discovery upper FALSE", loc: "rqlite://localhost:4001?disableClusterDiscovery=False", wantLog: false},
		{name: "discovery garbage value", loc: "rqlite://localhost:4001?disableClusterDiscovery=yes", wantLog: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			ctx := lg.NewContext(context.Background(), slog.New(h))

			src := &source.Source{Handle: "@rq", Location: tc.loc, Type: drivertype.Rqlite}
			maybeWarnLocalhostDiscovery(ctx, src)

			raw := strings.TrimSpace(buf.String())
			if !tc.wantLog {
				require.Empty(t, raw, "expected no log output, got: %s", raw)
				return
			}

			require.NotEmpty(t, raw, "expected one log entry, got none")
			var entry map[string]any
			require.NoError(t, json.Unmarshal([]byte(raw), &entry), "log line: %s", raw)
			require.Equal(t, "WARN", entry["level"], "expected level=WARN, got: %v", entry["level"])
			msg, _ := entry["msg"].(string)
			require.Contains(t, msg, "disableClusterDiscovery",
				"msg missing disableClusterDiscovery: %s", msg)
		})
	}
}

func Test_rewritePeerDNSError(t *testing.T) {
	// fakeDNSErr builds a *net.DNSError with the given peer name.
	// All real fields of net.DNSError (Err, Server, IsTimeout, etc.)
	// are zero/false — only Name matters to the helper under test.
	fakeDNSErr := func(name string) *net.DNSError {
		return &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
	// fakeDNSTimeoutErr builds a *net.DNSError representing a DNS
	// timeout (IsNotFound is false). The rewrite must pass these
	// through unchanged: they're not the cluster-discovery "no such
	// host" case the helper targets.
	fakeDNSTimeoutErr := func(name string) *net.DNSError {
		return &net.DNSError{Err: "i/o timeout", Name: name, IsTimeout: true}
	}

	const userLoc = "rqlite://localhost:4001"
	const userLocOff = "rqlite://localhost:4001?disableClusterDiscovery=true"

	testCases := []struct {
		name          string
		err           error
		loc           string
		wantRewrite   bool
		wantSubstrAll []string // every substring must appear in the rewritten msg
	}{
		{
			name:        "nil",
			err:         nil,
			loc:         userLoc,
			wantRewrite: false,
		},
		{
			name:        "non-dns error",
			err:         errors.New("connection refused"),
			loc:         userLoc,
			wantRewrite: false,
		},
		{
			name:          "discovered peer mismatch",
			err:           fakeDNSErr("rqlite1"),
			loc:           userLoc,
			wantRewrite:   true,
			wantSubstrAll: []string{"rqlite1", "localhost", "disableClusterDiscovery=true", "sq.io/docs/drivers/rqlite"},
		},
		{
			name: "discovered peer mismatch wrapped via fmt.Errorf",
			err: fmt.Errorf(
				"Post \"http://rqlite1:4001/db/query\": dial tcp: lookup rqlite1: %w",
				fakeDNSErr("rqlite1"),
			),
			loc:           userLoc,
			wantRewrite:   true,
			wantSubstrAll: []string{"rqlite1", "disableClusterDiscovery=true"},
		},
		{
			name:        "discovery already off",
			err:         fakeDNSErr("rqlite1"),
			loc:         userLocOff,
			wantRewrite: false,
		},
		{
			name:        "user host equals failed name",
			err:         fakeDNSErr("localhost"),
			loc:         userLoc,
			wantRewrite: false,
		},
		{
			name:        "user host equals failed name case-insensitive",
			err:         fakeDNSErr("localhost"),
			loc:         "rqlite://LOCALHOST:4001",
			wantRewrite: false,
		},
		{
			name:        "malformed url",
			err:         fakeDNSErr("rqlite1"),
			loc:         "rqlite://%zz",
			wantRewrite: false,
		},
		{
			name:        "dns timeout passes through",
			err:         fakeDNSTimeoutErr("rqlite1"),
			loc:         userLoc,
			wantRewrite: false,
		},
		{
			name:        "discovery off upper TRUE",
			err:         fakeDNSErr("rqlite1"),
			loc:         "rqlite://localhost:4001?disableClusterDiscovery=TRUE",
			wantRewrite: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := &source.Source{Handle: "@rq", Location: tc.loc, Type: drivertype.Rqlite}
			got := rewritePeerDNSError(tc.err, src)

			if tc.err == nil {
				require.Nil(t, got)
				return
			}

			// errors.As must still find the underlying *net.DNSError
			// (when there was one) — the rewrite must not break
			// downstream classification.
			var dnsErr *net.DNSError
			origHadDNS := errors.As(tc.err, &dnsErr)
			if origHadDNS {
				var afterDNS *net.DNSError
				require.True(t, errors.As(got, &afterDNS),
					"errors.As must still reach *net.DNSError after rewrite")
				require.Equal(t, dnsErr.Name, afterDNS.Name)
			}

			if !tc.wantRewrite {
				// Pass-through: helper must return the original error
				// unchanged so callers can errors.Is against it.
				require.True(t, errors.Is(got, tc.err),
					"expected pass-through (errors.Is), got rewritten: %v", got)
				return
			}

			msg := got.Error()
			for _, sub := range tc.wantSubstrAll {
				require.Contains(t, msg, sub, "rewritten message missing substring %q: %s", sub, msg)
			}
		})
	}
}
