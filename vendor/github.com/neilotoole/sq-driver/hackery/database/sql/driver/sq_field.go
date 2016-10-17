package driver

import (
	"fmt"
	"strings"

	"github.com/dustin/gojson"
)

// Fielder provides a means for getting detailed field type information from a result set.
type ColumnTyper interface {
	ColumnTypes() []ColumnType
}

type FieldType byte

func (f FieldType) String() string {

	switch f {
	case FieldTypeDecimal:
		return "Decimal"
	case FieldTypeTiny:
		return "Tiny"
	case FieldTypeShort:
		return "Short"
	case FieldTypeLong:
		return "Long"
	case FieldTypeFloat:
		return "Float"
	case FieldTypeDouble:
		return "Double"
	case FieldTypeNULL:
		return "NULL"
	case FieldTypeTimestamp:
		return "Timestamp"
	case FieldTypeLongLong:
		return "LongLong"
	case FieldTypeInt24:
		return "Int24"
	case FieldTypeDate:
		return "Date"
	case FieldTypeTime:
		return "Time"
	case FieldTypeDateTime:
		return "DateTime"
	case FieldTypeYear:
		return "Year"
	case FieldTypeNewDate:
		return "NewDate"
	case FieldTypeVarChar:
		return "VarChar"
	case FieldTypeBit:
		return "Bit"
	case FieldTypeNewDecimal:
		return "NewDecimal"
	case FieldTypeEnum:
		return "Enum"
	case FieldTypeSet:
		return "Set"
	case FieldTypeTinyBLOB:
		return "TinyBLOB"
	case FieldTypeMediumBLOB:
		return "MediumBLOB"
	case FieldTypeLongBLOB:
		return "LongBLOB"
	case FieldTypeBLOB:
		return "BLOB"
	case FieldTypeVarString:
		return "VarString"
	case FieldTypeString:
		return "String"
	case FieldTypeGeometry:
		return "Geometry"
	}
	return "Unknown"
}

// currently matches data in github.com/go-sql-driver/mysql/const.go
const (
	FieldTypeDecimal FieldType = iota
	FieldTypeTiny
	FieldTypeShort
	FieldTypeLong
	FieldTypeFloat
	FieldTypeDouble
	FieldTypeNULL
	FieldTypeTimestamp
	FieldTypeLongLong
	FieldTypeInt24
	FieldTypeDate
	FieldTypeTime
	FieldTypeDateTime
	FieldTypeYear
	FieldTypeNewDate
	FieldTypeVarChar
	FieldTypeBit
)
const (
	FieldTypeJSON FieldType = iota + 0xf5
	FieldTypeNewDecimal
	FieldTypeEnum
	FieldTypeSet
	FieldTypeTinyBLOB
	FieldTypeMediumBLOB
	FieldTypeLongBLOB
	FieldTypeBLOB
	FieldTypeVarString
	FieldTypeString
	FieldTypeGeometry
)

type Flags uint16

const (
	FlagNotNULL Flags = 1 << iota
	FlagPriKey
	FlagUniqueKey
	FlagMultipleKey
	FlagBLOB
	FlagUnsigned
	FlagZeroFill
	FlagBinary
	FlagEnum
	FlagAutoIncrement
	FlagTimestamp
	FlagSet
	FlagUnknown1
	FlagUnknown2
	FlagUnknown3
	FlagUnknown4
)

func (f Flags) String() string {
	vals := f.Names()
	return fmt.Sprintf("[%v]", strings.Join(vals, ", "))
}

func (f Flags) IsSet(flag Flags) bool {
	return (uint16(f) & uint16(flag)) > 0
}

func (f Flags) Names() []string {

	strs := []string{}

	if f&FlagNotNULL > 0 {
		strs = append(strs, "NotNULL")
	}
	if f&FlagPriKey > 0 {
		strs = append(strs, "PriKey")
	}
	if f&FlagUniqueKey > 0 {
		strs = append(strs, "UniqueKey")
	}
	if f&FlagMultipleKey > 0 {
		strs = append(strs, "MultipleKey")
	}
	if f&FlagBLOB > 0 {
		strs = append(strs, "BLOB")
	}
	if f&FlagUnsigned > 0 {
		strs = append(strs, "Unsigned")
	}
	if f&FlagZeroFill > 0 {
		strs = append(strs, "ZeroFill")
	}
	if f&FlagBinary > 0 {
		strs = append(strs, "Binary")
	}
	if f&FlagEnum > 0 {
		strs = append(strs, "Enum")
	}
	if f&FlagAutoIncrement > 0 {
		strs = append(strs, "AutoIncrement")
	}
	if f&FlagTimestamp > 0 {
		strs = append(strs, "Timestamp")
	}
	if f&FlagSet > 0 {
		strs = append(strs, "Set")
	}
	if f&FlagUnknown1 > 0 {
		strs = append(strs, "Unknown1")
	}
	if f&FlagUnknown2 > 0 {
		strs = append(strs, "Unknown2")
	}
	if f&FlagUnknown3 > 0 {
		strs = append(strs, "Unknown3")
	}
	if f&FlagUnknown4 > 0 {
		strs = append(strs, "Unknown4")
	}

	return strs
}

type ColumnType struct {
	TableName   string
	Name        string
	AliasedName string
	Flags       Flags
	FieldType   FieldType
	Decimals    byte
}

func (f ColumnType) String() string {

	bytes, _ := json.Marshal(f)
	return string(bytes)
}

//func (f *Field) HasFlag(flag Flags) bool {
//	return (uint16(f.Flags) & uint16(flag)) > 0
//}
