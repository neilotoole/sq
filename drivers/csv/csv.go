// Package csv implements the sq driver for CSV/TSV et al.
package csv

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"strconv"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// TypeCSV is the CSV driver type.
	TypeCSV = source.Type("csv")

	// TypeTSV is the TSV driver type.
	TypeTSV = source.Type("tsv")
)

// Provider implements driver.Provider.
type Provider struct {
	Log       *slog.Logger
	Scratcher driver.ScratchDatabaseOpener
	Files     *source.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	switch typ { //nolint:exhaustive
	case TypeCSV:
		return &driveri{log: d.Log, typ: TypeCSV, scratcher: d.Scratcher, files: d.Files}, nil
	case TypeTSV:
		return &driveri{log: d.Log, typ: TypeTSV, scratcher: d.Scratcher, files: d.Files}, nil
	}

	return nil, errz.Errorf("unsupported driver type %q", typ)
}

// Driver implements driver.Driver.
type driveri struct {
	log       *slog.Logger
	typ       source.Type
	scratcher driver.ScratchDatabaseOpener
	files     *source.Files
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	md := driver.Metadata{Type: d.typ, Monotable: true}
	if d.typ == TypeCSV {
		md.Description = "Comma-Separated Values"
		md.Doc = "https://en.wikipedia.org/wiki/Comma-separated_values"
	} else {
		md.Description = "Tab-Separated Values"
		md.Doc = "https://en.wikipedia.org/wiki/Tab-separated_values"
	}
	return md
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	dbase := &database{
		log:   d.log,
		src:   src,
		files: d.files,
	}

	var err error
	dbase.impl, err = d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		return nil, err
	}

	err = importCSV(ctx, src, d.files.OpenFunc(src), dbase.impl)
	if err != nil {
		return nil, err
	}

	return dbase, nil
}

// Truncate implements driver.Driver.
func (d *driveri) Truncate(_ context.Context, _ *source.Source, _ string, _ bool) (int64, error) {
	// TODO: CSV could support Truncate for local files
	return 0, errz.Errorf("truncate not supported for %s", d.DriverMetadata().Type)
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != d.typ {
		return nil, errz.Errorf("expected source type %q but got %q", d.typ, src.Type)
	}

	if src.Options != nil || len(src.Options) > 0 {
		d.log.Debug("Validating source",
			lga.Src, src,
			lga.Opts, src.Options.Encode(),
		)

		key := "header"
		v := src.Options.Get(key)

		if v != "" {
			_, err := strconv.ParseBool(v)
			if err != nil {
				return nil, errz.Errorf(`unable to parse option %q: %v`, key, err)
			}
		}
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(_ context.Context, src *source.Source) error {
	r, err := d.files.Open(src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	return nil
}

// database implements driver.Database.
type database struct {
	log   *slog.Logger
	src   *source.Source
	impl  driver.Database
	files *source.Files
}

// DB implements driver.Database.
func (d *database) DB() *sql.DB {
	return d.impl.DB()
}

// SQLDriver implements driver.Database.
func (d *database) SQLDriver() driver.SQLDriver {
	return d.impl.SQLDriver()
}

// Source implements driver.Database.
func (d *database) Source() *source.Source {
	return d.src
}

// TableMetadata implements driver.Database.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	if tblName != source.MonotableName {
		return nil, errz.Errorf("table name should be %s for CSV/TSV etc., but got: %s",
			source.MonotableName, tblName)
	}

	srcMeta, err := d.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}

	// There will only ever be one table for CSV.
	return srcMeta.Tables[0], nil
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	md, err := d.impl.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}

	md.Handle = d.src.Handle
	md.Location = d.src.Location
	md.SourceType = d.src.Type

	md.Name, err = source.LocationFileName(d.src)
	if err != nil {
		return nil, err
	}

	md.Size, err = d.files.Size(d.src)
	if err != nil {
		return nil, err
	}

	md.FQName = md.Name
	return md, nil
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Src, d.src)

	return errz.Err(d.impl.Close())
}

var (
	_ source.TypeDetectFunc = DetectCSV
	_ source.TypeDetectFunc = DetectTSV
)

// DetectCSV implements source.TypeDetectFunc.
func DetectCSV(ctx context.Context, openFn source.FileOpenFunc) (detected source.Type, score float32,
	err error,
) {
	return detectType(ctx, TypeCSV, openFn)
}

// DetectTSV implements source.TypeDetectFunc.
func DetectTSV(ctx context.Context, openFn source.FileOpenFunc) (detected source.Type,
	score float32, err error,
) {
	return detectType(ctx, TypeTSV, openFn)
}

func detectType(ctx context.Context, typ source.Type,
	openFn source.FileOpenFunc,
) (detected source.Type, score float32, err error) {
	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = openFn()
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	delim := csvw.Comma
	if typ == TypeTSV {
		delim = csvw.Tab
	}

	cr := csv.NewReader(&crFilterReader{r: r})
	cr.Comma = delim
	cr.FieldsPerRecord = -1

	score = isCSV(ctx, cr)
	if score > 0 {
		return typ, score, nil
	}

	return source.TypeNone, 0, nil
}

const (
	scoreNo       float32 = 0
	scoreMaybe    float32 = 0.1
	scoreProbably float32 = 0.2
	// scoreYes is less than 1.0 because other detectors
	// (e.g. XLSX) can be more confident.
	scoreYes float32 = 0.9
)

// isCSV returns a score indicating the
// the confidence that cr is reading legitimate CSV, where
// a score <= 0 is not CSV, a score >= 1 is definitely CSV.
func isCSV(ctx context.Context, cr *csv.Reader) (score float32) {
	const (
		maxRecords int = 100
	)

	var recordCount, totalFieldCount int
	var avgFields float32

	for i := 0; i < maxRecords; i++ {
		select {
		case <-ctx.Done():
			return 0
		default:
		}

		rec, err := cr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) && rec == nil {
				// This means end of data
				break
			}

			// It's a genuine error
			return scoreNo
		}
		totalFieldCount += len(rec)
		recordCount++
	}

	if recordCount == 0 {
		return scoreNo
	}

	avgFields = float32(totalFieldCount) / float32(recordCount)

	if recordCount == 1 {
		if avgFields <= 2 {
			return scoreMaybe
		}
		return scoreProbably
	}

	// recordCount >= 2
	switch {
	case avgFields <= 1:
		return scoreMaybe
	case avgFields <= 2:
		return scoreProbably
	default:
		// avgFields > 2
		return scoreYes
	}
}
