// Package rqlite implements the sq driver for rqlite, the distributed
// SQLite-based database (see https://rqlite.io). The backing SQL driver
// is github.com/rqlite/gorqlite's stdlib (database/sql) adapter.
//
// Unlike sq's sqlite3 driver, rqlite is networked: there is no file
// mode. The source location is an HTTP URL using a single scheme,
//
//	rqlite://user:pass@host:4001              (HTTP)
//	rqlite://user:pass@host:4001?tls=true     (HTTPS)
//
// which is translated to gorqlite's expected http(s):// URL at Open
// time. The optional ?insecure=true companion param (valid only with
// ?tls=true) opts out of TLS certificate verification.
//
// rqlite's HTTP API does not support interactive transactions, only
// atomic batches via /db/execute. gorqlite's database/sql driver
// surfaces Begin/Commit/Rollback as no-ops, so any sq code path that
// needs cross-statement atomicity (CopyTable's CREATE+INSERT-SELECT,
// AlterTableColumnKinds' table-rebuild dance) goes through the
// writeAtomic helper, which tunnels via sql.Conn.Raw() to gorqlite's
// native WriteParameterizedContext. Truncate emits its DELETE (and
// optional sqlite_sequence reset) as plain database/sql calls and is
// deliberately non-atomic across the two statements; see its godoc.
// NewBatchInsert delegates to driver.DefaultNewBatchInsert: each
// batch is one multi-row INSERT, one HTTP call, atomic at rqlite
// per batch.
package rqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rqlite/gorqlite"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// dbDrvr is the name registered by gorqlite/stdlib via
	// sql.Register("rqlite", ...). It is intentionally identical to
	// drivertype.Rqlite's string value.
	dbDrvr = "rqlite"

	// Prefix is the scheme+separator for rqlite sources.
	Prefix = "rqlite://"

	defaultPort = 4001
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the rqlite implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Rqlite {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the rqlite implementation of driver.SQLDriver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver. The values come from rqlite's
// connection URL grammar (see gorqlite/conn.go) and the small set of
// rqlite features sq actively cares about.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"level":                   {"none", "weak", "linearizable", "strong"},
		"disableClusterDiscovery": {"true", "false"},
		"timeout":                 nil,
		"tls":                     {"true", "false"},
		"insecure":                {"true", "false"},
	}
}

// LocationShape implements driver.SQLDriver.
func (d *driveri) LocationShape() driver.LocationShape {
	return driver.LocationShape{
		Type:    drivertype.Rqlite,
		Schemes: []string{"rqlite"},
		Segments: []driver.Segment{
			{Kind: driver.SegCredentials, Optional: true},
			{Kind: driver.SegAuthority},
			{Kind: driver.SegConnParams, Optional: true},
		},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DBProperties implements driver.SQLDriver. rqlite v10 surfaces a
// SQLite version and a small cluster-level status payload over its
// HTTP API rather than via pragmas, so this implementation returns
// just the SQLite version. Richer cluster status (leader address,
// node count) lands in a follow-up.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	const q = `SELECT sqlite_version()`
	var v string
	if err := db.QueryRowContext(ctx, q).Scan(&v); err != nil {
		return nil, errw(err)
	}
	return map[string]any{"sqlite_version": v}, nil
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Rqlite,
		Description: "rqlite",
		Doc:         "https://rqlite.io",
		IsSQL:       true,
		DefaultPort: defaultPort,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	db, err := d.doOpen(ctx, src)
	if err != nil {
		return nil, err
	}

	if err = driver.OpeningPing(ctx, src, db); err != nil {
		return nil, enrichConnError(err, src)
	}

	return &grip{log: d.log, db: db, src: src, drvr: d}, nil
}

func (d *driveri) doOpen(ctx context.Context, src *source.Source) (*sql.DB, error) {
	maybeWarnLocalhostDiscovery(ctx, src)

	loc, portAdded, err := locationWithDefaultPort(src.Location)
	if err != nil {
		return nil, err
	}
	if portAdded {
		lg.FromContext(ctx).Debug("rqlite: applied default port",
			lga.Src, src.Handle,
			lga.Default, defaultPort,
		)
	}

	dsn, opts, err := dsnFromLocation(loc)
	if err != nil {
		return nil, err
	}

	var db *sql.DB
	if opts.insecure {
		db = sql.OpenDB(&insecureConnector{
			dsn:    dsn,
			client: newInsecureHTTPClient(insecureClientTimeout(dsn, src.Options)),
		})
	} else {
		db, err = sql.Open(sqDBDrvrName, dsn)
		if err != nil {
			// Don't include dsn in the error: it may carry credentials.
			return nil, errz.Wrapf(enrichConnError(errw(err), src),
				"failed to open rqlite source %s", src.Handle)
		}
	}

	driver.ConfigureDB(ctx, db, src.Options)
	return db, nil
}

// insecureClientTimeout returns the HTTP client timeout for the
// insecure (skip-verify) connection path. gorqlite applies the
// gorqlite-native ?timeout=N URL param (integer seconds) only when it
// constructs its own http.Client; sq passes a custom client on the
// insecure path, so the param must be honored here. ?timeout takes
// precedence over conn.open-timeout, matching what gorqlite does on
// the default path when no custom client is supplied.
func insecureClientTimeout(dsn string, o options.Options) time.Duration {
	timeout := driver.OptConnOpenTimeout.Get(o)
	if u, err := url.Parse(dsn); err == nil {
		if v := u.Query().Get("timeout"); v != "" {
			if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
				timeout = time.Duration(secs) * time.Second
			}
		}
	}
	return timeout
}

