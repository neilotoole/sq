// Package xlsx implements the sq driver for Microsoft Excel.
package xlsx

import (
	"context"
	"database/sql"
	"io"
	"io/ioutil"

	"github.com/neilotoole/lg"
	"github.com/tealeg/xlsx/v2"

	"github.com/neilotoole/sq/libsq/cleanup"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/options"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// Type is the sq source driver type for XLSX.
	Type = source.Type("xlsx")
)

// Provider implements driver.Provider.
type Provider struct {
	Log       lg.Log
	Files     *source.Files
	Scratcher driver.ScratchDatabaseOpener
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type %q", typ)
	}

	return &Driver{log: p.Log, scratcher: p.Scratcher, files: p.Files}, nil
}

// DetectXLSX returns TypeXLSX and a score of 1.0 if r's bytes
// are valid XLSX.
func DetectXLSX(ctx context.Context, r io.Reader) (detected source.Type, score float32, err error) {
	data, err := ioutil.ReadAll(r)
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

// Driver implements driver.Driver.
type Driver struct {
	log       lg.Log
	scratcher driver.ScratchDatabaseOpener
	files     *source.Files
}

// DriverMetadata implements driver.Driver.
func (d *Driver) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "Microsoft Excel XLSX",
		Doc:         "https://en.wikipedia.org/wiki/Microsoft_Excel"}
}

// Open implements driver.Driver.
func (d *Driver) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	r, err := d.files.NewReader(ctx, src)
	if err != nil {
		return nil, err
	}
	defer d.log.WarnIfCloseError(r)

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errz.Err(err)
	}

	xlFile, err := xlsx.OpenBinary(b)
	if err != nil {
		return nil, err
	}

	scratchDB, err := d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		return nil, err
	}

	clnup := cleanup.New()
	clnup.AddE(scratchDB.Close)

	err = xlsxToScratch(ctx, d.log, src, xlFile, scratchDB)
	if err != nil {
		d.log.WarnIfError(clnup.Run())
		return nil, err
	}

	return &database{log: d.log, src: src, impl: scratchDB, files: d.files, clnup: clnup}, nil
}

// Truncate implements driver.Driver.
func (d *Driver) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64, err error) {
	// TODO: WE could actually implement Truncate for xlsx.
	//  It would just mean deleting the rows from a sheet, and then
	//  saving the sheet.
	return 0, errz.Errorf("source type %q (%s) doesn't support dropping tables", Type, src.Handle)
}

// ValidateSource implements driver.Driver.
func (d *Driver) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debugf("Validating source: %q", src.RedactedLocation())
	if src.Type != Type {
		return nil, errz.Errorf("expected source type %q but got %q", Type, src.Type)
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *Driver) Ping(ctx context.Context, src *source.Source) (err error) {
	r, err := d.files.NewReader(ctx, src)
	if err != nil {
		return err
	}

	defer d.log.WarnIfCloseError(r)

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return errz.Err(err)
	}

	_, err = xlsx.OpenBinaryWithRowLimit(b, 1)
	if err != nil {
		return errz.Err(err)
	}

	return nil
}

// database implements driver.Database.
type database struct {
	log   lg.Log
	src   *source.Source
	files *source.Files
	impl  driver.Database
	clnup *cleanup.Cleanup
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
	srcMeta, err := d.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return source.TableFromSourceMetadata(srcMeta, tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	meta := &source.Metadata{Handle: d.src.Handle}

	var err error
	meta.Size, err = d.files.Size(d.src)
	if err != nil {
		return nil, err
	}

	meta.Name, err = source.LocationFileName(d.src)
	if err != nil {
		return nil, err
	}

	meta.FQName = meta.Name
	meta.Location = d.src.Location
	meta.SourceType = Type

	b, err := d.files.ReadAll(d.src)
	if err != nil {
		return nil, errz.Err(err)
	}

	xlFile, err := xlsx.OpenBinary(b)
	if err != nil {
		return nil, errz.Errorf("unable to open XLSX file: ", d.src.Location, err)
	}

	hasHeader, _, err := options.HasHeader(d.src.Options)
	if err != nil {
		return nil, err
	}

	for _, sheet := range xlFile.Sheets {
		tbl := &source.TableMetadata{}

		tbl.Name = sheet.Name
		tbl.Size = -1
		tbl.RowCount = int64(len(sheet.Rows))

		if hasHeader && tbl.RowCount > 0 {
			tbl.RowCount--
		}

		colNames := getColNames(sheet, hasHeader)
		colTypes := getColTypes(sheet, hasHeader)

		for i, colType := range colTypes {
			col := &source.ColMetadata{}
			col.BaseType = cellTypeToString(colType)
			col.ColumnType = col.BaseType
			col.Position = int64(i)
			col.Name = colNames[i]
			tbl.Columns = append(tbl.Columns, col)
		}

		meta.Tables = append(meta.Tables, tbl)
	}

	return meta, nil
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debugf("Close database: %s", d.src)

	// No need to explicitly invoke c.impl.Close because
	// that's already added to c.clnup
	return d.clnup.Run()
}
