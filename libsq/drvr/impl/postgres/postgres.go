package postgres

import (
	"fmt"
	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	_ "github.com/neilotoole/sq-driver/hackery/drivers/postgres"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/util"
)

type Driver struct {
}

const typ = drvr.Type("postgres")

func (d *Driver) Type() drvr.Type {
	return drvr.Type("postgres")
}

func (d *Driver) ConnURI(source *drvr.Source) (string, error) {
	return "", util.Errorf("not implemented")
}

func (d *Driver) Open(src *drvr.Source) (*sql.DB, error) {
	return sql.Open(string(src.Type), src.ConnURI())
}

func (d *Driver) Release() error {
	return nil
}

func (d *Driver) ValidateSource(src *drvr.Source) (*drvr.Source, error) {
	if src.Type != typ {
		return nil, util.Errorf("expected source type %q but got %q", typ, src.Type)
	}
	return src, nil
}

func (d *Driver) Ping(src *drvr.Source) error {
	db, err := d.Open(src)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}

func (d *Driver) Metadata(src *drvr.Source) (*drvr.SourceMetadata, error) {

	meta := &drvr.SourceMetadata{}
	meta.Handle = src.Handle
	meta.Location = src.Location
	db, err := d.Open(src)
	if err != nil {
		return nil, util.WrapError(err)
	}
	defer db.Close()

	q := `SELECT current_database(), pg_database_size(current_database())`

	row := db.QueryRow(q)
	err = row.Scan(&meta.Name, &meta.Size)
	if err != nil {
		return nil, util.WrapError(err)
	}

	q = "SELECT table_catalog, table_schema, table_name FROM information_schema.tables WHERE table_schema = 'public';"
	lg.Debugf("SQL: %s", q)

	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {

		tbl := &drvr.Table{}

		var tblCatalog, tblSchema, tblName string

		err = rows.Scan(&tblCatalog, &tblSchema, &tblName)
		if err != nil {
			return nil, util.WrapError(err)
		}

		meta.Name = tblCatalog
		meta.FullyQualifiedName = tblCatalog + "." + tblSchema
		//tbl.Name = tblSchema + "." + tblName
		tbl.Name = tblName

		err = populateTblMetadata(db, tblCatalog, tblSchema, tblName, tbl)
		if err != nil {
			return nil, err
		}

		meta.Tables = append(meta.Tables, *tbl)
	}

	return meta, nil

}

