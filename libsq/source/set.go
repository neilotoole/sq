package source

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

const (
	msgUnknownSrc  = "unknown source %s"
	msgNoActiveSrc = "no active source"

	// RootGroup is the identifier for the default root group.
	RootGroup = "/"
)

// Set is a set of sources. Typically it is loaded from config
// at a start of a run. Set's methods are safe for concurrent use.
type Set struct {
	// mu is the mutex used by exported methods. A method
	// should never call an exported method. Many exported methods
	// have an internal equivalent, e.g. "Get" and "get", which should
	// be used instead.
	mu sync.Mutex

	// data holds the set's adata.
	data setData
}

// setData holds Set's for the purposes of serialization
// to YAML etc. (we don't want to expose setData's exported
// fields directly on Set.)
//
// This seemed like a good idea at the time, but probably wasn't.
type setData struct {
	// ActiveSrc is the active source.
	// TODO: Rename tag to "active_src" to match "active_group".
	ActiveSrc string `yaml:"active" json:"active"`

	// ActiveGroup is the active group. It is "" (empty string) or "/" by default.
	// The "correct" value is "/", but we also support empty string
	// so that the zero value is useful.
	ActiveGroup string `yaml:"active_group" json:"active_group"`

	// ScratchSrc is the handle of the scratchdb source.
	ScratchSrc string `yaml:"scratch" json:"scratch"`

	// Sources holds the set's sources.
	//
	// TODO: Rename tag to "sources".
	Sources []*Source `yaml:"items" json:"items"`
}

// Data returns the internal representation of the set data.
// This is a filthy hack so that the internal data can be passed
// directly to sq's colorizing json encoder (it can't handle colorization
// of values that implement json.Marshaler).
//
// There are two long-term solutions here:
//  1. The color encoder needs to be able to handle json.RawMessage.
//  2. Refactor source.Set so that it doesn't have this weird internal
//     representation.
func (s *Set) Data() any {
	if s == nil {
		return nil
	}

	return s.data
}

// MarshalJSON implements json.Marshaler.
func (s *Set) MarshalJSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return json.Marshal(s.data)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *Set) UnmarshalJSON(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return json.Unmarshal(b, &s.data)
}

// MarshalYAML implements yaml.Marshaler.
func (s *Set) MarshalYAML() (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.data, nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *Set) UnmarshalYAML(unmarshal func(any) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return unmarshal(&s.data)
}

// Sources returns a new slice containing the set's sources.
// It is safe to mutate the returned slice, but note that
// changes to the *Source elements themselves do take effect
// in the set's backing data.
func (s *Set) Sources() []*Source {
	s.mu.Lock()
	defer s.mu.Unlock()

	srcs := make([]*Source, len(s.data.Sources))
	copy(srcs, s.data.Sources)

	return srcs
}

// String returns a log/debug friendly representation.
func (s *Set) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return stringz.SprintJSON(s)
}

// Add adds src to s.
func (s *Set) Add(src *Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ValidHandle(src.Handle); err != nil {
		return err
	}

	if s.isExistingHandle(src.Handle) {
		return errz.Errorf("conflict: source with handle %s already exists", src.Handle)
	}

	srcGroup := src.Group()
	if s.isExistingHandle("@" + srcGroup) {
		return errz.Errorf("conflict: source's group %q conflicts with existing handle %s",
			srcGroup, "@"+srcGroup)
	}

	if s.isExistingGroup(src.Handle[1:]) {
		return errz.Errorf("conflict: handle %s clashes with existing group %q",
			src.Handle, src.Handle[1])
	}

	s.data.Sources = append(s.data.Sources, src)
	return nil
}

// IsExistingSource returns true if handle already exists in the set.
func (s *Set) IsExistingSource(handle string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isExistingHandle(handle)
}

func (s *Set) isExistingHandle(handle string) bool {
	i, _ := s.indexOf(handle)
	return i != -1
}

func (s *Set) indexOf(handle string) (int, *Source) {
	for i, src := range s.data.Sources {
		if src.Handle == handle {
			return i, src
		}
	}

	return -1, nil
}

// Active returns the active source, or nil if no active source.
func (s *Set) Active() *Source {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.active()
}

// RenameSource renames oldHandle to newHandle.
// If the source was the active source, it remains so (under
// the new handle).
// If the source's group was the active group and oldHandle was
// the only member of the group, newHandle's group becomes
// the new active group.
func (s *Set) RenameSource(oldHandle, newHandle string) (*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.renameSource(oldHandle, newHandle)
}

