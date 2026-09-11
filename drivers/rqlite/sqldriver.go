package rqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"io"
	"reflect"
	"strings"
	"unsafe"

	"github.com/rqlite/gorqlite"
	"github.com/rqlite/gorqlite/stdlib"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
)

// sqDBDrvrName is the registration name of sq's wrapper around
// gorqlite's database/sql driver. The wrapper exists to bridge three
// gaps between gorqlite/stdlib and what sq's record pipeline needs
// (the latter two are gh775):
//
//  1. Column type names: the stock gorqlite/stdlib driver does not
//     implement driver.RowsColumnTypeDatabaseTypeName, so
//     DatabaseTypeName always returns the empty string, which
//     downstream demotes every column kind to kind.Unknown. sqRows
//     returns the gorqlite QueryResult's Types (populated from the
//     JSON "types" array on every response, including empty result
//     sets).
//
//  2. Raw row values: sqRows.Next replaces stdlib.Rows.Next, which
//     pre-converts date/datetime columns via gorqlite's two-format
//     toTime and errors out the entire result set on any other
//     format (e.g. bare dates, or mattn/go-sqlite3's canonical
//     fractional-seconds format). sqRows delivers the raw wire
//     value instead, so the driver's own SQLite-format-complete
//     nullTime scanner (nulltime.go) does the parsing. It also
//     converts BOOLEAN cells that arrive as raw JSON numbers
//     (pre-v10 rqlite servers) to bools, and base64-decodes BLOB
//     cells, which rqlite's JSON API always returns base64-encoded.
//
//  3. Blob parameters: sqStmt.CheckNamedValue re-encodes []byte
//     arguments as JSON byte arrays, rqlite's typed-blob parameter
//     form. gorqlite marshals arguments with encoding/json, which
//     turns []byte into a base64 string that rqlite would store as
//     TEXT, silently corrupting binary columns.
const sqDBDrvrName = "rqlite-sq"

func init() { //nolint:gochecknoinits
	sql.Register(sqDBDrvrName, &sqDriver{inner: &stdlib.Driver{}})
}

// sqDriver wraps gorqlite/stdlib's driver to add the optional
// RowsColumnTypeDatabaseTypeName interface on the returned Rows.
type sqDriver struct {
	inner driver.Driver
}

// Open implements driver.Driver.
func (d *sqDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	sc, ok := c.(*stdlib.Conn)
	if !ok {
		// Defensive: gorqlite/stdlib only returns *stdlib.Conn. If
		// that ever changes, fall back to the unwrapped conn rather
		// than failing the open: callers lose the DBTypeName
		// enrichment but everything else still works.
		return c, nil
	}
	return &sqConn{Conn: sc}, nil
}

// sqConn wraps stdlib.Conn so its Prepare returns sqStmt. The embedded
// *stdlib.Conn keeps gorqlite.Connection's methods (notably
// WriteParameterizedContext) reachable via conn.Raw, which the
// writeAtomic helper depends on.
type sqConn struct {
	*stdlib.Conn
}

// Prepare implements driver.Conn.
func (c *sqConn) Prepare(query string) (driver.Stmt, error) {
	s, err := c.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	ss, ok := s.(*stdlib.Stmt)
	if !ok {
		return s, nil
	}
	return &sqStmt{Stmt: ss}, nil
}

// sqStmt wraps stdlib.Stmt so its Query / QueryContext return sqRows.
// Exec / ExecContext are inherited from the embedded *stdlib.Stmt.
type sqStmt struct {
	*stdlib.Stmt
}

// Query implements driver.Stmt.
func (s *sqStmt) Query(args []driver.Value) (driver.Rows, error) {
	r, err := s.Stmt.Query(args)
	return wrapRows(r), err
}

// QueryContext implements driver.StmtQueryContext.
func (s *sqStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	r, err := s.Stmt.QueryContext(ctx, args)
	return wrapRows(r), err
}

// CheckNamedValue implements driver.NamedValueChecker. Arguments go
// through the standard driver.DefaultParameterConverter, with one
// rqlite-specific addition: []byte arguments are re-encoded as []int,
// which gorqlite's json.Marshal renders as a JSON array of numbers:
// rqlite's typed-blob parameter form (see
// https://rqlite.io/docs/api/api/#blob-data). Without this, []byte
// marshals to a base64 JSON string and rqlite stores it as TEXT,
// silently corrupting every binary column on the write path (gh775).
//
// A nil []byte maps to SQL NULL, matching database/sql convention.
func (s *sqStmt) CheckNamedValue(nv *driver.NamedValue) error {
	v, err := driver.DefaultParameterConverter.ConvertValue(nv.Value)
	if err != nil {
		return errz.Err(err)
	}
	if b, ok := v.([]byte); ok {
		if b == nil {
			nv.Value = nil
		} else {
			nv.Value = blobParam(b)
		}
		return nil
	}
	nv.Value = v
	return nil
}

