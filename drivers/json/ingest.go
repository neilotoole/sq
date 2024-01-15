package json

// xmlud.go contains functionality common to the
// various JSON import mechanisms.

import (
	"bytes"
	"context"
	stdj "encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// ingestJob describes a single ingest job, where the JSON
// at fromSrc is read via openFn and the resulting records
// are written to destGrip.
type ingestJob struct {
	fromSrc  *source.Source
	openFn   source.FileOpenFunc
	destGrip driver.Grip

	// sampleSize is the maximum number of values to
	// sample to determine the kind of an element.
	sampleSize int

	// flatten specifies that the fields of nested JSON objects are
	// imported as fields of the single top-level table, with a
	// scoped column name.
	//
	// TODO: flatten should come from src.Options
	flatten bool
}

type ingestFunc func(ctx context.Context, job ingestJob) error

var (
	_ ingestFunc = ingestJSON
	_ ingestFunc = ingestJSONA
	_ ingestFunc = ingestJSONL
)

// getRecMeta returns record.Meta to use with RecordWriter.Open.
func getRecMeta(ctx context.Context, grip driver.Grip, tblDef *sqlmodel.TableDef) (record.Meta, error) {
	db, err := grip.DB(ctx)
	if err != nil {
		return nil, err
	}

	colTypes, err := grip.SQLDriver().TableColumnTypes(ctx, db, tblDef.Name, tblDef.ColNames())
	if err != nil {
		return nil, err
	}

	destMeta, _, err := grip.SQLDriver().RecordMeta(ctx, colTypes)
	if err != nil {
		return nil, err
	}

	return destMeta, nil
}

const (
	leftBrace    = stdj.Delim('{')
	rightBrace   = stdj.Delim('}')
	leftBracket  = stdj.Delim('[')
	rightBracket = stdj.Delim(']')

	// colScopeSep is used when generating flat column names. Thus
	// an entity "name.first" becomes "name_first".
	colScopeSep = "_"
)

// objectValueSet is the set of values for each of the fields of
// a top-level JSON object. It is a map of entity to a map
// of fieldName:fieldValue. For a nested JSON object, the value set
// may refer to several entities, and thus may decompose into
// insertions to several tables.
type objectValueSet map[*entity]map[string]any

// processor process JSON objects.
type processor struct {
	// if flattened is true, the JSON object will be flattened into a single table.
	flatten bool

	root   *entity
	schema *importSchema

	colNamesOrdered []string

	// schemaDirtyEntities tracks entities whose structure have been modified.
	schemaDirtyEntities map[*entity]struct{}

	unwrittenObjVals []objectValueSet
	curObjVals       objectValueSet
}

func newProcessor(flatten bool) *processor {
	return &processor{
		flatten:             flatten,
		schema:              &importSchema{},
		root:                &entity{name: source.MonotableName, detectors: map[string]*kind.Detector{}},
		schemaDirtyEntities: map[*entity]struct{}{},
	}
}

func (p *processor) markSchemaDirty(e *entity) {
	p.schemaDirtyEntities[e] = struct{}{}
}

func (p *processor) markSchemaClean() {
	for k := range p.schemaDirtyEntities {
		delete(p.schemaDirtyEntities, k)
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

	colName := ent.name + colScopeSep + fieldName
	return p.calcColName(ent.parent, colName)
}

// buildSchemaFlat currently only builds a flat (single table) schema.
func (p *processor) buildSchemaFlat() (*importSchema, error) {
	tblDef := &sqlmodel.TableDef{
		Name: source.MonotableName,
	}

	var colDefs []*sqlmodel.ColDef

	schema := &importSchema{
		colMungeFns: map[*sqlmodel.ColDef]kind.MungeFunc{},
		entityTbls:  map[*entity]*sqlmodel.TableDef{},
		tblDefs:     []*sqlmodel.TableDef{tblDef}, // Single table only because flat
	}

	visitFn := func(e *entity) error {
		schema.entityTbls[e] = tblDef

		for _, field := range e.fieldNames {
			if detector, ok := e.detectors[field]; ok {
				// If it has a detector, it's a regular field
				k, mungeFn, err := detector.Detect()
				if err != nil {
					return errz.Err(err)
				}

				if k == kind.Null {
					k = kind.Text
				}

				colDef := &sqlmodel.ColDef{
					Name:  p.calcColName(e, field),
					Table: tblDef,
					Kind:  k,
				}

				colDefs = append(colDefs, colDef)
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

	// Add the column names, in the correct order
	for _, colName := range p.colNamesOrdered {
		for j := range colDefs {
			if colDefs[j].Name == colName {
				tblDef.Cols = append(tblDef.Cols, colDefs[j])
			}
		}
	}

	return schema, nil
}

// processObject processes the parsed JSON object m. If the structure
// of the importSchema changes due to this object, dirtySchema returns true.
func (p *processor) processObject(m map[string]any, chunk []byte) (dirtySchema bool, err error) {
	p.curObjVals = objectValueSet{}
	err = p.doAddObject(p.root, m)
	dirtySchema = len(p.schemaDirtyEntities) > 0
	if err != nil {
		return dirtySchema, err
	}

	p.unwrittenObjVals = append(p.unwrittenObjVals, p.curObjVals)

	p.curObjVals = nil
	if dirtySchema {
		err = p.updateColNames(chunk)
	}

	return dirtySchema, err
}

func (p *processor) updateColNames(chunk []byte) error {
	colNames, err := columnOrderFlat(chunk)
	if err != nil {
		return err
	}

	for _, colName := range colNames {
		if !stringz.InSlice(p.colNamesOrdered, colName) {
			p.colNamesOrdered = append(p.colNamesOrdered, colName)
		}
	}

	return nil
}

func (p *processor) doAddObject(ent *entity, m map[string]any) error {
	for fieldName, val := range m {
		switch val := val.(type) {
		case map[string]any:
			// time to recurse
			child := ent.getChild(fieldName)
			if child == nil {
				p.markSchemaDirty(ent)

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
			} else if child.isArray {
				// Child already exists
				// Safety check
				return errz.Errorf("JSON entity {%s} previously detected as array, but now detected as object",
					ent.String())
			}

			err := p.doAddObject(child, val)
			if err != nil {
				return err
			}

		case []any:
			if !stringz.InSlice(ent.fieldNames, fieldName) {
				ent.fieldNames = append(ent.fieldNames, fieldName)
			}
		default:
			// It's a regular value
			detector, ok := ent.detectors[fieldName]
			if !ok {
				p.markSchemaDirty(ent)
				if stringz.InSlice(ent.fieldNames, fieldName) {
					return errz.Errorf("JSON field {%s} was previously detected as a nested field (object or array)",
						fieldName)
				}

				ent.fieldNames = append(ent.fieldNames, fieldName)

				detector = kind.NewDetector()
				ent.detectors[fieldName] = detector
			}

			entVals := p.curObjVals[ent]
			if entVals == nil {
				entVals = map[string]any{}
				p.curObjVals[ent] = entVals
			}

			colName := p.calcColName(ent, fieldName)
			entVals[colName] = val

			val = maybeFloatToInt(val)
			detector.Sample(val)
		}
	}

	return nil
}

// buildInsertionsFlat builds a set of DB insertions from the
// processor's unwrittenObjVals. After a non-error return, unwrittenObjVals
// is empty.
func (p *processor) buildInsertionsFlat(schema *importSchema) ([]*insertion, error) {
	if len(schema.tblDefs) != 1 {
		return nil, errz.Errorf("expected 1 table for flat JSON processing but got %d", len(schema.tblDefs))
	}

	tblDef := schema.tblDefs[0]
	var insertions []*insertion

	// Each of unwrittenObjVals is effectively an INSERT row
	for _, objValSet := range p.unwrittenObjVals {
		var colNames []string
		colVals := map[string]any{}

		for ent, fieldVals := range objValSet {
			// For each entity, we get its values and add them to colVals.
			for colName, val := range fieldVals {
				if _, ok := colVals[colName]; ok {
					return nil, errz.Errorf("column {%s} already exists, but found column with same name in {%s}",
						colName, ent)
				}

				colVals[colName] = val
				colNames = append(colNames, colName)
			}
		}

		sort.Strings(colNames)
		vals := make([]any, len(colNames))
		for i, colName := range colNames {
			vals[i] = colVals[colName]
		}
		insertions = append(insertions, newInsertion(tblDef.Name, colNames, vals))
	}

	p.unwrittenObjVals = p.unwrittenObjVals[:0]

	return insertions, nil
}

// entity models the structure of a JSON entity, either an object or an array.
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
func (e *entity) fqFieldName(field string) string { //nolint:unused
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

func execSchemaDelta(ctx context.Context, drvr driver.SQLDriver, db sqlz.DB,
	curSchema, newSchema *importSchema,
) error {
	log := lg.FromContext(ctx)
	var err error
	if curSchema == nil {
		for _, tblDef := range newSchema.tblDefs {
			err = drvr.CreateTable(ctx, db, tblDef)
			if err != nil {
				return err
			}

			log.Debug("Created table", lga.Table, tblDef.Name)
		}
		return nil
	}

	return errz.New("schema delta not yet implemented")
}

// columnOrderFlat parses the json chunk and returns a slice
// containing column names, in the order they appear in chunk.
// Nested fields are flattened, e.g:
//
//	{"a":1, "b": {"c":2, "d":3}}  -->  ["a", "b_c", "b_d"]
func columnOrderFlat(chunk []byte) ([]string, error) {
	dec := stdj.NewDecoder(bytes.NewReader(chunk))

	var (
		cols  []string
		stack []string
		tok   stdj.Token
		err   error
	)

	// Get the opening left-brace
	_, err = requireDelimToken(dec, leftBrace)
	if err != nil {
		return nil, err
	}

loop:
	for {
		// Expect tok to be a field name, or else the terminating right-brace.
		tok, err = dec.Token()
		if err != nil {
			if err == io.EOF { //nolint:errorlint
				break
			}
			return nil, errz.Err(err)
		}

		switch tok := tok.(type) {
		case string:
			// tok is a field name
			stack = append(stack, tok)

		case stdj.Delim:
			if tok == rightBrace {
				if len(stack) == 0 {
					// This is the terminating right-brace
					break loop
				}
				// Else we've come to the end of an object
				stack = stack[:len(stack)-1]
				continue
			}

		default:
			return nil, errz.Errorf("expected string field name but got %T: %s", tok, formatToken(tok))
		}

		// We've consumed the field name above, now let's see what
		// the next token is
		tok, err = dec.Token()
		if err != nil {
			return nil, errz.Err(err)
		}

		switch tok := tok.(type) {
		default:
			// This next token was a regular old value.

			// The field name is already on the stack. We generate
			// the column name...
			cols = append(cols, strings.Join(stack, colScopeSep))

			// And pop the stack.
			stack = stack[0 : len(stack)-1]

		case stdj.Delim:
			// The next token was a delimiter.

			if tok == leftBrace {
				// It's the start of a nested object.
				// Back to the top of the loop we go, so that
				// we can descend into the nested object.
				continue loop
			}

			if tok == leftBracket {
				// It's the start of an array.
				// Note that we don't descend into arrays.

				cols = append(cols, strings.Join(stack, colScopeSep))
				stack = stack[0 : len(stack)-1]

				err = decoderFindArrayClose(dec)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return cols, nil
}

// decoderFindArrayClose advances dec until a closing
// right-bracket ']' is located at the correct nesting level.
// The most-recently returned decoder token should have been
// the opening left-bracket '['.
func decoderFindArrayClose(dec *stdj.Decoder) error {
	var depth int
	var tok stdj.Token
	var err error

	for {
		tok, err = dec.Token()
		if err != nil {
			break
		}

		if tok == leftBracket {
			// Nested array
			depth++
			continue
		}

		if tok == rightBracket {
			if depth == 0 {
				return nil
			}
			depth--
		}
	}

	return errz.Err(err)
}

// execInsertions performs db INSERT for each of the insertions.
func execInsertions(ctx context.Context, drvr driver.SQLDriver, db sqlz.DB, insertions []*insertion) error {
	// FIXME: This is an inefficient way of performing insertion.
	//  We should be re-using the prepared statement, and probably
	//  should batch the inserts as well. See driver.BatchInsert.

	log := lg.FromContext(ctx)
	var err error
	var execer *driver.StmtExecer

	for _, insert := range insertions {
		execer, err = drvr.PrepareInsertStmt(ctx, db, insert.tbl, insert.cols, 1)
		if err != nil {
			return err
		}

		err = execer.Munge(insert.vals)
		if err != nil {
			lg.WarnIfCloseError(log, lgm.CloseDBStmt, execer)
			return err
		}

		_, err = execer.Exec(ctx, insert.vals...)
		if err != nil {
			lg.WarnIfCloseError(log, lgm.CloseDBStmt, execer)
			return err
		}

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
	vals []any
}

func newInsertion(tbl string, cols []string, vals []any) *insertion {
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
