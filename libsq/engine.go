package libsq

import (
	"context"
	"fmt"

	"github.com/neilotoole/sq/libsq/ast/render"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"golang.org/x/sync/errgroup"
)

// engine executes a queryModel and writes to a RecordWriter.
type engine struct {
	log *slog.Logger

	// qc is the context in which the query is executed.
	qc *QueryContext

	// rc is the Context for rendering SQL.
	// This field is set during engine.prepare. It can't be set before
	// then because the target DB to use is calculated during engine.prepare,
	// based on the input query and other context.
	rc *render.Context

	// tasks contains tasks that must be completed before targetSQL
	// is executed against targetDB. Typically tasks is used to
	// set up the joindb before it is queried.
	tasks []tasker

	// targetSQL is the ultimate SQL query to be executed against targetDB.
	targetSQL string

	// targetDB is the destination for the ultimate SQL query to
	// be executed against.
	targetDB driver.Database
}

func newEngine(ctx context.Context, qc *QueryContext, query string) (*engine, error) {
	log := lg.From(ctx)

	a, err := ast.Parse(log, query)
	if err != nil {
		return nil, err
	}

	qModel, err := buildQueryModel(log, a)
	if err != nil {
		return nil, err
	}

	ng := &engine{
		log: log,
		qc:  qc,
	}

	if err = ng.prepare(ctx, qModel); err != nil {
		return nil, err
	}

	return ng, nil
}

// execute executes the plan that was built by engine.prepare.
func (ng *engine) execute(ctx context.Context, recw RecordWriter) error {
	ng.log.Debug(
		"Execute SQL query",
		lga.Target, ng.targetDB.Source().Handle,
		lga.SQL, ng.targetSQL,
	)

	err := ng.executeTasks(ctx)
	if err != nil {
		return err
	}

	return QuerySQL(ctx, ng.targetDB, recw, ng.targetSQL)
}

// executeTasks executes any tasks in engine.tasks.
// These tasks may exist if preparatory work must be performed
// before engine.targetSQL can be executed.
func (ng *engine) executeTasks(ctx context.Context) error {
	switch len(ng.tasks) {
	case 0:
		return nil
	case 1:
		return ng.tasks[0].executeTask(ctx)
	default:
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(driver.Tuning.ErrgroupLimit)

	for _, task := range ng.tasks {
		task := task

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}
			return task.executeTask(gCtx)
		})
	}

	return g.Wait()
}

// buildTableFromClause builds the "FROM table" fragment.
//
// When this function returns, ng.rc will be set.
func (ng *engine) buildTableFromClause(ctx context.Context, tblSel *ast.TblSelectorNode) (fromClause string,
	fromConn driver.Database, err error,
) {
	src, err := ng.qc.Collection.Get(tblSel.Handle())
	if err != nil {
		return "", nil, err
	}

	fromConn, err = ng.qc.DBOpener.Open(ctx, src)
	if err != nil {
		return "", nil, err
	}

	rndr := fromConn.SQLDriver().Renderer()
	ng.rc = &render.Context{
		Renderer: rndr,
		Args:     ng.qc.Args,
		Dialect:  fromConn.SQLDriver().Dialect(),
	}

	fromClause, err = rndr.FromTable(ng.rc, tblSel)
	if err != nil {
		return "", nil, err
	}

	return fromClause, fromConn, nil
}

// buildJoinFromClause builds the "JOIN" clause.
//
// When this function returns, ng.rc will be set.
func (ng *engine) buildJoinFromClause(ctx context.Context, fnJoin *ast.JoinNode) (fromClause string,
	fromConn driver.Database, err error,
) {
	if fnJoin.LeftTbl() == nil || fnJoin.LeftTbl().TblName() == "" {
		return "", nil, errz.Errorf("JOIN is missing left table reference")
	}

	if fnJoin.RightTbl() == nil || fnJoin.RightTbl().TblName() == "" {
		return "", nil, errz.Errorf("JOIN is missing right table reference")
	}

	if fnJoin.LeftTbl().Handle() != fnJoin.RightTbl().Handle() {
		return ng.crossSourceJoin(ctx, fnJoin)
	}

	return ng.singleSourceJoin(ctx, fnJoin)
}

