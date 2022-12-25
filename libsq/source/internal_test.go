package source

import (
	"context"
	"io"
	"runtime"
	"testing"

	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

func TestFiles_Open(t *testing.T) {
	fs, err := NewFiles(testlg.New(t))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src1 := &Source{
		Location: proj.Abs(testsrc.PathXLSXTestHeader),
	}

	f, err := fs.openLocation(src1.Location)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f.Close()) })
	require.Equal(t, src1.Location, f.Name())

	src2 := &Source{
		Location: sakila.URLActorCSV,
	}

	f2, err := fs.openLocation(src2.Location)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f2.Close()) })

	b, err := io.ReadAll(f2)
	require.NoError(t, err)
	require.Equal(t, proj.ReadFile(sakila.PathCSVActor), b)
}

func TestParseLoc(t *testing.T) {
	const (
		dbuser = "sakila"
		dbpass = "p_ssW0rd"
	)

	testCases := []struct {
		loc     string
		want    parsedLoc
		wantErr bool
		windows bool
	}{
		{
			loc:  "/path/to/sakila.xlsx",
			want: parsedLoc{name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "relative/path/to/sakila.xlsx",
			want: parsedLoc{name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "./relative/path/to/sakila.xlsx",
			want: parsedLoc{name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "https://server:8080/path/to/sakila.xlsx",
			want: parsedLoc{scheme: "https", hostname: "server", port: 8080, name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "http://server/path/to/sakila.xlsx?param=val&param2=val2",
			want: parsedLoc{scheme: "http", hostname: "server", name: "sakila", ext: ".xlsx"},
		},
		{
			loc:     "sqlite3:/path/to/sakila.db",
			wantErr: true,
		}, // the scheme is malformed (should be "sqlite3://...")
		{
			loc: "sqlite3:///path/to/sakila.sqlite",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".sqlite",
				dsn: "/path/to/sakila.sqlite",
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite`,
			windows: true,
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".sqlite",
				dsn: `C:\path\to\sakila.sqlite`,
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite?param=val`,
			windows: true,
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".sqlite",
				dsn: `C:\path\to\sakila.sqlite?param=val`,
			},
		},
		{
			loc: "sqlite3:///path/to/sakila",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", dsn: "/path/to/sakila",
			},
		},
		{
			loc: "sqlite3://path/to/sakila.db",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".db", dsn: "path/to/sakila.db",
			},
		},
		{
			loc: "sqlite3:///path/to/sakila.db",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".db", dsn: "/path/to/sakila.db",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			want: parsedLoc{
				typ: typeMS, scheme: "sqlserver", user: dbuser, pass: dbpass, hostname: "localhost",
				name: "sakila", dsn: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			want: parsedLoc{
				typ: typeMS, scheme: "sqlserver", user: dbuser, pass: dbpass, hostname: "server",
				port: 1433, name: "sakila",
				dsn: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@localhost/sakila?sslmode=disable",
			want: parsedLoc{
				typ: typePg, scheme: "postgres", user: dbuser, pass: dbpass, hostname: "localhost",
				name: "sakila", dsn: "dbname=sakila host=localhost password=p_ssW0rd sslmode=disable user=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@server:5432/sakila?sslmode=disable",
			want: parsedLoc{
				typ: typePg, scheme: "postgres", user: dbuser, pass: dbpass, hostname: "server", port: 5432,
				name: "sakila",
				dsn:  "dbname=sakila host=server password=p_ssW0rd port=5432 sslmode=disable user=sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@localhost/sakila",
			want: parsedLoc{
				typ: typeMy, scheme: "mysql", user: dbuser, pass: dbpass, hostname: "localhost",
				name: "sakila", dsn: "sakila:p_ssW0rd@tcp(localhost:3306)/sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@server:3306/sakila",
			want: parsedLoc{
				typ: typeMy, scheme: "mysql", user: dbuser, pass: dbpass, hostname: "server", port: 3306,
				name: "sakila", dsn: "sakila:p_ssW0rd@tcp(server:3306)/sakila",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.loc, func(t *testing.T) {
			if tc.windows && runtime.GOOS != "windows" {
				return
			}

			tc.want.loc = tc.loc // set this here rather than verbosely loc the setup
			got, gotErr := parseLoc(tc.loc)
			if tc.wantErr {
				require.Error(t, gotErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, *got)
		})
	}
}

// FilesDetectTypeFn exports Files.detectType for testing.
var FilesDetectTypeFn = func(fs *Files, ctx context.Context, loc string) (typ Type, ok bool, err error) {
	return fs.detectType(ctx, loc)
}