// Truncate implements driver.Driver.
//
// rqlite has no interactive transactions. We emit DELETE FROM tbl
// through the standard database/sql path; one HTTP call, atomic
// at the rqlite layer for that single statement. When reset is true
// and the sqlite_sequence table exists, we follow with a separate
// UPDATE to clear the AUTOINCREMENT counter. The DELETE and the
// counter UPDATE are NOT atomic relative to each other; this is
// intentional. The simpler non-batch path reports the deleted-row
// count accurately, and atomic DELETE+UPDATE for Truncate is reserved
// for the cases where it materially matters (CopyTable,
// AlterTableColumnKinds).
//
// Cross-driver note: on reset-stage failure (the sqlite_sequence probe
// or UPDATE), this function returns (affectedFromDELETE, err). The
// DELETE genuinely committed. This diverges from drivers that wrap
// Truncate in a real Tx (sqlite3, postgres), where reset failure
// rolls the DELETE back and they return (0, err). Callers checking
// the affected count across drivers should be aware.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (int64, error) {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return 0, errw(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDB, db.Close)

	affected, err := sqlz.ExecAffected(ctx, db, fmt.Sprintf("DELETE FROM %q", tbl))
	if err != nil {
		return affected, errw(err)
	}

	if reset {
		const seqProbe = `SELECT COUNT(name) FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'`
		var seqCount int64
		if err = db.QueryRowContext(ctx, seqProbe).Scan(&seqCount); err != nil {
			return affected, errw(err)
		}
		if seqCount > 0 {
			if _, err = db.ExecContext(ctx,
				"UPDATE sqlite_sequence SET seq = 0 WHERE name = ?", tbl); err != nil {
				return affected, errw(err)
			}
		}
	}

	return affected, nil
}

// ValidateSource implements driver.Driver. Guards both the driver
// type and the rqlite-specific URL contradiction (?insecure=true
// requires ?tls=true). The latter is also checked in
// dsnFromLocation at every Open, but doing it here catches the
// error at sq add time even when --skip-verify suppresses Ping.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != drivertype.Rqlite {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", drivertype.Rqlite, src.Type)
	}

	// If the location contains a ${scheme:path} secret placeholder
	// (e.g. "${keyring:abc}" or "${env:DSN}"), it is opaque at
	// validation time: the URL grammar check runs at Open time on the
	// resolved location instead.
	//
	// Use secret.ExtractRefs rather than a "${"-substring scan: a
	// literal "${" or escaped "$${...}" in src.Location is not
	// actually a placeholder and shouldn't suppress the grammar
	// check. A malformed-placeholder error here surfaces as
	// ValidateSource's error rather than producing a confusing
	// parse failure later.
	refs, err := secret.ExtractRefs(src.Location)
	if err != nil {
		return nil, errw(err)
	}
	if len(refs) > 0 {
		return src, nil
	}

	// Reuse the dsnFromLocation parser, which performs the full
	// grammar + contradiction validation. We discard the dsn and
	// opts because we only want the error side-effect.
	if _, _, err := dsnFromLocation(src.Location); err != nil {
		return nil, err
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseDB, db)

	if err = db.PingContext(ctx); err != nil {
		return errz.Wrapf(enrichConnError(errw(err), src),
			"ping %s: %s", src.Handle, src.RedactedLocation())
	}

	return nil
}

