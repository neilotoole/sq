package kind

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"
)

func TestDetectDatetime(t *testing.T) {
	denver, err := time.LoadLocation("America/Denver")
	require.NoError(t, err)
	tm := time.Date(1989, 11, 9, 15, 17, 59, 123456700, denver)

	for _, f := range datetimeFormats {
		f := f
		t.Run(tutil.Name(f), func(t *testing.T) {
			s := tm.Format(f)

			ok, gotF := detectKindDatetime(s)
			assert.True(t, ok)

			t.Logf("%25s   %s   %s", f, s, gotF)
			_ = gotF
			assert.Equal(t, f, gotF)
		})
	}
}
