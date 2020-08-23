package sqlz_test

import (
	stdj "encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
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

func TestKindDetector(t *testing.T) {
	const (
		fixtTime1              = "00:00:00"
		fixtTime2              = "08:30:05"
		fixtTime3              = "15:30"
		fixtTime4              = "7:15PM"
		fixtDate1              = "1970-01-01"
		fixtDate2              = "1989-11-09"
		fixtDate3              = "02 Jan 2006"
		fixtDate4              = "2006/01/02"
		fixtDatetime1          = "1970-01-01T00:00:00Z" // RFC3339Nano
		fixtDatetime2          = "1989-11-09T00:00:00Z"
		fixtDatetimeAnsic      = "Mon Jan 2 15:04:05 2006"
		fixtDatetimeUnix       = "Mon Jan 2 15:04:05 MST 2006"
		fixtDatetimeRFC3339    = "2002-10-02T10:00:00-05:00"
		fixtDatetimeStamp      = "Jan 2 15:04:05"
		fixtDatetimeStampMilli = "Jan 2 15:04:05.000"
		fixtDatetimeStampMicro = "Jan 2 15:04:05.000000"
		fixtDatetimeStampNano  = "Jan 2 15:04:05.000000000"
	)

	testCases := []struct {
		in        []interface{}
		want      sqlz.Kind
		wantMunge bool
		wantErr   bool
	}{
		{in: nil, want: sqlz.KindText},
		{in: []interface{}{}, want: sqlz.KindText},
		{in: []interface{}{""}, want: sqlz.KindText},
		{in: []interface{}{nil}, want: sqlz.KindText},
		{in: []interface{}{nil, ""}, want: sqlz.KindText},
		{in: []interface{}{int(1), int8(8), int16(16), int32(32), int64(64)}, want: sqlz.KindInt},
		{in: []interface{}{1, "2", "3"}, want: sqlz.KindDecimal},
		{in: []interface{}{"99999999999999999999999999999999999999999999999999999999"}, want: sqlz.KindDecimal},
		{in: []interface{}{"99999999999999999999999999999999999999999999999999999999xxx"}, want: sqlz.KindText},
		{in: []interface{}{1, "2", stdj.Number("1000")}, want: sqlz.KindDecimal},
		{in: []interface{}{1.0, "2.0"}, want: sqlz.KindDecimal},
		{in: []interface{}{1, float64(2.0), float32(7.7), int32(3)}, want: sqlz.KindFloat},
		{in: []interface{}{nil, nil, nil}, want: sqlz.KindText},
		{in: []interface{}{"1.0", "2.0", "3.0", "4", nil, int64(6)}, want: sqlz.KindDecimal},
		{in: []interface{}{true, false, nil, "true", "false", "yes", "no", ""}, want: sqlz.KindBool},
		{in: []interface{}{"0", "1"}, want: sqlz.KindDecimal},
		{in: []interface{}{fixtTime1, nil, ""}, want: sqlz.KindTime, wantMunge: true},
		{in: []interface{}{fixtTime2}, want: sqlz.KindTime, wantMunge: true},
		{in: []interface{}{fixtTime3}, want: sqlz.KindTime, wantMunge: true},
		{in: []interface{}{fixtTime4}, want: sqlz.KindTime, wantMunge: true},
		{in: []interface{}{fixtDate1, nil, ""}, want: sqlz.KindDate, wantMunge: true},
		{in: []interface{}{fixtDate2}, want: sqlz.KindDate, wantMunge: true},
		{in: []interface{}{fixtDate3}, want: sqlz.KindDate, wantMunge: true},
		{in: []interface{}{fixtDate4}, want: sqlz.KindDate, wantMunge: true},
		{in: []interface{}{fixtDatetime1, nil, ""}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetime2}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetimeAnsic}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetimeUnix}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{time.RubyDate}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{time.RFC822}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{time.RFC822Z}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{time.RFC850}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{time.RFC1123}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{time.RFC1123Z}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetimeRFC3339}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetimeStamp}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetimeStampMilli}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetimeStampMicro}, want: sqlz.KindDatetime, wantMunge: true},
		{in: []interface{}{fixtDatetimeStampNano}, want: sqlz.KindDatetime, wantMunge: true},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(strconv.Itoa(i), func(t *testing.T) {
			kd := sqlz.NewKindDetector()

			for _, val := range tc.in {
				kd.Sample(val)
			}

			gotKind, gotMungeFn, gotErr := kd.Detect()
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.Equal(t, tc.want.String(), gotKind.String())

			if !tc.wantMunge {
				require.Nil(t, gotMungeFn)
			} else {
				require.NotNil(t, gotMungeFn)
				for _, val := range tc.in {
					_, err := gotMungeFn(val)
					require.NoError(t, err)
				}
			}
		})
	}
}
