package record

import (
	"database/sql"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"

	"github.com/neilotoole/sq/libsq/core/kind"
)

// FieldMeta is a bit of a strange entity, and in an ideal
// world, it wouldn't exist. It's here because:
//
// - The DB driver impls that sq utilizes (postgres, sqlite, etc)
// often have individual quirks (not reporting nullability etc)
// that necessitate modifying sql.ColumnType so that there's
// consistent behavior across the drivers.
//
// - We wanted to retain (and supplement) the method set of
// sql.ColumnType (basically, use the same "interface", even
// though sql.ColumnType is a struct, not interface) so that
// devs don't need to learn a whole new thing.
//
// - For that reason, stdlib sql.ColumnType needs to be
// supplemented with kind.Kind, and there needs to
// be a mechanism for modifying sql.ColumnType's fields.
//
// - But sql.ColumnType is sealed (its fields cannot be changed
// from outside its package).
//
// - Hence this construct where we have FieldMeta (which
// abstractly is an adapter around sql.ColumnType) and also
// a data holder struct (ColumnDataType), which permits the
// mutation of the column type fields.
//
// Likely there's a better design available than this one,
// but it suffices.
type FieldMeta struct {
	data       *ColumnTypeData
	mungedName string
}

// NewFieldMeta returns a new instance backed by the data arg.
// If mungedName is empty, ColumnTypeData.Name is used.
func NewFieldMeta(data *ColumnTypeData, mungedName string) *FieldMeta {
	if mungedName == "" {
		mungedName = data.Name
	}
	return &FieldMeta{data: data, mungedName: mungedName}
}

// String returns a log-debug friendly representation.
func (fm *FieldMeta) String() string {
	nullMsg := "?"
	if fm.data.HasNullable {
		nullMsg = strconv.FormatBool(fm.data.Nullable)
	}

	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s",
		fm.data.Name,
		fm.mungedName,
		fm.data.Kind.String(),
		fm.data.DatabaseTypeName,
		fm.ScanType().String(),
		nullMsg,
	)
}

// Name is documented by sql.ColumnType.Name.
func (fm *FieldMeta) Name() string {
	return fm.data.Name
}

// MungedName returns the (possibly-munged) column name.
// This value is what should be used for outputting the col name.
// This exists largely to handle the case of duplicate col names
// in a result set, e.g. when doing a JOIN on tables with
// identically-named columns. But typically this value is the same
// as that returned by FieldMeta.Name.
func (fm *FieldMeta) MungedName() string {
	return fm.mungedName
}

// Length is documented by sql.ColumnType.Length.
func (fm *FieldMeta) Length() (length int64, ok bool) {
	return fm.data.Length, fm.data.HasLength
}

// DecimalSize is documented by sql.ColumnType.DecimalSize.
func (fm *FieldMeta) DecimalSize() (precision, scale int64, ok bool) {
	return fm.data.Precision, fm.data.Scale, fm.data.HasPrecisionScale
}

// ScanType is documented by sql.ColumnType.ScanType.
func (fm *FieldMeta) ScanType() reflect.Type {
	return fm.data.ScanType
}

// Nullable is documented by sql.ColumnType.Nullable.
func (fm *FieldMeta) Nullable() (nullable, ok bool) {
	return fm.data.Nullable, fm.data.HasNullable
}

// DatabaseTypeName is documented by sql.ColumnType.DatabaseTypeName.
func (fm *FieldMeta) DatabaseTypeName() string {
	return fm.data.DatabaseTypeName
}

// Kind returns the data kind for the column.
func (fm *FieldMeta) Kind() kind.Kind {
	return fm.data.Kind
}

// Meta is a slice of *FieldMeta, encapsulating the metadata
// for a record.
type Meta []*FieldMeta

// Names returns the column names. These are the col names from
// the database. See also: MungedNames.
func (rm Meta) Names() []string {
	names := make([]string, len(rm))
	for i, col := range rm {
		names[i] = col.Name()
	}

	return names
}

// MungedNames returns the munged column names, which may be
// the same as those returned from Meta.Names.
func (rm Meta) MungedNames() []string {
	names := make([]string, len(rm))
	for i, col := range rm {
		names[i] = col.MungedName()
	}

	return names
}

// NewScanRow returns a new []any that can be scanned
// into by sql.Rows.Scan.
func (rm Meta) NewScanRow() []any {
	dests := make([]any, len(rm))

	for i, col := range rm {
		if col.data.ScanType == nil {
			// If there's no scan type set, fall back on *any
			dests[i] = new(any)
			continue
		}

		val := reflect.New(col.data.ScanType)
		dests[i] = val.Interface()
	}

	return dests
}

// Kinds returns the data kinds for the record.
func (rm Meta) Kinds() []kind.Kind {
	kinds := make([]kind.Kind, len(rm))
	for i, col := range rm {
		kinds[i] = col.Kind()
	}

	return kinds
}

// ScanTypes returns the scan types for the record.
func (rm Meta) ScanTypes() []reflect.Type {
	scanTypes := make([]reflect.Type, len(rm))
	for i, col := range rm {
		scanTypes[i] = col.ScanType()
	}

	return scanTypes
}

// LogValue implements slog.LogValuer.
func (rm Meta) LogValue() slog.Value {
	if len(rm) == 0 {
		return slog.Value{}
	}

	a := make([]string, len(rm))
	for i := range rm {
		a[i] = rm[i].String()
	}

	return slog.AnyValue(a)
}

// ColumnTypeData contains the same data as sql.ColumnType
// as well SQ's derived data kind. This type exists with
// exported fields instead of methods (as on sql.ColumnType)
// due to the need to work with the fields for testing, and
// also because for some drivers it's useful to twiddle with
// the scan type.
//
// This is all a bit ugly.
type ColumnTypeData struct {
	Name string `json:"name"`

	HasNullable       bool `json:"has_nullable"`
	HasLength         bool `json:"has_length"`
	HasPrecisionScale bool `json:"has_precision_scale"`

	Nullable         bool         `json:"nullable"`
	Length           int64        `json:"length"`
	DatabaseTypeName string       `json:"database_type_name"`
	Precision        int64        `json:"precision"`
	Scale            int64        `json:"scale"`
	ScanType         reflect.Type `json:"scan_type"`

	Kind kind.Kind `json:"kind"`
}

// NewColumnTypeData returns a new instance with field values
// taken from col, supplemented with the kind param.
func NewColumnTypeData(col *sql.ColumnType, knd kind.Kind) *ColumnTypeData {
	ct := &ColumnTypeData{
		Name:             col.Name(),
		DatabaseTypeName: col.DatabaseTypeName(),
		ScanType:         col.ScanType(),
		Kind:             knd,
	}

	ct.Nullable, ct.HasNullable = col.Nullable()
	ct.Length, ct.HasLength = col.Length()
	ct.Precision, ct.Scale, ct.HasPrecisionScale = col.DecimalSize()

	return ct
}

// SetKindIfUnknown sets meta[i].kind to k, iff the kind is
// currently kind.Unknown or kind.Null. This function can be used to set
// the kind after-the-fact, which is useful for some databases
// that don't always return sufficient type info upfront.
func SetKindIfUnknown(meta Meta, i int, k kind.Kind) {
	if meta[i].data.Kind == kind.Unknown || meta[i].data.Kind == kind.Null {
		meta[i].data.Kind = k
	}
}
