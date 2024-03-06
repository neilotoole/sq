package source

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

const (
	msgUnknownSrc  = "unknown source %s"
	msgNoActiveSrc = "no active source"

	// RootGroup is the identifier for the default root group.
	RootGroup = "/"
)

// Collection is a set of sources. Typically it is loaded from config
// at a start of a run. Collection's methods are safe for concurrent use.
type Collection struct {
	// data holds the set's data.
	data collData

	// mu is the mutex used by exported methods. A method
	// should never call an exported method. Many exported methods
	// have an internal equivalent, e.g. "IsExistingGroup" and
	// "isExistingGroup", which should be used instead.
	mu sync.Mutex
}

// collData holds Collection data for the purposes of serialization
// to YAML etc. (we don't want to expose collData's exported
// fields directly on Collection.)
//
// This seemed like a good idea at the time, but probably wasn't.
type collData struct {
	// ActiveSrc is the active source. It may be empty.
	ActiveSrc string `yaml:"active.source" json:"active.source"`

	// ActiveGroup is the active group. It is "" (empty string) or "/" by default.
	// The "correct" value is "/", but we also support empty string
	// so that the zero value is useful.
	ActiveGroup string `yaml:"active.group" json:"active.group"`

	// ScratchSrc is the handle of the scratchdb source.
	ScratchSrc string `yaml:"scratch" json:"scratch"`

	// Sources holds the collection's sources.
	Sources []*Source `yaml:"sources" json:"sources"`
}

// Data returns the internal representation of the set data.
// This is a filthy hack so that the internal data can be passed
// directly to sq's colorizing json encoder (it can't handle colorization
// of values that implement json.Marshaler).
//
// There are two long-term solutions here:
//  1. The color encoder needs to be able to handle json.RawMessage.
//  2. Refactor source.Collection so that it doesn't have this weird internal
//     representation.
func (c *Collection) Data() any {
	if c == nil {
		return nil
	}

	return c.data
}

// MarshalJSON implements json.Marshaler.
func (c *Collection) MarshalJSON() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return json.Marshal(c.data)
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Collection) UnmarshalJSON(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return json.Unmarshal(b, &c.data)
}

// MarshalYAML implements yaml.Marshaler.
func (c *Collection) MarshalYAML() (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.data, nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Collection) UnmarshalYAML(unmarshal func(any) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return unmarshal(&c.data)
}

// Sources returns a new slice containing the set's sources.
// It is safe to mutate the returned slice, but note that
// changes to the *Source elements themselves do take effect
// in the set's backing data.
func (c *Collection) Sources() []*Source {
	c.mu.Lock()
	defer c.mu.Unlock()

	srcs := make([]*Source, len(c.data.Sources))
	copy(srcs, c.data.Sources)

	return srcs
}

// Visit visits each source.
func (c *Collection) Visit(fn func(src *Source) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.data.Sources {
		if err := fn(c.data.Sources[i]); err != nil {
			return err
		}
	}

	return nil
}

// String returns a log/debug friendly representation.
func (c *Collection) String() string {
	return stringz.SprintJSON(c)
}

// Add adds src to s.
func (c *Collection) Add(src *Source) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ValidHandle(src.Handle); err != nil {
		return err
	}

	if c.isExistingHandle(src.Handle) {
		return errz.Errorf("conflict: source with handle %s already exists", src.Handle)
	}

	srcGroup := src.Group()
	if c.isExistingHandle("@" + srcGroup) {
		return errz.Errorf("conflict: source's group %q conflicts with existing handle %s",
			srcGroup, "@"+srcGroup)
	}

	if c.isExistingGroup(src.Handle[1:]) {
		return errz.Errorf("conflict: handle %s clashes with existing group %q",
			src.Handle, src.Handle[1])
	}

	c.data.Sources = append(c.data.Sources, src)
	return nil
}

// IsExistingSource returns true if handle already exists in the set.
func (c *Collection) IsExistingSource(handle string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isExistingHandle(handle)
}

func (c *Collection) isExistingHandle(handle string) bool {
	i, _ := c.indexOf(handle)
	return i != -1
}

func (c *Collection) indexOf(handle string) (int, *Source) {
	for i, src := range c.data.Sources {
		if src.Handle == handle {
			return i, src
		}
	}

	return -1, nil
}