// singleSourceJoin sets up a join against a single source.
//
// On return, ng.rc will be set.
func (ng *engine) singleSourceJoin(ctx context.Context, fnJoin *ast.JoinNode) (fromClause string,
	fromDB driver.Database, err error,
) {
	src, err := ng.qc.Collection.Get(fnJoin.LeftTbl().Handle())
	if err != nil {
		return "", nil, err
	}

	fromDB, err = ng.qc.DBOpener.Open(ctx, src)
	if err != nil {
		return "", nil, err
	}

	rndr := fromDB.SQLDriver().Renderer()
	ng.rc = &render.Context{
		Renderer: rndr,
		Args:     ng.qc.Args,
		Dialect:  fromDB.SQLDriver().Dialect(),
	}

	fromClause, err = rndr.Join(ng.rc, fnJoin)
	if err != nil {
		return "", nil, err
	}

	return fromClause, fromDB, nil
}

// crossSourceJoin returns a FROM clause that forms part of
// the SQL SELECT statement against fromDB.
//
// On return, ng.rc will be set.
func (ng *engine) crossSourceJoin(ctx context.Context, fnJoin *ast.JoinNode) (fromClause string, fromDB driver.Database,
	err error,
) {
	leftTblName, rightTblName := fnJoin.LeftTbl().TblName(), fnJoin.RightTbl().TblName()
	if leftTblName == rightTblName {
		return "", nil, errz.Errorf("JOIN tables must have distinct names (or use aliases): duplicate tbl name {%s}",
			fnJoin.LeftTbl().TblName())
	}

	leftSrc, err := ng.qc.Collection.Get(fnJoin.LeftTbl().Handle())
	if err != nil {
		return "", nil, err
	}

	rightSrc, err := ng.qc.Collection.Get(fnJoin.RightTbl().Handle())
	if err != nil {
		return "", nil, err
	}

	// Open the join db
	joinDB, err := ng.qc.JoinDBOpener.OpenJoin(ctx, leftSrc, rightSrc)
	if err != nil {
		return "", nil, err
	}

	rndr := joinDB.SQLDriver().Renderer()
	ng.rc = &render.Context{
		Renderer: rndr,
		Args:     ng.qc.Args,
		Dialect:  joinDB.SQLDriver().Dialect(),
	}

	leftDB, err := ng.qc.DBOpener.Open(ctx, leftSrc)
	if err != nil {
		return "", nil, err
	}
	leftCopyTask := &joinCopyTask{
		fromDB:      leftDB,
		fromTblName: leftTblName,
		toDB:        joinDB,
		toTblName:   leftTblName,
	}

	rightDB, err := ng.qc.DBOpener.Open(ctx, rightSrc)
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

	fromClause, err = rndr.Join(ng.rc, fnJoin)
	if err != nil {
		return "", nil, err
	}

	return fromClause, joinDB, nil
}

// tasker is the interface for executing a DB task.
type tasker interface {
	// executeTask executes a task against the DB.
	executeTask(ctx context.Context) error
}

// joinCopyTask is a specification of a table data copy task to be performed
// for a cross-source join. That is, the data in fromDB.fromTblName will
// be copied to a table in toDB. If colNames is
// empty, all cols in fromTblName are to be copied.
type joinCopyTask struct {
	fromDB      driver.Database
	fromTblName string
	toDB        driver.Database
	toTblName   string
}

func (jt *joinCopyTask) executeTask(ctx context.Context) error {
	return execCopyTable(ctx, jt.fromDB, jt.fromTblName, jt.toDB, jt.toTblName)
}

