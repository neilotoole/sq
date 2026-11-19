// Package typetest is a shared harness for testing how a SQL driver handles
// each of its database's column types. A driver declares a table of cases,
// one per column type, and Run checks what sq reports for each: the inspect
// fields (base_type, column_type, kind, nullable, default_value) from both
// the single-table and the source-wide inspect paths, the kind of the query
// result column, and the values read back through sq's query pipeline.
//
// Expectations record what sq does today, including values known to be
// wrong. A known-wrong expectation carries the issue that tracks it in
// Case.Issue, so that fixing the issue shows up as a reviewed change to the
// expected value. See https://github.com/neilotoole/sq/issues/1368.
//
// # Rows
//
// Each case is declared as one column per variant: by default a NOT NULL
// column, a nullable one, and a NOT NULL one with a declared default. Run
// inserts three rows into the table, giving every column but the defaulted
// one a value in each:
//
//	row_id | NotNull column | Nullable column           | Defaulted column
//	-------+----------------+---------------------------+-----------------
//	1      | Case.Zero      | Case.Zero                 | (left out)
//	2      | Case.Val       | Case.Val                  | (left out)
//	3      | Case.Val       | NULL (bound as NullInput) | (left out)
//
// Row 1 holds a zero value (0, "", the zero time) and row 2 a non-zero one,
// usually an edge value such as the type's maximum, so a value that reads back
// wrong shows up in one row or the other, and the two can't be confused.
// Row 3 is where the nullable column reads back a real NULL. The NOT NULL
// column can't hold NULL, so it repeats Val there.
//
// Every query orders by row_id, and Run checks that row_id reads back as 1,
// 2 and 3, so a value check failure is about the value, not row order. A
// column's expected values for the three rows are Want.Zero, Want.Val (each
// defaulting to its input) and, in row 3, Want.Val for NotNull and NULL for
// Nullable. Column.In and Column.Want hold them after Resolve.
//
// The Defaulted column is never inserted, so it takes its default in every
// row. The default is the SQL literal for Case.Val (Config.Literal, rewritten
// by Config.DefaultExpr, or Case.DefaultExpr in its place), so the column's
// expected value is the same in all three rows: Want.LiteralVal, defaulting
// to Want.Val. Its expected default_value is required: set it with
// Case.WithWantDefault. A type that can't take a default leaves Defaulted out
// of Case.Variants, or of an Override's Variants on older servers.
//
// # Generated DDL
//
// A driver's case tables also generate testdata/coltypes.ddl, for manual and
// ad hoc testing: the tables the column-type tests create, with every case
// for the newest server version, and their three rows as INSERT statements
// with each input written as a SQL literal in the driver's dialect
// (Config.Literal, falling back to DefaultLiteral). The literals store what
// the bound inserts store, where the dialect allows. Config.DDLPrelude starts
// the file with any session settings its literals assume. Load it with the
// database's own client to query the tables or inspect them with sq.
//
// The driver's TestColumnTypesDDL calls CheckDDL, which renders the file and
// compares it with the one in testdata. It needs no database, so it runs
// under -short, and a case table changed without regenerating the file fails
// at once. Set EnvUpdateDDL to true to regenerate it.
//
// Run's ddl subtest renders the same statements for the connected server's
// version, executes them one by one through the Go driver, on a session set
// up by Config.DDLCheckSession to an odd time zone, and runs the whole-table
// value checks against the result. That proves the literals load on each
// server version the tests run against, and read back as expected wherever
// the loading session's time zone is. It doesn't run the committed file
// itself, through a native client: a case gated by MinVer, or a variant gated
// by an Override's Variants, is in the file but can fail on an older server.
// MySQL's file, for one, doesn't load on 5.x. A row loaded from literals is expected to read
// back the same as the bound row, unless Want.LiteralZero or Want.LiteralVal
// says otherwise: typically where the driver mangles a bound value that the
// literal stores intact. A column with a QueryErr is queried on its own, and
// expected to fail as it does in the bound table.
package typetest

import (
	"github.com/neilotoole/sq/libsq/core/kind"
)

// Variant identifies one of the columns created for a case. A case tests one
// column type, but declares it as several columns that differ in a single
// property, such as nullability or a declared default, so that each variant's
// inspect fields and values are checked separately. For example, the case
// "int" creates the columns "col_int" (NotNull), "col_int_n" (Nullable) and
// "col_int_d" (Defaulted), all of type INT.
//
// Case.Variants selects which variants a case creates. Expectations that
// apply to only one variant go in PerVariant.
type Variant int