// Active returns the active source, or nil if no active source.
func (c *Collection) Active() *Source {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.active()
}

// RenameSource renames oldHandle to newHandle.
// If the source was the active source, it remains so (under
// the new handle).
// If the source's group was the active group and oldHandle was
// the only member of the group, newHandle's group becomes
// the new active group.
func (c *Collection) RenameSource(oldHandle, newHandle string) (*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.renameSource(oldHandle, newHandle)
}

func (c *Collection) renameSource(oldHandle, newHandle string) (*Source, error) {
	if err := ValidHandle(newHandle); err != nil {
		return nil, err
	}

	src, err := c.get(oldHandle)
	if err != nil {
		return nil, err
	}

	if newHandle == oldHandle {
		// no-op
		return src, nil
	}

	if c.isExistingHandle(newHandle) {
		return nil, errz.Errorf("conflict: new handle %s already exists", newHandle)
	}

	if c.isExistingGroup(newHandle[1:]) {
		return nil, errz.Errorf("conflict: new handle %s conflicts with existing group %q",
			newHandle, newHandle[1:])
	}

	oldGroup := src.Group()

	// Do the actual renaming of the handle.
	src.Handle = newHandle

	if c.data.ActiveSrc == oldHandle {
		if _, err = c.setActive(newHandle, false); err != nil {
			return nil, err
		}
	}

	if oldGroup == c.activeGroup() {
		// oldGroup was the active group
		if err = c.requireGroupExists(oldGroup); err != nil {
			// oldGroup no longer exists, so...
			// we set the
			if err = c.setActiveGroup(src.Group()); err != nil {
				return nil, err
			}
		}
	}

	return src, nil
}

// RenameGroup renames oldGroup to newGroup. Each affected source
// is returned. This effectively "moves" sources in oldGroup to newGroup,
// by renaming those sources.
func (c *Collection) RenameGroup(oldGroup, newGroup string) ([]*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if oldGroup == "/" || oldGroup == "" {
		return nil, errz.New("cannot rename root group")
	}

	if err := ValidGroup(oldGroup); err != nil {
		return nil, err
	}
	if err := ValidGroup(newGroup); err != nil {
		return nil, err
	}

	if err := c.requireGroupExists(oldGroup); err != nil {
		return nil, err
	}

	if c.isExistingHandle("@" + newGroup) {
		return nil, errz.Errorf("conflict: new group %q conflicts with existing handle %s",
			newGroup, "@"+newGroup)
	}

	if newGroup == "/" {
		newGroup = ""
	}

	oldHandles, err := c.handlesInGroup(oldGroup)
	if err != nil {
		return nil, err
	}

	var affectedSrcs []*Source

	var newHandle string
	for _, oldHandle := range oldHandles {
		if newGroup == "" {
			if i := strings.LastIndex(oldHandle, "/"); i != -1 {
				newHandle = "@" + oldHandle[i+1:]
			}
		} else { // else, it's a non-root new group
			newHandle = strings.Replace(oldHandle, oldGroup, newGroup, 1)
		}

		var src *Source
		if src, err = c.renameSource(oldHandle, newHandle); err != nil {
			return nil, err
		}

		affectedSrcs = append(affectedSrcs, src)
	}

	if c.data.ActiveGroup == oldGroup {
		c.data.ActiveGroup = newGroup
	}

	return affectedSrcs, nil
}

// MoveHandleToGroup moves renames handle to be in toGroup.
//
//	$ sq mv @prod/db production
//	@production/db
//
//	$ sq mv @prod/db /
//	@db
func (c *Collection) MoveHandleToGroup(handle, toGroup string) (*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	src, err := c.get(handle)
	if err != nil {
		return nil, err
	}

	if err := ValidGroup(toGroup); err != nil {
		return nil, err
	}

	if c.isExistingHandle("@" + toGroup) {
		return nil, errz.Errorf("conflict: dest group %q conflicts with existing handle %s",
			toGroup, "@"+toGroup)
	}

	var newHandle string
	oldGroup := src.Group()

	switch {
	case toGroup == "/":
		newHandle = strings.Replace(handle, oldGroup+"/", "", 1)
	case oldGroup == "":
		newHandle = "@" + toGroup + "/" + handle[1:]
	default:
		newHandle = strings.Replace(handle, oldGroup, toGroup, 1)
	}

	return c.renameSource(handle, newHandle)
}

