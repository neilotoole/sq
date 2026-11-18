package typetest

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// DiffMeta compares the inspect metadata got with want's expected
// base_type, column_type, kind, nullable and default_value. path names the
// inspect path that produced got, e.g. "TableMetadata" or "SourceMetadata".
// It returns one message per mismatched field, or nil if all match. Each
// message names path, the column, its declaration, serverVer and the field.
func DiffMeta(path, serverVer string, got *metadata.Column, want Column) []string {
	var msgs []string
	diff := func(field string, wantVal, gotVal any) {
		if wantVal == gotVal {
			return
		}
		msgs = append(msgs, fmt.Sprintf("%s: column %s (%s) on server %s: %s: want %#v, got %#v",
			path, want.Name, want.Case.Decl, serverVer, field, wantVal, gotVal))
	}

	diff("base_type", want.BaseType, got.BaseType)
	diff("column_type", want.ColumnType, got.ColumnType)
	diff("kind", want.Kind, got.Kind)
	diff("nullable", want.Nullable, got.Nullable)
	diff("default_value", want.Default, got.DefaultValue)
	return msgs
}

// MatchQueryErr returns nil if err matches col's expected query error: err
// is non-nil, and its text contains both col.Name and col.QueryErr.Val.
// Requiring the column name keeps an unrelated error, or another column's,
// from satisfying the expectation.
func MatchQueryErr(err error, col Column) error {
	if err == nil {
		return errz.Errorf("column %s (%s): want query error containing {%s}, got no error",
			col.Name, col.Case.Decl, col.QueryErr.Val)
	}

	msg := err.Error()
	if !strings.Contains(msg, col.Name) || !strings.Contains(msg, col.QueryErr.Val) {
		return errz.Errorf("column %s (%s): want query error containing {%s} and the column name, got: %s",
			col.Name, col.Case.Decl, col.QueryErr.Val, msg)
	}
	return nil
}