func populateTblMetadata(db *sql.DB, tblCatalog string, tblSchema string, tblName string, tbl *drvr.Table) error {

	//row := db.QueryRow(fmt.Sprintf("SELECT pg_total_relation_size('%s')"), tbl.Name)
	//row := db.QueryRow(fmt.Sprintf(`SELECT pg_total_relation_size('%s')`), tbl.Name)

	// TODO: One day some postgres guru can come along and collapse these separate
	// queries into AwesomeQuery: I'm not the guy.

	tpl := `SELECT pg_total_relation_size('%s.%s'), obj_description('%s.%s'::REGCLASS, 'pg_class'), COUNT(*) FROM "%s"."%s"`
	q := fmt.Sprintf(tpl, tblSchema, tblName, tblSchema, tblName, tblSchema, tblName)
	lg.Debugf("SQL: %s", q)
	row := db.QueryRow(q)

	tblComment := &sql.NullString{}
	err := row.Scan(&tbl.Size, tblComment, &tbl.RowCount)
	if err != nil {
		return util.WrapError(err)
	}
	tbl.Comment = tblComment.String

	// get the primary keys

	primaryKeys := []string{}

	tpl = `SELECT a.attname AS pk_col_name, format_type(a.atttypid, a.atttypmod) AS data_type
FROM  pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
WHERE  i.indrelid = '%s.%s'::regclass AND i.indisprimary`

	q = fmt.Sprintf(tpl, tblSchema, tblName)
	lg.Debugf("SQL: %s", q)
	rows, err := db.Query(q)
	if err != nil {
		return util.WrapError(err)
	}

	for rows.Next() {
		var pkCol, colTypeFormatted string

		err = rows.Scan(&pkCol, &colTypeFormatted)
		if err != nil {
			return util.WrapError(err)
		}

		primaryKeys = append(primaryKeys, pkCol)
	}

	err = rows.Close()
	if err != nil {
		return util.WrapError(err)
	}

	lg.Debugf("primary keys for %s: %v", tblName, primaryKeys)

	tpl = `SELECT column_name,
	CASE
		WHEN domain_name IS NOT NULL THEN domain_name
		WHEN data_type='character varying' THEN 'varchar('||character_maximum_length||')'
		WHEN data_type='character' THEN 'char('||character_maximum_length||')'
		WHEN data_type='numeric' THEN 'numeric('||numeric_precision||','||numeric_scale||')'
		ELSE data_type
	END AS col_type, data_type, ordinal_position, is_nullable,
	(
		SELECT
		    pg_catalog.col_description(c.oid, cols.ordinal_position::int)
		FROM
		    pg_catalog.pg_class c
		WHERE
		    c.oid = (SELECT ('"' || cols.table_name || '"')::regclass::oid)
		    AND c.relname = cols.table_name
    	) AS column_comment
	FROM information_schema.columns cols WHERE cols.table_catalog = '%s' AND cols.table_schema = '%s' AND cols.table_name = '%s'`
	q = fmt.Sprintf(tpl, tblCatalog, tblSchema, tblName)

	//tpl := "SELECT column_name, data_type, column_type, ordinal_position, is_nullable, column_key, column_comment, extra, (SELECT COUNT(*) FROM `%s`) AS row_count FROM information_schema.columns cols WHERE cols.TABLE_SCHEMA = '%s' AND cols.TABLE_NAME = '%s' ORDER BY cols.ordinal_position ASC"
	//sql := fmt.Sprintf(tpl, tbl.Name, dbName, tbl.Name)

	lg.Debugf("SQL:\n%s", q)

	rows, err = db.Query(q)
	if err != nil {
		return util.WrapError(err)
	}
	defer rows.Close()

	for rows.Next() {

		col := &drvr.Column{}
		//var isNullable string

		var r struct {
			ColName  sql.NullString
			ColType  sql.NullString
			Datatype sql.NullString
			Position sql.NullInt64
			Nullable sql.NullString
			Comment  sql.NullString
		}

		//comment := &sql.NullString{}

		//err = rows.Scan(&col.Name, &col.ColType, &col.Datatype, &col.Position, &isNullable, comment)
		err = rows.Scan(&r.ColName, &r.ColType, &r.Datatype, &r.Position, &r.Nullable, &r.Comment)
		if err != nil {
			return util.WrapError(err)
		}

		col.Name = r.ColName.String
		col.ColType = r.ColType.String
		col.Datatype = r.Datatype.String

		// HACK: no idea why this is happening
		if col.ColType == "" {
			col.ColType = r.Datatype.String
		}

		if col.Datatype == "" {
			col.Datatype = r.ColType.String
		}

		col.Position = r.Position.Int64
		col.Comment = r.Comment.String

		if "YES" == strings.ToUpper(r.Nullable.String) {
			col.Nullable = true
		}

		if util.InArray(primaryKeys, col.Name) {
			col.PrimaryKey = true
		}

		// Need to roll this query up into the main query at some point
		tpl = `SELECT f.adsrc AS default_val
FROM pg_attribute a LEFT JOIN pg_attrdef f ON f.adrelid = a.attrelid AND f.adnum = a.attnum
WHERE  a.attnum > 0 AND NOT a.attisdropped AND a.attrelid = '%s.%s'::regclass  AND a.attname = '%s'`
		q = fmt.Sprintf(tpl, tblSchema, tblName, col.Name)

		lg.Debugf("SQL:\n%s", q)

		row := db.QueryRow(q)
		defVal := &sql.NullString{}
		err = row.Scan(defVal)
		if err != nil {
			return util.WrapError(err)
		}

		col.DefaultValue = defVal.String

		tbl.Columns = append(tbl.Columns, *col)
	}

	return nil
}

func init() {
	d := &Driver{}
	drvr.Register(d)
}

