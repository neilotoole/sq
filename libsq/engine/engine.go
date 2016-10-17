package engine

import (
	"fmt"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/drvr/scratch"
	"github.com/neilotoole/sq/libsq/drvr/sqlh"
	"github.com/neilotoole/sq/libsq/shutdown"
	"github.com/neilotoole/sq/libsq/util"
)

// RecordWriter outputs query results. The caller must invoke Close() when
// all records are written.
type RecordWriter interface {
	Records(records []*sqlh.Record) error
	Close() error
}

// BuildPlan encapsulates construction of a Plan from SLQ input.
func BuildPlan(srcs *drvr.SourceSet, slq string) (*Plan, error) {
	ptree, err := ast.NewParser().Parse(slq)
	if err != nil {
		return nil, err
	}

	ast, err := ast.NewBuilder(srcs).Build(ptree)
	if err != nil {
		return nil, err
	}
	plan, err := NewPlanBuilder(srcs).Build(ast)
	return plan, err
}

// Error is an error generated within the engine package.
type Error struct {
	msg string
}

func (e *Error) Error() string {
	return e.msg
}

func errorf(format string, v ...interface{}) *Error {
	err := &Error{msg: fmt.Sprintf(format, v...)}
	lg.Depth(1).Warnf("error created: %s", err.msg)
	return err
}

// Engine is the engine for executing a sq Plan.
type Engine struct {
	srcs   *drvr.SourceSet
	plan   *Plan
	writer RecordWriter
}

// New returns a new execution engine.
func New(srcs drvr.SourceSet, plan *Plan, writer RecordWriter) *Engine {
	xng := &Engine{srcs: &srcs, plan: plan, writer: writer}
	return xng
}

// Execute begins execution of the plan.
func (en *Engine) Execute() error {
	selectable := en.plan.Selectable
	lg.Debugf("using selectable: %T: %s", selectable, selectable)

	var src *drvr.Source
	var fromClause string
	var err error

	switch selectable := selectable.(type) {

	case *ast.TblSelector:
		src, fromClause, err = en.handleTblSelectable(selectable)
	case *ast.FnJoin:
		src, fromClause, err = en.handleJoinSelectable(selectable)
	default:
		return errorf("unknown selectable %T: %q", selectable, selectable)
	}

	if err != nil {
		return err
	}

	rndr, err := RendererFor(src.Type)
	if err != nil {
		return err
	}

	selectColsClause, err := rndr.SelectCols(en.plan.Cols)
	if err != nil {
		return err
	}

	var rangeClause string

	if en.plan.Range != nil {
		rangeClause, err = rndr.Range(en.plan.Range)
		if err != nil {
			return err
		}
	}

	sql := selectColsClause + " " + fromClause

	if rangeClause != "" {
		sql = sql + " " + rangeClause
	}

	lg.Debugf("SQL: %s", sql)

	db, err := NewDatabase(src)
	if err != nil {
		return err
	}
	err = db.Query(sql, en.writer)
	return err

}

