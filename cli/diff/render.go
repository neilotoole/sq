package diff

import (
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/source"
)

// renderSourceMeta2YAML returns a YAML rendering of source.Metadata.
// The returned YAML is subtly different from that
// returned by yamlw.NewSourceWriter. For example, it
// adds a "table_count" field.
func renderSourceMeta2YAML(sm *source.Metadata) (string, error) {
	if sm == nil {
		return "", nil
	}

	// sourceMeta holds values of source.Metadata in the structure
	// that diff wants.
	type sourceMeta struct {
		Handle     string            `json:"handle" yaml:"handle"`
		Location   string            `json:"location" yaml:"location"`
		Name       string            `json:"name" yaml:"name"`
		FQName     string            `json:"name_fq" yaml:"name_fq"`
		Schema     string            `json:"schema,omitempty" yaml:"schema,omitempty"`
		Driver     source.DriverType `json:"driver" yaml:"driver"`
		DBDriver   source.DriverType `json:"db_driver" yaml:"db_driver"`
		DBProduct  string            `json:"db_product" yaml:"db_product"`
		DBVersion  string            `json:"db_version" yaml:"db_version"`
		User       string            `json:"user,omitempty" yaml:"user,omitempty"`
		Size       int64             `json:"size" yaml:"size"`
		TableCount int64             `json:"table_count" yaml:"table_count"`
	}

	smr := &sourceMeta{
		Handle:     sm.Handle,
		Location:   source.RedactLocation(sm.Location),
		Name:       sm.Name,
		FQName:     sm.FQName,
		Schema:     sm.Schema,
		Driver:     sm.Driver,
		DBDriver:   sm.DBDriver,
		DBProduct:  sm.DBProduct,
		DBVersion:  sm.DBVersion,
		User:       sm.User,
		Size:       sm.Size,
		TableCount: int64(len(sm.Tables)),
	}

	b, err := ioz.MarshalYAML(smr)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// renderTableMeta2YAML returns a YAML rendering of source.TableMetadata.
// The returned YAML is subtly different from that
// returned by yamlw.NewSourceWriter. For example, it
// adds a "column_count" field.
func renderTableMeta2YAML(tm *source.TableMetadata) (string, error) {
	if tm == nil {
		return "", nil
	}

	// tableMeta hosts values of source.TableMetadata in the
	// structure that diff wants.
	type tableMeta struct {
		Name        string                `json:"name" yaml:"name"`
		FQName      string                `json:"name_fq,omitempty" yaml:"name_fq,omitempty"`
		TableType   string                `json:"table_type,omitempty" yaml:"table_type,omitempty"`
		DBTableType string                `json:"table_type_db,omitempty" yaml:"table_type_db,omitempty"`
		RowCount    int64                 `json:"row_count" yaml:"row_count"`
		Size        *int64                `json:"size,omitempty" yaml:"size,omitempty"`
		Comment     string                `json:"comment,omitempty" yaml:"comment,omitempty"`
		ColumnCount int64                 `json:"column_count" yaml:"column_count"`
		Columns     []*source.ColMetadata `json:"columns" yaml:"columns"`
	}

	tmr := &tableMeta{
		Name:        tm.Name,
		FQName:      tm.FQName,
		TableType:   tm.TableType,
		DBTableType: tm.DBTableType,
		RowCount:    tm.RowCount,
		// TODO: Printing of Size should be controlled by a param,
		// e.g. "show-volatile-fields". Until then, we omit it.
		Size:        nil,
		Comment:     tm.Comment,
		ColumnCount: int64(len(tm.Columns)),
		Columns:     tm.Columns,
	}

	b, err := ioz.MarshalYAML(tmr)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
