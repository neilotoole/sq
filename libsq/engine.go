package libsq

import (
	"context"
	"fmt"

	"github.com/neilotoole/errgroup"
	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// engine executes a queryModel and writes to a RecordWriter.
type engine struct {
	log          lg.Log
	srcs         *source.Set
	dbOpener     driver.DatabaseOpener
	joinDBOpener driver.JoinDatabaseOpener

	// tasks contains tasks that must be completed before targetSQL
	// is executed against targetDB. Typically tasks is used to
	// set up the joindb before it is queried.
	tasks []tasker

	// targetSQL is the ultimate SQL query to be executed against
	// targetDB.
	targetSQL string

	// targetDB is the destination for the ultimate SQL query to
	// be executed against.
	targetDB driver.Database
}

// prepare prepares the engine to execute queryModel.
// When this method returns, targetDB and targetSQL will be set,
// as will any tasks (may be empty). The tasks must be executed
// against targetDB before targetSQL is executed (the engine.execute
// method does this work).
func (ng *engine) prepare(ctx context.Context, qm *queryModel) error {
	selectable := qm.Selectable

	var fromClause string
	var err error

	switch selectable := selectable.(type) {
	case *ast.TblSelector:
		fromClause, ng.targetDB, err = ng.buildTableFromClause(ctx, selectable)
		if err != nil {
			return err
		}
	case *ast.Join:
		fromClause, ng.targetDB, err = ng.buildJoinFromClause(ctx, selectable)
		if err != nil {
			return err
		}
	default:
		return errz.Errorf("unknown selectable %T: %q", selectable, selectable)
	}

	fragBuilder, qb := ng.targetDB.SQLDriver().SQLBuilder()

	selectColsClause, err := fragBuilder.SelectCols(qm.Cols)
	if err != nil {
		return err
	}

	qb.SetSelect(selectColsClause)
	qb.SetFrom(fromClause)

	var rangeClause string

	if qm.Range != nil {
		rangeClause, err = fragBuilder.Range(qm.Range)
		if err != nil {
			return err
		}
		qb.SetRange(rangeClause)
	}

	if qm.Where != nil {
		whereClause, err := fragBuilder.Where(qm.Where)
		if err != nil {
			return err
		}

		qb.SetWhere(whereClause)
	}

	ng.targetSQL, err = qb.SQL()
	if err != nil {
		return err
	}

	return nil
}

// execute executes the plan that was built by engine.prepare.
func (ng *engine) execute(ctx context.Context, recw RecordWriter) error {
	ng.log.Debugf("engine.execute: [%s]: %s", ng.targetDB.Source().Handle, ng.targetSQL)

	err := ng.executeTasks(ctx)
	if err != nil {
		return err
	}

	return QuerySQL(ctx, ng.log, ng.targetDB, recw, ng.targetSQL)
}

// executeTasks executes any tasks in engine.tasks.
// These tasks may exist if preparatory work must be performed
// before engine.targetSQL can be executed.
func (ng *engine) executeTasks(ctx context.Context) error {
	switch len(ng.tasks) {
	case 0:
		return nil
	case 1:
		return ng.tasks[0].executeTask(ctx, ng.log)
	default:
	}

	g, gCtx := errgroup.WithContextN(ctx, driver.Tuning.ErrgroupNumG, driver.Tuning.ErrgroupQSize)
	for _, task := range ng.tasks {
		task := task

		g.Go(func() error {
			return task.executeTask(gCtx, ng.log)
		})
	}

	return g.Wait()
}

func (ng *engine) buildTableFromClause(ctx context.Context, tblSel *ast.TblSelector) (fromClause string, fromConn driver.Database, err error) {
	src, err := ng.srcs.Get(tblSel.DSName)
	if err != nil {
		return "", nil, err
	}

	fromConn, err = ng.dbOpener.Open(ctx, src)
	if err != nil {
		return "", nil, err
	}

	fragBuilder, _ := fromConn.SQLDriver().SQLBuilder()
	fromClause, err = fragBuilder.FromTable(tblSel)
	if err != nil {
		return "", nil, err
	}

	return fromClause, fromConn, nil
}

func (ng *engine) buildJoinFromClause(ctx context.Context, fnJoin *ast.Join) (fromClause string, fromConn driver.Database, err error) {
	if fnJoin.LeftTbl() == nil || fnJoin.LeftTbl().SelValue() == "" {
		return "", nil, errz.Errorf("JOIN is missing left table reference")
	}

	if fnJoin.RightTbl() == nil || fnJoin.RightTbl().SelValue() == "" {
		return "", nil, errz.Errorf("JOIN is missing right table reference")
	}

	if fnJoin.LeftTbl().DSName != fnJoin.RightTbl().DSName {
		return ng.crossSourceJoin(ctx, fnJoin)
	}

	return ng.singleSourceJoin(ctx, fnJoin)
}

func (ng *engine) singleSourceJoin(ctx context.Context, fnJoin *ast.Join) (fromClause string, fromDB driver.Database, err error) {
	src, err := ng.srcs.Get(fnJoin.LeftTbl().DSName)
	if err != nil {
		return "", nil, err
	}

	fromDB, err = ng.dbOpener.Open(ctx, src)
	if err != nil {
		return "", nil, err
	}

	fragBuilder, _ := fromDB.SQLDriver().SQLBuilder()
	fromClause, err = fragBuilder.Join(fnJoin)
	if err != nil {
		return "", nil, err
	}

	return fromClause, fromDB, nil
}

// crossSourceJoin returns a FROM clause that forms part of
// the SQL SELECT statement against fromDB.
func (ng *engine) crossSourceJoin(ctx context.Context, fnJoin *ast.Join) (fromClause string, fromDB driver.Database, err error) {
	leftTblName, rightTblName := fnJoin.LeftTbl().SelValue(), fnJoin.RightTbl().SelValue()
	if leftTblName == rightTblName {
		return "", nil, errz.Errorf("JOIN tables must have distinct names (or use aliases): duplicate tbl name %q", fnJoin.LeftTbl().SelValue())
	}

	leftSrc, err := ng.srcs.Get(fnJoin.LeftTbl().DSName)
	if err != nil {
		return "", nil, err
	}

	rightSrc, err := ng.srcs.Get(fnJoin.RightTbl().DSName)
	if err != nil {
		return "", nil, err
	}

	// Open the join db
	joinDB, err := ng.joinDBOpener.OpenJoin(ctx, leftSrc, rightSrc)
	if err != nil {
		return "", nil, err
	}

	leftDB, err := ng.dbOpener.Open(ctx, leftSrc)
	if err != nil {
		return "", nil, err
	}
	leftCopyTask := &joinCopyTask{
		fromDB:      leftDB,
		fromTblName: leftTblName,
		toDB:        joinDB,
		toTblName:   leftTblName,
	}

	rightDB, err := ng.dbOpener.Open(ctx, rightSrc)
	if err != nil {
		return "", nil, err
	}
	rightCopyTask := &joinCopyTask{
		fromDB:      rightDB,
		fromTblName: rightTblName,
		toDB:        joinDB,
		toTblName:   rightTblName,
	}

	ng.tasks = append(ng.tasks, leftCopyTask)
	ng.tasks = append(ng.tasks, rightCopyTask)

	//err = execJoinCopyTasks(ctx, ng.log, joinDB, joinCopyTasks)
	//if err != nil {
	//	return "", nil, err
	//}

	joinDBFragBuilder, _ := joinDB.SQLDriver().SQLBuilder()
	fromClause, err = joinDBFragBuilder.Join(fnJoin)
	if err != nil {
		return "", nil, err
	}

	return fromClause, joinDB, nil
}

// tasker is the interface for executing a DB task.
type tasker interface {
	// executeTask executes a task against the DB.
	executeTask(ctx context.Context, log lg.Log) error
}

// joinCopyTask is a specification of a table data copy task to be performed
// for a cross-source join. That is, the data in fromDB.fromTblName will
// be copied to a table in toDB. If colNames is
// empty, all cols in fromTblName are to be copied.
type joinCopyTask struct {
	fromDB      driver.Database
	fromTblName string
	colNames    []string
	toDB        driver.Database
	toTblName   string
}

func (jt *joinCopyTask) executeTask(ctx context.Context, log lg.Log) error {
	return execCopyTable(ctx, log, jt.fromDB, jt.fromTblName, jt.toDB, jt.toTblName)
}

// execCopyTable performs the work of copying fromDB.fromTblName to destDB.destTblName.
func execCopyTable(ctx context.Context, log lg.Log, fromDB driver.Database, fromTblName string, destDB driver.Database, destTblName string) error {
	createTblHook := func(ctx context.Context, originRecMeta sqlz.RecordMeta, destDB driver.Database, tx sqlz.DB) error {
		destColNames := originRecMeta.Names()
		destColKinds := originRecMeta.Kinds()
		destTblDef := sqlmodel.NewTableDef(destTblName, destColNames, destColKinds)

		err := destDB.SQLDriver().CreateTable(ctx, tx, destTblDef)
		if err != nil {
			return errz.Wrapf(err, "failed to create dest table %s.%s", destDB.Source().Handle, destTblName)
		}

		return nil
	}

	inserter := NewDBWriter(log, destDB, destTblName, driver.Tuning.RecordChSize, createTblHook)

	query := "SELECT * FROM " + fromDB.SQLDriver().Dialect().Enquote(fromTblName)
	err := QuerySQL(ctx, log, fromDB, inserter, query)
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destDB.Source().Handle, destTblName)
	}

	affected, err := inserter.Wait() // Wait for the writer to finish processing
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destDB.Source().Handle, destTblName)
	}
	log.Debugf("Copied %d rows to %s.%s", affected, destDB.Source().Handle, destTblName)
	return nil
}

