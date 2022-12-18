package xmlud

import (
	"strings"

	"github.com/emirpasic/gods/stacks/arraystack"

	"github.com/neilotoole/sq/drivers/userdriver"
)

// rowState is a working struct for holding the state of a DB row as the XML doc is processed.
type rowState struct {
	tbl *userdriver.TableMapping

	dirtyColVals map[string]any
	savedColVals map[string]any
	curCol       *userdriver.ColMapping
}

// created returns true if this rowState has already been persisted (at least partially)
// to the database.
func (r *rowState) created() bool {
	return len(r.savedColVals) > 0
}

// dirty returns true if values have been dirtied since
// the last save to the database, or if the rowState has never been saved.
func (r *rowState) dirty() bool {
	return len(r.savedColVals) == 0 || len(r.dirtyColVals) > 0
}

// markDirtyAsSaved marks all of the dirty cols as having been already saved
// to the db.
func (r *rowState) markDirtyAsSaved() {
	for k, v := range r.dirtyColVals {
		r.savedColVals[k] = v
		delete(r.dirtyColVals, k)
	}
}

func newRowStack() *rowStack {
	return &rowStack{stack: arraystack.New()}
}

// rowStack is a trivial stack impl for tracking rowState instances
// as the XML doc is processed.
type rowStack struct {
	stack *arraystack.Stack
}

func (r *rowStack) size() int { //nolint:unused
	return r.stack.Size()
}

func (r *rowStack) push(ro *rowState) {
	r.stack.Push(ro)
}

func (r *rowStack) pop() *rowState {
	ro, ok := r.stack.Pop()
	if !ok {
		return nil
	}
	return ro.(*rowState)
}

func (r *rowStack) peek() *rowState {
	ro, ok := r.stack.Peek()
	if !ok {
		return nil
	}
	return ro.(*rowState)
}

func (r *rowStack) peekN(n int) *rowState {
	if n == 0 {
		return r.peek()
	}

	it := r.stack.Iterator()

	for i := 0; i <= n; i++ {
		ok := it.Next()
		if !ok {
			return nil
		}
	}

	val := it.Value()

	if val == nil {
		return nil
	}

	return val.(*rowState)
}

func newSelStack() *selStack {
	return &selStack{stack: arraystack.New()}
}

// selStack is a simple stack impl for tracking the element selector
// value as the XML doc is processed.
type selStack struct {
	stack *arraystack.Stack
}

func (s *selStack) push(sel string) {
	s.stack.Push(sel)
}

func (s *selStack) pop() string {
	val, ok := s.stack.Pop()
	if !ok {
		return ""
	}
	return val.(string)
}

func (s *selStack) peek() string { //nolint:unused
	val, ok := s.stack.Peek()
	if !ok {
		return ""
	}
	return val.(string)
}

// selector returns the current full selector path.
func (s *selStack) selector() string {
	// this is a really ugly way of doing this, must revisit
	strs := make([]string, s.stack.Size())
	it := s.stack.Iterator()

	i := s.stack.Size() - 1

	for it.Next() {
		val := it.Value()
		strs[i], _ = val.(string)
		i--
	}

	return "/" + strings.Join(strs, "/")
}