// blobParam re-encodes b as []int so that gorqlite's json.Marshal
// produces a JSON array of byte values, which rqlite accepts as a
// typed blob parameter. An empty b yields the empty array, which
// rqlite stores as the zero-length blob.
func blobParam(b []byte) []int {
	a := make([]int, len(b))
	for i, v := range b {
		a[i] = int(v)
	}
	return a
}

// wrapRows wraps a *stdlib.Rows in sqRows. nil and non-stdlib Rows
// pass through unchanged so transient error paths don't blow up.
func wrapRows(r driver.Rows) driver.Rows {
	if r == nil {
		return nil
	}
	sr, ok := r.(*stdlib.Rows)
	if !ok {
		return r
	}
	vals, valsOK := rawValues(&sr.QueryResult)
	return &sqRows{
		Rows:   sr,
		vals:   vals,
		valsOK: valsOK,
		convs:  wireConvsForTypes(typesOf(sr)),
	}
}

// sqRows wraps stdlib.Rows to implement
// driver.RowsColumnTypeDatabaseTypeName, sourcing per-column types
// from the gorqlite QueryResult, and to replace stdlib.Rows.Next with
// a raw-value implementation (see Next).
type sqRows struct {
	*stdlib.Rows

	// vals is the QueryResult's raw row data (the JSON-decoded
	// "values" array), extracted by rawValues at wrap time. valsOK
	// records whether the extraction succeeded.
	vals [][]any

	// convs holds the per-column wire conversion derived from the
	// response's "types" array.
	convs []wireConv

	valsOK bool
}

// Next implements driver.Rows, shadowing the embedded
// stdlib.Rows.Next. The stdlib implementation reads each row via
// gorqlite's QueryResult.Slice, which pre-converts any column whose
// rqlite type is date/datetime using a two-format parser (toTime) and
// returns an error for every other format. Because database/sql
// surfaces that error from Rows.Next, a single bare date or
// fractional-seconds datetime aborts the whole result set (gh775).
//
// This implementation reads the raw JSON-decoded values instead:
// date/datetime cells pass through as raw strings for the driver's
// nullTime scanner (which holds SQLite's full timestamp format list),
// BOOLEAN cells arriving as raw JSON numbers become bools, and BLOB
// cells are base64-decoded. See convertWireValue.
func (r *sqRows) Next(dest []driver.Value) error {
	if !r.QueryResult.Next() {
		return io.EOF
	}
	if !r.valsOK {
		// rawValues failed: gorqlite's internal layout has changed.
		// Fail loudly rather than fall back to stdlib.Rows.Next, whose
		// toTime conversion and undecoded blobs corrupt or abort
		// result sets.
		return errz.New("rqlite: cannot read raw row values from gorqlite QueryResult")
	}
	idx := r.RowNumber()
	if idx < 0 || idx >= int64(len(r.vals)) {
		// Same defensive posture as the valsOK guard: if gorqlite's
		// RowNumber semantics ever change, fail loudly, never panic.
		return errz.Errorf("rqlite: row index %d out of range (%d rows)",
			idx, len(r.vals))
	}
	row := r.vals[idx]
	if len(row) < len(dest) {
		return errz.Errorf("rqlite: row has %d values but %d columns declared",
			len(row), len(dest))
	}
	for i := range dest {
		var conv wireConv
		if i < len(r.convs) {
			conv = r.convs[i]
		}
		dest[i] = convertWireValue(conv, row[i])
	}
	return nil
}

// wireConv enumerates the conversions sqRows.Next applies to raw wire
// values, chosen per column from the response's "types" array.
type wireConv uint8

const (
	// convNone passes the raw value through unchanged.
	convNone wireConv = iota
	// convBool converts raw JSON numbers to bool for BOOLEAN columns.
	convBool
	// convBlob base64-decodes strings for BLOB columns.
	convBlob
	// convInt converts raw JSON numbers to int64 for integer columns.
	convInt
)

// wireConvsForTypes derives the per-column conversion from rqlite's
// column type names. Parameterized type names (e.g. "blob(16)") are
// normalized first. Unrecognized and empty type names get convNone.
func wireConvsForTypes(types []string) []wireConv {
	convs := make([]wireConv, len(types))
	for i, typ := range types {
		if j := strings.IndexRune(typ, '('); j >= 0 {
			typ = typ[:j]
		}
		switch strings.ToLower(strings.TrimSpace(typ)) {
		case "boolean", "bool":
			convs[i] = convBool
		case "blob":
			convs[i] = convBlob
		default:
			// gorqlite decodes every JSON number as float64. For an
			// integer-kind column the scan destination is *sql.NullInt64,
			// and database/sql formats the float64 via FormatFloat(_, 'g', _)
			// before ParseInt; any value whose shortest form is exponential
			// (every integer >= 1e6, and all values > 2^53) then fails the
			// scan outright. Converting to int64 at the wire layer sidesteps
			// that path. Exact for |v| <= 2^53; larger magnitudes are already
			// imprecise from gorqlite's float64 decode (a full fix needs
			// gorqlite to decode with json.Number). Keyed off kindFromDBTypeName
			// so it tracks exactly which columns scan into *sql.NullInt64;
			// rqlite reports "integer" for literals, COUNT, SUM-of-int, and
			// arithmetic too, so those are covered.
			if kindFromDBTypeName(context.Background(), "", typ, nil) == kind.Int {
				convs[i] = convInt
			}
		}
	}
	return convs
}

