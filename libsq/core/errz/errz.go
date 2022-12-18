// Package errz is sq's error package. It exists to combine
// functionality from several error packages.
package errz

import (
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// Err is documented by pkg/errors.WithStack.
var Err = errors.WithStack

// Wrap is documented by pkg/errors.Wrap.
var Wrap = errors.Wrap

// Wrapf is documented by pkg/errors.Wrapf.
var Wrapf = errors.Wrapf

// New is documented by pkg/errors.New.
var New = errors.New

// Errorf is documented by pkg/errors.Errorf.
var Errorf = errors.Errorf

// Cause is documented by pkg/errors.Cause.
var Cause = errors.Cause

// Append is documented by multierr.Append.
var Append = multierr.Append

// TODO: ^^ Should implement our own version of Append that checks
// if the args have already been wrapped (WithStack), and if not,
// automatically wrap them. That is, this:
//
//   return errz.Append(err, errz.Err(tx.Rollback())
//   // becomes
//   return errz.Append(err, tx.Rollback())

// Combine is documented by multierr.Combine.
var Combine = multierr.Combine

// Errors is documented by multierr.Errors.
var Errors = multierr.Errors
