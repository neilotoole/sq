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
package typetest

import (
	"github.com/neilotoole/sq/libsq/core/kind"
)

// Variant identifies one of the columns created for a case. A case tests one
// column type, but declares it as several columns that differ in a single
// property, such as nullability, so that each variant's inspect fields and
// values are checked separately. For example, the case "int" creates the
// columns "col_int" (NotNull) and "col_int_n" (Nullable), both of type INT.
//
// Case.Variants selects which variants a case creates. Expectations that
// apply to only one variant go in PerVariant.
type Variant int

const (
	// NotNull is the column "col_<name>", declared NOT NULL.
	NotNull Variant = iota

	// Nullable is the column "col_<name>_n", declared NULL.
	Nullable
)

// String implements fmt.Stringer.
func (v Variant) String() string {
	switch v {
	case NotNull:
		return "not_null"
	case Nullable:
		return "nullable"
	default:
		return "unknown"
	}
}

// suffix returns the column name suffix for v.
func (v Variant) suffix() string {
	if v == Nullable {
		return "_n"
	}
	return ""
}

// defaultVariants is the variants a case gets when Case.Variants is nil.
var defaultVariants = []Variant{NotNull, Nullable}

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
	NotNull  Want
	Nullable Want
}

// get returns the expectations for v.
func (pv PerVariant) get(v Variant) Want {
	switch v {
	case NotNull:
		return pv.NotNull
	case Nullable:
		return pv.Nullable
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

	// PerVariant overrides this Override's Want for individual variants,
	// field by field.
	PerVariant PerVariant
}

// Case is one column type to test.
type Case struct {
	// Zero and Val are the inputs for rows 1 and 2: a Go value bound as a
	// query parameter, or a SQL expression. Inputs are the same on every
	// server version and every variant.
	Zero, Val any

	// Name is the column base name, e.g. "varchar_16". It must match
	// ^[a-z][a-z0-9_]*$.
	Name string

	// Decl is the type as written in a column definition, e.g. "VARCHAR(16)".
	Decl string

	// MinVer, if set, excludes the case on servers older than this canonical
	// semver.
	MinVer string

	// Issue references the issue tracking a known-wrong expectation, e.g.
	// "#1363". Documentation only: the test asserts the expectation
	// regardless.
	Issue string

	// Variants selects which columns to create for the case, one per listed
	// Variant, in the order listed. Leave it nil to create the usual pair,
	// NotNull ("col_<name>") then Nullable ("col_<name>_n"). Set it only
	// when the type can't be declared one of those ways, e.g.
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
	// In holds the inputs for rows 1 to 3. Row 3 of a Nullable column is nil.
	In [3]any

	// Want holds the expected record values for rows 1 to 3.
	Want [3]any

	// Case is the case the column was resolved from.
	Case *Case

	// Name is the column name: "col_" + Case.Name + the variant's suffix.
	Name string

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
