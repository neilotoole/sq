package ioz

// FileURIPath converts an OS filesystem path that has already been
// slash-separated (via filepath.ToSlash) into the path component of a
// file://-style URI. It ensures a leading slash precedes a Windows volume so
// the volume never lands in the URI authority position: without this, a path
// like "C:/db.sqlite" appended after a "sqlite3://" prefix would yield
// "sqlite3://C:/db.sqlite", where "C:" is parsed as the authority/host rather
// than part of the path (gh #797). On Unix, where paths carry no volume, the
// input is returned unchanged, so the encoded form is bit-identical to before.
//
//	C:/Users/x -> /C:/Users/x   (becomes sqlite3:///C:/Users/x)
//	/var/x     -> /var/x        (becomes sqlite3:///var/x, unchanged)
func FileURIPath(slashPath string) string {
	if hasVolumePrefix(slashPath) {
		return "/" + slashPath
	}
	return slashPath
}

// FilePathFromURI is the inverse of [FileURIPath]: it strips a single leading
// slash that precedes a Windows volume, recovering the OS filesystem path from
// the path component of a file://-style URI. On Unix (no volume) it is a no-op.
//
//	/C:/Users/x -> C:/Users/x
//	/var/x      -> /var/x   (unchanged)
func FilePathFromURI(uriPath string) string {
	if len(uriPath) >= 1 && uriPath[0] == '/' && hasVolumePrefix(uriPath[1:]) {
		return uriPath[1:]
	}
	return uriPath
}

// hasVolumePrefix reports whether s begins with a Windows drive volume such as
// "C:". The check is explicit (rather than filepath.VolumeName) so the result
// is identical on every OS: a location string may have been produced on
// Windows but is processed elsewhere, and vice versa.
func hasVolumePrefix(s string) bool {
	return len(s) >= 2 && s[1] == ':' &&
		((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z'))
}
