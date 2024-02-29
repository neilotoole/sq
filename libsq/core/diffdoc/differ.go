package diffdoc

import (
	"context"
	"io"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
)

// Differ encapsulates a [Doc] and a function that populates the [Doc].
// Create one via [NewDiffer], and then pass it to [Execute].
type Differ struct {
	doc Doc
	fn  func(ctx context.Context, cancelFn func(error))
}

// NewDiffer returns a new [Differ] that can be passed to [Execute]. Arg doc is
// the [Doc] to be populated, and fn populates the [Doc]. The cancelFn arg to fn
// must only be invoked in the event of an error; it must not be invoked on the
// happy path.
func NewDiffer(doc Doc, fn func(ctx context.Context, cancelFn func(error))) *Differ {
	return &Differ{doc: doc, fn: fn}
}

// execute returns a function that, when invoked, populates the doc by executing
// the function passed to NewDiffer. If that function returns an error,
// cancelFn is invoked with that error, and the error is returned.
func (d *Differ) execute(ctx context.Context, cancelFn func(error)) func() error {
	if cancelFn == nil {
		ctx, cancelFn = context.WithCancelCause(ctx)
	}
	return func() error {
		d.fn(ctx, cancelFn)
		err := d.doc.Err()
		if err != nil {
			cancelFn(err)
		}
		return err
	}
}

// Execute executes differs concurrently, writing output sequentially to w.
//
// Arg concurrency specifies the maximum number of concurrent Differ executions.
// Zero indicates sequential execution; a negative values indicates unbounded
// concurrency.
//
// The first error encountered is returned; hasDiff returns true if differences
// were found, and false if no differences.
func Execute(ctx context.Context, w io.Writer, concurrency int, differs []*Differ) (hasDiffs bool, err error) {
	defer func() {
		for _, differ := range differs {
			if differs == nil || differ.doc == nil {
				continue
			}
			if closeErr := differ.doc.Close(); closeErr != nil {
				lg.FromContext(ctx).Warn(lgm.CloseDiffDoc, lga.Doc, differ.doc.String(), lga.Err, closeErr)
			}
		}
	}()

	var cancelFn context.CancelCauseFunc
	ctx, cancelFn = context.WithCancelCause(ctx)
	defer func() { cancelFn(err) }()

	g := &errgroup.Group{}
	g.SetLimit(concurrency)
	for i := range differs {
		if differs[i] == nil {
			continue
		}
		g.Go(differs[i].execute(ctx, cancelFn))
	}

	// We don't call g.Wait() here because we're using errgroup solely to limit
	// the number of concurrent goroutines. We don't actually want to wait for all
	// the goroutines to finish; we want to stream the output (via io.Copy below)
	// as soon as it's available.

	rdrs := make([]io.Reader, 0, len(differs))
	for i := range differs {
		if differs[i] == nil {
			continue
		}
		rdrs = append(rdrs, differs[i].doc)
	}

	var n int64
	n, err = io.Copy(w, contextio.NewReader(ctx, io.MultiReader(rdrs...)))
	return n > 0, err
}