const (
	// NotNull is the column "col_<name>", declared NOT NULL.
	NotNull Variant = iota

	// Nullable is the column "col_<name>_n", declared NULL.
	Nullable

	// Defaulted is the column "col_<name>_d", declared NOT NULL with a
	// default, and left out of every INSERT, so that every row reads back the
	// default. The default expression is the SQL literal for Case.Val (see
	// Config.DefaultExpr and Case.DefaultExpr). Its expected default_value
	// must be set, with Case.WithWantDefault.
	Defaulted
)

// String implements fmt.Stringer.
func (v Variant) String() string {
	switch v {
	case NotNull:
		return "not_null"
	case Nullable:
		return "nullable"
	case Defaulted:
		return "defaulted"
	default:
		return "unknown"
	}
}

// suffix returns the column name suffix for v.
func (v Variant) suffix() string {
	switch v {
	case NotNull:
		return ""
	case Nullable:
		return "_n"
	case Defaulted:
		return "_d"
	default:
		return ""
	}
}

// defaultVariants is the variants a case gets when Case.Variants is nil.
var defaultVariants = []Variant{NotNull, Nullable, Defaulted}

// Opt is an optional expectation with an explicit presence flag, so that
// "unset" is distinct from every value, including kind.Unknown (the zero
// kind.Kind) and nil (an expected SQL NULL).
type Opt[T any] struct {
	Val T
	Set bool
}

// Is returns an Opt that is set to v.
func Is[T any](v T) Opt[T] {
	return Opt[T]{Set: true, Val: v}
}

// WantNull returns an expected record value of SQL NULL.
func WantNull() Opt[any] {
	return Opt[any]{Set: true, Val: nil}
}

// SQL is a raw SQL expression used as an input in place of a bound
// parameter, for values a driver can't bind, e.g.
// SQL("ST_GeomFromText('POINT(1 1)')"). A SQL input always needs an explicit
// expected value.
type SQL string

// Want is a set of expectations. Every field is optional, so that Wants
// layer field by field: see Resolve.
type Want struct {
	// Zero and Val are the expected record values for the Zero and Val
	// inputs. They default to the inputs themselves.
	Zero, Val Opt[any]

	// LiteralZero and LiteralVal are the expected record values for the Zero
	// and Val inputs when the row is loaded from the generated DDL file, with
	// each input written as a SQL literal (see CheckDDL). They default to Zero
	// and Val. Set one to record a value that the bound insert and the literal
	// store differently: typically the driver mangling a bound value that the
	// literal stores intact. They layer as the other fields do (see Resolve),
	// with one difference: a layer that sets Val but not LiteralVal clears
	// the LiteralVal of earlier layers, and one that sets Zero but not
	// LiteralZero clears their LiteralZero, since it says what both paths read
	// back. A layer that sets both keeps them apart. Within one layer, the
	// builder methods (WithWant, WithPerVariant, ...) merge field by field and
	// clear nothing.
	LiteralZero, LiteralVal Opt[any]

	// BaseType is the expected base_type. Required after resolution.
	BaseType Opt[string]

	// ColumnType is the expected column_type. Required after resolution.
	ColumnType Opt[string]

	// Default is the expected default_value. Defaults to "" (no default).
	Default Opt[string]

	// QueryErr, if set, records that querying the column fails today. The
	// error must contain both the column name and this text.
	QueryErr Opt[string]

	// Kind is the expected inspect kind. Required after resolution.
	Kind Opt[kind.Kind]

	// QueryKind is the expected kind of the query result column. Defaults to
	// Kind.
	QueryKind Opt[kind.Kind]

	// Nullable is the expected nullable. Defaults to what the variant
	// declares: false for NotNull, true for Nullable. Set it to record a
	// database reporting a column's nullability wrongly.
	Nullable Opt[bool]
}

// PerVariant holds expectations that apply to only one Variant of a case,
// with one field per Variant. Each field overrides, field by field, the Want
// shared by every variant. For example, if only the nullable column fails to
// query:
//
//	c.PerVariant = PerVariant{Nullable: Want{QueryErr: Is("sql.Null[uint64]")}}
//
// Fields left as the zero Want change nothing. PerVariant is a struct rather
// than a map keyed by Variant, so that a case names only the variants it
// overrides, and adding a Variant adds a field here without touching
// existing cases.
type PerVariant struct {
	NotNull   Want
	Nullable  Want
	Defaulted Want
}