// Dialect implements driver.SQLDriver. rqlite speaks SQLite SQL, so the
// dialect mirrors the sqlite3 driver's.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.Rqlite,
		Placeholders:   placeholders,
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 500,
		Ops:            dialect.DefaultOps(),
		ExecModeFor:    dialect.DefaultExecModeFor,
		Joins:          jointype.All(),
		Catalog:        false,
	}
}

func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)
	for i := 0; i < numRows; i++ {
		rows[i] = "(" + stringz.RepeatJoin("?", numCols, driver.Comma) + ")"
	}
	return strings.Join(rows, driver.Comma)
}

// Renderer implements driver.SQLDriver. The SLQ function overrides
// mirror the sqlite3 driver's since rqlite executes SQLite SQL.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()

	// rqlite/SQLite has no real concept of schemas or catalogs; surface
	// the same conventional values the sqlite3 driver uses so the
	// rendered SQL doesn't fail on `schema()` / `catalog()`.
	const schemaFrag = `(SELECT name FROM pragma_database_list ORDER BY seq limit 1)`
	r.FunctionOverrides[ast.FuncNameSchema] = render.FuncOverrideString(schemaFrag)
	const catalogFrag = `(SELECT 'default')`
	r.FunctionOverrides[ast.FuncNameCatalog] = render.FuncOverrideString(catalogFrag)

	r.FunctionOverrides[ast.FuncNameContains] = renderFuncContainsInstr
	r.FunctionOverrides[ast.FuncNameStartsWith] = renderFuncStartsWithSubstr
	r.FunctionOverrides[ast.FuncNameEndsWith] = renderFuncEndsWithSubstr
	r.FunctionOverrides[ast.FuncNameIContains] = renderFuncIContainsLike
	r.FunctionOverrides[ast.FuncNameIStartsWith] = renderFuncIStartsWithLike
	r.FunctionOverrides[ast.FuncNameIEndsWith] = renderFuncIEndsWithLike
	// SQLite's default LIKE is ASCII case-insensitive, so like and ilike
	// register the same renderer.
	r.FunctionOverrides[ast.FuncNameLike] = renderFuncLike
	r.FunctionOverrides[ast.FuncNameILike] = renderFuncLike

	return r
}

// locationWithDefaultPort returns the location string with the default
// port (4001) added if no port is specified. The second return value
// is true if the port was added. gorqlite's stdlib driver does not
// fall back to a default port on its own; without this injection,
// `sq add 'rqlite://host'` would attempt port 80 (Go's HTTP default)
// rather than rqlite's conventional 4001.
//
// rqlite uses the same port (4001) for HTTP and HTTPS by default.
func locationWithDefaultPort(loc string) (string, bool, error) {
	u, err := url.Parse(loc)
	if err != nil {
		// Don't include loc in the error: it may carry credentials.
		return "", false, errz.Wrap(err, "rqlite: parse location")
	}

	if u.Hostname() == "" {
		// Reject an empty/host-less location outright: injecting a
		// default port into ":4001" would mask the real problem and
		// produce a confusing downstream error.
		return "", false, errz.New("rqlite: location is missing host")
	}

	if u.Port() != "" {
		return loc, false, nil
	}

	u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(defaultPort))
	return u.String(), true, nil
}

// dsnOpts captures the sq-synthetic query params that dsnFromLocation
// strips before handing the URL to gorqlite. These params do not appear
// in gorqlite's connection-string grammar; they are sq's TLS knobs.
type dsnOpts struct {
	// tls is true when the user opted into HTTPS via ?tls=true.
	tls bool

	// insecure is true when the user opted out of TLS certificate
	// verification via ?insecure=true. Requires tls=true; the
	// contradiction check lives in dsnFromLocation.
	insecure bool
}

