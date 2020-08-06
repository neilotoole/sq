package sqlz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/sqlz"
)

func TestKind(t *testing.T) {
	testCases := map[sqlz.Kind]string{
		sqlz.KindUnknown:  "unknown",
		sqlz.KindNull:     "null",
		sqlz.KindText:     "text",
		sqlz.KindInt:      "int",
		sqlz.KindFloat:    "float",
		sqlz.KindDecimal:  "decimal",
		sqlz.KindBool:     "bool",
		sqlz.KindDatetime: "datetime",
		sqlz.KindDate:     "date",
		sqlz.KindTime:     "time",
		sqlz.KindBytes:    "bytes",
	}

	for kind, testText := range testCases {
		kind, testText := kind, testText

		t.Run(kind.String(), func(t *testing.T) {
			gotBytes, err := kind.MarshalText()
			require.NoError(t, err)
			require.Equal(t, testText, string(gotBytes))

			gotString := kind.String()
			require.Equal(t, testText, gotString)

			gotJSON, err := kind.MarshalJSON()
			require.NoError(t, err)
			require.Equal(t, `"`+testText+`"`, string(gotJSON))

			var dt2 sqlz.Kind
			require.NoError(t, dt2.UnmarshalText([]byte(testText)))
			require.True(t, kind == dt2)
		})
	}

	d := sqlz.Kind(666)
	bytes, err := d.MarshalText()
	require.Error(t, err)
	require.Nil(t, bytes)

	bytes, err = d.MarshalJSON()
	require.Error(t, err)
	require.Nil(t, bytes)

	d = sqlz.KindBytes // pick any valid type
	require.Error(t, d.UnmarshalText([]byte("invalid_text")))
	require.Equal(t, sqlz.KindBytes, d, "d should not be mutated on UnmarshalText err")
}
