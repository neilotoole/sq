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

// Variant is one column created for a case.
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

// Vars holds per-variant expectations, one field per Variant. Each overrides
// the shared expectations for its variant, field by field. A struct rather
// than a map keyed by Variant, so that a case sets only the variants it
// needs; adding a variant adds a field here, and existing cases are
// unaffected.
type Vars struct {
	NotNull  Want
	Nullable Want
}

// get returns the expectations for v.
func (vs Vars) get(v Variant) Want {
	switch v {
	case NotNull:
		return vs.NotNull
	case Nullable:
		return vs.Nullable
	default:
		return Want{}
	}
}

// Override holds expectations for older servers: it applies when the server
// version is less than Ver.
type Override struct {
	// Ver is a canonical semver, e.g. "v8.0.19".
	Ver string

	// Want overrides Case.Want, field by field.
	Want Want

	// Vars overrides the expectations per variant, field by field.
	Vars Vars
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

	// Variants lists the columns to create. Nil means NotNull and Nullable.
	Variants []Variant

	// Before overrides expectations on older servers. Entries must be in
	// strictly ascending order of Ver; the first entry whose Ver is greater
	// than the server version applies.
	Before []Override

	// Want holds the expectations shared by every variant, on the newest
	// supported server version.
	Want Want

	// Vars overrides Want per variant, field by field.
	Vars Vars
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