// dsnFromLocation translates an rqlite:// source location into the
// http(s):// URL that gorqlite.Open expects, and reports the
// sq-synthetic query-param opts (?tls, ?insecure) parsed out of the
// location. Synthetic params are stripped from the returned DSN so
// gorqlite never sees them.
//
// Returns an error if the location's scheme is unrecognized, ?tls or
// ?insecure has a value other than "true"/"false", or ?insecure is
// set without ?tls=true.
func dsnFromLocation(loc string) (string, dsnOpts, error) {
	var opts dsnOpts
	if !strings.HasPrefix(loc, Prefix) {
		// Don't include loc: it may carry credentials.
		return "", opts, errz.Errorf("rqlite: location must start with %q", Prefix)
	}
	scheme := "http"

	u, err := url.Parse(loc)
	if err != nil {
		// url.Error embeds the raw input URL in its message, which
		// would echo inline credentials. Strip that wrapper so the
		// underlying cause (e.g. "missing ']' in host") is preserved
		// without the URL.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return "", opts, errz.Wrap(err, "rqlite: invalid location")
	}

	q := u.Query()
	// Use q.Has, not a q.Get-non-empty check: a bare "?tls" or empty
	// "?tls=" must be rejected, not silently forwarded to gorqlite.
	if q.Has("tls") {
		switch v := q.Get("tls"); v {
		case "true":
			scheme = "https"
			opts.tls = true
		case "false":
			scheme = "http"
			opts.tls = false
		default:
			return "", opts, errz.Errorf(
				`rqlite: tls must be "true" or "false", got %q`, v)
		}
		q.Del("tls")
	}

	if q.Has("insecure") {
		switch v := q.Get("insecure"); v {
		case "true":
			opts.insecure = true
		case "false":
			opts.insecure = false
		default:
			return "", opts, errz.Errorf(
				`rqlite: insecure must be "true" or "false", got %q`, v)
		}
		q.Del("insecure")
	}

	if opts.insecure && !opts.tls {
		return "", opts, errz.New(
			"rqlite: insecure has no effect without tls=true; " +
				"either add tls=true or remove insecure")
	}

	u.Scheme = scheme
	u.RawQuery = q.Encode()
	return u.String(), opts, nil
}

