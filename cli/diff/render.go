package diff

import (
	"bytes"
	"context"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)



func renderRecords(ctx context.Context, cfg *Config, recMeta record.Meta, recs []record.Record) ([]byte, error) {
	if len(recs) == 0 {
		return nil, nil
	}

	pr := cfg.Printing.Clone()
	pr.EnableColor(false)
	pr.ShowHeader = false
	buf := &bytes.Buffer{}
	recw := cfg.RecordWriterFn(buf, pr)

	if err := recw.Open(ctx, recMeta); err != nil {
		return nil, err
	}
	if err := recw.WriteRecords(ctx, recs); err != nil {
		return nil, err
	}
	if err := recw.Flush(ctx); err != nil {
		return nil, err
	}
	if err := recw.Close(ctx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderSourceMeta2YAML returns a YAML rendering of metadata.Source.
// The returned YAML is subtly different from that
// returned by yamlw.NewSourceWriter. For example, it
// adds a "table_count" field.
func renderSourceMeta2YAML(sm *metadata.Source) (string, error) {
	if sm == nil {
		return "", nil
	}

	// sourceMeta holds values of metadata.Source in the structure
	// that diff wants.
	type sourceMeta struct {
		Handle     string          `json:"handle" yaml:"handle"`
		Location   string          `json:"location" yaml:"location"`
		Name       string          `json:"name" yaml:"name"`
		FQName     string          `json:"name_fq" yaml:"name_fq"`
		Schema     string          `json:"schema,omitempty" yaml:"schema,omitempty"`
		Driver     drivertype.Type `json:"driver" yaml:"driver"`
		DBDriver   drivertype.Type `json:"db_driver" yaml:"db_driver"`
		DBProduct  string          `json:"db_product" yaml:"db_product"`
		DBVersion  string          `json:"db_version" yaml:"db_version"`
		User       string          `json:"user,omitempty" yaml:"user,omitempty"`
		Size       int64           `json:"size" yaml:"size"`
		TableCount int64           `json:"table_count" yaml:"table_count"`
		ViewCount  int64           `json:"view_count" yaml:"view_count"`
	}

	smr := &sourceMeta{
		Handle:     sm.Handle,
		Location:   location.Redact(sm.Location),
		Name:       sm.Name,
		FQName:     sm.FQName,
		Schema:     sm.Schema,
		Driver:     sm.Driver,
		DBDriver:   sm.DBDriver,
		DBProduct:  sm.DBProduct,
		DBVersion:  sm.DBVersion,
		User:       sm.User,
		Size:       sm.Size,
		TableCount: sm.TableCount,
		ViewCount:  sm.ViewCount,
	}

	b, err := ioz.MarshalYAML(smr)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// renderTableMeta2YAML returns a YAML rendering of metadata.Table.
// The returned YAML is subtly different from that
// returned by yamlw.NewSourceWriter. For example, it
// adds a "column_count" field.
func renderTableMeta2YAML(showRowCounts bool, tm *metadata.Table) (string, error) {
	if tm == nil {
		return "", nil
	}

	// tableMeta hosts values of metadata.Table in the
	// structure that diff wants.
	//nolint:govet // field alignment
	type tableMeta struct {
		Name        string `json:"name" yaml:"name"`
		FQName      string `json:"name_fq,omitempty" yaml:"name_fq,omitempty"`
		TableType   string `json:"table_type,omitempty" yaml:"table_type,omitempty"`
		DBTableType string `json:"table_type_db,omitempty" yaml:"table_type_db,omitempty"`
		// RowCount is a pointer, because its display is controlled
		// by a variable.
		RowCount    *int64             `json:"row_count,omitempty" yaml:"row_count,omitempty"`
		Size        *int64             `json:"size,omitempty" yaml:"size,omitempty"`
		Comment     string             `json:"comment,omitempty" yaml:"comment,omitempty"`
		ColumnCount int64              `json:"column_count" yaml:"column_count"`
		Columns     []*metadata.Column `json:"columns" yaml:"columns"`
	}

	tmr := &tableMeta{
		Name:        tm.Name,
		FQName:      tm.FQName,
		TableType:   tm.TableType,
		DBTableType: tm.DBTableType,
		// TODO: Printing of Size should be controlled by a param,
		// e.g. "show-volatile-fields". Until then, we omit it.
		Size:        nil,
		Comment:     tm.Comment,
		ColumnCount: int64(len(tm.Columns)),
		Columns:     tm.Columns,
	}

	if showRowCounts {
		tmr.RowCount = &tm.RowCount
	}

	b, err := ioz.MarshalYAML(tmr)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// renderDBProperties2YAML returns a YAML rendering of db properties.
func renderDBProperties2YAML(props map[string]any) (string, error) {
	if len(props) == 0 {
		return "", nil
	}

	b, err := ioz.MarshalYAML(props)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
