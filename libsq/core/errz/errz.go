// Package errz is sq's error package. It exists to combine
// functionality from several error packages, including
// annotating errors with stack trace.
//
// At some point this package may become redundant, particularly in
// light of the proposed stdlib multiple error support:
// https://github.com/golang/go/issues/53435
package errz

import (
	pkgerrors "github.com/pkg/errors"
	"go.uber.org/multierr"
)

// Err is documented by pkg/errors.WithStack.
var Err = pkgerrors.WithStack

// Wrap is documented by pkg/errors.Wrap.
var Wrap = pkgerrors.Wrap

// Wrapf is documented by pkg/errors.Wrapf.
var Wrapf = pkgerrors.Wrapf

// New is documented by pkg/errors.New.
var New = pkgerrors.New

// Errorf is documented by pkg/errors.Errorf.
var Errorf = pkgerrors.Errorf

// Cause is documented by pkg/errors.Cause.
var Cause = pkgerrors.Cause

// Append is documented by multierr.Append.
var Append = multierr.Append

// Combine is documented by multierr.Combine.
var Combine = multierr.Combine

// Errors is documented by multierr.Errors.
var Errors = multierr.Errors