// CopyTable implements driver.SQLDriver.
//
// rqlite has no interactive transactions, so the implementation uses
// the faithful-DDL-rewrite shape via writeAtomic: read the source
// table's CREATE statement from sqlite_master, extract the table
// identifier with the shared sqlparser package, substitute the
// destination name, and re-execute. When copyData is true the
// rewritten CREATE and the INSERT-FROM-SELECT are sent as one atomic
// batch.
//
// The rewrite preserves the original CREATE TABLE text modulo the
// table identifier substitution and any self-referential FK rewrites.
// Constraints carried across: UNIQUE, FOREIGN KEY, AUTOINCREMENT,
// CHECK, composite PRIMARY KEY, exact DEFAULT expressions, WITHOUT
// ROWID, and column comments. Self-referential foreign keys
// (REFERENCES <src>(...) inside the same CREATE TABLE) are rewritten
// to point at the destination so the destination's FKs resolve
// against itself rather than the source (gh759). Cross-table FKs are
// left untouched.
//
// Not preserved: indexes and triggers. These live as separate
// sqlite_master rows and are out of scope for CopyTable. Matches the
// sqlite3 driver's behavior.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB,
	fromTbl, toTbl tablefq.T, copyData bool,
) (int64, error) {
	masterTbl := tablefq.T{Schema: fromTbl.Schema, Table: "sqlite_master"}
	q := fmt.Sprintf("SELECT sql FROM %s WHERE type='table' AND name=?",
		masterTbl.Render(stringz.DoubleQuote))
	var ogDDL string
	if err := db.QueryRowContext(ctx, q, fromTbl.Table).Scan(&ogDDL); err != nil {
		return 0, errz.Wrapf(errw(err),
			"rqlite: copy table: failed to read DDL for {%s}", fromTbl.Table)
	}

	// Extract the table identifier with byte offsets so we can splice the
	// new identifier in place without unanchored strings.Replace, which
	// can misfire when the identifier recurs in CHECK exprs, default
	// literals, or column names. Mirrors the sqlite3 driver's approach.
	ogIdent, err := sqlparser.ExtractTableIdentFromCreateTableStmt(ogDDL)
	if err != nil {
		return 0, errz.Wrap(err, "rqlite: copy table")
	}
	identStart := ogIdent.TableOffset
	if ogIdent.SchemaOffset >= 0 {
		identStart = ogIdent.SchemaOffset
	}
	identEnd := ogIdent.TableOffset + len(ogIdent.RawTable)
	destQuoted := toTbl.Render(stringz.DoubleQuote)

	edits := []sqlparser.Edit{{
		Start:       identStart,
		End:         identEnd,
		Replacement: destQuoted,
	}}

	// Rewrite self-FKs so the destination's REFERENCES point at itself
	// rather than the source (gh759). Cross-table FKs are left alone.
	// SQLite's foreign_table grammar rule is a single any_name (no
	// schema qualification permitted), so the replacement here is the
	// destination's bare table token even when destQuoted carries a
	// "schema"."table" prefix for the CREATE TABLE identifier edit.
	destTableQuoted := stringz.DoubleQuote(toTbl.Table)
	fkRefs, err := sqlparser.ExtractForeignTableRefsFromCreateTableStmt(ogDDL)
	if err != nil {
		return 0, errz.Wrap(err, "rqlite: copy table")
	}
	for _, r := range fkRefs {
		if !strings.EqualFold(r.Table, ogIdent.Table) {
			continue
		}
		edits = append(edits, sqlparser.Edit{
			Start:       r.TableOffset,
			End:         r.TableOffset + len(r.RawTable),
			Replacement: destTableQuoted,
		})
	}

	destDDL, err := sqlparser.ApplyEdits(ogDDL, edits)
	if err != nil {
		return 0, errz.Wrap(err, "rqlite: copy table: failed to apply DDL rewrites")
	}

	if !copyData {
		if _, err = db.ExecContext(ctx, destDDL); err != nil {
			return 0, errw(err)
		}
		return 0, nil
	}

	stmts := []gorqlite.ParameterizedStatement{
		{Query: destDDL},
		{Query: fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`,
			toTbl.Render(stringz.DoubleQuote), fromTbl.Render(stringz.DoubleQuote))},
	}
	results, err := writeAtomic(ctx, db, stmts...)
	if err != nil {
		return 0, err
	}
	return results[1].RowsAffected, nil
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (
	record.Meta, driver.NewRecordFunc, error,
) {
	recMeta, err := recordMetaFromColumnTypes(ctx, colTypes)
	if err != nil {
		return nil, nil, errw(err)
	}

	mungeFn := func(vals []any) (record.Record, error) {
		return newRecordFromScanRow(recMeta, vals), nil
	}

	return recMeta, mungeFn, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string
	if ifExists {
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %s", tbl)
	} else {
		stmt = fmt.Sprintf("DROP TABLE %s", tbl)
	}
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("rqlite: CREATE SCHEMA is not supported (SQLite has no schemas)")
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("rqlite: DROP SCHEMA is not supported (SQLite has no schemas)")
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
	query := buildCreateTableStmt(tblDef)

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return errw(err)
	}

	_, err = stmt.ExecContext(ctx)
	if err != nil {
		lg.WarnIfCloseError(d.log, lgm.CloseDBStmt, stmt)
		return errw(err)
	}

	return errw(stmt.Close())
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	const q = `SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1`
	var name string
	if err := db.QueryRowContext(ctx, q).Scan(&name); err != nil {
		return "", errw(err)
	}
	return name, nil
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	if schma == "" {
		return false, nil
	}
	const q = `SELECT COUNT(name) FROM pragma_database_list WHERE name = ?`
	var count int
	return count > 0, errw(db.QueryRowContext(ctx, q, schma).Scan(&count))
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	const q = `SELECT name FROM pragma_database_list ORDER BY name`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(d.log, rows)

	var schemas []string
	for rows.Next() {
		var schma string
		if err = rows.Scan(&schma); err != nil {
			return nil, errw(err)
		}
		schemas = append(schemas, schma)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return schemas, nil
}

// ListTableNames implements driver.SQLDriver. System tables
// (sqlite_master, sqlite_sequence, etc.) are filtered out.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string,
	tables, views bool,
) ([]string, error) {
	var tblClause string
	switch {
	case tables && views:
		tblClause = " WHERE (type = 'table' OR type = 'view')"
	case tables:
		tblClause = " WHERE type = 'table'"
	case views:
		tblClause = " WHERE type = 'view'"
	default:
		return []string{}, nil
	}
	tblClause += " AND name NOT LIKE 'sqlite_%'"

	q := "SELECT name FROM "
	if schma == "" {
		q += "sqlite_master"
	} else {
		q += stringz.DoubleQuote(schma) + ".sqlite_master"
	}
	q += tblClause + " ORDER BY name"

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}
	names, err := sqlz.RowsScanColumn[string](ctx, rows)
	return names, errw(err)
}

// ListSchemaMetadata implements driver.SQLDriver. The returned schemas
// carry the conventional catalog value "default".
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	names, err := d.ListSchemas(ctx, db)
	if err != nil {
		return nil, err
	}
	schemas := make([]*metadata.Schema, len(names))
	for i, name := range names {
		schemas[i] = &metadata.Schema{Name: name, Catalog: "default"}
	}
	return schemas, nil
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, nil
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("rqlite: catalogs are not supported (SQLite has no catalogs)")
}

// ListCatalogs implements driver.SQLDriver.
func (d *driveri) ListCatalogs(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("rqlite: catalogs are not supported (SQLite has no catalogs)")
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM sqlite_master WHERE name = ? AND type='table'`
	var count int64
	if err := db.QueryRowContext(ctx, query, tbl).Scan(&count); err != nil {
		return false, errw(err)
	}
	return count == 1, nil
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	numRows int,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

// newStmtExecFunc adapts a *sql.Stmt to driver.StmtExecFunc.
func newStmtExecFunc(stmt *sql.Stmt) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		res, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errw(err)
		}
		affected, err := res.RowsAffected()
		return affected, errw(err)
	}
}

