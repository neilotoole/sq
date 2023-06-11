// Package urlz contains URL utility functionality.
package urlz

import (
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// QueryParamKeys returns the keys of a URL query. This function
// exists because url.ParseQuery returns a url.Values, which is a
// map type, and the keys don't preserve order.
func QueryParamKeys(query string) (keys []string, err error) {
	// Code is adapted from url.ParseQuery.
	for query != "" {
		var key string
		key, query, _ = strings.Cut(query, "&")
		if strings.Contains(key, ";") {
			err = errz.Errorf("invalid semicolon separator in query")
			continue
		}
		if key == "" {
			continue
		}
		key, _, _ = strings.Cut(key, "=")
		key, err1 := url.QueryUnescape(key)
		if err1 != nil {
			if err == nil {
				err = errz.Err(err1)
			}
			continue
		}

		keys = append(keys, key)
	}
	return keys, err
}

// RenameQueryParamKey renames all instances of oldKey in query
// to newKey, where query is a URL query string.
func RenameQueryParamKey(query, oldKey, newKey string) string {
	if query == "" {
		return ""
	}

	parts := strings.Split(query, "&")
	for i, part := range parts {
		if part == oldKey {
			parts[i] = newKey
			continue
		}

		if strings.HasPrefix(part, oldKey+"=") {
			parts[i] = strings.Replace(part, oldKey, newKey, 1)
		}
	}

	return strings.Join(parts, "&")
}

// StripQuery strips the query params from u.
func StripQuery(u url.URL) string {
	u.RawQuery = ""
	u.ForceQuery = false
	return u.String()
}

// StripPath strips the url's path. If stripQuery is true, the
// query is also stripped.
func StripPath(u url.URL, stripQuery bool) string {
	u.Path = ""
	if stripQuery {
		u.RawQuery = ""
		u.ForceQuery = false
	}
	return u.String()
}
