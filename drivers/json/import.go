package json

// import.go contains functionality common to the
// various JSON import mechanisms.

import (
	"context"
	"sort"
	"strings"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

type importFunc func(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error

var (
	_ importFunc = importJSON
	_ importFunc = importJSONA
	_ importFunc = importJSONL
)

// getRecMeta returns RecordMeta to use with RecordWriter.Open.
func getRecMeta(ctx context.Context, scratchDB driver.Database, tblDef *sqlmodel.TableDef) (sqlz.RecordMeta, error) {
	colTypes, err := scratchDB.SQLDriver().TableColumnTypes(ctx, scratchDB.DB(), tblDef.Name, tblDef.ColNames())
	if err != nil {
		return nil, err
	}

	destMeta, _, err := scratchDB.SQLDriver().RecordMeta(colTypes)
	if err != nil {
		return nil, err
	}

	return destMeta, nil
}

// objectValueSet is the set of values for each of the fields of
// a top-level JSON object. It is a map of entity to a map
// of fieldName:fieldValue. For a nested JSON object, the value set
// may refer to several entities, and thus may decompose into
// insertions to several tables.
type objectValueSet map[*entity]map[string]interface{}

// processor process JSON objects.
type processor struct {
	// if flag is true, the JSON object will be flattened into a single table.
	flatten bool
	root    *entity
	schema  *importSchema

	// dirtyEntities tracks entities whose structure have been modified.
	dirtyEntities map[*entity]struct{}

	unwrittenObjVals []objectValueSet
	curObjVals       objectValueSet
}

func newProcessor(flatten bool) *processor {
	return &processor{
		flatten:       flatten,
		schema:        &importSchema{},
		root:          &entity{name: source.MonotableName, detectors: map[string]*kind.Detector{}},
		dirtyEntities: map[*entity]struct{}{},
	}
}

func (p *processor) markDirty(e *entity, dirty bool) {
	if dirty {
		p.dirtyEntities[e] = struct{}{}
	} else {
		delete(p.dirtyEntities, e)
	}
}

func (p *processor) clearDirty() {
	for k := range p.dirtyEntities {
		delete(p.dirtyEntities, k)
	}
}

// calcColName calculates the appropriate DB column name from
// a field. The result is different if p.flatten is true (in which
// case the column name may have a prefix derived from the entity's
// parent).
func (p *processor) calcColName(ent *entity, fieldName string) string {
	if !p.flatten {
		return fieldName
	}

	// Otherwise we namespace the column name.
	if ent.parent == nil {
		return fieldName
	}

	colName := ent.parent.name + "_" + fieldName
	return p.calcColName(ent.parent, colName)
}

// buildSchemaFlat currently only builds a flat (single table) schema.
func (p *processor) buildSchemaFlat() (*importSchema, error) {

	tblDef := &sqlmodel.TableDef{
		Name: source.MonotableName,
	}
	schema := &importSchema{
		colMungeFns: map[*sqlmodel.ColDef]kind.MungeFunc{},
		entityTbls:  map[*entity]*sqlmodel.TableDef{},
		tblDefs:     []*sqlmodel.TableDef{tblDef},
	}

	visitFn := func(e *entity) error {

		schema.entityTbls[e] = tblDef

		for _, field := range e.fieldNames {
			if detector, ok := e.detectors[field]; ok {
				// If it has a detector, it's a regular field
				kind, mungeFn, err := detector.Detect()
				if err != nil {
					return errz.Err(err)
				}

				colDef := &sqlmodel.ColDef{
					Name:  p.calcColName(e, field),
					Table: tblDef,
					Kind:  kind,
				}

				tblDef.Cols = append(tblDef.Cols, colDef)
				if mungeFn != nil {
					schema.colMungeFns[colDef] = mungeFn
				}
				continue
			}
		}

		return nil
	}

	err := walkEntity(p.root, visitFn)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

// processObject processes the parsed JSON object m. If the structure
// of the importSchema changes due to this object, dirtySchema is true.
func (p *processor) processObject(m map[string]interface{}) (dirtySchema bool, err error) {
	p.curObjVals = objectValueSet{}
	err = p.doAddObject(p.root, m)
	if err == nil {
		p.unwrittenObjVals = append(p.unwrittenObjVals, p.curObjVals)
	}

	p.curObjVals = nil
	dirtySchema = len(p.dirtyEntities) > 0
	return dirtySchema, err
}

func (p *processor) doAddObject(ent *entity, m map[string]interface{}) error {
	for fieldName, val := range m {
		switch val := val.(type) {
		case map[string]interface{}:
			// time to recurse
			child := ent.getChild(fieldName)
			if child == nil {
				p.markDirty(ent, true)

				if !stringz.InSlice(ent.fieldNames, fieldName) {
					// The field name could already exist (even without
					// the child existing) if we encountered
					// the field before but it was nil
					ent.fieldNames = append(ent.fieldNames, fieldName)
				}

				child = &entity{
					name:      fieldName,
					parent:    ent,
					detectors: map[string]*kind.Detector{},
				}
				ent.children = append(ent.children, child)
			} else {
				// Child already exists
				if child.isArray {
					// Safety check
					return errz.Errorf("JSON entity %q previously detected as array, but now detected as object", ent.String())
				}
			}

			err := p.doAddObject(child, val)
			if err != nil {
				return err
			}

		case []interface{}:
			if !stringz.InSlice(ent.fieldNames, fieldName) {
				ent.fieldNames = append(ent.fieldNames, fieldName)
			}
		default:
			// It's a regular value
			detector, ok := ent.detectors[fieldName]
			if !ok {
				p.markDirty(ent, true)
				if stringz.InSlice(ent.fieldNames, fieldName) {
					return errz.Errorf("JSON field %q was previously detected as a nested field (object or array)")
				} else {
					ent.fieldNames = append(ent.fieldNames, fieldName)
				}

				detector = kind.NewDetector()
				ent.detectors[fieldName] = detector
			}

			var entVals = p.curObjVals[ent]
			if entVals == nil {
				entVals = map[string]interface{}{}
				p.curObjVals[ent] = entVals
			}

			colName := p.calcColName(ent, fieldName)
			entVals[colName] = val

			detector.Sample(val)
		}
	}

	return nil
}

func (p *processor) buildInsertionsFlat(schema *importSchema) ([]*insertion, error) {
	if len(schema.tblDefs) != 1 {
		return nil, errz.Errorf("expected 1 table for flat JSON processing but got %d", len(schema.tblDefs))
	}

	tblDef := schema.tblDefs[0]
	var insertions []*insertion

	// Each of unwrittenObjVals is effectively an INSERT row
	for _, objValSet := range p.unwrittenObjVals {
		var colNames []string
		colVals := map[string]interface{}{}

		for ent, fieldVals := range objValSet {
			// For each entity, we get its values and add them to colVals.
			for colName, val := range fieldVals {
				if _, ok := colVals[colName]; ok {
					return nil, errz.Errorf("column %q already exists, but found column with same name in %q", ent)
				}

				colVals[colName] = val
				colNames = append(colNames, colName)
			}
		}

		sort.Strings(colNames)
		vals := make([]interface{}, len(colNames))
		for i, colName := range colNames {
			vals[i] = colVals[colName]
		}
		insertions = append(insertions, newInsertion(tblDef.Name, colNames, vals))

	}

	return insertions, nil
}

// entity is a JSON entity, either an object or an array.
type entity struct {
	// isArray is true if the entity is an array, false if an object.
	isArray bool

	name     string
	parent   *entity
	children []*entity

	// fieldName holds the names of each field. This includes simple
	// fields (such as a number or string) and nested types like
	// object or array.
	fieldNames []string

	// detectors holds a kind detector for each non-entity field
	// of entity. That is, it holds a detector for each string or number
	// field etc, but not for an object or array field.
	detectors map[string]*kind.Detector
}

func (e *entity) String() string {
	name := e.name
	if name == "" {
		name = source.MonotableName
	}

	parent := e.parent
	for parent != nil {
		name = parent.String() + "." + name
		parent = parent.parent
	}

	return name
}

// fqFieldName returns the fully-qualified field name, such
// as "data.name.first_name".
func (e *entity) fqFieldName(field string) string {
	return e.String() + "." + field
}

// getChild returns the named child, or nil.
func (e *entity) getChild(name string) *entity {
	for _, child := range e.children {
		if child.name == name {
			return child
		}
	}
	return nil
}

func walkEntity(ent *entity, visitFn func(*entity) error) error {
	err := visitFn(ent)
	if err != nil {
		return err
	}

	for _, child := range ent.children {
		err = walkEntity(child, visitFn)
		if err != nil {
			return err
		}
	}

	return nil
}

// importSchema encapsulates the table definitions that
// the JSON is imported to.
type importSchema struct {
	tblDefs     []*sqlmodel.TableDef
	colMungeFns map[*sqlmodel.ColDef]kind.MungeFunc

	// entityTbls is a mapping of entity to the table in which
	// the entity's fields will be inserted.
	entityTbls map[*entity]*sqlmodel.TableDef
}

func (is *importSchema) getTableDef(tblName string) *sqlmodel.TableDef {
	for _, t := range is.tblDefs {
		if t.Name == tblName {
			return t
		}
	}

	return nil
}

func execSchemaDelta(ctx context.Context, log lg.Log, drvr driver.SQLDriver, db sqlz.DB, curSchema, newSchema *importSchema) error {
	var err error
	if curSchema == nil {
		for _, tblDef := range newSchema.tblDefs {
			err = drvr.CreateTable(ctx, db, tblDef)
			if err != nil {
				return err
			}

			log.Debugf("Created table %q", tblDef.Name)
		}
		return nil
	}

	return errz.New("schema delta not yet implemented")
}

// execInsertions performs db INSERT for each of the insertions.
func execInsertions(ctx context.Context, log lg.Log, drvr driver.SQLDriver, db sqlz.DB, insertions []*insertion) error {
	// FIXME: This is not the proper way of performing insertion. See
	//  use of the driver.BatchInsert mechanism.

	var err error
	var execer *driver.StmtExecer
	var affected int64

	for _, insert := range insertions {
		execer, err = drvr.PrepareInsertStmt(ctx, db, insert.tbl, insert.cols, 1)
		if err != nil {
			return err
		}

		err = execer.Munge(insert.vals)
		if err != nil {
			log.WarnIfCloseError(execer)
			return err
		}

		affected, err = execer.Exec(ctx, insert.vals...)
		if err != nil {
			log.WarnIfCloseError(execer)
			return err
		}

		log.Debugf("Inserted %d row [%s] into table %q", affected, strings.Join(insert.cols, ", "), insert.tbl)

		err = execer.Close()
		if err != nil {
			return err
		}

	}

	return nil
}

type insertion struct {
	// stmtKey is a concatenation of tbl and cols that can
	// uniquely identify a db insert statement.
	stmtKey string

	tbl  string
	cols []string
	vals []interface{}
}

func newInsertion(tbl string, cols []string, vals []interface{}) *insertion {
	return &insertion{
		stmtKey: buildInsertStmtKey(tbl, cols),
		tbl:     tbl,
		cols:    cols,
		vals:    vals,
	}
}

// buildInsertStmtKey returns a concatenation of tbl and cols that can
// uniquely identify a db insert statement.
func buildInsertStmtKey(tbl string, cols []string) string {
	return tbl + "__" + strings.Join(cols, "_")
}