// NewBatchInsert implements driver.SQLDriver. Each batch is one
// multi-row INSERT statement, which gorqlite stdlib sends as one
// HTTP POST to /db/execute, atomic per batch at the rqlite layer.
// No rqlite-specific batching machinery is needed; we delegate to
// the standard framework.
func (d *driveri) NewBatchInsert(ctx context.Context, msg string, db sqlz.DB,
	_ *source.Source, destTbl string, destColNames []string,
) (*driver.BatchInsert, error) {
	return driver.DefaultNewBatchInsert(ctx, msg, d, db, destTbl, destColNames)
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string,
	destColNames []string, where string,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	query, err := buildUpdateStmt(destTbl, destColNames, where)
	if err != nil {
		return nil, err
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

// TableColumnTypes implements driver.SQLDriver. The implementation
// mirrors the sqlite3 driver: SELECT a single row from the table so
// rows.ColumnTypes returns richer info than it would for an empty
// result set. When the table is empty we fall back to the
// no-rows ColumnTypes call.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	const queryTpl = "SELECT %s FROM %s LIMIT 1"

	enquote := d.Dialect().Enquote
	tblNameQuoted := enquote(tblName)

	colsClause := "*"
	if len(colNames) > 0 {
		colsClause = strings.Join(langz.Apply(colNames, enquote), driver.Comma)
	}

	query := fmt.Sprintf(queryTpl, colsClause, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		sqlz.CloseRows(d.log, rows)
		return nil, errw(err)
	}

	if rows.Next() {
		colTypes, err = rows.ColumnTypes()
		if err != nil {
			sqlz.CloseRows(d.log, rows)
			return nil, errw(err)
		}
	}

	if err = rows.Err(); err != nil {
		sqlz.CloseRows(d.log, rows)
		return nil, errw(err)
	}

	if err = rows.Close(); err != nil {
		return nil, errw(err)
	}

	return colTypes, nil
}

// getTableRecordMeta returns the record.Meta for the named columns of
// tblName. If colNames is empty, all columns are returned.
func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (
	record.Meta, error,
) {
	colTypes, err := d.TableColumnTypes(ctx, db, tblName, colNames)
	if err != nil {
		return nil, err
	}

	destCols, _, err := d.RecordMeta(ctx, colTypes)
	if err != nil {
		return nil, err
	}

	return destCols, nil
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf(`ALTER TABLE %q RENAME TO %q`, tbl, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "rqlite: alter table: failed to rename table {%s} to {%s}", tbl, newName)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q %s", tbl, col, DBTypeForKind(knd))
	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(errw(err), "rqlite: alter table: failed to add column {%s} to table {%s}", col, tbl)
	}
	return nil
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE %q RENAME COLUMN %q TO %q", tbl, col, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "rqlite: alter table: failed to rename column {%s.%s} to {%s}", tbl, col, newName)
}