func (s *Set) renameSource(oldHandle, newHandle string) (*Source, error) {
	if err := ValidHandle(newHandle); err != nil {
		return nil, err
	}

	src, err := s.get(oldHandle)
	if err != nil {
		return nil, err
	}

	if newHandle == oldHandle {
		// no-op
		return src, nil
	}

	if s.isExistingHandle(newHandle) {
		return nil, errz.Errorf("conflict: new handle %s already exists", newHandle)
	}

	if s.isExistingGroup(newHandle[1:]) {
		return nil, errz.Errorf("conflict: new handle %s conflicts with existing group %q",
			newHandle, newHandle[1:])
	}

	oldGroup := src.Group()

	// Do the actual renaming of the handle.
	src.Handle = newHandle

	if s.data.ActiveSrc == oldHandle {
		if _, err = s.setActive(newHandle, false); err != nil {
			return nil, err
		}
	}

	if oldGroup == s.activeGroup() {
		// oldGroup was the active group
		if err = s.requireGroupExists(oldGroup); err != nil {
			// oldGroup no longer exists, so...
			// we set the
			if err = s.setActiveGroup(src.Group()); err != nil {
				return nil, err
			}
		}
	}

	return src, nil
}

// RenameGroup renames oldGroup to newGroup. Each affected source
// is returned. This effectively "moves" sources in oldGroup to newGroup,
// by renaming those sources.
func (s *Set) RenameGroup(oldGroup, newGroup string) ([]*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if oldGroup == "/" || oldGroup == "" {
		return nil, errz.New("cannot rename root group")
	}

	if err := ValidGroup(oldGroup); err != nil {
		return nil, err
	}
	if err := ValidGroup(newGroup); err != nil {
		return nil, err
	}

	if err := s.requireGroupExists(oldGroup); err != nil {
		return nil, err
	}

	if s.isExistingHandle("@" + newGroup) {
		return nil, errz.Errorf("conflict: new group %q conflicts with existing handle %s",
			newGroup, "@"+newGroup)
	}

	if newGroup == "/" {
		newGroup = ""
	}

	oldHandles, err := s.handlesInGroup(oldGroup)
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
		if src, err = s.renameSource(oldHandle, newHandle); err != nil {
			return nil, err
		}

		affectedSrcs = append(affectedSrcs, src)
	}

	if s.data.ActiveGroup == oldGroup {
		s.data.ActiveGroup = newGroup
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
func (s *Set) MoveHandleToGroup(handle, toGroup string) (*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, err := s.get(handle)
	if err != nil {
		return nil, err
	}

	if err := ValidGroup(toGroup); err != nil {
		return nil, err
	}

	if s.isExistingHandle("@" + toGroup) {
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

	return s.renameSource(handle, newHandle)
}

// ActiveHandle returns the handle of the active source,
// or empty string if no active src.
func (s *Set) ActiveHandle() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	src := s.active()
	if src == nil {
		return ""
	}

	return src.Handle
}

func (s *Set) active() *Source {
	if s.data.ActiveSrc == "" {
		return nil
	}

	i, src := s.indexOf(s.data.ActiveSrc)
	if i == -1 {
		return nil
	}

	return src
}

// Scratch returns the scratch source, or nil.
func (s *Set) Scratch() *Source {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data.ScratchSrc == "" {
		return nil
	}

	i, src := s.indexOf(s.data.ScratchSrc)
	if i == -1 {
		return nil
	}

	return src
}

// Get gets the src with handle, or returns an error.
func (s *Set) Get(handle string) (*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.get(handle)
}

// Get gets the src with handle, or returns an error.
func (s *Set) get(handle string) (*Source, error) {
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
		activeSrc := s.active()
		if activeSrc == nil {
			return nil, errz.New(msgNoActiveSrc)
		}
		return activeSrc, nil
	}

	i, src := s.indexOf(handle)
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
func (s *Set) SetActive(handle string, force bool) (*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.setActive(handle, force)
}

func (s *Set) setActive(handle string, force bool) (*Source, error) {
	if handle == "" {
		s.data.ActiveSrc = ""
		return nil, nil //nolint:nilnil
	}

	if err := ValidHandle(handle); err != nil {
		return nil, err
	}

	if force {
		s.data.ActiveSrc = handle
		src, _ := s.get(handle)
		return src, nil
	}

	for _, src := range s.data.Sources {
		if src.Handle == handle {
			s.data.ActiveSrc = handle
			return src, nil
		}
	}

	return nil, errz.Errorf(msgUnknownSrc, handle)
}

// SetScratch sets the scratch src to handle. If handle
// is empty string, the scratch src is unset, and nil,nil
// is returned.
func (s *Set) SetScratch(handle string) (*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if handle == "" {
		s.data.ScratchSrc = ""
		return nil, nil //nolint:nilnil
	}
	for _, src := range s.data.Sources {
		if src.Handle == handle {
			s.data.ScratchSrc = handle
			return src, nil
		}
	}

	return nil, errz.Errorf(msgUnknownSrc, handle)
}

// Remove removes from the set the src having handle.
func (s *Set) Remove(handle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.remove(handle)
}

// RemoveGroup removes all sources that are children of group.
// The removed sources are returned. If group was the active
// group, the active group is set to "/" (root group).
func (s *Set) RemoveGroup(group string) ([]*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeGroup := s.activeGroup()

	srcs, err := s.sourcesInGroup(group, false)
	if err != nil {
		return nil, err
	}

	for i := range srcs {
		if err = s.remove(srcs[i].Handle); err != nil {
			return nil, err
		}
	}

	if err = s.requireGroupExists(activeGroup); err != nil {
		if err = s.setActiveGroup("/"); err != nil {
			return nil, err
		}
	}

	return srcs, nil
}

// remove handle from the set. By virtue of removing
// handle, the active source and active group may be reset
// to their defaults.
func (s *Set) remove(handle string) error {
	if len(s.data.Sources) == 0 {
		return errz.Errorf(msgUnknownSrc, handle)
	}

	activeG := s.activeGroup()

	i, _ := s.indexOf(handle)
	if i == -1 {
		return errz.Errorf(msgUnknownSrc, handle)
	}

	if s.data.ActiveSrc == handle {
		s.data.ActiveSrc = ""
	}

	if s.data.ScratchSrc == handle {
		s.data.ScratchSrc = ""
	}

	if len(s.data.Sources) == 1 {
		s.data.Sources = s.data.Sources[0:0]
		return nil
	}

	pre := s.data.Sources[:i]
	post := s.data.Sources[i+1:]

	s.data.Sources = pre
	s.data.Sources = append(s.data.Sources, post...)

	if s.data.ActiveSrc == handle {
		s.data.ActiveSrc = ""
	}

	if !s.isExistingGroup(activeG) {
		return s.setActiveGroup(RootGroup)
	}

	return nil
}

// Handles returns a new slice containing the set of all source handles.
func (s *Set) Handles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.handles()
}