// ActiveHandle returns the handle of the active source,
// or empty string if no active src.
func (c *Collection) ActiveHandle() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	src := c.active()
	if src == nil {
		return ""
	}

	return src.Handle
}

func (c *Collection) active() *Source {
	if c.data.ActiveSrc == "" {
		return nil
	}

	i, src := c.indexOf(c.data.ActiveSrc)
	if i == -1 {
		return nil
	}

	return src
}

// Scratch returns the scratch source, or nil.
func (c *Collection) Scratch() *Source {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.data.ScratchSrc == "" {
		return nil
	}

	i, src := c.indexOf(c.data.ScratchSrc)
	if i == -1 {
		return nil
	}

	return src
}

// Get gets the src with handle, or returns an error.
func (c *Collection) Get(handle string) (*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.get(handle)
}

// Get gets the src with handle, or returns an error.
func (c *Collection) get(handle string) (*Source, error) {
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return nil, errz.Errorf(msgUnknownSrc, handle)
	}

	if !strings.HasPrefix(handle, "@") {
		handle = "@" + handle
	}

	// Special handling for "@active", which is the reserved
	// handle for the active source.
	if handle == ActiveHandle {
		activeSrc := c.active()
		if activeSrc == nil {
			return nil, errz.New(msgNoActiveSrc)
		}
		return activeSrc, nil
	}

	i, src := c.indexOf(handle)
	if i == -1 {
		return nil, errz.Errorf(msgUnknownSrc, handle)
	}

	return src, nil
}

// SetActive sets the active src, or unsets any active
// src if handle is empty (and thus returns nil,nil).
// If handle does not exist, an error is returned, unless
// arg force is true. In which case, the returned *Source may
// be nil.
//
// TODO: Revisit SetActive(force) mechanism. It's a hack that
// we shouldn't need.
func (c *Collection) SetActive(handle string, force bool) (*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.setActive(handle, force)
}

func (c *Collection) setActive(handle string, force bool) (*Source, error) {
	if handle == "" {
		c.data.ActiveSrc = ""
		return nil, nil //nolint:nilnil
	}

	if err := ValidHandle(handle); err != nil {
		return nil, err
	}

	if force {
		c.data.ActiveSrc = handle
		src, _ := c.get(handle)
		return src, nil
	}

	for _, src := range c.data.Sources {
		if src.Handle == handle {
			c.data.ActiveSrc = handle
			return src, nil
		}
	}

	return nil, errz.Errorf(msgUnknownSrc, handle)
}

// SetScratch sets the scratch src to handle. If handle
// is empty string, the scratch src is unset, and nil,nil
// is returned.
func (c *Collection) SetScratch(handle string) (*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if handle == "" {
		c.data.ScratchSrc = ""
		return nil, nil //nolint:nilnil
	}
	for _, src := range c.data.Sources {
		if src.Handle == handle {
			c.data.ScratchSrc = handle
			return src, nil
		}
	}

	return nil, errz.Errorf(msgUnknownSrc, handle)
}

// Remove removes from the set the src having handle.
func (c *Collection) Remove(handle string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.remove(handle)
}

// RemoveGroup removes all sources that are children of group.
// The removed sources are returned. If group was the active
// group, the active group is set to "/" (root group).
func (c *Collection) RemoveGroup(group string) ([]*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	activeGroup := c.activeGroup()

	srcs, err := c.sourcesInGroup(group, false)
	if err != nil {
		return nil, err
	}

	for i := range srcs {
		if err = c.remove(srcs[i].Handle); err != nil {
			return nil, err
		}
	}

	if err = c.requireGroupExists(activeGroup); err != nil {
		if err = c.setActiveGroup("/"); err != nil {
			return nil, err
		}
	}

	return srcs, nil
}