/*
SELECT
U.usename                AS user_name,
ns.nspname               AS schema_name,
idx.indrelid :: REGCLASS AS table_name,
i.relname                AS index_name,
idx.indisunique          AS is_unique,
idx.indisprimary         AS is_primary,
am.amname                AS index_type,
idx.indkey,
ARRAY(
SELECT pg_get_indexdef(idx.indexrelid, k + 1, TRUE)
FROM
generate_subscripts(idx.indkey, 1) AS k
ORDER BY k
) AS index_keys,
(idx.indexprs IS NOT NULL) OR (idx.indkey::int[] @> array[0]) AS is_functional,
idx.indpred IS NOT NULL AS is_partial
FROM pg_index AS idx
JOIN pg_class AS i
ON i.oid = idx.indexrelid
JOIN pg_am AS am
ON i.relam = am.oid
JOIN pg_namespace AS NS ON i.relnamespace = NS.OID
JOIN pg_user AS U ON i.relowner = U.usesysid
WHERE NOT nspname LIKE 'pg%';
*/

/*

SELECT a.attnum
  ,a.attname                            AS name
  ,format_type(a.atttypid, a.atttypmod) AS typ
  ,a.attnotnull                         AS notnull
  ,coalesce(p.indisprimary, FALSE)      AS primary_key
  ,f.adsrc                              AS default_val
  ,d.description                        AS col_comment
FROM   pg_attribute    a
  LEFT   JOIN pg_index   p ON p.indrelid = a.attrelid AND a.attnum = ANY(p.indkey)
  LEFT   JOIN pg_description d ON d.objoid  = a.attrelid AND d.objsubid = a.attnum
  LEFT   JOIN pg_attrdef f ON f.adrelid = a.attrelid  AND f.adnum = a.attnum
WHERE  a.attnum > 0
       AND    NOT a.attisdropped
       AND    a.attrelid = 'public.tblorder'::regclass  -- table may be schema-qualified
ORDER  BY a.attnum;

*/

/*


SELECT
  U.usename                AS user_name,
  ns.nspname               AS schema_name,
  idx.indrelid :: REGCLASS AS table_name,
  i.relname                AS index_name,
  idx.indisunique          AS is_unique,
  idx.indisprimary         AS is_primary,
  am.amname                AS index_type,
  idx.indkey,
  ARRAY(
      SELECT pg_get_indexdef(idx.indexrelid, k + 1, TRUE)
      FROM
            generate_subscripts(idx.indkey, 1) AS k
      ORDER BY k
  ) AS index_keys,
  (idx.indexprs IS NOT NULL) OR (idx.indkey::int[] @> array[0]) AS is_functional,
  idx.indpred IS NOT NULL AS is_partial
FROM pg_index AS idx
  JOIN pg_class AS i
    ON i.oid = idx.indexrelid
  JOIN pg_am AS am
    ON i.relam = am.oid
  JOIN pg_namespace AS NS ON i.relnamespace = NS.OID
  JOIN pg_user AS U ON i.relowner = U.usesysid
WHERE NOT nspname LIKE 'pg%';
*/

/*
SELECT a.attname AS pk_col_name, format_type(a.atttypid, a.atttypmod) AS data_type
FROM   pg_index i
  JOIN   pg_attribute a ON a.attrelid = i.indrelid
                           AND a.attnum = ANY(i.indkey)
WHERE  i.indrelid = 'public.tblall'::regclass
       AND    i.indisprimary;
*/

/*
SELECT DISTINCT a.attnum
  ,a.attname                            AS name
  ,format_type(a.atttypid, a.atttypmod) AS typ
  ,a.attnotnull                         AS notnull
--   ,coalesce(p.indisprimary, FALSE)      AS primary_key
  ,f.adsrc                              AS default_val
FROM   pg_attribute    a
--   LEFT   JOIN pg_index   p ON p.indrelid = a.attrelid AND a.attnum = ANY(p.indkey)
  LEFT   JOIN pg_attrdef f ON f.adrelid = a.attrelid  AND f.adnum = a.attnum
WHERE  a.attnum > 0
       AND    NOT a.attisdropped
       AND    a.attrelid = 'public.tblorder'::regclass  -- table may be schema-qualified
ORDER  BY a.attnum;
*/