func (s *Set) handles() []string {
	handles := make([]string, len(s.data.Sources))
	for i := range s.data.Sources {
		handles[i] = s.data.Sources[i].Handle
	}

	return handles
}

// HandlesInGroup returns the set of handles in the active group.
func (s *Set) HandlesInGroup(group string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.handlesInGroup(group)
}

func (s *Set) handlesInGroup(group string) ([]string, error) {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		return s.handles(), nil
	}

	if err := s.requireGroupExists(group); err != nil {
		return nil, err
	}

	groupSrcs, err := s.sourcesInGroup(group, false)
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
func (s *Set) Clone() *Set {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data := setData{
		ActiveGroup: s.data.ActiveGroup,
		ActiveSrc:   s.data.ActiveSrc,
		ScratchSrc:  s.data.ScratchSrc,
		Sources:     make([]*Source, len(s.data.Sources)),
	}

	for i, src := range s.data.Sources {
		data.Sources[i] = src.Clone()
	}

	return &Set{
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
func (s *Set) Groups() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.groups()
}

func (s *Set) groups() []string {
	groups := make([]string, 0, len(s.data.Sources)+1)
	groups = append(groups, "/")
	for _, src := range s.data.Sources {
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
func (s *Set) ActiveGroup() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.activeGroup()
}

func (s *Set) activeGroup() string {
	if s.data.ActiveGroup == "" {
		return "/"
	}
	return s.data.ActiveGroup
}

// IsExistingGroup returns false if group does not exist.
func (s *Set) IsExistingGroup(group string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.isExistingGroup(group)
}

func (s *Set) isExistingGroup(group string) bool {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		return true
	}

	groups := s.groups()
	return slices.Contains(groups, group)
}

// requireGroupExists returns an error if group does not exist.
func (s *Set) requireGroupExists(group string) error {
	if !s.isExistingGroup(group) {
		return errz.Errorf("group does not exist: %s", group)
	}
	return nil
}

// SetActiveGroup sets the active group, returning an error
// if group does not exist.
func (s *Set) SetActiveGroup(group string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.setActiveGroup(group)
}

func (s *Set) setActiveGroup(group string) error {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		s.data.ActiveGroup = "/"
		return nil
	}

	if err := s.requireGroupExists(group); err != nil {
		return err
	}

	s.data.ActiveGroup = group
	return nil
}

// SourcesInGroup returns all sources that are descendants of group.
// If group is "" or "/", all sources are returned.
func (s *Set) SourcesInGroup(group string) ([]*Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sourcesInGroup(group, false)
}

