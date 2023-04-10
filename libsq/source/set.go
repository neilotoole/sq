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

	// DefaultGroup is the identifier for the default group.
	DefaultGroup = "/"
)

// Set is a set of sources. Typically it is loaded from config
// at a start of a run.
type Set struct {
	mu   sync.Mutex
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

	if i, _ := s.indexOf(src.Handle); i != -1 {
		return errz.Errorf("source with handle %s already exists", src.Handle)
	}

	s.data.Sources = append(s.data.Sources, src)
	return nil
}

// Exists returns true if handle already exists loc the set.
func (s *Set) Exists(handle string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	oldGroup := src.Group()

	// rename the handle
	src.Handle = newHandle

	if s.data.ActiveSrc == oldHandle {
		if _, err = s.setActive(newHandle, false); err != nil {
			return nil, err
		}
	}

	if oldGroup == s.activeGroup() {
		// oldGroup was the active group
		if err = s.groupExists(oldGroup); err != nil {
			// oldGroup no longer exists, so...
			// we set the
			if err = s.setActiveGroup(src.Group()); err != nil {
				return nil, err
			}
		}
	}

	return src, nil
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

func (s *Set) remove(handle string) error {
	if len(s.data.Sources) == 0 {
		return errz.Errorf(msgUnknownSrc, handle)
	}

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
	return nil
}

// Handles returns the set of all source handles.
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

	if err := s.groupExists(group); err != nil {
		return nil, err
	}

	groupSrcs, err := s.sourcesInGroup(s.activeGroup())
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
	groups := make([]string, 1, len(s.data.Sources))
	groups[0] = "/"
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

// GroupMeta holds metadata about a group.
type GroupMeta struct {
	// Name is the group name, which may be "/" for the root group.
	Name string

	// Children is the count of direct src children of the group.
	Children int

	// Descendants is the total number of source descendants of the
	// group.
	Descendants int
}

// // GroupsMeta returns group metadata.
// func (s *Set) GroupsMeta() []GroupMeta {
// }

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

// GroupExists returns false if group does not exist.
func (s *Set) GroupExists(group string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.groupExists(group)
	return err == nil
}

func (s *Set) groupExists(group string) error {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		return nil
	}

	groups := s.groups()
	if !slices.Contains(groups, group) {
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

	if err := s.groupExists(group); err != nil {
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

	return s.sourcesInGroup(group)
}

func (s *Set) sourcesInGroup(group string) ([]*Source, error) {
	group = strings.TrimSpace(group)
	if group == "" || group == "/" {
		return s.data.Sources, nil
	}

	if err := s.groupExists(group); err != nil {
		return nil, err
	}

	rez := make([]*Source, 0)
	for i := range s.data.Sources {
		srcGroup := s.data.Sources[i].Group()
		if srcGroup == group || strings.HasPrefix(srcGroup, group+"/") {
			rez = append(rez, s.data.Sources[i])
		}
	}

	return rez, nil
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

// validSource performs basic checking on source s.
func validSource(s *Source) error {
	if s == nil {
		return errz.New("source is nil")
	}

	err := ValidHandle(s.Handle)
	if err != nil {
		return err
	}

	if strings.TrimSpace(s.Location) == "" {
		return errz.New("source location is empty")
	}

	if s.Type == TypeNone {
		return errz.Errorf("source type is empty or unknown: {%s}", s.Type)
	}

	return nil
}
