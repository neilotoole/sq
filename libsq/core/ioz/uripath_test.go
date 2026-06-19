package ioz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
)

func TestFileURIPath_RoundTrip(t *testing.T) {
	testCases := []struct {
		name    string
		fsPath  string // slash-separated OS path (post filepath.ToSlash)
		uriPath string // expected URI path component
	}{
		{name: "unix_abs", fsPath: "/var/db/x.sqlite", uriPath: "/var/db/x.sqlite"},
		{name: "unix_dollar", fsPath: "/var/db/q$$file.sqlite", uriPath: "/var/db/q$$file.sqlite"},
		{name: "windows_drive", fsPath: "C:/Users/x/db.sqlite", uriPath: "/C:/Users/x/db.sqlite"},
		{name: "windows_drive_lower", fsPath: "d:/db.sqlite", uriPath: "/d:/db.sqlite"},
		{name: "windows_drive_dollar", fsPath: `C:/Temp/my$$file.db`, uriPath: `/C:/Temp/my$$file.db`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotURI := ioz.FileURIPath(tc.fsPath)
			require.Equal(t, tc.uriPath, gotURI, "FileURIPath")
			// The authority position (immediately after "//") must never be a
			// volume: an encoded path always begins with a slash.
			require.True(t, gotURI[0] == '/', "encoded URI path must start with '/'")
			// Round-trip back to the OS path.
			require.Equal(t, tc.fsPath, ioz.FilePathFromURI(gotURI), "FilePathFromURI(FileURIPath(x))")
		})
	}
}
