package pq

import (
	"github.com/neilotoole/sq-driver/hackery/database/sql/driver"
	"github.com/neilotoole/sq-driver/hackery/drivers/postgres/oid"
)

func (rs *rows) ColumnTypes() []driver.ColumnType {
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
	//

	fields := make([]driver.ColumnType, len(rs.colNames))

	for i := range fields {

		typ, flg := rs.getFieldType(i)
		fields[i] = driver.ColumnType{
			TableName: "", // rows.columns[i].tableName,
			Name:      rs.colNames[i],
			Flags:     flg, //driver.Flags(rows.columns[i].flags),
			FieldType: typ, // driver.FieldType(rows.columns[i].fieldType),
			Decimals:  0,   /*rows.columns[i].decimals*/
		}

		//fields[i].FieldType = typ
		//fields[i].Flags = flg

		//
		//if rs.colFmts[i] == formatBinary {
		//	fields[i].Flags = driver.FlagBinary
		//}

	}

	return fields
}

func (rs *rows) getFieldType(i int) (driver.FieldType, driver.Flags) {

	typ := rs.colTyps[i]

	//fmt.Printf("%s: %s\n", rs.colNames[i], typ)

	//thing := oid.T_inet
	//
	switch typ {

	case oid.T_bool:
		return driver.FieldTypeTiny, 0
	case oid.T_bytea:
		return driver.FieldTypeBLOB, driver.FlagBinary
	case oid.T_char:

	case oid.T_name:

	case oid.T_int8:
		return driver.FieldTypeLong, 0
	case oid.T_int2:
		return driver.FieldTypeLong, 0
	case oid.T_int2vector:

	case oid.T_int4:
		return driver.FieldTypeLong, 0
	case oid.T_regproc:

	case oid.T_text:

	case oid.T_oid:

	case oid.T_tid:

	case oid.T_xid:

	case oid.T_cid:

	case oid.T_oidvector:

	case oid.T_pg_type:

	case oid.T_pg_attribute:

	case oid.T_pg_proc:

	case oid.T_pg_class:

	case oid.T_json:

	case oid.T_xml:

	case oid.T__xml:

	case oid.T_pg_node_tree:

	case oid.T__json:

	case oid.T_smgr:

	case oid.T_point:

	case oid.T_lseg:

	case oid.T_path:

	case oid.T_box:

	case oid.T_polygon:

	case oid.T_line:

	case oid.T__line:

	case oid.T_cidr:

	case oid.T__cidr:

	case oid.T_float4:
		return driver.FieldTypeFloat, 0
	case oid.T_float8:
		return driver.FieldTypeFloat, 0

	case oid.T_abstime:

	case oid.T_reltime:

	case oid.T_tinterval:

	case oid.T_unknown:

	case oid.T_circle:

	case oid.T__circle:

	case oid.T_money:

	case oid.T__money:

	case oid.T_macaddr:

	case oid.T_inet:

	case oid.T__bool:
		return driver.FieldTypeTiny, 0
	case oid.T__bytea:
		return driver.FieldTypeBLOB, driver.FlagBinary
	case oid.T__char:

	case oid.T__name:

	case oid.T__int2:
		return driver.FieldTypeLong, 0
	case oid.T__int2vector:

	case oid.T__int4:
		return driver.FieldTypeLong, 0

	case oid.T__regproc:

	case oid.T__text:

	case oid.T__tid:

	case oid.T__xid:

	case oid.T__cid:

	case oid.T__oidvector:

	case oid.T__bpchar:

	case oid.T__varchar:
		return driver.FieldTypeString, 0

	case oid.T__int8:
		return driver.FieldTypeLong, 0

	case oid.T__point:

	case oid.T__lseg:

	case oid.T__path:

	case oid.T__box:

	case oid.T__float4:
		return driver.FieldTypeDouble, 0

	case oid.T__float8:
		return driver.FieldTypeDouble, 0

	case oid.T__abstime:

	case oid.T__reltime:

	case oid.T__tinterval:

	case oid.T__polygon:

	case oid.T__oid:

	case oid.T_aclitem:

	case oid.T__aclitem:

	case oid.T__macaddr:

	case oid.T__inet:

	case oid.T_bpchar:

	case oid.T_varchar:
		return driver.FieldTypeString, 0

	case oid.T_date, oid.T__date:
		return driver.FieldTypeDate, 0

	case oid.T_time, oid.T__time:
		return driver.FieldTypeTime, 0

	case oid.T_timestamp:
		return driver.FieldTypeTimestamp, 0

	case oid.T__timestamp:
		return driver.FieldTypeTimestamp, 0

	case oid.T_timestamptz:

	case oid.T__timestamptz:

	case oid.T_interval:

	case oid.T__interval:

	case oid.T__numeric:

	case oid.T_pg_database:

	case oid.T__cstring:

	case oid.T_timetz:

	case oid.T__timetz:

	case oid.T_bit:

	case oid.T__bit:

	case oid.T_varbit:

	case oid.T__varbit:

	case oid.T_numeric:

	case oid.T_refcursor:

	case oid.T__refcursor:

	case oid.T_regprocedure:

	case oid.T_regoper:

	case oid.T_regoperator:

	case oid.T_regclass:

	case oid.T_regtype:

	case oid.T__regprocedure:

	case oid.T__regoper:

	case oid.T__regoperator:

	case oid.T__regclass:

	case oid.T__regtype:

	case oid.T_record:

	case oid.T_cstring:

	case oid.T_any:

	case oid.T_anyarray:

	case oid.T_void:

	case oid.T_trigger:

	case oid.T_language_handler:

	case oid.T_internal:

	case oid.T_opaque:

	case oid.T_anyelement:

	case oid.T__record:

	case oid.T_anynonarray:

	case oid.T_pg_authid:

	case oid.T_pg_auth_members:

	case oid.T__txid_snapshot:

	case oid.T_uuid:

	case oid.T__uuid:

	case oid.T_txid_snapshot:

	case oid.T_fdw_handler:

	case oid.T_anyenum:

	case oid.T_tsvector:

	case oid.T_tsquery:

	case oid.T_gtsvector:

	case oid.T__tsvector:

	case oid.T__gtsvector:

	case oid.T__tsquery:

	case oid.T_regconfig:

	case oid.T__regconfig:

	case oid.T_regdictionary:

	case oid.T__regdictionary:

	case oid.T_anyrange:

	case oid.T_event_trigger:

	case oid.T_int4range:

	case oid.T__int4range:

	case oid.T_numrange:

	case oid.T__numrange:

	case oid.T_tsrange:

	case oid.T__tsrange:

	case oid.T_tstzrange:

	case oid.T__tstzrange:

	case oid.T_daterange:

	case oid.T__daterange:

	case oid.T_int8range:

	case oid.T__int8range:
	}

	return driver.FieldTypeString, 0
}