// execCopyTable performs the work of copying fromDB.fromTblName to destDB.destTblName.
func execCopyTable(ctx context.Context, fromDB driver.Database, fromTblName string,
	destDB driver.Database, destTblName string,
) error {
	log := lg.From(ctx)

	createTblHook := func(ctx context.Context, originRecMeta sqlz.RecordMeta, destDB driver.Database,
		tx sqlz.DB,
	) error {
		destColNames := originRecMeta.Names()
		destColKinds := originRecMeta.Kinds()
		destTblDef := sqlmodel.NewTableDef(destTblName, destColNames, destColKinds)

		err := destDB.SQLDriver().CreateTable(ctx, tx, destTblDef)
		if err != nil {
			return errz.Wrapf(err, "failed to create dest table %s.%s", destDB.Source().Handle, destTblName)
		}

		return nil
	}

	inserter := NewDBWriter(destDB, destTblName, driver.Tuning.RecordChSize, createTblHook)

	query := "SELECT * FROM " + fromDB.SQLDriver().Dialect().Enquote(fromTblName)
	err := QuerySQL(ctx, fromDB, inserter, query)
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destDB.Source().Handle, destTblName)
	}

	affected, err := inserter.Wait() // Wait for the writer to finish processing
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destDB.Source().Handle, destTblName)
	}
	log.Debug("Copied %d rows to %s.%s", affected, destDB.Source().Handle, destTblName)
	return nil
}

// queryModel is a model of an SLQ query built from the AST.
type queryModel struct {
	AST      *ast.AST
	Table    ast.Tabler
	Cols     []ast.ResultColumn
	Range    *ast.RowRangeNode
	Where    *ast.WhereNode
	OrderBy  *ast.OrderByNode
	GroupBy  *ast.GroupByNode
	Distinct *ast.UniqueNode
}

func (qm *queryModel) String() string {
	return fmt.Sprintf("%v | %v  |  %v", qm.Table, qm.Cols, qm.Range)
}

// buildQueryModel creates a queryModel instance from the AST.
func buildQueryModel(log *slog.Logger, a *ast.AST) (*queryModel, error) {
	if len(a.Segments()) == 0 {
		return nil, errz.Errorf("query model error: query does not have enough segments")
	}

	insp := ast.NewInspector(a)
	tablerSeg, err := insp.FindFinalTablerSegment()
	if err != nil {
		return nil, err
	}

	if len(tablerSeg.Children()) != 1 {
		return nil, errz.Errorf(
			"the final selectable segment must have exactly one selectable element, but found %d elements",
			len(tablerSeg.Children()))
	}

	tabler, ok := tablerSeg.Children()[0].(ast.Tabler)
	if !ok {
		return nil, errz.Errorf(
			"the final selectable segment must have exactly one selectable element, but found element %T(%s)",
			tablerSeg.Children()[0], tablerSeg.Children()[0].Text())
	}

	qm := &queryModel{AST: a, Table: tabler}

	// Look for range
	for seg := tablerSeg.Next(); seg != nil; seg = seg.Next() {
		// Check if the first element of the segment is a row range, if not, just skip
		if rr, ok := seg.Children()[0].(*ast.RowRangeNode); ok {
			if len(seg.Children()) != 1 {
				return nil, errz.Errorf(
					"segment [%d] with row range must have exactly one element, but found %d: %s",
					seg.SegIndex(), len(seg.Children()), seg.Text())
			}

			if qm.Range != nil {
				return nil, errz.Errorf("only one row range permitted, but found {%s} and {%s}",
					qm.Range.Text(), rr.Text())
			}

			log.Debug("found row range: %s", rr.Text())
			qm.Range = rr
		}
	}

	seg, err := insp.FindColExprSegment()
	if err != nil {
		return nil, err
	}

	if seg != nil {
		elems := seg.Children()
		colExprs := make([]ast.ResultColumn, len(elems))
		for i, elem := range elems {
			colExpr, ok := elem.(ast.ResultColumn)
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

	if qm.OrderBy, err = insp.FindOrderByNode(); err != nil {
		return nil, err
	}

	if qm.GroupBy, err = insp.FindGroupByNode(); err != nil {
		return nil, err
	}

	if qm.Distinct, err = insp.FindUniqueNode(); err != nil {
		return nil, err
	}

	return qm, nil
}
