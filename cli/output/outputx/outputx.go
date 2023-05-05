// Package outputx contains extensions to pkg output, and helpers
// for implementing output writers.
package outputx

import (
	"reflect"

	"github.com/neilotoole/sq/libsq/core/options"
)

// VerboseOpt is a verbose realization of an options.Opt value.
type VerboseOpt struct {
	Key          string `json:"key"`
	Type         string `json:"type"`
	IsSet        bool   `json:"is_set"`
	DefaultValue any    `json:"default_value"`
	Value        any    `json:"value"`
	Comment      string `json:"comment"`
}

// NewVerboseOpt returns a VerboseOpt built from opt and o.
func NewVerboseOpt(opt options.Opt, o options.Options) VerboseOpt {
	v := VerboseOpt{
		Key:          opt.Key(),
		DefaultValue: opt.GetAny(nil),
		IsSet:        o.IsSet(opt),
		Comment:      opt.Comment(),
		Value:        opt.GetAny(o),
		Type:         reflect.TypeOf(opt.GetAny(nil)).String(),
	}

	return v
}