func (s *Set) sourcesInGroup(group string, directMembersOnly bool) ([]*Source, error) {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		srcs := make([]*Source, len(s.data.Sources))
		copy(srcs, s.data.Sources)

		if directMembersOnly {
			srcs = lo.Reject(srcs, func(item *Source, index int) bool {
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

	if err := s.requireGroupExists(group); err != nil {
		return nil, err
	}

	srcs := make([]*Source, 0)
	for i := range s.data.Sources {
		srcGroup := s.data.Sources[i].Group()
		if srcGroup == group || strings.HasPrefix(srcGroup, group+"/") {
			srcs = append(srcs, s.data.Sources[i])
		}
	}

	if directMembersOnly {
		srcs = lo.Reject(srcs, func(item *Source, index int) bool {
			return item.Group() != group
		})
	}

	Sort(srcs)
	return srcs, nil
}

// Tree returns a new Group representing the structure of the set
// starting at fromGroup downwards. If fromGroup is empty, RootGroup is used.
// The Group structure is a snapshot of the Set at the time Tree is invoked.
// Thus, any change to Set structure is not reflected in the Group. However,
// the Source elements of Group are pointers back to the Set elements, and
// thus changes to the fields of a Source are reflected in the Set.
func (s *Set) Tree(fromGroup string) (*Group, error) {
	if s == nil {
		return nil, nil //nolint:nilnil
	}

	if fromGroup == "" {
		fromGroup = RootGroup
	}

	if err := ValidGroup(fromGroup); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.tree(fromGroup)
}

func (s *Set) tree(fromGroup string) (*Group, error) {
	group := &Group{
		Name:   fromGroup,
		Active: fromGroup == s.activeGroup(),
	}

	var err error
	if group.Sources, err = s.sourcesInGroup(fromGroup, true); err != nil {
		return nil, err
	}

	// This part does a bunch of repeated work, but probably doesn't matter.
	groupNames := s.groups()
	// We only want the direct children of fromGroup.
	groupNames = groupsFilterOnlyDirectChildren(fromGroup, groupNames)

	group.Groups = make([]*Group, len(groupNames))
	for i := range groupNames {
		if group.Groups[i], err = s.tree(groupNames[i]); err != nil {
			return nil, err
		}
	}

	return group, nil
}

// Group models the hierarchical group structure of a set.
type Group struct {
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
	groups = lo.Reject(groups, func(item string, index int) bool {
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

// VerifySetIntegrity verifies the internal state of s.
// Typically this func is invoked after s has been loaded
// from config, verifying that the config is not corrupt.
// If err is returned non-nil, repaired may be returned true
// to indicate that ss has been repaired and modified. The
// caller should save the config to persist the repair.
func VerifySetIntegrity(ss *Set) (repaired bool, err error) {
	if ss == nil {
		return false, errz.New("source set is nil")
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()

	handles := map[string]*Source{}
	for i := range ss.data.Sources {
		src := ss.data.Sources[i]
		if src == nil {
			return false, errz.Errorf("source set item %d is nil", i)
		}

		err := validSource(src)
		if err != nil {
			return false, errz.Wrapf(err, "source set item %d", i)
		}

		if _, exists := handles[src.Handle]; exists {
			return false, errz.Errorf("source set item %d duplicates handle %s", i, src.Handle)
		}

		handles[src.Handle] = src
	}

	if strings.TrimSpace(ss.data.ActiveSrc) != "" {
		if _, exists := handles[ss.data.ActiveSrc]; !exists {
			// The active source doesn't exist. We'll unset the active source.
			activeSrc := ss.data.ActiveSrc
			ss.data.ActiveSrc = ""
			repaired = true
			// Note that the caller will still need to save the source set
			// to the config file for the repair to take effect.
			return repaired, errz.Errorf("active source {%s} does not exist in source set: config has been repaired: please try again", activeSrc) //nolint:lll
		}
	}

	return repaired, nil
}

// Sort sorts a slice of sources by handle.
func Sort(srcs []*Source) {
	slices.SortFunc(srcs, func(a, b *Source) bool {
		switch {
		case a == nil && b == nil:
			return false
		case a == nil:
			return true
		case b == nil:
			return false
		default:
			return a.Handle < b.Handle
		}
	})
}

// SortGroups sorts a slice of groups by name.
func SortGroups(groups []*Group) {
	slices.SortFunc(groups, func(a, b *Group) bool {
		switch {
		case a == nil && b == nil:
			return false
		case a == nil:
			return true
		case b == nil:
			return false
		default:
			return a.Name < b.Name
		}
	})
}
