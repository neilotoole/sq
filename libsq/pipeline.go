package libsq

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// pipeline is used to execute a SLQ query,
// and write the resulting records to a RecordWriter.
type pipeline struct {
	// targetGrip is the destination for the ultimate SQL query to
	// be executed against.
	targetGrip driver.Grip

	// qc is the context in which the query is executed.
	qc *QueryContext

	// rc is the Context for rendering SQL.
	// This field is set during pipeline.prepare. It can't be set before
	// then because the target DB to use is calculated during pipeline.prepare,
	// based on the input query and other context.
	rc *render.Context

	// query is the SLQ query
	query string

	// targetSQL is the ultimate SQL query to be executed against targetGrip.
	targetSQL string

	// tasks contains tasks that must be completed before targetSQL
	// is executed against targetGrip. Typically tasks is used to
	// set up the joindb before it is queried.
	tasks []tasker

	// tasksSingleWriter, when true, forces executeTasks to run its tasks
	// one at a time instead of concurrently up to the errgroup limit. It is
	// set from the join destination's dialect.SingleWriter so that a
	// single-writer joindb (SQLite) serializes its copy tasks rather than
	// contending on the write lock and failing with "database is locked"
	// (gh975).
	tasksSingleWriter bool
}

// newPipeline parses query, returning a pipeline prepared for
// execution via pipeline.execute.
func newPipeline(ctx context.Context, qc *QueryContext, query string) (*pipeline, error) {
	log := lg.FromContext(ctx)

	a, err := ast.Parse(log, query)
	if err != nil {
		return nil, err
	}

	qModel, err := buildQueryModel(qc, a)
	if err != nil {
		return nil, err
	}

	p := &pipeline{
		qc:    qc,
		query: query,
	}

	if err = p.prepare(ctx, qModel); err != nil {
		return nil, err
	}

	return p, nil
}

// execute executes the pipeline, writing results to recw.
func (p *pipeline) execute(ctx context.Context, recw RecordWriter) error {
	log := lg.FromContext(ctx)
	log.Info("Execute SQL query", lga.Src, p.targetGrip.Source(), lga.SQL, p.targetSQL)

	errw := p.targetGrip.SQLDriver().ErrWrapFunc()

	// TODO: The tasks might like to be executed in parallel. However,
	// what happens if a task does something that is session/connection-dependent?
	// When the query executes later (below), it could be on a different
	// connection. Maybe the tasks need a means of declaring that they
	// must be executed on the same connection as the main query?
	if err := p.executeTasks(ctx); err != nil {
		return errw(err)
	}

	var conn sqlz.DB
	if len(p.qc.PreExecStmts) > 0 || len(p.qc.PostExecStmts) > 0 {
		// If there's pre/post exec work to do, we need to
		// obtain a connection from the pool. We are responsible
		// for closing these resources.
		db, err := p.targetGrip.DB(ctx)
		if err != nil {
			return errw(err)
		}
		defer lg.WarnIfCloseError(log, lgm.CloseDB, db)

		if conn, err = db.Conn(ctx); err != nil {
			return errw(err)
		}
		defer lg.WarnIfCloseError(log, lgm.CloseConn, conn.(*sql.Conn))

		for _, stmt := range p.qc.PreExecStmts {
			if _, err = conn.ExecContext(ctx, stmt); err != nil {
				return errw(err)
			}
		}
	}

	// Pass any result-column kind hints recorded during rendering (e.g.
	// SQLite/rqlite pinning sum() to kind.Decimal) so the driver can apply them
	// when building record metadata. See issues #839 and #848.
	var hints map[int]kind.Kind
	if p.rc != nil {
		hints = p.rc.ResultColumnKinds
	}

	if err := QuerySQL(ctx, p.targetGrip, conn, recw, hints, p.targetSQL); err != nil {
		return err
	}

	if conn != nil && len(p.qc.PostExecStmts) > 0 {
		for _, stmt := range p.qc.PostExecStmts {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return errw(err)
			}
		}
	}

	return nil
}

