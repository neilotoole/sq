package source

import (
	"context"
	"io"
	"mime"
	"time"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// DriverDetectFunc interrogates a byte stream to determine
// the source driver type. A score is returned indicating
// the confidence that the driver type has been detected.
// A score <= 0 is failure, a score >= 1 is success; intermediate
// values indicate some level of confidence.
// An error is returned only if an IO problem occurred.
// The implementation gets access to the byte stream by invoking openFn,
// and is responsible for closing any reader it opens.
type DriverDetectFunc func(ctx context.Context, openFn FileOpenFunc) (
	detected drivertype.Type, score float32, err error)

var _ DriverDetectFunc = DetectMagicNumber

// AddDriverDetectors adds driver type detectors.
func (fs *Files) AddDriverDetectors(detectFns ...DriverDetectFunc) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.detectFns = append(fs.detectFns, detectFns...)
}

// DriverType returns the driver type of loc.
// This may result in loading files into the cache.
func (fs *Files) DriverType(ctx context.Context, handle, loc string) (drivertype.Type, error) {
	log := lg.FromContext(ctx).With(lga.Loc, loc)
	ploc, err := parseLoc(loc)
	if err != nil {
		return drivertype.None, err
	}

	if ploc.typ != drivertype.None {
		return ploc.typ, nil
	}

	if ploc.ext != "" {
		mtype := mime.TypeByExtension(ploc.ext)
		if mtype == "" {
			log.Debug("unknown mime type", lga.Type, mtype)
		} else {
			if typ, ok := typeFromMediaType(mtype); ok {
				return typ, nil
			}
			log.Debug("unknown driver type for media type", lga.Type, mtype)
		}
	}

	// FIXME: We really should try to be smarter here, esp with sqlite files.

	fs.mu.Lock()
	defer fs.mu.Unlock()
	// Fall back to the byte detectors
	typ, ok, err := fs.detectType(ctx, handle, loc)
	if err != nil {
		return drivertype.None, err
	}

	if !ok {
		return drivertype.None, errz.Errorf("unable to determine driver type: %s", loc)
	}

	return typ, nil
}

func (fs *Files) detectType(ctx context.Context, handle, loc string) (typ drivertype.Type, ok bool, err error) {
	if len(fs.detectFns) == 0 {
		return drivertype.None, false, nil
	}
	log := lg.FromContext(ctx).With(lga.Loc, loc)
	start := time.Now()

	openFn := func(ctx context.Context) (io.ReadCloser, error) {
		return fs.newReader(ctx, handle, loc)
	}

	// We do the magic number first, because it's so fast.
	detected, score, err := DetectMagicNumber(ctx, openFn)
	if err == nil && score >= 1.0 {
		return detected, true, nil
	}

	type result struct {
		typ   drivertype.Type
		score float32
	}

	resultCh := make(chan result, len(fs.detectFns))

	select {
	case <-ctx.Done():
		return drivertype.None, false, ctx.Err()
	default:
	}

	g, gCtx := errgroup.WithContext(ctx)

	for _, detectFn := range fs.detectFns {
		detectFn := detectFn

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			gTyp, gScore, gErr := detectFn(gCtx, openFn)
			if gErr != nil {
				return gErr
			}

			if gScore > 0 {
				resultCh <- result{typ: gTyp, score: gScore}
			}
			return nil
		})
	}

	// REVISIT: We shouldn't have to wait for all goroutines to complete.
	// This logic could be refactored to return as soon as a single
	// goroutine returns a score >= 1.0 (then cancelling the other detector
	// goroutines).

	err = g.Wait()
	if err != nil {
		log.Error(err.Error())
		return drivertype.None, false, errz.Err(err)
	}
	close(resultCh)

	var highestScore float32
	for res := range resultCh {
		if res.score > highestScore {
			highestScore = res.score
			typ = res.typ
		}
	}

	const detectScoreThreshold = 0.5
	if highestScore >= detectScoreThreshold {
		log.Debug("Type detected", lga.Type, typ, lga.Elapsed, time.Since(start))
		return typ, true, nil
	}

	log.Warn("No type detected", lga.Type, typ, lga.Elapsed, time.Since(start))
	return drivertype.None, false, nil
}

// DetectMagicNumber is a DriverDetectFunc that uses an external
// pkg (h2non/filetype) to detect the "magic number" from
// the start of files.
func DetectMagicNumber(ctx context.Context, openFn FileOpenFunc,
) (detected drivertype.Type, score float32, err error) {
	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = openFn(ctx)
	if err != nil {
		return drivertype.None, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	// We only have to pass the file header = first 261 bytes
	head := make([]byte, 261)
	_, err = r.Read(head)
	if err != nil {
		return drivertype.None, 0, errz.Wrapf(err, "failed to read header")
	}

	ftype, err := filetype.Match(head)
	if err != nil {
		if err != nil {
			return drivertype.None, 0, errz.Err(err)
		}
	}

	switch ftype {
	default:
		return drivertype.None, 0, nil
	case matchers.TypeXlsx:
		// This doesn't seem to work, because .xlsx files are
		// zipped, so the type returns as a zip. Perhaps there's
		// something we can do about it, such as first extracting
		// the zip, and then reading the inner magic number, but
		// the xlsx.DetectXLSX func should catch the type anyway.
		return typeXLSX, 1.0, nil
	case matchers.TypeXls:
		// TODO: our xlsx driver doesn't yet support XLS
		return typeXLSX, 1.0, errz.Errorf("Microsoft XLS (%s) not currently supported", ftype)
	case matchers.TypeSqlite:
		return typeSL3, 1.0, nil
	}
}

// DetectStdinType detects the type of stdin as previously added
// by AddStdin. An error is returned if AddStdin was not
// first invoked. If the type cannot be detected, TypeNone and
// nil are returned.
func (fs *Files) DetectStdinType(ctx context.Context) (drivertype.Type, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if !fs.fscache.Exists(StdinHandle) {
		return drivertype.None, errz.New("must invoke Files.AddStdin before invoking DetectStdinType")
	}

	typ, ok, err := fs.detectType(ctx, "", StdinHandle)
	if err != nil {
		return drivertype.None, err
	}

	if !ok {
		return drivertype.None, nil
	}

	return typ, nil
}
