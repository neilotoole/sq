package source

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
)

var (
	handlePattern = regexp.MustCompile(`\A@([a-zA-Z][a-zA-Z0-9_]*)(/[a-zA-Z][a-zA-Z0-9_]*)*$`)
	groupPattern  = regexp.MustCompile(`\A([a-zA-Z][a-zA-Z0-9_]*)(/[a-zA-Z][a-zA-Z0-9_]*)*$`)
	tablePattern  = regexp.MustCompile(`\A[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// ValidHandle returns an error if handle is
// not an acceptable source handle value.
// Valid input must match:
//
//	\A@([a-zA-Z][a-zA-Z0-9_]*)(/[a-zA-Z][a-zA-Z0-9_]*)*$
//
// Examples:
//
//	@handle
//	@group/handle
//	@group/sub/sub2/handle
//
// On failure, the returned error names the specific reason (missing
// '@', illegal character, empty segment, etc.) rather than just
// repeating the input.
//
// See also: IsValidHandle.
func ValidHandle(handle string) error {
	if handlePattern.MatchString(handle) {
		return nil
	}
	return diagnoseInvalidRef(handle, true /* wantAt */)
}

// IsValidHandle returns false if handle is not a valid handle.
//
// See also: ValidHandle.
func IsValidHandle(handle string) bool {
	return handlePattern.MatchString(handle)
}

// validTableName returns an error if table is not an
// acceptable table name. Valid input must match:
//
//	\A[a-zA-Z_][a-zA-Z0-9_]*$`
func validTableName(table string) error {
	const msg = `invalid table name: %s`

	matches := tablePattern.MatchString(table)
	if !matches {
		return errz.Errorf(msg, table)
	}
	return nil
}

// IsValidGroup returns true if group is a valid group.
// Examples:
//
//	/
//	prod
//	prod/customer
//	prod/customer/pg
//
// Note that "/" is a special case, representing the root group.
func IsValidGroup(group string) bool {
	if group == "" || group == "/" {
		return true
	}

	return groupPattern.MatchString(group)
}

// ValidGroup returns an error if group is not a valid group name.
// On failure, the returned error names the specific reason
// (stray '@', illegal character, empty segment, etc.).
func ValidGroup(group string) error {
	if IsValidGroup(group) {
		return nil
	}
	return diagnoseInvalidRef(group, false /* wantAt */)
}

// diagnoseInvalidRef returns a specific, human-readable error
// describing why s is not a valid source handle (wantAt=true) or
// group (wantAt=false). Callers should only invoke this when
// validation has already failed; the function does not re-check the
// canonical regex and assumes some failure mode is present. The
// generic "invalid handle/group" fallback at the end exists only
// for forward-compatibility — every currently-known failure mode is
// caught explicitly above it.
func diagnoseInvalidRef(s string, wantAt bool) error {
	kind := "group"
	if wantAt {
		kind = "handle"
	}

	if s == "" {
		return errz.Errorf("invalid %s: empty", kind)
	}
	if strings.TrimSpace(s) == "" {
		return errz.Errorf("invalid %s %q: whitespace only", kind, s)
	}

	body := s
	switch {
	case wantAt && !strings.HasPrefix(s, "@"):
		return errz.Errorf("invalid handle %q: must start with '@'", s)
	case wantAt:
		body = s[1:]
		if body == "" {
			return errz.Errorf("invalid handle %q: no name after '@'", s)
		}
	case strings.HasPrefix(s, "@"):
		return errz.Errorf("invalid group %q: leading '@' is reserved for handles "+
			"(drop the '@' to use as a group, or move to handle form)", s)
	}

	for seg := range strings.SplitSeq(body, "/") {
		switch {
		case seg == "":
			return errz.Errorf("invalid %s %q: contains an empty segment "+
				"(check for consecutive or trailing '/')", kind, s)
		case !isASCIILetter(seg[0]):
			return errz.Errorf("invalid %s %q: segment %q must start with a letter, got %q",
				kind, s, seg, string(seg[0]))
		}
		for i := 1; i < len(seg); i++ {
			if c := seg[i]; !isASCIIAlnumOrUnderscore(c) {
				return errz.Errorf("invalid %s %q: segment %q contains illegal character %q "+
					"(only letters, digits, and underscore are allowed)",
					kind, s, seg, string(c))
			}
		}
	}

	// Fallback: handlePattern rejected s but no enumerated mode caught
	// it. Shouldn't reach here in practice; preserve the old message.
	return errz.Errorf("invalid %s: %s", kind, s)
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isASCIIAlnumOrUnderscore(b byte) bool {
	return isASCIILetter(b) || (b >= '0' && b <= '9') || b == '_'
}

// handleTypeAliases is a map of type names to the
// more user-friendly suffix returned by SuggestHandle.
var handleTypeAliases = map[string]string{
	drivertype.SQLite.String(): "sqlite",
	drivertype.Pg.String():     "pg",
	drivertype.MSSQL.String():  "mssql",
	drivertype.MySQL.String():  "my",
}

// SuggestHandle suggests a handle based on location and type.
// If typ is TypeNone, the type will be inferred from loc.
// The takenFn is used to determine if a suggested handle
// is free to be used (e.g. "@csv/sakila" -> "@csv/sakila1", etc).
//
// If the base name (derived from loc) contains illegal handle runes,
// those are replaced with underscore. If the handle starts with
// a number or underscore, it will be prefixed with "h" (for "handle").
// Thus "123.xlsx" becomes "@h123_xlsx".
func SuggestHandle(coll *Collection, typ drivertype.Type, loc string) (string, error) {
	// Bare-placeholder Locations (e.g. "${env:PBDSN}") don't have a
	// DSN to parse for the database name. Derive the handle from the
	// placeholder body per-scheme — sq is not going to resolve at
	// suggest time, because the resolved value would be machine-
	// dependent (defeats placeholder portability).
	if name, ok := suggestNameFromPlaceholder(loc); ok {
		return finalizeSuggestedHandle(coll, name), nil
	}

	locFields, err := location.Parse(loc)
	if err != nil {
		return "", err
	}

	if typ == drivertype.None {
		typ = locFields.DriverType
	}

	// use the type name as the _ext suffix if possible
	ext := typ.String()
	if ext == "" {
		if len(locFields.Ext) > 0 {
			ext = locFields.Ext[1:] // trim the leading period in ".xlsx" etc
		}
	}

	if alias, ok := handleTypeAliases[ext]; ok {
		ext = alias
	}
	// make sure there's nothing funky in ext or name
	ext = stringz.SanitizeAlphaNumeric(ext, '_')
	// NOTE: We used to utilize ext in the suggested handle name,
	// e.g. "@actor_csv". With the advent of source groups, we now
	// use the active group instead, e.g. "@prod/actor". So, it's
	// probably safe to rip out all the ext stuff, although maybe
	// UX reports will suggest that "@prod/csv/actor" is preferable,
	// and thus we would still need ext.
	_ = ext
	name := stringz.SanitizeAlphaNumeric(locFields.Name, '_')

	return finalizeSuggestedHandle(coll, name), nil
}

// finalizeSuggestedHandle takes a raw name (already alphanumeric-ish)
// and produces the final "@[group/]<name>[<n>]" handle, applying the
// 'h'-prefix rule for non-letter starts, the active-group prefix, and
// the uniqueness-suffix loop.
func finalizeSuggestedHandle(coll *Collection, name string) string {
	// if the name is empty, we use "h" (for "handle"), e.g "@h".
	if name == "" {
		name = "h"
	} else if !unicode.IsLetter([]rune(name)[0]) {
		// If the first rune is not a letter, we prepend "h".
		// So "123" becomes "h123", or "_123" becomes "h_123".
		name = "h" + name
	}

	g := coll.ActiveGroup()
	switch g {
	case "/", "":
		g = ""
	default:
		g += "/"
	}

	base := "@" + g + name

	// Beginning with base as candidate, check if
	// candidate is taken; if so, append N, where
	// N is a count starting at 1. For example:
	//
	//  @actor
	//  @actor2
	//  @actor3
	candidate := base
	for count := 1; ; count++ {
		if count > 1 {
			candidate = base + strconv.Itoa(count)
		}
		if !coll.IsExistingSource(candidate) && !coll.IsExistingGroup(candidate[1:]) {
			return candidate
		}
	}
}

// suggestNameFromPlaceholder returns a handle-name candidate derived
// from loc when loc is a bare ${scheme:body} placeholder (the entire
// Location is exactly one placeholder, no surrounding literal text).
// The mapping is per-scheme and uses only the placeholder body — the
// resolved value, which would be machine/user-dependent, is
// deliberately NOT consulted.
//
// Returns ok=false for non-placeholder inputs, composition inputs
// (placeholder embedded in a literal URL), and any scheme/body shape
// from which no meaningful name can be lifted; those fall through to
// the URL-parse path.
func suggestNameFromPlaceholder(loc string) (name string, ok bool) {
	refs, err := secret.ExtractRefs(loc)
	if err != nil || len(refs) != 1 {
		return "", false
	}
	// Only the BARE-placeholder case. Composition (placeholders
	// embedded in a URL, e.g. postgres://...:${env:PW}@host/db) is
	// handled by the URL-parse path below.
	if loc != "${"+refs[0].Scheme+":"+refs[0].Path+"}" {
		return "", false
	}
	return suggestNameForScheme(refs[0].Scheme, refs[0].Path)
}

// suggestNameForScheme returns a lowercased handle-name candidate
// drawn from body for a given resolver scheme. The picked segment is
// the one with most identity-bearing meaning under each scheme's
// convention: env-var name; file basename without extension;
// 1Password's "item" segment; Vault's last path segment; etc.
// Returns ok=false when no meaningful name can be lifted (e.g. opaque
// Crockford keyring IDs); the caller falls back to the generic path.
func suggestNameForScheme(scheme, body string) (string, bool) {
	switch scheme {
	case "env":
		if body == "" {
			return "", false
		}
		return strings.ToLower(body), true

	case "file":
		base := filepath.Base(body)
		if base == "." || base == "/" || base == "" {
			return "", false
		}
		if ext := filepath.Ext(base); ext != "" {
			base = base[:len(base)-len(ext)]
		}
		if base == "" {
			return "", false
		}
		return strings.ToLower(base), true

	case "op":
		// op://<vault>/<item>/[<section>/]<field>. The body starts
		// with "//"; the item segment (index 1 after splitting) is
		// the identity-bearing slot.
		parts := strings.Split(strings.TrimPrefix(body, "//"), "/")
		if len(parts) < 2 || parts[1] == "" {
			return "", false
		}
		return strings.ToLower(parts[1]), true

	case "vault":
		// HashiCorp Vault path, possibly with sq's "#field" fragment.
		// The last path segment is the name.
		body = strings.SplitN(body, "#", 2)[0]
		parts := strings.Split(body, "/")
		if last := parts[len(parts)-1]; last != "" {
			return strings.ToLower(last), true
		}
		return "", false

	case "aws-sm":
		// arn:aws:secretsmanager:<region>:<acct>:secret:<name>[#field]
		body = strings.SplitN(body, "#", 2)[0]
		if idx := strings.LastIndex(body, ":"); idx >= 0 && idx < len(body)-1 {
			return strings.ToLower(body[idx+1:]), true
		}
		return "", false

	case "keyring":
		// Legacy handle-encoded form "@<handle>/<slot>" — extract the
		// handle. Opaque Crockford IDs have no meaningful segment;
		// fall through to the generic path.
		if strings.HasPrefix(body, "@") && strings.Contains(body, "/") {
			h := strings.TrimPrefix(strings.SplitN(body, "/", 2)[0], "@")
			if h != "" {
				return strings.ToLower(h), true
			}
		}
		return "", false
	}
	return "", false
}

// Table represents a table (or view) in a source, e.g. "@sakila.actor".
type Table struct {
	// Handle is the source handle, e.g. "@sakila".
	Handle string
	// Name is the table name, e.g. "actor".
	Name string
}

// String returns @handle.name, e.g. "@sakila.actor".
func (t Table) String() string {
	return t.Handle + "." + t.Name
}

// ParseTableHandle attempts to parse a SLQ source handle and/or table name.
// Surrounding whitespace is trimmed. Examples of valid input values are:
//
//	@handle.tblName
//	@handle
//	.tblName
func ParseTableHandle(input string) (handle, table string, err error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", errz.New("empty input")
	}

	if strings.Contains(trimmed, ".") {
		if trimmed[0] == '.' {
			// starts with a period; so it's only the table name
			err = validTableName(trimmed[1:])
			if err != nil {
				return "", "", err
			}
			return "", trimmed[1:], nil
		}

		// input contains both handle and table
		parts := strings.Split(trimmed, ".")
		if len(parts) != 2 {
			return "", "", errz.Errorf("invalid handle/table input: %s", input)
		}

		err = ValidHandle(parts[0])
		if err != nil {
			return "", "", err
		}

		err = validTableName(parts[1])
		if err != nil {
			return "", "", err
		}

		return parts[0], parts[1], nil
	}

	// input does not contain a period, therefore it must be a handle by itself
	err = ValidHandle(trimmed)
	if err != nil {
		return "", "", err
	}

	return trimmed, "", err
}

// Contains returns true if srcs contains s, where s is a Source or a source handle.
func Contains[S *Source | ~string](srcs []*Source, s S) bool {
	if len(srcs) == 0 {
		return false
	}

	switch s := any(s).(type) {
	case *Source:
		return slices.Contains(srcs, s)
	case string:
		for i := range srcs {
			if srcs[i] != nil {
				if srcs[i].Handle == s {
					return true
				}
			}
		}
	default:
		// Can never happen
		panic(fmt.Sprintf("unknown type %T: %v", s, s))
	}

	return false
}

// Handle2SafePath returns a string derived from handle that
// is safe to use as a file path.
func Handle2SafePath(handle string) string {
	return strings.ReplaceAll(strings.TrimPrefix(handle, "@"), "/", "__")
}