// remove handle from the set. By virtue of removing
// handle, the active source and active group may be reset
// to their defaults.
func (c *Collection) remove(handle string) error {
	if len(c.data.Sources) == 0 {
		return errz.Errorf(msgUnknownSrc, handle)
	}

	activeG := c.activeGroup()

	i, _ := c.indexOf(handle)
	if i == -1 {
		return errz.Errorf(msgUnknownSrc, handle)
	}

	if c.data.ActiveSrc == handle {
		c.data.ActiveSrc = ""
	}

	if c.data.ScratchSrc == handle {
		c.data.ScratchSrc = ""
	}

	if len(c.data.Sources) == 1 {
		c.data.Sources = c.data.Sources[0:0]
		return nil
	}

	pre := c.data.Sources[:i]
	post := c.data.Sources[i+1:]

	c.data.Sources = pre
	c.data.Sources = append(c.data.Sources, post...)

	if c.data.ActiveSrc == handle {
		c.data.ActiveSrc = ""
	}

	if !c.isExistingGroup(activeG) {
		return c.setActiveGroup(RootGroup)
	}

	return nil
}

// Handles returns a new slice containing the set of all source handles.
func (c *Collection) Handles() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.handles()
}

func (c *Collection) handles() []string {
	handles := make([]string, len(c.data.Sources))
	for i := range c.data.Sources {
		handles[i] = c.data.Sources[i].Handle
	}

	return handles
}

// HandlesInGroup returns the set of handles in the active group.
func (c *Collection) HandlesInGroup(group string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.handlesInGroup(group)
}

func (c *Collection) handlesInGroup(group string) ([]string, error) {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		return c.handles(), nil
	}

	if err := c.requireGroupExists(group); err != nil {
		return nil, err
	}

	groupSrcs, err := c.sourcesInGroup(group, false)
	if err != nil {
		return nil, err
	}

	handles := make([]string, len(groupSrcs))
	for i := range groupSrcs {
		handles[i] = groupSrcs[i].Handle
	}

	return handles, nil
}

// Clone returns a deep copy of s. If s is nil, nil is returned.
func (c *Collection) Clone() *Collection {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	data := collData{
		ActiveGroup: c.data.ActiveGroup,
		ActiveSrc:   c.data.ActiveSrc,
		ScratchSrc:  c.data.ScratchSrc,
		Sources:     make([]*Source, len(c.data.Sources)),
	}

	for i, src := range c.data.Sources {
		data.Sources[i] = src.Clone()
	}

	return &Collection{
		mu:   sync.Mutex{},
		data: data,
	}
}

// Groups returns the sorted set of groups, as defined
// via the handle names.
//
// Given a set of handles:
//
//	@handle1
//	@group1/handle2
//	@group1/handle3
//	@group2/handle4
//	@group2/sub1/handle5
//	@group2/sub1/sub2/sub3/handle6
//
// Then these groups will be returned.
//
//	/
//	group1
//	group2
//	group2/sub1
//	group2/sub1/sub2
//	group2/sub1/sub2/sub3
//
// Note that default or root group is represented by "/".
func (c *Collection) Groups() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.groups()
}

func (c *Collection) groups() []string {
	groups := make([]string, 0, len(c.data.Sources)+1)
	groups = append(groups, "/")
	for _, src := range c.data.Sources {
		h := src.Handle

		if !strings.ContainsRune(h, '/') {
			continue
		}

		// Trim the '@' prefix
		h = h[1:]

		parts := strings.Split(h, "/")
		parts = parts[:len(parts)-1]

		groups = append(groups, parts[0])

		for i := 1; i < len(parts); i++ {
			arr := parts[0 : i+1]
			g := strings.Join(arr, "/")
			groups = append(groups, g)
		}
	}

	slices.Sort(groups)
	groups = lo.Uniq(groups)
	return groups
}

// ActiveGroup returns the active group, which may be
// the root group, represented by "/".
func (c *Collection) ActiveGroup() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.activeGroup()
}

func (c *Collection) activeGroup() string {
	if c.data.ActiveGroup == "" {
		return "/"
	}
	return c.data.ActiveGroup
}

// IsExistingGroup returns false if group does not exist.
func (c *Collection) IsExistingGroup(group string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.isExistingGroup(group)
}

func (c *Collection) isExistingGroup(group string) bool {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		return true
	}

	groups := c.groups()
	return slices.Contains(groups, group)
}

// requireGroupExists returns an error if group does not exist.
func (c *Collection) requireGroupExists(group string) error {
	if !c.isExistingGroup(group) {
		return errz.Errorf("group does not exist: %s", group)
	}
	return nil
}

