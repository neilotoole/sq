// Package commonw contains miscellaneous common output writer functionality.
package commonw

import (
	"reflect"
	"strings"

	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// VerboseOpt is a verbose realization of an options.Opt value.
// This is used primarily to print metadata about the opt.
type VerboseOpt struct { //nolint:govet // field alignment
	Key          string `json:"key"`
	Usage        string `json:"usage"`
	Type         string `json:"type"`
	IsSet        bool   `json:"is_set"`
	DefaultValue any    `json:"default_value"`
	Value        any    `json:"value"`
	// FIXME: Append Flag?
	Help string `json:"help"`
}

// NewVerboseOpt returns a VerboseOpt built from opt and o.
func NewVerboseOpt(opt options.Opt, o options.Options) VerboseOpt {
	v := VerboseOpt{
		Key:          opt.Key(),
		Usage:        opt.Usage(),
		DefaultValue: opt.GetAny(nil),
		IsSet:        o.IsSet(opt),
		Help:         opt.Help(),
		Value:        opt.GetAny(o),
		Type:         reflect.TypeOf(opt.GetAny(nil)).String(),
	}

	return v
}

// ColumnKey returns the combined "PK,FK,UK" marker for a column, or "" when
// the column participates in no key.
func ColumnKey(col *metadata.Column, fkCols, ucCols map[string]bool) string {
	var parts []string
	if col.PrimaryKey {
		parts = append(parts, "PK")
	}
	if fkCols[col.Name] {
		parts = append(parts, "FK")
	}
	if ucCols[col.Name] {
		parts = append(parts, "UK")
	}
	return strings.Join(parts, ",")
}

// FKColumnSet returns the set of column names on tbl that participate in any
// outgoing foreign key.
func FKColumnSet(tbl *metadata.Table) map[string]bool {
	if tbl.FK == nil {
		return nil
	}
	set := make(map[string]bool)
	for _, fk := range tbl.FK.Outgoing {
		if fk == nil {
			continue
		}
		for _, c := range fk.Columns {
			set[c] = true
		}
	}
	return set
}

// UCColumnSet returns the set of column names on tbl that participate in any
// unique constraint.
func UCColumnSet(tbl *metadata.Table) map[string]bool {
	if len(tbl.UniqueConstraints) == 0 {
		return nil
	}
	set := make(map[string]bool)
	for _, uc := range tbl.UniqueConstraints {
		if uc == nil {
			continue
		}
		for _, c := range uc.Columns {
			set[c] = true
		}
	}
	return set
}