// queryModel is a model of a SLQ query built from the AST.
type queryModel struct {
	AST        *ast.AST
	Selectable ast.Selectable
	Cols       []ast.ColExpr
	Range      *ast.RowRange
	Where      *ast.Where
}

func (qm *queryModel) String() string {
	return fmt.Sprintf("%v | %v  |  %v", qm.Selectable, qm.Cols, qm.Range)
}

// buildQueryModel creates a queryModel instance from the AST.
func buildQueryModel(log lg.Log, a *ast.AST) (*queryModel, error) {
	if len(a.Segments()) == 0 {
		return nil, errz.Errorf("query model error: query does not have enough segments")
	}

	insp := ast.NewInspector(log, a)
	selectableSeg, err := insp.FindFinalSelectableSegment()
	if err != nil {
		return nil, err
	}

	if len(selectableSeg.Children()) != 1 {
		return nil, errz.Errorf("the final selectable segment must have exactly one selectable element, but found %d elements",
			len(selectableSeg.Children()))
	}

	selectable, ok := selectableSeg.Children()[0].(ast.Selectable)
	if !ok {
		return nil, errz.Errorf("the final selectable segment must have exactly one selectable element, but found element %T(%q)",
			selectableSeg.Children()[0], selectableSeg.Children()[0].Text())
	}

	qm := &queryModel{AST: a, Selectable: selectable}

	// Look for range
	for seg := selectableSeg.Next(); seg != nil; seg = seg.Next() {
		// Check if the first element of the segment is a row range, if not, just skip
		if rr, ok := seg.Children()[0].(*ast.RowRange); ok {
			if len(seg.Children()) != 1 {
				return nil, errz.Errorf("segment [%d] with row range must have exactly one element, but found %d: %q",
					seg.SegIndex(), len(seg.Children()), seg.Text())
			}

			if qm.Range != nil {
				return nil, errz.Errorf("only one row range permitted, but found %q and %q", qm.Range.Text(), rr.Text())
			}

			log.Debugf("found row range: %q", rr.Text())
			qm.Range = rr
		}
	}

	seg, err := insp.FindColExprSegment()
	if err != nil {
		return nil, err
	}

	if seg != nil {
		elems := seg.Children()
		colExprs := make([]ast.ColExpr, len(elems))
		for i, elem := range elems {
			colExpr, ok := elem.(ast.ColExpr)
			if !ok {
				return nil, errz.Errorf("expected element in segment [%d] to be col expr, but was %T", i, elem)
			}

			colExprs[i] = colExpr
		}

		qm.Cols = colExprs
	}

	whereClauses, err := insp.FindWhereClauses()
	if err != nil {
		return nil, err
	}

	if len(whereClauses) > 1 {
		return nil, errz.Errorf("only one WHERE clause is supported, but found %d", len(whereClauses))
	} else if len(whereClauses) == 1 {
		qm.Where = whereClauses[0]
	}

	return qm, nil
}
