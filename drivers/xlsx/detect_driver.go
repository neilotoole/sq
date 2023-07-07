package xlsx

import (
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/tealeg/xlsx/v2"
)

var _ source.DriverDetectFunc = DetectXLSX

// DetectXLSX implements source.DriverDetectFunc, returning
// TypeXLSX and a score of 1.0 if valid XLSX.
func DetectXLSX(ctx context.Context, openFn source.FileOpenFunc) (detected source.DriverType, score float32,
	err error,
) {
	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = openFn()
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	data, err := io.ReadAll(r)
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}

	// We don't need to read all rows, one will do.
	const rowLimit = 1
	_, err = xlsx.OpenBinaryWithRowLimit(data, rowLimit)

	if err != nil {
		return source.TypeNone, 0, nil
	}

	return Type, 1.0, nil
}
