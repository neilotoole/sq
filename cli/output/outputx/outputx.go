// Package outputx contains extensions to pkg output, and helpers
// for implementing output writers.
//
// REVISIT: This package is a bit of an odd duck. Can the functionality
// be moved elsewhere?
package outputx

import (
	"reflect"

	"github.com/neilotoole/sq/libsq/core/options"
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
