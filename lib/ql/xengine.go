package ql

import (
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/driver/scratch"
	"github.com/neilotoole/sq/lib/out"
	"github.com/neilotoole/sq/lib/util"
)

// XEngine is the QL Execution Engine.
type XEngine struct {
	stmt   *SelectStmt
	ast    *AST
	writer out.ResultWriter
}

// NewXEngine returns an execution engine for the given statement.
func NewXEngine(stmt *SelectStmt, writer out.ResultWriter) *XEngine {
	xng := &XEngine{stmt: stmt, ast: stmt.AST, writer: writer}
	return xng
}

func (x *XEngine) Execute() error {

	//insp := NewInspector(x.ast)
	//finalSelSeg, err := insp.findFinalSelectableSegment()
	//if err != nil {
	//	return err
	//}
	//
	//children := finalSelSeg.Children()
	//if len(children) != 1 {
	//	return errorf("expected final selectable segment to have one child")
	//}
	//
	//selectable, ok := children[0].(Selectable)
	//if !ok {
	//	return errorf("expected child of final selectable segment to implement %q", TypeSelectable)
	//}

	selectable := x.stmt.Selectable
	lg.Debugf("using selectable: %T: %s", selectable, selectable)

	var src *driver.Source
	var fromClause string
	var err error

	switch selectable := selectable.(type) {

	case *TblSelector:
		src, fromClause, err = x.handleTblSelectable(selectable)
	case *FnJoin:
		src, fromClause, err = x.handleJoinSelectable(selectable)
	default:
		return errorf("unknown selectable %T: %q", selectable, selectable)
	}

	if err != nil {
		return err
	}

	//lg.Debugf("using datasource: %s", src)
	//lg.Debugf("using FROM clause: %s", fromClause)

	rndr, err := RendererFor(src.Type)
	if err != nil {
		return err
	}

	selectColsClause, err := rndr.SelectCols(x.stmt.Cols)
	if err != nil {
		return err
	}

	var rangeClause string

	if x.stmt.Range != nil {
		rangeClause, err = rndr.Range(x.stmt.Range)
		if err != nil {
			return err
		}
	}

	sql := selectColsClause + " " + fromClause

	if rangeClause != "" {
		sql = sql + " " + rangeClause
	}

	lg.Debugf("SQL: %s", sql)

	database, err := NewDatabase(src)
	if err != nil {
		return err
	}
	err = database.ExecuteAndWrite(sql, x.writer)
	return err

}

func (x *XEngine) handleTblSelectable(tblSel *TblSelector) (*driver.Source, string, error) {

	src, err := sourceSet.Get(tblSel.dsName)
	if err != nil {
		return nil, "", err
	}

	rndr, err := RendererFor(src.Type)
	if err != nil {
		return nil, "", err
	}

	fragment, err := rndr.FromTable(tblSel)
	if err != nil {
		return nil, "", err
	}

	return src, fragment, nil
}

func (x *XEngine) handleJoinSelectable(fnJoin *FnJoin) (*driver.Source, string, error) {

	if fnJoin.leftTbl == nil || fnJoin.leftTbl.SelValue() == "" {
		return nil, "", errorf("JOIN is missing left table reference")
	}

	if fnJoin.rightTbl == nil || fnJoin.rightTbl.SelValue() == "" {
		return nil, "", errorf("JOIN is missing right table reference")
	}

	if fnJoin.leftTbl.dsName != fnJoin.rightTbl.dsName {
		return x.handleCrossDatasourceJoin(fnJoin)
	}

	src, err := sourceSet.Get(fnJoin.leftTbl.dsName)
	if err != nil {
		return nil, "", err
	}

	rndr, err := RendererFor(src.Type)
	if err != nil {
		return nil, "", err
	}

	fragment, err := rndr.Join(fnJoin)
	if err != nil {
		return nil, "", err
	}

	return src, fragment, nil
}

func (x *XEngine) handleCrossDatasourceJoin(fnJoin *FnJoin) (*driver.Source, string, error) {

	if fnJoin.leftTbl.SelValue() == fnJoin.rightTbl.SelValue() {
		return nil, "", errorf("JOIN tables must have distinct names (use aliases): duplicate tbl name %q", fnJoin.leftTbl.SelValue())
	}

	// This is a super-naive strategy (needs to be optimized): just copy both
	// tables to the scratch database
	// let's open the scratch db
	//scratchDB, err := scratch.Open()
	//if err != nil {
	//	return nil, "", err
	//}

	scratchSrc, scratchDB, err := scratch.OpenNew()
	if err != nil {
		return nil, "", err
	}
	//scratchDB, err := sql.Open(string(scratchSrc.Type), scratchSrc.ConnURI())
	//if err != nil {
	//	return nil, "", util.WrapError(err)
	//}

	scratchRndr, err := RendererFor(scratch.Type())
	if err != nil {
		return nil, "", err
	}

	// TODO: parallelize these imports
	leftSrc, _, _, err := x.importJoinTbl(scratchDB, scratchRndr, fnJoin.leftTbl)
	if err != nil {
		return nil, "", err
	}
	rightSrc, _, _, err := x.importJoinTbl(scratchDB, scratchRndr, fnJoin.rightTbl)
	if err != nil {
		return nil, "", err
	}

	lg.Debugf("import succeeded for left source: %v", leftSrc)
	lg.Debugf("import succeeded for right source: %v", rightSrc)

	sql, err := scratchRndr.Join(fnJoin)
	if err != nil {
		return nil, "", err
	}

	return scratchSrc, sql, nil

	//return nil, "", errorf("cross-datasource JOIN not yet supported:  %q  |  %q", fnJoin.leftTbl.dsName, fnJoin.rightTbl.dsName)
}

