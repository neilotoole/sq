package typetest

import (
	"slices"

	"github.com/neilotoole/sq/libsq/core/kind"
)

// NewCase returns a case for the column type decl, with inputs zero and val,
// and the inspect expectations every case needs: baseType, columnType and
// kind k. The With methods add the rest, for example:
//
//	NewCase("year", "YEAR", int64(0), int64(7), "year", "year", kind.Int).
//		WithWant(Want{Val: Is[any](int64(2007))}).
//		WithVersionOverrides(Override{Before: "v8.0.19", Want: Want{ColumnType: Is("year(4)")}})
func NewCase(name, decl string, zero, val any, baseType, columnType string, k kind.Kind) Case {
	return Case{
		Name: name,
		Decl: decl,
		Zero: zero,
		Val:  val,
		Want: Want{
			BaseType:   Is(baseType),
			ColumnType: Is(columnType),
			Kind:       Is(k),
		},
	}
}

// The With methods return a modified copy of the case, so that a base case
// can be reused. Slices are copied before they're appended to.

// WithWant returns a copy of c with w layered onto c.Want, field by field:
// fields set in w replace those in c.Want, and the rest are kept.
func (c Case) WithWant(w Want) Case {
	layer(&c.Want, w)
	return c
}

// WithPerVariant returns a copy of c with pv layered onto c.PerVariant,
// variant by variant and field by field.
func (c Case) WithPerVariant(pv PerVariant) Case {
	layer(&c.PerVariant.NotNull, pv.NotNull)
	layer(&c.PerVariant.Nullable, pv.Nullable)
	return c
}

// WithVersionOverrides returns a copy of c with overrides appended to
// c.VersionOverrides. Resolve requires the result to be in strictly
// ascending order of Override.Before.
func (c Case) WithVersionOverrides(overrides ...Override) Case {
	c.VersionOverrides = append(slices.Clone(c.VersionOverrides), overrides...)
	return c
}

// WithVariants returns a copy of c with Variants set to vs. Calling it with no
// arguments sets an empty, non-nil Variants, which Resolve rejects: to get
// the default variants, don't call WithVariants.
func (c Case) WithVariants(vs ...Variant) Case {
	c.Variants = append([]Variant{}, vs...)
	return c
}

// WithMinVer returns a copy of c with MinVer set to ver.
func (c Case) WithMinVer(ver string) Case {
	c.MinVer = ver
	return c
}

// WithNullInput returns a copy of c that binds v for NULL in the nullable
// column's row 3. See Case.NullInput.
func (c Case) WithNullInput(v any) Case {
	c.NullInput = v
	return c
}

// Creates reports whether c creates a column for variant v: whether v is in
// c.Variants, or in the default variants when c.Variants is nil.
func (c Case) Creates(v Variant) bool {
	if c.Variants == nil {
		return slices.Contains(defaultVariants, v)
	}
	return slices.Contains(c.Variants, v)
}

// WithIssue returns a copy of c with Issue set to issue.
func (c Case) WithIssue(issue string) Case {
	c.Issue = issue
	return c
}

// WithWantZero returns a copy of c expecting the Zero input to read back as
// v, for every variant. A nil v expects NULL.
func (c Case) WithWantZero(v any) Case {
	c.Want.Zero = Is(v)
	return c
}

// WithWantVal returns a copy of c expecting the Val input to read back as v,
// for every variant. A nil v expects NULL.
func (c Case) WithWantVal(v any) Case {
	c.Want.Val = Is(v)
	return c
}

// WithWantLiteralVal returns a copy of c expecting the Val input, loaded from
// the generated DDL file as a SQL literal, to read back as v, for every
// variant. See Want.LiteralVal.
func (c Case) WithWantLiteralVal(v any) Case {
	c.Want.LiteralVal = Is(v)
	return c
}