// convertWireValue converts a raw JSON-decoded cell value per conv:
//
//   - convBool: raw JSON numbers become bools using SQLite truthiness
//     (non-zero is true). Older rqlite servers deliver BOOLEAN cells
//     as numbers; without this, sql.NullBool.Scan(float64) fails.
//     rqlite v10+ converts server-side, so JSON bools pass through.
//   - convBlob: strings are base64-decoded, since rqlite's JSON API
//     returns BLOB storage base64-encoded. A string that is not valid
//     base64 can only be TEXT stored in a BLOB-typed column, and is
//     surfaced as its literal bytes. (A text value that happens to be
//     valid base64 is indistinguishable from a real blob in rqlite's
//     wire format; rqlite's ?blob_array disambiguator is not
//     reachable through gorqlite, which hardcodes its API query
//     strings.)
//   - convInt: raw JSON numbers become int64 for integer columns, whose
//     scan destination (*sql.NullInt64) otherwise rejects a float64 that
//     database/sql formats in exponential notation. See wireConvsForTypes.
//
// All other values pass through unchanged.
func convertWireValue(conv wireConv, v any) driver.Value {
	switch conv {
	case convNone:
	case convBool:
		if f, ok := v.(float64); ok {
			return f != 0
		}
	case convBlob:
		if s, ok := v.(string); ok {
			if b, err := base64.StdEncoding.Strict().DecodeString(s); err == nil {
				return b
			}
			return []byte(s)
		}
	case convInt:
		if f, ok := v.(float64); ok {
			return int64(f)
		}
	}
	return v
}

// rtypeRawValues is the expected reflect.Type of
// gorqlite.QueryResult's values field.
var rtypeRawValues = reflect.TypeOf([][]any(nil))

// rawValues extracts the raw row data (the JSON-decoded "values"
// array) from qr. gorqlite provides no public access to it: Map,
// Slice, and Scan all pre-convert date/datetime columns via the
// two-format toTime, which errors on most SQLite timestamp formats
// and so cannot be used (gh775). The unexported field is read via
// reflect+unsafe instead, guarded so a future gorqlite layout change
// reports ok=false (and Test_rawValues_GorqliteLayout fails) rather
// than panicking.
func rawValues(qr *gorqlite.QueryResult) (vals [][]any, ok bool) {
	f := reflect.ValueOf(qr).Elem().FieldByName("values")
	if !f.IsValid() || f.Type() != rtypeRawValues {
		return nil, false
	}
	f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	vals, ok = f.Interface().([][]any)
	return vals, ok
}

// Compile-time assertion that sqRows satisfies the optional
// interface; if the upstream Rows shape changes such that the
// embedded QueryResult Types method goes away, this becomes a
// build error.
var _ driver.RowsColumnTypeDatabaseTypeName = (*sqRows)(nil)

// ColumnTypeDatabaseTypeName implements
// driver.RowsColumnTypeDatabaseTypeName. The strings come from the
// SELECT's "types" array, which rqlite populates from SQLite's
// pragma-style column metadata. For empty result sets the array is
// still populated, so this works equally well for empty tables.
//
// Out-of-range indexes return the empty string, mirroring the
// database/sql default and matching what callers in
// kindFromDBTypeName already handle.
func (r *sqRows) ColumnTypeDatabaseTypeName(index int) string {
	types := typesOf(r.Rows)
	if index < 0 || index >= len(types) {
		return ""
	}
	return types[index]
}

// typesOf returns the per-column type strings from the gorqlite
// QueryResult embedded inside a stdlib.Rows. The result may be nil
// if the underlying response carried no types array.
func typesOf(r *stdlib.Rows) []string {
	if r == nil {
		return nil
	}
	// stdlib.Rows embeds gorqlite.QueryResult by value, and
	// QueryResult exposes Types(). The interface match shields
	// against future internal renames.
	qr, ok := any(&r.QueryResult).(interface{ Types() []string })
	if !ok {
		return nil
	}
	return qr.Types()
}

// Compile-time check that gorqlite.QueryResult still exposes Types.
// If gorqlite ever renames or removes the method, this fails the
// build rather than silently leaving every column kind as Unknown.
var _ interface{ Types() []string } = (*gorqlite.QueryResult)(nil)
