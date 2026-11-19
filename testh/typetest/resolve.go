package typetest

import (
	"fmt"
	"regexp"
	"slices"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
)

var caseNameRx = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Resolve returns the columns for cases on a server at serverVer, a
// canonical semver such as "v8.0.36". Cases whose MinVer is greater than
// serverVer are excluded. Each remaining case yields one column per variant
// it creates on serverVer, in case order: the variants of the first matching
// VersionOverrides entry if it sets Variants, otherwise Case.Variants,
// otherwise the default variants.
//
// A column's expectations start from the built-in fallbacks (QueryKind from
// Kind, Default "", Zero and Val from the inputs, no QueryErr), then layer,
// field by field where set: Case.Want, Case.PerVariant for the variant, the
// first VersionOverrides entry whose Before is greater than serverVer, then
// that entry's PerVariant for the variant. Want.LiteralVal describes the one
// exception to field-by-field layering.
//
// Resolve returns an error for a mistake in the case table, rather than
// letting it surface as a confusing assertion failure: an invalid name or
// semver, VersionOverrides out of order, an empty or repeating variant list
// (Case.Variants or Override.Variants), colliding column names, a required
// expectation left unset (Default, for a Defaulted column), a SQL input with
// no expected value, an empty QueryErr, or Zero or LiteralZero set for the
// Defaulted variant, which has no Zero row.
func Resolve(cases []Case, serverVer string) ([]Column, error) {
	if !semver.IsValid(serverVer) {
		return nil, errz.Errorf("typetest: invalid server version {%s}: must be a canonical semver",
			serverVer)
	}

	var cols []Column
	seen := map[string]string{} // column name -> case name
	for i := range cases {
		c := &cases[i]
		if err := validateCase(c); err != nil {
			return nil, err
		}

		if c.MinVer != "" && semver.Compare(serverVer, c.MinVer) < 0 {
			continue
		}

		variants := c.Variants
		if o := matchingOverride(c, serverVer); o != nil && o.Variants != nil {
			variants = o.Variants
		}
		if variants == nil {
			variants = defaultVariants
		}

		for _, v := range variants {
			col, err := resolveColumn(c, v, serverVer)
			if err != nil {
				return nil, err
			}

			if other, ok := seen[col.Name]; ok {
				return nil, errz.Errorf("typetest: case {%s}: column {%s} collides with case {%s}",
					c.Name, col.Name, other)
			}
			seen[col.Name] = c.Name
			cols = append(cols, col)
		}
	}

	return cols, nil
}

// validateCase checks c's fields that don't depend on the server version.
func validateCase(c *Case) error {
	if !caseNameRx.MatchString(c.Name) {
		return errz.Errorf("typetest: case {%s}: name must match %s", c.Name, caseNameRx)
	}

	if err := validateVariants(c, "Variants", c.Variants); err != nil {
		return err
	}

	if c.MinVer != "" && !semver.IsValid(c.MinVer) {
		return errz.Errorf("typetest: case {%s}: invalid MinVer {%s}: must be a canonical semver",
			c.Name, c.MinVer)
	}

	for i, o := range c.VersionOverrides {
		if !semver.IsValid(o.Before) {
			return errz.Errorf("typetest: case {%s}: VersionOverrides[%d]: invalid Before {%s}: must be a canonical semver",
				c.Name, i, o.Before)
		}
		if i > 0 && semver.Compare(c.VersionOverrides[i-1].Before, o.Before) >= 0 {
			return errz.Errorf(
				"typetest: case {%s}: VersionOverrides must be in strictly ascending order of Before: {%s} then {%s}",
				c.Name, c.VersionOverrides[i-1].Before, o.Before,
			)
		}
		if err := validateVariants(c, fmt.Sprintf("VersionOverrides[%d].Variants", i), o.Variants); err != nil {
			return err
		}
	}

	for _, pv := range append([]PerVariant{c.PerVariant}, overridePerVariants(c)...) {
		if pv.Defaulted.Zero.Set || pv.Defaulted.LiteralZero.Set {
			return errz.Errorf("typetest: case {%s}: PerVariant.Defaulted sets Zero or LiteralZero, but the "+
				"Defaulted column has no Zero row: every row reads back its default", c.Name)
		}
	}

	return nil
}

// validateVariants checks a variant list of c, named field: nil, or
// non-empty with no variant listed twice.
func validateVariants(c *Case, field string, vs []Variant) error {
	if vs != nil && len(vs) == 0 {
		return errz.Errorf("typetest: case {%s}: %s is empty: use nil for the default variants", c.Name, field)
	}
	for i, v := range vs {
		if slices.Contains(vs[:i], v) {
			return errz.Errorf("typetest: case {%s}: variant {%s} is listed twice in %s", c.Name, v, field)
		}
	}
	return nil
}

// overridePerVariants returns the PerVariant of each of c's VersionOverrides.
func overridePerVariants(c *Case) []PerVariant {
	pvs := make([]PerVariant, len(c.VersionOverrides))
	for i, o := range c.VersionOverrides {
		pvs[i] = o.PerVariant
	}
	return pvs
}

