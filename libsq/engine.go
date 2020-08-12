package libsq

import (
	"context"
	"fmt"

	"github.com/neilotoole/errgroup"
	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlmodel"
	"github.com/neilotoole/sq/libsq/sqlz"
)

// engine executes a queryModel and writes to a RecordWriter.
type engine struct {
	log          lg.Log
	srcs         *source.Set
	dbOpener     driver.DatabaseOpener
	joinDBOpener driver.JoinDatabaseOpener
}

// execute executes the queryModel.
func (ng *engine) execute(ctx context.Context, qm *queryModel, recw RecordWriter) error {
	selectable := qm.Selectable

	var targetDB driver.Database
	var fromClause string
	var err error

	switch selectable := selectable.(type) {
	case *ast.TblSelector:
		fromClause, targetDB, err = ng.buildTableFromClause(ctx, selectable)
		if err != nil {
			return err
		}
	case *ast.Join:
		fromClause, targetDB, err = ng.buildJoinFromClause(ctx, selectable)
		if err != nil {
			return err
		}
	default:
		return errz.Errorf("unknown selectable %T: %q", selectable, selectable)
	}

	fragBuilder, qb := targetDB.SQLDriver().SQLBuilder()

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

	sqlQuery, err := qb.SQL()
	if err != nil {
		return err
	}

	ng.log.Debugf("SQL BUILDER [%s]: %s", targetDB.Source().Handle, sqlQuery)
	err = QuerySQL(ctx, ng.log, targetDB, recw, sqlQuery)
	return err
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

	ng.log.Debugf("Starting cross-source JOIN...")

	var joinCopyTasks []*joinCopyTask

	leftSrc, err := ng.srcs.Get(fnJoin.LeftTbl().DSName)
	if err != nil {
		return "", nil, err
	}
	leftDB, err := ng.dbOpener.Open(ctx, leftSrc)
	if err != nil {
		return "", nil, err
	}
	joinCopyTasks = append(joinCopyTasks, &joinCopyTask{
		fromDB:      leftDB,
		fromTblName: leftTblName,
	})

	rightSrc, err := ng.srcs.Get(fnJoin.RightTbl().DSName)
	if err != nil {
		return "", nil, err
	}
	rightDB, err := ng.dbOpener.Open(ctx, rightSrc)
	if err != nil {
		return "", nil, err
	}
	joinCopyTasks = append(joinCopyTasks, &joinCopyTask{
		fromDB:      rightDB,
		fromTblName: rightTblName,
	})

	// Open the join db
	joinDB, err := ng.joinDBOpener.OpenJoin(ctx, leftSrc, rightSrc)
	if err != nil {
		return "", nil, err
	}
	err = execJoinCopyTasks(ctx, ng.log, joinDB, joinCopyTasks)
	if err != nil {
		return "", nil, err
	}

	joinDBFragBuilder, _ := joinDB.SQLDriver().SQLBuilder()
	fromClause, err = joinDBFragBuilder.Join(fnJoin)
	if err != nil {
		return "", nil, err
	}

	return fromClause, joinDB, nil
}

// joinCopyTask is a specification of a table data copy task to be performed
// for a cross-source join. That is, the data in fromDB.fromTblName will
// be copied to a similar target table in the scratch DB. If colNames is
// empty, all cols in fromTblName are to be copied.
type joinCopyTask struct {
	fromDB      driver.Database
	fromTblName string
	colNames    []string
}

// execJoinCopyTasks executes tasks, returning any error.
func execJoinCopyTasks(ctx context.Context, log lg.Log, joinDB driver.Database, tasks []*joinCopyTask) error {
	g, gCtx := errgroup.WithContextN(ctx, driver.Tuning.ErrgroupNumG, driver.Tuning.ErrgroupQSize)
	for _, task := range tasks {
		task := task

		g.Go(func() error {
			return execCopyTable(gCtx, log, task.fromDB, task.fromTblName, joinDB, task.fromTblName)
		})
	}

	return g.Wait()
}

// execCopyTable performs the work of copying fromDB.fromTblName to destDB.destTblName.
func execCopyTable(ctx context.Context, log lg.Log, fromDB driver.Database, fromTblName string, destDB driver.Database, destTblName string) error {
	inserter := NewDBWriter(log, destDB, destTblName, DefaultRecordChSize)

	// the preInsertHook creates the dest table to copy into
	inserter.preWriteHook = func(ctx context.Context, originRecMeta sqlz.RecordMeta, tx sqlz.DB) error {
		destColNames := originRecMeta.Names()
		destColKinds := originRecMeta.Kinds()
		destTblDef := sqlmodel.NewTableDef(destTblName, destColNames, destColKinds)

		err := destDB.SQLDriver().CreateTable(ctx, tx, destTblDef)
		if err != nil {
			return errz.Wrapf(err, "failed to create dest table %s.%s", destDB.Source().Handle, destTblName)
		}

		return nil
	}

	query := "SELECT * FROM " + fromTblName
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
