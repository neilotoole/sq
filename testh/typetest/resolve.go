package typetest

import (
	"regexp"
	"slices"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
)

var caseNameRx = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Resolve returns the columns for cases on a server at serverVer, a
// canonical semver such as "v8.0.36". Cases whose MinVer is greater than
// serverVer are excluded. Each remaining case yields one column per variant,
// in case order.
//
// A column's expectations start from the built-in fallbacks (QueryKind from
// Kind, Default "", Zero and Val from the inputs, no QueryErr), then layer,
// field by field where set: Case.Want, Case.PerVariant for the variant, the
// first VersionOverrides entry whose Before is greater than serverVer, then
// that entry's PerVariant for the variant.
//
// Resolve returns an error for a mistake in the case table, rather than
// letting it surface as a confusing assertion failure: an invalid name or
// semver, VersionOverrides out of order, colliding column names, a required
// expectation left unset, a SQL input with no expected value, or an empty
// QueryErr.
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

	if c.Variants != nil && len(c.Variants) == 0 {
		return errz.Errorf("typetest: case {%s}: Variants is empty: use nil for both variants", c.Name)
	}

	for i, v := range c.Variants {
		if slices.Contains(c.Variants[:i], v) {
			return errz.Errorf("typetest: case {%s}: variant {%s} is listed twice in Variants", c.Name, v)
		}
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
	}

	return nil
}

// resolveColumn resolves the expectations for one variant of c.
func resolveColumn(c *Case, v Variant, serverVer string) (Column, error) {
	w := Want{}
	layer(&w, c.Want)
	layer(&w, c.PerVariant.get(v))
	for _, o := range c.VersionOverrides {
		if semver.Compare(serverVer, o.Before) < 0 {
			layer(&w, o.Want)
			layer(&w, o.PerVariant.get(v))
			break
		}
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

	col.In = [3]any{c.Zero, c.Val, c.Val}
	col.Want = [3]any{zeroWant, valWant, valWant}
	if v == Nullable {
		col.In[2], col.Want[2] = nil, nil
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

// layer copies each set field of src onto dst.
func layer(dst *Want, src Want) {
	setIf(&dst.BaseType, src.BaseType)
	setIf(&dst.ColumnType, src.ColumnType)
	setIf(&dst.Kind, src.Kind)
	setIf(&dst.QueryKind, src.QueryKind)
	setIf(&dst.Default, src.Default)
	setIf(&dst.Zero, src.Zero)
	setIf(&dst.Val, src.Val)
	setIf(&dst.QueryErr, src.QueryErr)
}

func setIf[T any](dst *Opt[T], src Opt[T]) {
	if src.Set {
		*dst = src
	}
}
