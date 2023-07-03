// Package jointype enumerates the various SQL JOIN types.
package jointype

import "github.com/neilotoole/sq/libsq/core/errz"

// Type indicates the type of join, e.g. "INNER JOIN"
// or "RIGHT OUTER JOIN", etc.
type Type string

// String returns the string value.
func (jt Type) String() string {
	return string(jt)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (jt *Type) UnmarshalText(text []byte) error {
	switch string(text) {
	case string(Inner), JoinAlias:
		*jt = Inner
	case string(Left), LeftAlias:
		*jt = Left
	case string(LeftOuter), LeftOuterAlias:
		*jt = LeftOuter
	case string(Right), RightAlias:
		*jt = Right
	case string(RightOuter), RightOuterAlias:
		*jt = RightOuter
	case string(FullOuter), FullOuterAlias:
		*jt = FullOuter
	case string(Cross), CrossAlias:
		*jt = Cross
	default:
		return errz.Errorf("invalid join type {%s}", string(text))
	}

	return nil
}

// HasPredicate returns true if the join type accepts a
// join predicate. Only jointype.Cross returns false.
func (jt Type) HasPredicate() bool {
	return jt != Cross
}

const (
	Inner           Type   = "inner_join"
	JoinAlias       string = "join"
	Left            Type   = "left_join"
	LeftAlias       string = "ljoin"
	LeftOuter       Type   = "left_outer_join"
	LeftOuterAlias  string = "lojoin"
	Right           Type   = "right_join"
	RightAlias      string = "rjoin"
	RightOuter      Type   = "right_outer_join"
	RightOuterAlias string = "rojoin"
	FullOuter       Type   = "full_outer_join"
	FullOuterAlias  string = "fojoin"
	Cross           Type   = "cross_join"
	CrossAlias      string = "xjoin"
)

// All returns the set of join.Type values.
func All() []Type {
	return []Type{
		Inner,
		Left,
		LeftOuter,
		Right,
		RightOuter,
		FullOuter,
		Cross,
	}
}

// AllValues returns all possible join type values, including
// both canonical names ("cross_join") and aliases ("xjoin").
func AllValues() []string {
	return []string{
		JoinAlias,
		string(Inner),
		string(Left),
		LeftAlias,
		string(LeftOuter),
		LeftOuterAlias,
		string(Right),
		RightAlias,
		string(RightOuter),
		RightOuterAlias,
		string(FullOuter),
		FullOuterAlias,
		string(Cross),
		CrossAlias,
	}
}