/*




T__abstime
T__aclitem
T__bit
T__bool
T__box
T__bpchar
T__bytea
T__char
T__cid
T__cidr
T__circle
T__cstring
T__date
T__daterange
T__float4
T__float8
T__gtsvector
T__inet
T__int2
T__int2vector
T__int4
T__int4range
T__int8
T__int8range
T__interval
T__json
T__line
T__lseg
T__macaddr
T__money
T__name
T__numeric
T__numrange
T__oid
T__oidvector
T__path
T__point
T__polygon
T__record
T__refcursor
T__regclass
T__regconfig
T__regdictionary
T__regoper
T__regoperator
T__regproc
T__regprocedure
T__regtype
T__reltime
T__text
T__tid
T__time
T__timestamp
T__timestamptz
T__timetz
T__tinterval
T__tsquery
T__tsrange
T__tstzrange
T__tsvector
T__txid_snapshot
T__uuid
T__varbit
T__varchar
T__xid
T__xml
T_abstime
T_aclitem
T_any
T_anyarray
T_anyelement
T_anyenum
T_anynonarray
T_anyrange
T_bit
T_bool
T_box
T_bpchar
T_bytea
T_char
T_cid
T_cidr
T_circle
T_cstring
T_date
T_daterange
T_event_trigger
T_fdw_handler
T_float4
T_float8
T_gtsvector
T_inet
T_int2
T_int2vector
T_int4
T_int4range
T_int8
T_int8range
T_internal
T_interval
T_json
T_language_handler
T_line
T_lseg
T_macaddr
T_money
T_name
T_numeric
T_numrange
T_oid
T_oidvector
T_opaque
T_path
T_pg_attribute
T_pg_auth_members
T_pg_authid
T_pg_class
T_pg_database
T_pg_node_tree
T_pg_proc
T_pg_type
T_point
T_polygon
T_record
T_refcursor
T_regclass
T_regconfig
T_regdictionary
T_regoper
T_regoperator
T_regproc
T_regprocedure
T_regtype
T_reltime
T_smgr
T_text
T_tid
T_time
T_timestamp
T_timestamptz
T_timetz
T_tinterval
T_trigger
T_tsquery
T_tsrange
T_tstzrange
T_tsvector
T_txid_snapshot
T_unknown
T_uuid
T_varbit
T_varchar
T_void
T_xid
T_xml


*/