// executeTasks executes any tasks in pipeline.tasks.
// These tasks may exist if preparatory work must be performed
// before pipeline.targetSQL can be executed.
func (p *pipeline) executeTasks(ctx context.Context) error {
	switch len(p.tasks) {
	case 0:
		return nil
	case 1:
		return p.tasks[0].executeTask(ctx)
	default:
	}

	limit := tuning.OptErrgroupLimit.Get(options.FromContext(ctx))
	if p.tasksSingleWriter {
		limit = 1
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	for _, task := range p.tasks {
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

// prepareNoTable is invoked when the queryModel doesn't have a table.
// That is to say, the query doesn't have a "FROM table" clause. It is
// this function's responsibility to figure out what source to use, and
// to set the relevant pipeline fields.
func (p *pipeline) prepareNoTable(ctx context.Context, qm *queryModel) error {
	log := lg.FromContext(ctx)
	log.Debug("No table in query; will look for source to use...")

	var (
		src    *source.Source
		err    error
		handle = ast.NewInspector(qm.AST).FindFirstHandle()
	)

	if handle == "" {
		src = p.qc.Collection.Active()
		if src == nil || !p.qc.Grips.IsSQLSource(src) {
			log.Debug("No active SQL source, will use an ephemeral db.")
			p.targetGrip, err = p.qc.Grips.OpenEphemeral(ctx)
			if err != nil {
				return err
			}

			p.rc = &render.Context{
				Renderer: p.targetGrip.SQLDriver().Renderer(),
				Args:     p.qc.Args,
				Dialect:  p.targetGrip.SQLDriver().Dialect(),
				DBSemver: dbSemverOf(ctx, p.targetGrip),
			}
			return nil
		}

		log.Debug("Using active source.", lga.Src, src)
	} else if src, err = p.qc.Collection.Get(handle); err != nil {
		return err
	}

	// At this point, src is non-nil.
	if p.targetGrip, err = p.qc.Grips.Open(ctx, src, p.qc.openModeFor(src.Handle)); err != nil {
		return err
	}

	p.rc = &render.Context{
		Renderer: p.targetGrip.SQLDriver().Renderer(),
		Args:     p.qc.Args,
		Dialect:  p.targetGrip.SQLDriver().Dialect(),
		DBSemver: dbSemverOf(ctx, p.targetGrip),
	}

	return nil
}

// prepareFromTable builds the "FROM table" fragment.
//
// When this function returns, pipeline.rc will be set.
func (p *pipeline) prepareFromTable(ctx context.Context, tblSel *ast.TblSelectorNode) (fromClause string,
	fromGrip driver.Grip, err error,
) {
	handle := tblSel.Handle()
	if handle == "" {
		handle = p.qc.Collection.ActiveHandle()
		if handle == "" {
			return "", nil, errz.New("query does not specify source, and no active source")
		}
	}

	src, err := p.qc.Collection.Get(handle)
	if err != nil {
		return "", nil, err
	}

	fromGrip, err = p.qc.Grips.Open(ctx, src, p.qc.openModeFor(src.Handle))
	if err != nil {
		return "", nil, err
	}

	rndr := fromGrip.SQLDriver().Renderer()
	p.rc = &render.Context{
		Renderer: rndr,
		Args:     p.qc.Args,
		Dialect:  fromGrip.SQLDriver().Dialect(),
		DBSemver: dbSemverOf(ctx, fromGrip),
	}

	fromClause, err = rndr.FromTable(p.rc, tblSel)
	if err != nil {
		return "", nil, err
	}

	return fromClause, fromGrip, nil
}

// dbSemverOf returns grip's canonical semver, or "" if it can't be determined.
// Renderers compare "" below every feature-version threshold, so an
// undeterminable version falls back to SQL valid on all server versions rather
// than failing the query.
func dbSemverOf(ctx context.Context, grip driver.Grip) string {
	v, _ := grip.DBSemver(ctx)
	return v
}

// joinClause models the SQL "JOIN" construct.
type joinClause struct {
	leftTbl *ast.TblSelectorNode
	joins   []*ast.JoinNode
}

// tables returns a new slice containing all referenced tables.
func (jc *joinClause) tables() []*ast.TblSelectorNode {
	tbls := make([]*ast.TblSelectorNode, len(jc.joins)+1)
	tbls[0] = jc.leftTbl
	for i := range jc.joins {
		tbls[i+1] = jc.joins[i].Table()
	}

	return tbls
}

// handles returns the set of (non-empty) handles from the tables,
// without any duplicates.
func (jc *joinClause) handles() []string {
	handles := make([]string, len(jc.joins)+1)
	handles[0] = jc.leftTbl.Handle()
	for i := 0; i < len(jc.joins); i++ {
		handles[i+1] = jc.joins[i].Table().Handle()
	}

	handles = lo.Uniq(handles)
	handles = lo.Without(handles, "")
	return handles
}

// isSingleSource returns true if the joins refer to the same handle.
func (jc *joinClause) isSingleSource() bool {
	leftHandle := jc.leftTbl.Handle()

	for _, join := range jc.joins {
		joinHandle := join.Table().Handle()
		if joinHandle == "" {
			continue
		}

		if joinHandle != leftHandle {
			return false
		}
	}

	return true
}

// prepareFromJoin builds the "JOIN" clause.
//
// When this function returns, pipeline.rc will be set.
func (p *pipeline) prepareFromJoin(ctx context.Context, jc *joinClause) (fromClause string,
	fromConn driver.Grip, err error,
) {
	if jc.isSingleSource() {
		return p.joinSingleSource(ctx, jc)
	}

	return p.joinCrossSource(ctx, jc)
}

// joinSingleSource sets up a join against a single source.
//
// On return, pipeline.rc will be set.
func (p *pipeline) joinSingleSource(ctx context.Context, jc *joinClause) (fromClause string,
	fromGrip driver.Grip, err error,
) {
	src, err := p.qc.Collection.Get(jc.leftTbl.Handle())
	if err != nil {
		return "", nil, err
	}

	fromGrip, err = p.qc.Grips.Open(ctx, src, p.qc.openModeFor(src.Handle))
	if err != nil {
		return "", nil, err
	}

	rndr := fromGrip.SQLDriver().Renderer()
	p.rc = &render.Context{
		Renderer: rndr,
		Args:     p.qc.Args,
		Dialect:  fromGrip.SQLDriver().Dialect(),
		DBSemver: dbSemverOf(ctx, fromGrip),
	}

	fromClause, err = rndr.Join(p.rc, jc.leftTbl, jc.joins)
	if err != nil {
		return "", nil, err
	}

	return fromClause, fromGrip, nil
}

// joinCrossSource returns a FROM clause that forms part of
// the SQL SELECT statement against fromDB.
//
// On return, pipeline.rc will be set.
func (p *pipeline) joinCrossSource(ctx context.Context, jc *joinClause) (fromClause string,
	fromDB driver.Grip, err error,
) {
	handles := jc.handles()
	srcs := make([]*source.Source, 0, len(handles))
	for _, handle := range handles {
		var src *source.Source
		if src, err = p.qc.Collection.Get(handle); err != nil {
			return "", nil, err
		}
		srcs = append(srcs, src)
	}

	// Open the join db
	joinGrip, err := p.qc.Grips.OpenJoin(ctx, srcs...)
	if err != nil {
		return "", nil, err
	}

	rndr := joinGrip.SQLDriver().Renderer()
	p.rc = &render.Context{
		Renderer: rndr,
		Args:     p.qc.Args,
		Dialect:  joinGrip.SQLDriver().Dialect(),
		DBSemver: dbSemverOf(ctx, joinGrip),
	}

	leftHandle := jc.leftTbl.Handle()
	// TODO: verify not empty

	tbls := jc.tables()

	// Every cross-source join participant is copied into the join scratch
	// DB under a destination name taken from its user alias (if any) or
	// its bare table name. When two participants resolve to the same
	// destination, the second CREATE TABLE collides and the rendered
	// FROM/ON would also reference the same name on both sides
	// (ambiguous regardless). Disambiguate as follows:
	//
	//   - User aliases are sacrosanct. Two colliding user aliases are a
	//     user error and rejected up front.
	//   - Each unaliased participant tries to claim its bare table name;
	//     the first unaliased occurrence encountered wins it, unless a
	//     user alias elsewhere already claimed that name.
	//   - The remaining unaliased participants get a synthesized
	//     numeric-suffixed alias (name_2, name_3, ...) picked to be
	//     unique against every other destination in use, including any
	//     pre-existing name that happens to match a generated suffix.
	//
	// Collision checks are case-insensitive: the join scratch DB is
	// always SQLite (see Grips.OpenJoin), and SQLite compares identifier
	// names case-insensitively even when double-quoted, so "Actor" and
	// "actor" would still collide at CREATE TABLE time. Original casing
	// is preserved in the assigned alias and copied destination name —
	// only the membership-test key is folded.
	used := map[string]bool{}
	for _, tbl := range tbls {
		if tbl.Alias() == "" {
			continue
		}
		key := strings.ToLower(tbl.Alias())
		if used[key] {
			return "", nil, errz.Errorf("cross-source join: duplicate table alias %q", tbl.Alias())
		}
		used[key] = true
	}
	// keepsBare keys nodes by pointer identity; relies on jc.tables()
	// returning the same *ast.TblSelectorNode instances across the loops
	// below.
	keepsBare := map[*ast.TblSelectorNode]bool{}
	for _, tbl := range tbls {
		if tbl.Alias() != "" {
			continue
		}
		name := tbl.Table().Table
		key := strings.ToLower(name)
		if used[key] {
			continue
		}
		used[key] = true
		keepsBare[tbl] = true
	}
	log := lg.FromContext(ctx)
	for _, tbl := range tbls {
		if tbl.Alias() != "" || keepsBare[tbl] {
			continue
		}
		name := tbl.Table().Table
		n := 2
		candidate := fmt.Sprintf("%s_%d", name, n)
		for used[strings.ToLower(candidate)] {
			n++
			candidate = fmt.Sprintf("%s_%d", name, n)
		}
		used[strings.ToLower(candidate)] = true
		tbl.SetAlias(candidate)
		log.Debug("Synthesized cross-source join alias to avoid table-name collision",
			lga.Handle, tbl.Handle(), lga.From, name, lga.To, candidate)
	}

	for _, tbl := range tbls {
		handle := tbl.Handle()
		if handle == "" {
			handle = leftHandle
		}
		var src *source.Source
		if src, err = p.qc.Collection.Get(handle); err != nil {
			return "", nil, err
		}
		var db driver.Grip
		if db, err = p.qc.Grips.Open(ctx, src, p.qc.openModeFor(src.Handle)); err != nil {
			return "", nil, err
		}

		task := &joinCopyTask{
			fromGrip: db,
			fromTbl:  tbl.Table(),
			toGrip:   joinGrip,
			toTbl:    tbl.TblAliasOrName(),
		}

		tbl.SyncTblNameAlias()

		// specifyTableFully populated the AST node's table value with
		// any Catalog/Schema overrides from the source. Those are
		// source-side qualifiers and would render as
		// "catalog"."schema"."table" against the scratch DB, which
		// only knows the bare table name we copied into it. Normalize
		// the AST table to scratch-local form before rndr.Join below.
		// The copy task above already captured the source-qualified
		// fromTbl, so this doesn't affect the source-side fetch.
		if t := tbl.Table(); t.Catalog != "" || t.Schema != "" {
			tbl.SetTable(tablefq.New(t.Table))
		}

		p.tasks = append(p.tasks, task)
	}

	// Cap copy-task concurrency to what the joindb tolerates. A
	// single-writer joindb (SQLite) reports SingleWriter, so its copies
	// serialize instead of contending on the write lock and failing with
	// "database is locked" (gh975); a multi-writer joindb leaves the
	// errgroup limit in force.
	p.tasksSingleWriter = joinGrip.SQLDriver().Dialect().SingleWriter

	fromClause, err = rndr.Join(p.rc, jc.leftTbl, jc.joins)
	if err != nil {
		return "", nil, err
	}

	return fromClause, joinGrip, nil
}

// tasker is the interface for executing a DB task.
type tasker interface {
	// executeTask executes a task against the DB.
	executeTask(ctx context.Context) error
}

// joinCopyTask is a specification of a table data copy task to be performed
// for a cross-source join. That is, the data in fromDB.fromTblName will
// be copied to a table in toGrip. If colNames is
// empty, all cols in fromTbl are to be copied.
type joinCopyTask struct {
	fromGrip driver.Grip
	fromTbl  tablefq.T
	toGrip   driver.Grip
	toTbl    tablefq.T
}

func (jt *joinCopyTask) executeTask(ctx context.Context) error {
	return execCopyTable(ctx, jt.fromGrip, jt.fromTbl, jt.toGrip, jt.toTbl)
}

// execCopyTable performs the work of copying fromDB.fromTbl to destGrip.destTbl.
func execCopyTable(ctx context.Context, fromDB driver.Grip, fromTbl tablefq.T,
	destGrip driver.Grip, destTbl tablefq.T,
) error {
	log := lg.FromContext(ctx)

	createTblHook := func(ctx context.Context, originRecMeta record.Meta, destGrip driver.Grip,
		tx sqlz.DB,
	) error {
		destColNames := originRecMeta.Names()
		destColKinds := originRecMeta.Kinds()
		destTblDef := schema.NewTable(destTbl.Table, destColNames, destColKinds)

		err := destGrip.SQLDriver().CreateTable(ctx, tx, destTblDef)
		if err != nil {
			return errz.Wrapf(err, "failed to create dest table %s.%s", destGrip.Source().Handle, destTbl)
		}

		return nil
	}

	inserter := NewDBWriter(
		"Copy records",
		destGrip,
		destTbl.Table,
		tuning.OptRecBufSize.Get(destGrip.Source().Options),
		createTblHook,
	)

	query := "SELECT * FROM " + fromTbl.Render(fromDB.SQLDriver().Dialect().Enquote)
	err := QuerySQL(ctx, fromDB, nil, inserter, nil, query)
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destGrip.Source().Handle, destTbl)
	}

	affected, err := inserter.Wait() // Stop for the writer to finish processing
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", destGrip.Source().Handle, destTbl)
	}
	log.Debug("Copied rows to dest", lga.Count, affected,
		lga.From, fmt.Sprintf("%s.%s", fromDB.Source().Handle, fromTbl),
		lga.To, fmt.Sprintf("%s.%s", destGrip.Source().Handle, destTbl))
	return nil
}
