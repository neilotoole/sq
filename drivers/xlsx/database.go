package xlsx

import (
	"context"
	"database/sql"

	"github.com/neilotoole/lg"
	"github.com/tealeg/xlsx/v2"

	"github.com/neilotoole/sq/libsq/cleanup"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/options"
	"github.com/neilotoole/sq/libsq/source"
)

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
