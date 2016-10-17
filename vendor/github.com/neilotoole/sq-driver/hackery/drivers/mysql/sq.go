package mysql

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq-driver/hackery/database/sql/driver"
)

// HACK: This functionality is a hack to smuggle detailed column info out of the
// mysql driver. This is ugly. It works as follows:

// - In rows.Columns, we call registerFields, passing the column names
//   []string, and the *mysqlRows pointer.
// - We know that the Columns() []string gets returned to the client unmolested,
//   so we will use the memory address of the first element of that slice as a
//   key into a map that stores the []Field.
// - The client can than call Fields(columns []string); this function
//   looks up the []Field in the fieldMap using the same memory address.
// - Since the fieldMap has to be static, we need to do cleanup to avoid
//   memory leaks. So in rows.Close(), we call unregisterFields(*mysqlRows).
// - We have conveniently created a mapping of the *mysqlRows -> ColumnInfo key
//   in another m ap (rowsToKeyMap), so we can use *mysqlRows to delete the entry
//   in columnInfoMap, and then delete the entry in the rowsToKeyMap.

var fieldMap = make(map[string][]*driver.ColumnType)
var rowsToKeyMap = make(map[*mysqlRows]string)

func registerFields(columnNames []string, rows *mysqlRows) {

	key := fmt.Sprintf("%p", &columnNames[0])

	fields := make([]*driver.ColumnType, len(rows.columns))

	for i := range fields {

		fields[i] = &driver.ColumnType{
			TableName: rows.columns[i].tableName,
			Name:      columnNames[i],
			Flags:     driver.Flags(rows.columns[i].flags),
			FieldType: driver.FieldType(rows.columns[i].fieldType),
			Decimals:  rows.columns[i].decimals}
	}

	fieldMap[key] = fields
	rowsToKeyMap[rows] = key
}

// Invoked to cleanup memory.
func unregisterFields(rows *mysqlRows) {
	key := rowsToKeyMap[rows]
	delete(fieldMap, key)
	delete(rowsToKeyMap, rows)
}

// Return full Field for a result set. The passed parameter must
// be the actual []string returned by rows.Columns(). Example:
//
// cols, err := rows.Columns()
// fields := mysql.Fields(cols).
//
// This function must be called immediately after the call to rows.Columns().
func Fields(columns []string) ([]*driver.ColumnType, error) {

	key := fmt.Sprintf("%p", &columns[0])

	val, ok := fieldMap[key]
	if !ok {
		return nil, fmt.Errorf(`field information not found for columns "%v"`, strings.Join(columns, ", "))
	}
	return val, nil
}

func (rows *mysqlRows) ColumnTypes() []driver.ColumnType {
	//columns := make([]string, len(rows.columns))
	//if rows.mc != nil && rows.mc.cfg.ColumnsWithAlias {
	//	for i := range columns {
	//		if tableName := rows.columns[i].tableName; len(tableName) > 0 {
	//			columns[i] = tableName + "." + rows.columns[i].name
	//		} else {
	//			columns[i] = rows.columns[i].name
	//		}
	//	}
	//} else {
	//	for i := range columns {
	//		columns[i] = rows.columns[i].name
	//	}
	//}
	//
	//registerFields(columns, rows)

	columnNames := rows.Columns()
	fields := make([]driver.ColumnType, len(rows.columns))

	for i := range fields {

		fields[i] = driver.ColumnType{
			TableName: rows.columns[i].tableName,
			Name:      columnNames[i],
			Flags:     driver.Flags(rows.columns[i].flags),
			FieldType: driver.FieldType(rows.columns[i].fieldType),
			Decimals:  rows.columns[i].decimals}
		//fmt.Printf("%s: %s\n", fields[i].Name, fields[i].FieldType)
	}

	return fields
}
