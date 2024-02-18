package diffdoc

import (
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

// Differ encapsulates a [Doc] and a function that populates the [Doc].
// Create one via [NewDiffer], and then pass it to [Execute].
type Differ struct {
	doc Doc
	fn  func(ctx context.Context, cancelFn func(error))
}

// NewDiffer returns a new [Differ] that can be passed to [Execute]. Arg doc is
// the [Doc] to be populated, and fn populates the [Doc].
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
func Execute(ctx context.Context, w io.Writer, concurrency int, differs []*Differ) (err error) {
	log := lg.FromContext(ctx)
	differs = lo.WithoutEmpty(differs)

	defer func() {
		for i := range differs {
			doc := differs[i].doc
			if closeErr := doc.Close(); closeErr != nil {
				log.Warn(lgm.CloseDiffDoc, closeErr, lga.Doc, doc.String())
			}
		}
	}()

	var cancelFn context.CancelCauseFunc
	ctx, cancelFn = context.WithCancelCause(ctx)
	defer func() { cancelFn(err) }()

	g := &errgroup.Group{}
	g.SetLimit(concurrency)
	for i := range differs {
		g.Go(differs[i].execute(ctx, cancelFn))
	}

	// We don't call g.Wait() here because we're using errgroup solely to limit
	// the number of concurrent goroutines. We don't actually want to wait for all
	// the goroutines to finish; we want to stream the output (via io.Copy below)
	// as soon as it's available.

	rdrs := make([]io.Reader, len(differs))
	for i := range differs {
		rdrs[i] = differs[i].doc
	}

	_, err = io.Copy(w, contextio.NewReader(ctx, io.MultiReader(rdrs...)))
	return err
}