// AlterTableColumnKinds implements driver.SQLDriver.
//
// SQLite has no ALTER COLUMN TYPE, so the implementation rebuilds the
// table via an atomic batch through writeAtomic:
//
//	PRAGMA foreign_keys=off
//	CREATE TABLE <tmp> (... new kinds, original constraints ...)
//	INSERT INTO <tmp> SELECT * FROM <original>
//	DROP TABLE <original>
//	ALTER TABLE <tmp> RENAME TO <original>
//	(restore sqlite_sequence row, if the original table had one)
//	PRAGMA foreign_keys=<prev>
//
// All statements ride one /db/execute HTTP call and are atomic
// at rqlite. The prior foreign_keys value is read outside the batch
// (rqlite has no interactive transactions) and inlined as the final
// statement, restoring the session state rather than blindly forcing
// it on. This mirrors the sqlite3 driver's pragmaDisableForeignKeys
// restore pattern (drivers/sqlite3/alter.go).
//
// The new CREATE TABLE is built by reading the original DDL from
// sqlite_master, patching the column type tokens for the requested
// columns, and renaming the table identifier to a unique temporary
// so the original can be dropped at the end. This preserves UNIQUE,
// FOREIGN KEY, AUTOINCREMENT, CHECK, composite PRIMARY KEY, the
// exact DEFAULT expressions, WITHOUT ROWID, and column comments.
// Self-referential FKs resolve correctly because the DROP-and-
// RENAME-back restores the original table name (the inner REFERENCES
// clause is untouched because the rewrite is anchored to the table
// identifier's byte offset).
//
// AUTOINCREMENT sequence continuity is preserved (gh757): the DROP
// removes the original table's sqlite_sequence row, so the row is
// captured outside the batch and restored as part of the batch after
// the rename, matching the sqlite3 driver. The restore takes
// max(seq, captured), so it never lowers the value the rebuilt table
// already holds; like the rebuild as a whole, it makes no guarantee
// against writes that race the batch.
func (d *driveri) AlterTableColumnKinds(ctx context.Context, db sqlz.DB,
	tbl string, colNames []string, kinds []kind.Kind,
) error {
	if len(colNames) != len(kinds) {
		return errz.New("rqlite: alter table: mismatched count of columns and kinds")
	}

	const q = `SELECT sql FROM sqlite_master WHERE type='table' AND name=?`
	var ogDDL string
	if err := db.QueryRowContext(ctx, q, tbl).Scan(&ogDDL); err != nil {
		return errz.Wrapf(errw(err),
			"rqlite: alter table: failed to read DDL for {%s}", tbl)
	}

	allColDefs, err := sqlparser.ExtractCreateTableStmtColDefs(ogDDL)
	if err != nil {
		return errz.Wrap(err, "rqlite: alter table: failed to extract column definitions")
	}

	colDefs := make([]*sqlparser.ColDef, 0, len(colNames))
	for _, colName := range colNames {
		var found *sqlparser.ColDef
		for _, cd := range allColDefs {
			if cd.Name == colName {
				found = cd
				break
			}
		}
		if found == nil {
			return errz.Errorf("rqlite: alter table: column {%s} not found in table DDL", colName)
		}
		colDefs = append(colDefs, found)
	}

	// Locate the table identifier in the original DDL so its byte offset
	// is known. Avoids unanchored strings.Replace, which can misfire when
	// the table name appears earlier as a column-name prefix or inside a
	// comment / default literal. Mirrors the sqlite3 driver.
	tblIdent, err := sqlparser.ExtractTableIdentFromCreateTableStmt(ogDDL)
	if err != nil {
		return errz.Wrap(err, "rqlite: alter table: failed to extract table identifier from DDL")
	}

	tmpName := "tmp_tbl_alter_" + stringz.Uniq8()
	edits := make([]sqlparser.Edit, 0, len(colDefs)+1)
	for i, colDef := range colDefs {
		edits = append(edits, sqlparser.Edit{
			Start:       colDef.RawTypeOffset,
			End:         colDef.RawTypeOffset + len(colDef.RawType),
			Replacement: DBTypeForKind(kinds[i]),
		})
	}
	edits = append(edits, sqlparser.Edit{
		Start:       tblIdent.TableOffset,
		End:         tblIdent.TableOffset + len(tblIdent.RawTable),
		Replacement: stringz.DoubleQuote(tmpName),
	})

	nuDDL, err := sqlparser.ApplyEdits(ogDDL, edits)
	if err != nil {
		return errz.Wrap(err, "rqlite: alter table: failed to apply DDL rewrites")
	}

	// Read the prior foreign_keys pragma so we can restore it at the
	// end of the atomic batch rather than blindly forcing it on.
	// Matches the sqlite3 driver's pragmaDisableForeignKeys/restore
	// pattern in drivers/sqlite3/alter.go, adapted for rqlite's
	// no-interactive-transactions model: the restore value is inlined
	// as the last statement of the batch rather than captured in a
	// defer.
	var fkPrev int64
	if err = db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fkPrev); err != nil {
		return errz.Wrapf(errw(err), "rqlite: alter table: failed to read foreign_keys pragma")
	}

	// Capture the table's sqlite_sequence row (if any) outside the batch.
	// The DROP TABLE in the batch removes the row, so without a restore
	// the next AUTOINCREMENT insert would pick MAX(rowid)+1 rather than
	// seq+1, silently reusing rowids of previously deleted rows (gh757).
	srcSeq, err := readSqliteSequence(ctx, db, tbl)
	if err != nil {
		return err
	}

	stmts := []gorqlite.ParameterizedStatement{
		{Query: "PRAGMA foreign_keys=off"},
		{Query: nuDDL},
		{Query: fmt.Sprintf(`INSERT INTO %s SELECT * FROM %s`,
			stringz.DoubleQuote(tmpName), stringz.DoubleQuote(tbl))},
		{Query: "DROP TABLE " + stringz.DoubleQuote(tbl)},
		{Query: fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`,
			stringz.DoubleQuote(tmpName), stringz.DoubleQuote(tbl))},
	}

	if srcSeq.Valid {
		// Restore AUTOINCREMENT continuity (gh757). After the rename, the
		// rebuilt table's sqlite_sequence row, created by the copy and
		// renamed along with the table, holds MAX(rowid) of the copied
		// rows (0 if the copy moved no rows) rather than the original
		// seq. The UPDATE takes max(seq, captured) so the restore never
		// lowers the value already in place, even if the capture read
		// (outside the batch) returned a stale low value; the conditional
		// INSERT covers the case of no row existing. sqlite_sequence has
		// no unique constraint on name, which rules out INSERT OR
		// REPLACE.
		stmts = append(stmts,
			gorqlite.ParameterizedStatement{
				Query:     "UPDATE sqlite_sequence SET seq = max(seq, ?) WHERE name = ?",
				Arguments: []any{srcSeq.Int64, tbl},
			},
			gorqlite.ParameterizedStatement{
				Query: "INSERT INTO sqlite_sequence (name, seq) SELECT ?, ? " +
					"WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = ?)",
				Arguments: []any{tbl, srcSeq.Int64, tbl},
			},
		)
	}

	stmts = append(stmts, gorqlite.ParameterizedStatement{
		Query: fmt.Sprintf("PRAGMA foreign_keys=%d", fkPrev),
	})

	_, err = writeAtomic(ctx, db, stmts...)
	return err
}

// readSqliteSequence returns the sqlite_sequence row for tbl. The result is
// invalid (and err nil) if the sqlite_sequence table doesn't exist, or has
// no row for tbl. Mirrors the sqlite3 driver's helper of the same name.
func readSqliteSequence(ctx context.Context, db sqlz.DB, tbl string) (sql.NullInt64, error) {
	var seq sql.NullInt64

	// sqlite_sequence only exists once an AUTOINCREMENT table has been
	// created in the DB; querying it blindly would error.
	var n int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'",
	).Scan(&n); err != nil {
		return seq, errz.Wrap(errw(err),
			"rqlite: alter table: failed to check for sqlite_sequence table")
	}
	if n == 0 {
		return seq, nil
	}

	if err := db.QueryRowContext(ctx,
		"SELECT seq FROM sqlite_sequence WHERE name=?", tbl,
	).Scan(&seq); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return seq, errz.Wrapf(errw(err),
			"rqlite: alter table: failed to read sqlite_sequence for {%s}", tbl)
	}

	return seq, nil
}