func (en *Engine) handleTblSelectable(tblSel *ast.TblSelector) (*drvr.Source, string, error) {

	src, err := en.srcs.Get(tblSel.DSName)
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

func (en *Engine) handleJoinSelectable(fnJoin *ast.FnJoin) (*drvr.Source, string, error) {

	if fnJoin.LeftTbl() == nil || fnJoin.LeftTbl().SelValue() == "" {
		return nil, "", errorf("JOIN is missing left table reference")
	}

	if fnJoin.RightTbl() == nil || fnJoin.RightTbl().SelValue() == "" {
		return nil, "", errorf("JOIN is missing right table reference")
	}

	if fnJoin.LeftTbl().DSName != fnJoin.RightTbl().DSName {
		return en.handleCrossDatasourceJoin(fnJoin)
	}

	src, err := en.srcs.Get(fnJoin.LeftTbl().DSName)
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

func (en *Engine) handleCrossDatasourceJoin(fnJoin *ast.FnJoin) (*drvr.Source, string, error) {
	if fnJoin.LeftTbl().SelValue() == fnJoin.RightTbl().SelValue() {
		return nil, "", errorf("JOIN tables must have distinct names (use aliases): duplicate tbl name %q", fnJoin.LeftTbl().SelValue())
	}

	// This is a highly naive strategy (needs to be optimized, greatly!)
	// currently we just copy both tables to the scratch database

	// let's open the scratch db
	scratchSrc, scratchDB, cleanup, err := scratch.OpenNew()
	shutdown.Add(cleanup)
	if err != nil {
		return nil, "", err
	}

	scratchRndr, err := RendererFor(scratch.Type())
	if err != nil {
		return nil, "", err
	}

	// TODO: parallelize these imports
	leftSrc, _, _, err := en.importJoinTbl(scratchDB, scratchRndr, fnJoin.LeftTbl())
	if err != nil {
		return nil, "", err
	}
	rightSrc, _, _, err := en.importJoinTbl(scratchDB, scratchRndr, fnJoin.RightTbl())
	if err != nil {
		return nil, "", err
	}

	lg.Debugf("import succeeded for left source: %v", leftSrc)
	lg.Debugf("import succeeded for right source: %v", rightSrc)

	sqlFragment, err := scratchRndr.Join(fnJoin)
	if err != nil {
		return nil, "", err
	}

	return scratchSrc, sqlFragment, nil
}

func (en *Engine) importJoinTbl(scratchDB *sql.DB, scratchRndr Renderer, tblSel *ast.TblSelector) (*drvr.Source, int64, string, error) {

	returnErr := func(err error) (*drvr.Source, int64, string, error) {
		return nil, -1, "", err
	}

	src, err := en.srcs.Get(tblSel.DSName)
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

	queryDB, queryRows, err := en.execQuery(src, queryAll)
	if err != nil {
		return returnErr(err)
	}
	defer queryRows.Close()
	defer queryDB.Close()

	colNames, err := queryRows.Columns()
	if err != nil {
		return returnErr(err)
	}

	fields, err := queryRows.ColumnTypes()
	if err != nil {
		return returnErr(err)
	}

	rr, err := sqlh.NewRecord(fields)
	if err != nil {
		return returnErr(err)
	}

	lg.Debugf("table types: %v", rr.ReflectTypes())

	scratchTblCreateStmt, err := scratchRndr.CreateTable(tblSel.TblName, colNames, rr.ReflectTypes())
	if err != nil {
		return returnErr(err)
	}

	lg.Debugf("table create stmt: %s", scratchTblCreateStmt)
	_, err = scratchDB.Exec(scratchTblCreateStmt)
	if err != nil {
		return returnErr(util.WrapError(err))
	}

	lg.Debugf("scratch DB table created: %s", tblSel.TblName)

	scratchInsertStmtTpl, err := scratchRndr.CreateInsertStmt(tblSel.TblName, colNames)
	if err != nil {
		return returnErr(err)
	}

	lg.Debugf("using insert tpl: %s", scratchInsertStmtTpl)

	rowsAffected := int64(0)
	for queryRows.Next() {
		rr, err := sqlh.NewRecord(fields)
		if err != nil {
			return returnErr(err)
		}

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

		rowsAffected = rowsAffected + ra
	}

	return src, rowsAffected, tblSel.TblName, nil
}

// Execute the query against the source. The caller is responsible for invoking
// Close() on the returned DB and Rows.
func (en *Engine) execQuery(src *drvr.Source, query string) (*sql.DB, *sql.Rows, error) {
	drv, err := drvr.For(src)
	if err != nil {
		return nil, nil, err
	}

	lg.Debugf("attempting to open SQL connection for datasource %q with query: %q", src, query)
	db, err := drv.Open(src)
	if err != nil {
		return nil, nil, err
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}

	return db, rows, nil

}
