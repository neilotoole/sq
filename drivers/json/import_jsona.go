package json

import (
	"bufio"
	"context"
	stdj "encoding/json"
	"io"
	"math"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// DetectJSONA returns a source.DriverDetectFunc for TypeJSONA.
// Each line of input must be a valid JSON array.
func DetectJSONA(sampleSize int) source.DriverDetectFunc {
	return func(ctx context.Context, openFn source.FileOpenFunc) (detected source.DriverType,
		score float32, err error,
	) {
		log := lg.FromContext(ctx)
		var r io.ReadCloser
		r, err = openFn()
		if err != nil {
			return source.TypeNone, 0, errz.Err(err)
		}
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

		sc := bufio.NewScanner(r)
		var validLines int
		var line []byte

		for sc.Scan() {
			select {
			case <-ctx.Done():
				return source.TypeNone, 0, ctx.Err()
			default:
			}

			if err = sc.Err(); err != nil {
				return source.TypeNone, 0, errz.Err(err)
			}

			line = sc.Bytes()
			if len(line) == 0 {
				// Probably want to skip blank lines? Maybe
				continue
			}

			// Each line of JSONA must open with left bracket
			if line[0] != '[' {
				return source.TypeNone, 0, nil
			}

			// If the line is JSONA, it should marshall into []any
			var fields []any
			err = stdj.Unmarshal(line, &fields)
			if err != nil {
				return source.TypeNone, 0, nil
			}

			// JSONA must consist only of values, not objects. Any object
			// would get marshalled into a map[string]any, so
			// we check for that.
			for _, field := range fields {
				if _, ok := field.(map[string]any); ok {
					return source.TypeNone, 0, nil
				}
			}

			validLines++
			if validLines >= sampleSize {
				break
			}
		}

		if err = sc.Err(); err != nil {
			return source.TypeNone, 0, errz.Err(err)
		}

		if validLines > 0 {
			return TypeJSONA, 1.0, nil
		}

		return source.TypeNone, 0, nil
	}
}

func importJSONA(ctx context.Context, job importJob) error {
	log := lg.FromContext(ctx)

	predictR, err := job.openFn()
	if err != nil {
		return errz.Err(err)
	}

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, predictR)

	colKinds, readMungeFns, err := detectColKindsJSONA(ctx, predictR, job.sampleSize)
	if err != nil {
		return err
	}

	if len(colKinds) == 0 || len(readMungeFns) == 0 {
		return errz.Errorf("import %s: number of fields is zero", TypeJSONA)
	}

	colNames := make([]string, len(colKinds))
	for i := 0; i < len(colNames); i++ {
		colNames[i] = stringz.GenerateAlphaColName(i, true)
	}

	// And now we need to create the dest table in destDB
	tblDef := sqlmodel.NewTableDef(source.MonotableName, colNames, colKinds)
	err = job.destDB.SQLDriver().CreateTable(ctx, job.destDB.DB(), tblDef)
	if err != nil {
		return errz.Wrapf(err, "import %s: failed to create dest scratch table", TypeJSONA)
	}

	recMeta, err := getRecMeta(ctx, job.destDB, tblDef)
	if err != nil {
		return err
	}

	r, err := job.openFn()
	if err != nil {
		return errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	insertWriter := libsq.NewDBWriter(
		job.destDB,
		tblDef.Name,
		driver.OptTuningRecChanSize.Get(job.destDB.Source().Options),
	)

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()

	recordCh, errCh, err := insertWriter.Open(ctx, cancelFn, recMeta)
	if err != nil {
		return err
	}

	// After startInsertJSONA returns, we still need to wait
	// for the insertWriter to finish.
	err = startInsertJSONA(ctx, recordCh, errCh, r, readMungeFns)
	if err != nil {
		return err
	}

	inserted, err := insertWriter.Wait()
	if err != nil {
		return err
	}

	log.Debug("Inserted rows",
		lga.Count, inserted,
		lga.Target, source.Target(job.destDB.Source(), tblDef.Name),
	)
	return nil
}

// startInsertJSONA reads JSON records from r and sends
// them on recordCh.
func startInsertJSONA(ctx context.Context, recordCh chan<- sqlz.Record, errCh <-chan error, r io.Reader,
	mungeFns []kind.MungeFunc,
) error {
	defer close(recordCh)

	sc := bufio.NewScanner(r)
	var line []byte
	var err error

	for sc.Scan() {
		if err = sc.Err(); err != nil {
			return errz.Err(err)
		}

		line = sc.Bytes()
		if len(line) == 0 {
			// Probably want to skip blank lines? Maybe
			continue
		}

		// Each line of JSONA must open with left bracket
		if line[0] != '[' {
			return errz.New("malformed JSONA input")
		}

		// If the line is JSONA, it should marshal into []any
		var rec []any
		err = stdj.Unmarshal(line, &rec)
		if err != nil {
			return errz.Err(err)
		}

		for i := 0; i < len(rec); i++ {
			fn := mungeFns[i]
			if fn != nil {
				var v any
				v, err = fn(rec[i])
				if err != nil {
					return errz.Err(err)
				}
				rec[i] = v
			}
		}

		select {
		case err = <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case recordCh <- rec:
		}
	}

	if err = sc.Err(); err != nil {
		return errz.Err(err)
	}

	return nil
}

// detectColKindsJSONA reads JSONA lines from r, and returns
// the kind of each field. The []readMungeFunc may contain a munge
// func that should be applied to each value (or the element may be nil).
func detectColKindsJSONA(ctx context.Context, r io.Reader, sampleSize int) ([]kind.Kind, []kind.MungeFunc, error) {
	var (
		err            error
		totalLineCount int
		// jLineCount is the number of JSONA lines (totalLineCount minus empty lines)
		jLineCount int
		line       []byte
		kinds      []kind.Kind
		detectors  []*kind.Detector
		mungeFns   []kind.MungeFunc
	)

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		if jLineCount > sampleSize {
			break
		}

		if err = sc.Err(); err != nil {
			return nil, nil, errz.Err(err)
		}

		line = sc.Bytes()
		totalLineCount++
		if len(line) == 0 {
			// Probably want to skip blank lines? Maybe
			continue
		}

		jLineCount++

		// Each line of JSONA must open with left bracket
		if line[0] != '[' {
			return nil, nil, errz.New("line does not begin with left bracket '['")
		}

		// If the line is JSONA, it should marshall into []any
		var vals []any
		err = stdj.Unmarshal(line, &vals)
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		if len(vals) == 0 {
			return nil, nil, errz.Errorf("zero field count at line %d", totalLineCount)
		}

		if kinds == nil {
			kinds = make([]kind.Kind, len(vals))
			mungeFns = make([]kind.MungeFunc, len(vals))
			detectors = make([]*kind.Detector, len(vals))
			for i := range detectors {
				detectors[i] = kind.NewDetector()
			}
		}

		if len(vals) != len(kinds) {
			return nil, nil, errz.Errorf("inconsistent field count: expected %d but got %d at line %d",
				len(kinds), len(vals), totalLineCount)
		}

		for i, val := range vals {
			val = maybeFloatToInt(val)
			detectors[i].Sample(val)
		}
	}

	if jLineCount == 0 {
		return nil, nil, errz.New("empty JSONA input")
	}

	for i := range kinds {
		kinds[i], mungeFns[i], err = detectors[i].Detect()
		if err != nil {
			return nil, nil, err
		}
		if kinds[i] == kind.Null {
			kinds[i] = kind.Text
		}
	}

	return kinds, mungeFns, nil
}

// maybeFloatToInt returns an int64 if val is a float64 with a
// round integer value. If val is not a float64, it is returned
// unchanged.
//
// The JSON decoder decodes numbers into float64.
// We don't want that if the number is really an integer
// (especially important for id columns).
// So, if the float64 has zero after the decimal point '.' (that
// is to say, it's a round float like 1.0), we return the int64 value.
func maybeFloatToInt(val any) any {
	if f64, ok := val.(float64); ok {
		floor := math.Floor(f64)
		if f64-floor == 0 {
			return int64(floor)
		}
	}

	return val
}
