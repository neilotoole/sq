package cli

import (
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"
)

func TestParseLoc_stage(t *testing.T) {
	testCases := []struct {
		loc  string
		want plocStage
	}{
		{"", plocInit},
		{"postgres", plocInit},
		{"postgres:/", plocInit},
		{"postgres://", plocScheme},
		{"postgres://alice", plocScheme},
		{"postgres://alice:", plocUser},
		{"postgres://alice:pass", plocUser},
		{"postgres://alice:pass@", plocPass},
		{"postgres://alice:@", plocPass},
		{"postgres://alice@", plocPass},
		{"postgres://alice@localhost", plocPass},
		{"postgres://alice:@localhost", plocPass},
		{"postgres://alice:pass@localhost", plocPass},
		{"postgres://alice@localhost:", plocHostname},
		{"postgres://alice:@localhost:", plocHostname},
		{"postgres://alice:pass@localhost:", plocHostname},
		{"postgres://alice@localhost:5432", plocHostname},
		{"postgres://alice@localhost:5432/", plocHost},
		{"postgres://alice@localhost:5432/s", plocHost},
		{"postgres://alice@localhost:5432/sakila", plocHost},
		{"postgres://alice@localhost:5432/sakila?", plocPath},
		{"postgres://alice@localhost:5432/sakila?sslmode=verify-ca", plocPath},
		{"postgres://alice:@localhost:5432/sakila?sslmode=verify-ca", plocPath},
		{"postgres://alice:pass@localhost:5432/sakila?sslmode=verify-ca", plocPath},
	}

	/*
		sq add postgres://sakila:p_ssW0rd@192.168.50.132/sakila
		sq add postgres://sakila:p_ssW0rd@192.168.50.132/sakila?sslmode=verify-ca
		sq add sqlserver://sakila:p_ssW0rd@192.168.50.130\?database=sakila
		sq add sqlserver://sakila:p_ssW0rd@192.168.50.130\?database=sakila&\keepAlive=30

	*/

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.loc), func(t *testing.T) {
			t.Log(tc.loc)
			lch := &locCompleteHelper{
				log: slogt.New(t),
			}

			ploc, _ := lch.parseLoc(tc.loc)
			require.NotNil(t, ploc)
			gotStage := ploc.stageDone
			require.Equal(t, tc.want, gotStage)
		})
	}
}
