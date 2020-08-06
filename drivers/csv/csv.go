// Package csv implements the sq driver for CSV/TSV et al.
package csv

import (
	"context"
	"encoding/csv"
	"io"
	"io/ioutil"
	"strconv"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/libsq/cleanup"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
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
	Log       lg.Log
	Scratcher driver.ScratchDatabaseOpener
	Files     *source.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	switch typ {
	case TypeCSV:
		return &drvr{log: d.Log, typ: TypeCSV, scratcher: d.Scratcher, files: d.Files}, nil
	case TypeTSV:
		return &drvr{log: d.Log, typ: TypeTSV, scratcher: d.Scratcher, files: d.Files}, nil
	}

	return nil, errz.Errorf("unsupported driver type %q", typ)
}

// DetectCSV implements source.TypeDetectorFunc.
func DetectCSV(ctx context.Context, r io.Reader) (detected source.Type, score float32, err error) {
	return detectType(ctx, TypeCSV, r)
}

// DetectTSV implements source.TypeDetectorFunc.
func DetectTSV(ctx context.Context, r io.Reader) (detected source.Type, score float32, err error) {
	return detectType(ctx, TypeTSV, r)
}

func detectType(ctx context.Context, typ source.Type, r io.Reader) (detected source.Type, score float32, err error) {
	var delim = csvw.Comma
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

// Driver implements driver.Driver.
type drvr struct {
	log       lg.Log
	typ       source.Type
	scratcher driver.ScratchDatabaseOpener
	files     *source.Files
}

// DriverMetadata implements driver.Driver.
func (d *drvr) DriverMetadata() driver.Metadata {
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
func (d *drvr) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	dbase := &database{log: d.log, src: src, clnup: cleanup.New(), files: d.files}

	r, err := d.files.NewReader(ctx, src)
	if err != nil {
		return nil, err
	}

	dbase.impl, err = d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		d.log.WarnIfCloseError(r)
		d.log.WarnIfFuncError(dbase.clnup.Run)
		return nil, err
	}

	err = d.csvToScratch(ctx, src, r, dbase.impl)
	if err != nil {
		d.log.WarnIfCloseError(r)
		d.log.WarnIfFuncError(dbase.clnup.Run)
		return nil, err
	}

	err = r.Close()
	if err != nil {
		return nil, err
	}

	return dbase, nil
}

// Truncate implements driver.Driver.
func (d *drvr) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (int64, error) {
	// TODO: CSV could support Truncate for local files
	return 0, errz.Errorf("truncate not supported for %s", d.DriverMetadata().Type)
}

// ValidateSource implements driver.Driver.
func (d *drvr) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debugf("validating source: %q", src.Location)

	if src.Type != d.typ {
		return nil, errz.Errorf("expected source type %q but got %q", d.typ, src.Type)
	}

	if src.Options != nil || len(src.Options) > 0 {
		d.log.Debugf("opts: %v", src.Options.Encode())

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
func (d *drvr) Ping(ctx context.Context, src *source.Source) error {
	d.log.Debugf("driver %q attempting to ping %q", d.typ, src)

	r, err := d.files.NewReader(ctx, src)
	if err != nil {
		return err
	}
	defer d.log.WarnIfCloseError(r)

	// FIXME: this is a poor version of ping, but for now we just read the entire thing
	//  Ultimately we should execute isCSV to verify that the src is indeed CSV.
	_, err = ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	return nil
}

const (
	scoreNo       float32 = 0
	scoreMaybe    float32 = 0.1
	scoreProbably float32 = 0.2
	// scoreYes is less than 1.0 because other detectors
	// (e.g. XLSX) can be more confident
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
			if err == io.EOF && rec == nil {
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
