package cli

import (
	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/stringz"
)

// candidateSet builds the deduplicated, prefix-filtered candidate
// list returned by the completer. It encapsulates the
// uniq/filter-prefix/exclude-prefix footer that the legacy code
// repeated at every plocStage case.
//
//nolint:unused // wired up by B13 generateCandidates via suggestX helpers.
type candidateSet struct {
	prefix string
	items  []string
}

// add appends candidates verbatim.
//
//nolint:unused // wired up by B13 generateCandidates via suggestX helpers.
func (c *candidateSet) add(s ...string) {
	c.items = append(c.items, s...)
}

// addPrefixed appends candidates each prefixed by p.
//
//nolint:unused // wired up by B13 generateCandidates via suggestX helpers.
func (c *candidateSet) addPrefixed(p string, ss ...string) {
	for _, s := range ss {
		c.items = append(c.items, p+s)
	}
}

// build returns the final deduplicated, prefix-filtered list, with
// the exact prefix string excluded (avoids the "completes to itself"
// noise).
//
//nolint:unused // wired up by B13 generateCandidates via suggestX helpers.
func (c *candidateSet) build() []string {
	out := lo.Uniq(c.items)
	out = stringz.FilterPrefix(c.prefix, out...)
	return lo.Without(out, c.prefix)
}