// SetActiveGroup sets the active group, returning an error
// if group does not exist.
func (c *Collection) SetActiveGroup(group string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.setActiveGroup(group)
}

func (c *Collection) setActiveGroup(group string) error {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		c.data.ActiveGroup = "/"
		return nil
	}

	if err := c.requireGroupExists(group); err != nil {
		return err
	}

	c.data.ActiveGroup = group
	return nil
}

// SourcesInGroup returns all sources that are descendants of group.
// If group is "" or "/", all sources are returned.
func (c *Collection) SourcesInGroup(group string) ([]*Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.sourcesInGroup(group, false)
}

func (c *Collection) sourcesInGroup(group string, directMembersOnly bool) ([]*Source, error) {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		srcs := make([]*Source, len(c.data.Sources))
		copy(srcs, c.data.Sources)

		if directMembersOnly {
			srcs = lo.Reject(srcs, func(item *Source, _ int) bool {
				srcGroup := item.Group()
				if srcGroup == "/" || srcGroup == "" {
					return false
				}
				return srcGroup != group
			})
		}

		Sort(srcs)
		return srcs, nil
	}

	if err := c.requireGroupExists(group); err != nil {
		return nil, err
	}

	srcs := make([]*Source, 0)
	for i := range c.data.Sources {
		srcGroup := c.data.Sources[i].Group()
		if srcGroup == group || strings.HasPrefix(srcGroup, group+"/") {
			srcs = append(srcs, c.data.Sources[i])
		}
	}

	if directMembersOnly {
		srcs = lo.Reject(srcs, func(item *Source, _ int) bool {
			return item.Group() != group
		})
	}

	Sort(srcs)
	return srcs, nil
}

// Tree returns a new Group representing the structure of the set
// starting at fromGroup downwards. If fromGroup is empty, RootGroup is used.
// The Group structure is a snapshot of the Collection at the time Tree is invoked.
// Thus, any change to Collection structure is not reflected in the Group. However,
// the Source elements of Group are pointers back to the Collection elements, and
// thus changes to the fields of a Source are reflected in the Collection.
func (c *Collection) Tree(fromGroup string) (*Group, error) {
	if c == nil {
		return nil, nil //nolint:nilnil
	}

	if fromGroup == "" {
		fromGroup = RootGroup
	}

	if err := ValidGroup(fromGroup); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.tree(fromGroup)
}

func (c *Collection) tree(fromGroup string) (*Group, error) {
	group := &Group{
		Name:   fromGroup,
		Active: fromGroup == c.activeGroup(),
	}

	var err error
	if group.Sources, err = c.sourcesInGroup(fromGroup, true); err != nil {
		return nil, err
	}

	// This part does a bunch of repeated work, but probably doesn't matter.
	groupNames := c.groups()
	// We only want the direct children of fromGroup.
	groupNames = groupsFilterOnlyDirectChildren(fromGroup, groupNames)

	group.Groups = make([]*Group, len(groupNames))
	for i := range groupNames {
		if group.Groups[i], err = c.tree(groupNames[i]); err != nil {
			return nil, err
		}
	}

	return group, nil
}

// Group models the hierarchical group structure of a set.
type Group struct { //nolint:govet // field alignment
	// Name is the group name. For the root group, this is source.RootGroup ("/").
	Name string `json:"name" yaml:"name"`

	// Active is true if this is the active group in the set.
	Active bool `json:"active" yaml:"active"`

	// Sources are the direct members of the group.
	Sources []*Source `json:"sources,omitempty" yaml:"sources,omitempty"`

	// Groups holds any subgroups.
	Groups []*Group `json:"groups,omitempty" yaml:"groups,omitempty"`
}

// Counts returns counts for g.
//
// - directSrc: direct source child members of g
// - totalSrc: all source descendants of g
// - directGroup: direct group child members of g
// - totalGroup: all group descendants of g
//
// If g is empty, {0,0,0,0} is returned.
func (g *Group) Counts() (directSrc, totalSrc, directGroup, totalGroup int) {
	if g == nil {
		return 0, 0, 0, 0
	}

	directSrc = len(g.Sources)
	directGroup = len(g.Groups)

	totalSrc = directSrc
	totalGroup = directGroup

	for i := range g.Groups {
		_, srcCount, _, groupCount := g.Groups[i].Counts()
		totalSrc += srcCount
		totalGroup += groupCount
	}

	return directSrc, totalSrc, directGroup, totalGroup
}

