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

// Execer encapsulates a [Doc] and a function that populates the [Doc].
type Execer struct {
	doc Doc
	fn  func(ctx context.Context, cancelFn func(error))
}

// NewExecer returns a new Execer that can be passed to Execute.
func NewExecer(doc Doc, fn func(ctx context.Context, cancelFn func(error))) *Execer {
	return &Execer{doc: doc, fn: fn}
}

// execute returns a function that, when invoked, populates the doc by executing
// the function passed to NewExecer. If that function returns an error,
// cancelFn is invoked with that error, and the error is returned.
func (d *Execer) execute(ctx context.Context, cancelFn func(error)) func() error {
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

// Execute executes execers concurrently, writing output sequentially to w.
func Execute(ctx context.Context, w io.Writer, concurrency int, execers []*Execer) (err error) {
	log := lg.FromContext(ctx)
	execers = lo.WithoutEmpty(execers)
	defer func() {
		for i := range execers {
			doc := execers[i].doc
			if closeErr := doc.Close(); closeErr != nil {
				log.Warn(lgm.CloseDiffDoc, closeErr, lga.Doc, doc.Title().String())
			}
		}
	}()

	var cancelFn context.CancelCauseFunc
	ctx, cancelFn = context.WithCancelCause(ctx)
	defer func() { cancelFn(err) }()

	g := &errgroup.Group{}
	g.SetLimit(concurrency)
	for i := range execers {
		g.Go(execers[i].execute(ctx, cancelFn))
	}

	// We don't call g.Wait() here because we're using the errgroup solely to
	// limit the number of concurrent goroutines. We don't actually want to wait
	// for all the goroutines to finish; we want to stream the output (via io.Copy
	// below) as soon as it's available.

	rdrs := make([]io.Reader, len(execers))
	for i := range execers {
		rdrs[i] = execers[i].doc
	}

	_, err = io.Copy(w, contextio.NewReader(ctx, io.MultiReader(rdrs...)))
	return err
}