// matchingOverride returns the override of c that applies on a server at
// serverVer: the first whose Before is greater than serverVer. It returns
// nil if none applies.
func matchingOverride(c *Case, serverVer string) *Override {
	for i := range c.VersionOverrides {
		if semver.Compare(serverVer, c.VersionOverrides[i].Before) < 0 {
			return &c.VersionOverrides[i]
		}
	}
	return nil
}

// resolveColumn resolves the expectations for one variant of c.
func resolveColumn(c *Case, v Variant, serverVer string) (Column, error) {
	w := Want{}
	resolveLayer(&w, c.Want)
	resolveLayer(&w, c.PerVariant.get(v))
	if o := matchingOverride(c, serverVer); o != nil {
		resolveLayer(&w, o.Want)
		resolveLayer(&w, o.PerVariant.get(v))
	}

	col := Column{
		Case:     c,
		Variant:  v,
		Name:     "col_" + c.Name + v.suffix(),
		Nullable: v == Nullable,
		QueryErr: w.QueryErr,
	}

	switch {
	case !w.BaseType.Set:
		return Column{}, missingErr(c, v, "BaseType")
	case !w.ColumnType.Set:
		return Column{}, missingErr(c, v, "ColumnType")
	case !w.Kind.Set:
		return Column{}, missingErr(c, v, "Kind")
	}
	if w.Nullable.Set {
		col.Nullable = w.Nullable.Val
	}
	col.BaseType = w.BaseType.Val
	col.ColumnType = w.ColumnType.Val
	col.Kind = w.Kind.Val

	col.QueryKind = col.Kind
	if w.QueryKind.Set {
		col.QueryKind = w.QueryKind.Val
	}
	if w.Default.Set {
		col.Default = w.Default.Val
	}

	if w.QueryErr.Set && w.QueryErr.Val == "" {
		return Column{}, errz.Errorf("typetest: case {%s}: variant {%s}: QueryErr is set but empty",
			c.Name, v)
	}

	zeroWant, err := expectedValue(c, v, "Zero", c.Zero, w.Zero)
	if err != nil {
		return Column{}, err
	}
	valWant, err := expectedValue(c, v, "Val", c.Val, w.Val)
	if err != nil {
		return Column{}, err
	}

	litZeroWant, litValWant := zeroWant, valWant
	if w.LiteralZero.Set {
		litZeroWant = w.LiteralZero.Val
	}
	if w.LiteralVal.Set {
		litValWant = w.LiteralVal.Val
	}

	if v == Defaulted {
		// The column is never inserted: every row reads back its default,
		// the SQL literal for Val, on both the bound and the ddl table.
		if !w.Default.Set {
			return Column{}, missingErr(c, v, "Default")
		}
		col.Want = [3]any{litValWant, litValWant, litValWant}
		col.LiteralWant = col.Want
		return col, nil
	}

	col.In = [3]any{c.Zero, c.Val, c.Val}
	col.Want = [3]any{zeroWant, valWant, valWant}
	col.LiteralWant = [3]any{litZeroWant, litValWant, litValWant}
	if v == Nullable {
		col.In[2], col.Want[2], col.LiteralWant[2] = c.NullInput, nil, nil
	}

	return col, nil
}

// expectedValue returns the expected record value for an input: the
// expectation if set, otherwise the input itself. A SQL input has no usable
// fallback, so its expectation must be set.
func expectedValue(c *Case, v Variant, field string, in any, want Opt[any]) (any, error) {
	if want.Set {
		return want.Val, nil
	}
	if _, ok := in.(SQL); ok {
		return nil, errz.Errorf(
			"typetest: case {%s}: variant {%s}: %s input is a SQL expression, so its expected value must be set",
			c.Name, v, field,
		)
	}
	return in, nil
}

func missingErr(c *Case, v Variant, field string) error {
	return errz.Errorf("typetest: case {%s}: variant {%s}: expectation %s is not set", c.Name, v, field)
}

// resolveLayer layers src onto dst for Resolve, as layer does, except that
// a src that sets Zero or Val without its literal counterpart first clears
// dst's LiteralZero or LiteralVal: a later layer that says what Zero or Val
// reads back says it for both paths. See Want.LiteralVal.
func resolveLayer(dst *Want, src Want) {
	if src.Zero.Set && !src.LiteralZero.Set {
		dst.LiteralZero = Opt[any]{}
	}
	if src.Val.Set && !src.LiteralVal.Set {
		dst.LiteralVal = Opt[any]{}
	}
	layer(dst, src)
}

// layer copies each set field of src onto dst. The builder methods merge
// with it, field by field, within one layer.
func layer(dst *Want, src Want) {
	setIf(&dst.BaseType, src.BaseType)
	setIf(&dst.ColumnType, src.ColumnType)
	setIf(&dst.Kind, src.Kind)
	setIf(&dst.QueryKind, src.QueryKind)
	setIf(&dst.Default, src.Default)
	setIf(&dst.Zero, src.Zero)
	setIf(&dst.Val, src.Val)
	setIf(&dst.LiteralZero, src.LiteralZero)
	setIf(&dst.LiteralVal, src.LiteralVal)
	setIf(&dst.QueryErr, src.QueryErr)
	setIf(&dst.Nullable, src.Nullable)
}

func setIf[T any](dst *Opt[T], src Opt[T]) {
	if src.Set {
		*dst = src
	}
}