// String returns a log/debug friendly representation.
func (g *Group) String() string {
	return g.Name
}

// AllSources returns a new flattened slice of *Source containing
// all the sources in g and its descendants.
func (g *Group) AllSources() []*Source {
	if g == nil {
		return []*Source{}
	}

	srcs := make([]*Source, 0, len(g.Sources))
	srcs = append(srcs, g.Sources...)
	for i := range g.Groups {
		srcs = append(srcs, g.Groups[i].AllSources()...)
	}

	Sort(srcs)
	return srcs
}

// RedactGroup modifies g, cloning each descendant Source, and setting
// the Source.Location field of each contained source to its redacted value.
func RedactGroup(g *Group) {
	if g == nil {
		return
	}

	for i := range g.Sources {
		g.Sources[i] = g.Sources[i].Clone()
		g.Sources[i].Location = g.Sources[i].RedactedLocation()
	}

	for i := range g.Groups {
		RedactGroup(g.Groups[i])
	}
}

// AllGroups returns a new flattened slice of Groups containing g
// and any subgroups.
func (g *Group) AllGroups() []*Group {
	if g == nil {
		return []*Group{}
	}
	groups := make([]*Group, 1, len(g.Groups)+1)
	groups[0] = g
	for i := range g.Groups {
		groups = append(groups, g.Groups[i].AllGroups()...)
	}

	SortGroups(groups)
	return groups
}

// groupsFilterOnlyDirectChildren rejects from groups any element that
// is not a direct child of parentGroup.
func groupsFilterOnlyDirectChildren(parentGroup string, groups []string) []string {
	groups = lo.Reject(groups, func(item string, _ int) bool {
		if parentGroup == "/" {
			return strings.ContainsRune(item, '/')
		}

		if !strings.HasPrefix(item, parentGroup+"/") {
			return true
		}

		item = strings.TrimPrefix(item, parentGroup+"/")
		return strings.ContainsRune(item, '/')
	})

	return groups
}

// VerifyIntegrity verifies the internal state of coll.
// Typically this func is invoked after coll has been loaded
// from config, verifying that the config is not corrupt.
// If err is returned non-nil, repaired may be returned true
// to indicate that coll has been repaired and modified. The
// caller should save the config to persist the repair.
func VerifyIntegrity(coll *Collection) (repaired bool, err error) {
	if coll == nil {
		return false, errz.New("collection is nil")
	}

	coll.mu.Lock()
	defer coll.mu.Unlock()

	handles := map[string]*Source{}
	for i := range coll.data.Sources {
		src := coll.data.Sources[i]
		if src == nil {
			return false, errz.Errorf("collection item %d is nil", i)
		}

		err := validSource(src)
		if err != nil {
			return false, errz.Wrapf(err, "collection item %d", i)
		}

		if _, exists := handles[src.Handle]; exists {
			return false, errz.Errorf("collection item %d duplicates handle %s", i, src.Handle)
		}

		handles[src.Handle] = src
	}

	if strings.TrimSpace(coll.data.ActiveSrc) != "" {
		if _, exists := handles[coll.data.ActiveSrc]; !exists {
			// The active source doesn't exist. We'll unset the active source.
			activeSrc := coll.data.ActiveSrc
			coll.data.ActiveSrc = ""
			repaired = true
			// Note that the caller will still need to save the collection
			// to the config file for the repair to take effect.
			return repaired, errz.Errorf("active source {%s} does not exist in collection: config has been repaired: please try again", activeSrc) //nolint:lll
		}
	}

	return repaired, nil
}

// Sort sorts a slice of sources by handle.
func Sort(srcs []*Source) {
	slices.SortFunc(srcs, func(a, b *Source) int {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		case b == nil:
			return 1
		default:
			return cmp.Compare(a.Handle, b.Handle)
		}
	})
}

// SortGroups sorts a slice of groups by name.
func SortGroups(groups []*Group) {
	slices.SortFunc(groups, func(a, b *Group) int {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		case b == nil:
			return 1
		default:
			return cmp.Compare(a.Name, b.Name)
		}
	})
}
