package libsq

import (
	"context"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/libsq/core/options"

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

	// query is the SLQ query
	query string

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
	log := lg.FromContext(ctx)

	a, err := ast.Parse(log, query)
	if err != nil {
		return nil, err
	}

	qModel, err := buildQueryModel(qc, a)
	if err != nil {
		return nil, err
	}

	ng := &engine{
		log:   log,
		qc:    qc,
		query: query,
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
		lga.Src, ng.targetDB.Source(),
		// lga.Target, ng.targetDB.Source().Handle,
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
	g.SetLimit(driver.OptTuningErrgroupLimit.Get(options.FromContext(ctx)))

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

// prepareNoTabler is invoked when the queryModel doesn't have a tabler.
// That is to say, the query doesn't have a "FROM table" clause. It is
// this function's responsibility to figure out what source to use, and
// to set the relevant engine fields.
func (ng *engine) prepareNoTabler(ctx context.Context, qm *queryModel) error {
	ng.log.Debug("No Tabler in query; will look for source to use...")

	var (
		src    *source.Source
		err    error
		handle = ast.NewInspector(qm.AST).FindFirstHandle()
	)

	if handle == "" {
		if src = ng.qc.Collection.Active(); src == nil {
			ng.log.Debug("No active source, will use scratchdb.")
			ng.targetDB, err = ng.qc.ScratchDBOpener.OpenScratch(ctx, "scratch")
			if err != nil {
				return err
			}

			ng.rc = &render.Context{
				Renderer: ng.targetDB.SQLDriver().Renderer(),
				Args:     ng.qc.Args,
				Dialect:  ng.targetDB.SQLDriver().Dialect(),
			}
			return nil
		}

		ng.log.Debug("Using active source.", lga.Src, src)
	} else if src, err = ng.qc.Collection.Get(handle); err != nil {
		return err
	}

	// At this point, src is non-nil.
	if ng.targetDB, err = ng.qc.DBOpener.Open(ctx, src); err != nil {
		return err
	}

	ng.rc = &render.Context{
		Renderer: ng.targetDB.SQLDriver().Renderer(),
		Args:     ng.qc.Args,
		Dialect:  ng.targetDB.SQLDriver().Dialect(),
	}

	return nil
}

// prepareFromTable builds the "FROM table" fragment.
//
// When this function returns, ng.rc will be set.
func (ng *engine) prepareFromTable(ctx context.Context, tblSel *ast.TblSelectorNode) (fromClause string,
	fromConn driver.Database, err error,
) {
	handle := tblSel.Handle()
	if handle == "" {
		handle = ng.qc.Collection.ActiveHandle()
		if handle == "" {
			return "", nil, errz.New("query does not specify source, and no active source")
		}
	}

	src, err := ng.qc.Collection.Get(handle)
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

type joinClause struct {
	leftTbl *ast.TblSelectorNode
	joins   []*ast.JoinNode
}

// tables returns a new slice containing all referenced tables.
func (jc *joinClause) tables() []*ast.TblSelectorNode {
	tbls := make([]*ast.TblSelectorNode, len(jc.joins)+1)
	tbls[0] = jc.leftTbl
	for i := range jc.joins {
		tbls[i+1] = jc.joins[i].RightTbl()
	}

	return tbls
}

func (jc *joinClause) handles() []string {
	handles := make([]string, len(jc.joins)+1)
	handles[0] = jc.leftTbl.Handle()
	for i := 0; i < len(jc.joins); i++ {
		handles[i+1] = jc.joins[i].RightTbl().Handle()
	}
	return handles
}

//func (jc *joinClause) sources(coll *source.Collection) ([]*source.Source, error) {
//	activeHandle := coll.Active()
//	handles := jc.handles()
//	handles = lo.Uniq(handles)
//}

// isSingleSource returns true if the joins refer to the same handle.
func (jc *joinClause) isSingleSource() bool {
	leftHandle := jc.leftTbl.Handle()

	for _, join := range jc.joins {
		joinHandle := join.RightTbl().Handle()
		if joinHandle == "" {
			continue
		}

		if joinHandle != leftHandle {
			return false
		}
	}

	return true
	//
	//
	//tbls := jc.tables()
	//handles := map[string]struct{}{}
	//for _, tbl := range tbls {
	//	if tbl.Handle() == "" {
	//		handles[activeHandle] = struct{}{}
	//		continue
	//	}
	//
	//	handles[tbl.Handle()] = struct{}{}
	//}
	//
	//return len(handles) <= 1
}

// prepareFromJoin builds the "JOIN" clause.
//
// When this function returns, ng.rc will be set.
func (ng *engine) prepareFromJoin(ctx context.Context, jc *joinClause) (fromClause string,
	fromConn driver.Database, err error,
) {
	if jc.isSingleSource() {
		return ng.joinSingleSource(ctx, jc)
	}

	return ng.joinCrossSource(ctx, jc)
}

// joinSingleSource sets up a join against a single source.
//
// On return, ng.rc will be set.
func (ng *engine) joinSingleSource(ctx context.Context, jc *joinClause) (fromClause string,
	fromDB driver.Database, err error,
) {
	src, err := ng.qc.Collection.Get(jc.leftTbl.Handle())
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

	fromClause, err = rndr.Join(ng.rc, jc.leftTbl, jc.joins)
	if err != nil {
		return "", nil, err
	}

	return fromClause, fromDB, nil
}

// joinCrossSource returns a FROM clause that forms part of
// the SQL SELECT statement against fromDB.
//
// On return, ng.rc will be set.
func (ng *engine) joinCrossSource(ctx context.Context, jc *joinClause) (fromClause string, fromDB driver.Database,
	err error,
) {
	return "", nil, errz.New("not implemented")
	//
	//leftTblName, rightTblName := fnJoin.LeftTbl().TblName(), fnJoin.RightTbl().TblName()
	//if leftTblName == rightTblName {
	//	return "", nil, errz.Errorf("JOIN tables must have distinct names (or use aliases): duplicate tbl name {%s}",
	//		fnJoin.LeftTbl().TblName())
	//}
	//
	//leftSrc, err := ng.qc.Collection.Get(fnJoin.LeftTbl().Handle())
	//if err != nil {
	//	return "", nil, err
	//}
	//
	//rightSrc, err := ng.qc.Collection.Get(fnJoin.RightTbl().Handle())
	//if err != nil {
	//	return "", nil, err
	//}
	//
	//// Open the join db
	//joinDB, err := ng.qc.JoinDBOpener.OpenJoin(ctx, leftSrc, rightSrc)
	//if err != nil {
	//	return "", nil, err
	//}
	//
	//rndr := joinDB.SQLDriver().Renderer()
	//ng.rc = &render.Context{
	//	Renderer: rndr,
	//	Args:     ng.qc.Args,
	//	Dialect:  joinDB.SQLDriver().Dialect(),
	//}
	//
	//leftDB, err := ng.qc.DBOpener.Open(ctx, leftSrc)
	//if err != nil {
	//	return "", nil, err
	//}
	//leftCopyTask := &joinCopyTask{
	//	fromDB:      leftDB,
	//	fromTblName: leftTblName,
	//	toDB:        joinDB,
	//	toTblName:   leftTblName,
	//}
	//
	//rightDB, err := ng.qc.DBOpener.Open(ctx, rightSrc)
	//if err != nil {
	//	return "", nil, err
	//}
	//rightCopyTask := &joinCopyTask{
	//	fromDB:      rightDB,
	//	fromTblName: rightTblName,
	//	toDB:        joinDB,
	//	toTblName:   rightTblName,
	//}
	//
	//ng.tasks = append(ng.tasks, leftCopyTask)
	//ng.tasks = append(ng.tasks, rightCopyTask)
	//
	//fromClause, err = rndr.Join(ng.rc, jc.leftTbl, jc.joins)
	//if err != nil {
	//	return "", nil, err
	//}
	//
	//return fromClause, joinDB, nil
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
	log := lg.FromContext(ctx)

	createTblHook := func(ctx context.Context, originRecMeta record.Meta, destDB driver.Database,
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

	inserter := NewDBWriter(
		destDB,
		destTblName,
		driver.OptTuningRecChanSize.Get(destDB.Source().Options),
		createTblHook,
	)

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