// get returns the expectations for v.
func (pv PerVariant) get(v Variant) Want {
	switch v {
	case NotNull:
		return pv.NotNull
	case Nullable:
		return pv.Nullable
	case Defaulted:
		return pv.Defaulted
	default:
		return Want{}
	}
}

// Override holds expectations for older servers: it applies when the server
// version is less than Before.
type Override struct {
	// Before is the first server version the override doesn't apply to, a
	// canonical semver such as "v8.0.19". The override applies to servers
	// older than Before.
	Before string

	// Want overrides Case.Want, field by field.
	Want Want

	// Variants, if non-nil, replaces Case.Variants on servers older than
	// Before: for a type that can take a default only from some version, for
	// example. Like the rest of an Override, it applies only if this is the
	// first override whose Before is greater than the server version.
	Variants []Variant

	// PerVariant overrides this Override's Want for individual variants,
	// field by field.
	PerVariant PerVariant
}

// Case is one column type to test.
type Case struct {
	// Zero and Val are the inputs for rows 1 and 2, and Val also for row 3 of
	// the NotNull column (see the package doc's Rows section): a Go value
	// bound as a query parameter, or a SQL expression. Inputs are the same on
	// every server version and every variant.
	Zero, Val any

	// NullInput is what's bound for NULL in row 3 of the Nullable column.
	// Nil, the default, binds a Go nil. Set it when a driver can't bind a
	// plain nil for the type: go-mssqldb, for example, sends nil as an
	// nvarchar NULL, which a BINARY column rejects, but accepts []byte(nil).
	// The expected value is NULL either way.
	NullInput any

	// Name is the column base name, e.g. "varchar_16". It must match
	// ^[a-z][a-z0-9_]*$.
	Name string

	// Decl is the type as written in a column definition, e.g. "VARCHAR(16)".
	Decl string

	// DefaultExpr, if set, is the Defaulted column's default expression,
	// used as is. Leave it empty for the SQL literal for Val, rewritten by
	// Config.DefaultExpr if that's set: set it only where neither will do.
	// The column's expected value still comes from Val (Want.LiteralVal, then
	// Want.Val), so an expression that reads back as something else needs
	// PerVariant.Defaulted's Val set to match.
	DefaultExpr string

	// MinVer, if set, excludes the case on servers older than this canonical
	// semver.
	MinVer string

	// Issue references the issue tracking a known-wrong expectation, e.g.
	// "#1363". Documentation only: the test asserts the expectation
	// regardless.
	Issue string

	// Variants selects which columns to create for the case, one per listed
	// Variant, in the order listed. Leave it nil to create the default
	// variants (see defaultVariants). Set it only when the type can't be
	// declared one of those ways, e.g.
	// []Variant{Nullable} for a type the database won't accept as NOT NULL.
	//
	// A non-nil empty slice is an error, as is listing a Variant twice:
	// Resolve reports both rather than silently testing nothing or creating
	// a duplicate column.
	Variants []Variant

	// VersionOverrides overrides Want and PerVariant on older servers.
	// Entries must be in strictly ascending order of Before. On a given
	// server, the first entry whose Before is greater than the server version
	// applies, and the rest are ignored.
	VersionOverrides []Override

	// Want holds the expectations shared by every variant, on the newest
	// supported server version.
	Want Want

	// PerVariant overrides Want for individual variants, field by field.
	PerVariant PerVariant
}

// Column is one resolved column: one case, one variant, one server version.
type Column struct {
	// In holds the inputs for rows 1 to 3. Row 3 of a Nullable column is
	// Case.NullInput (nil by default). See the package doc's Rows section.
	In [3]any

	// Want holds the expected record values for rows 1 to 3.
	Want [3]any

	// LiteralWant holds the expected record values for rows 1 to 3 when
	// they're loaded from the generated DDL file. See Want.LiteralVal.
	LiteralWant [3]any

	// Case is the case the column was resolved from.
	Case *Case

	// Name is the column name: "col_" + Case.Name + the variant's suffix.
	Name string

	// DefaultExpr is the default expression of a Defaulted column, set by
	// ResolveDefaultExprs once the column's literal is rendered. It's empty
	// for the other variants.
	DefaultExpr string

	BaseType   string
	ColumnType string
	Default    string

	// QueryErr, if set, is expected text of the column's query error.
	QueryErr Opt[string]

	Kind      kind.Kind
	QueryKind kind.Kind

	Variant  Variant
	Nullable bool
}

// Inserted reports whether c is given a value by the INSERT statements: every
// column but a Defaulted one, which is left out so that it takes its default.
func (c Column) Inserted() bool {
	return c.Variant != Defaulted
}