func (x *XEngine) importJoinTbl(scratchDB *sql.DB, scratchRndr Renderer, tblSel *TblSelector) (*driver.Source, int64, string, error) {

	returnErr := func(err error) (*driver.Source, int64, string, error) {
		return nil, -1, "", err
	}

	src, err := sourceSet.Get(tblSel.dsName)
	if err != nil {
		return returnErr(err)
	}

	rndr, err := RendererFor(src.Type)
	if err != nil {
		return returnErr(err)
	}

	queryAll, err := rndr.SelectAll(tblSel)
	if err != nil {
		return returnErr(err)
	}

	queryDB, queryRows, err := x.execQuery(src, queryAll)
	if err != nil {
		return returnErr(err)
	}
	defer queryRows.Close()
	defer queryDB.Close()

	colNames, err := queryRows.Columns()
	if err != nil {
		return returnErr(err)
	}

	fields, err := queryRows.Fields()
	if err != nil {
		return returnErr(err)
	}

	rr := common.NewResultRow(fields)

	lg.Debugf("table types: %v", rr.Types())

	scratchTblCreateStmt, err := scratchRndr.CreateTable(tblSel.tblName, colNames, rr.Types())
	if err != nil {
		return returnErr(err)
	}

	lg.Debugf("table create stmt: %s", scratchTblCreateStmt)
	_, err = scratchDB.Exec(scratchTblCreateStmt)
	if err != nil {
		return returnErr(util.WrapError(err))
	}

	lg.Debugf("scratch DB table created: %s", tblSel.tblName)

	scratchInsertStmtTpl, err := scratchRndr.CreateInsertStmt(tblSel.tblName, colNames)
	if err != nil {
		return returnErr(err)
	}

	lg.Debugf("using insert tpl: %s", scratchInsertStmtTpl)

	rowsAffected := int64(0)
	for queryRows.Next() {

		rr := common.NewResultRow(fields)

		err = queryRows.Scan(rr.Values...)
		if err != nil {
			return returnErr(err)
		}

		result, err := scratchDB.Exec(scratchInsertStmtTpl, rr.Values...)
		if err != nil {
			return returnErr(err)
		}

		//lastInsertID, err := result.LastInsertId()
		//if err != nil {
		//	return returnErr(err)
		//}

		ra, err := result.RowsAffected()
		if err != nil {
			return returnErr(err)
		}

		//lg.Debugf("insert success %q: row id [%d]: affected %d", tblSel.tblName, lastInsertID, ra)
		rowsAffected = rowsAffected + ra

	}

	return src, rowsAffected, tblSel.tblName, nil
}

//func (x *XEngine) getCrossDatasourceLoadQuery(src *driver.Source, tblSel *TblSelector) (string, error) {
//	rndr, err := RendererFor(src.Type)
//	if err != nil {
//		return "", err
//	}
//
//	query, err := rndr.SelectAll(tblSel)
//	return query, err
//}

// Execute the query against the source. The caller is responsible for invoking
// Close() on the returned DB and Rows.
func (x *XEngine) execQuery(src *driver.Source, query string) (*sql.DB, *sql.Rows, error) {

	drvr, err := driver.For(src)
	if err != nil {
		return nil, nil, err
	}

	//db, err := sql.Open(string(d.src.Type), d.src.ConnURI())
	lg.Debugf("attempting to open SQL connection for datasource %q with query: %q", src, query)
	db, err := drvr.Open(src)
	if err != nil {
		return nil, nil, err
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}

	return db, rows, nil

}

//renderer := translator.NewRenderer("mysql")
//sql, err := renderer.Select(stmt)
//
//lg.Debugf("SQL query: %q", sql)
//
//ds, err := sourceSet.Get(ast.Datasource)
//if err != nil {
//return err
//}
//
//database, err := NewDatabase(ds)
//if err != nil {
//return err
//}
//
//err = database.Execute(sql, writer)
//return err
