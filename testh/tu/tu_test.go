package tu

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/lg/lgt"

	"github.com/stretchr/testify/require"
)

// TestFieldExtractionFunctions tests StructFieldValue, SliceFieldValues,
// SliceFieldKeyValues.
func TestFieldExtractionFunctions(t *testing.T) {
	type person struct {
		UUID     string
		Age      int
		Nickname *string
	}

	p1 := &person{
		UUID:     "235a50d7-3955-431c-8641-6ce171abf589",
		Age:      42,
		Nickname: nil,
	}

	nn := "The Great"
	p2 := &person{
		UUID:     "81975a8f-6add-441a-8c81-3806a9f4c6f0",
		Age:      27,
		Nickname: &nn,
	}

	uu := StructFieldValue("UUID", p1)
	require.Equal(t, uu, p1.UUID)
	age := StructFieldValue("Age", p1)
	require.Equal(t, age, 42)

	require.Panics(t, func() {
		_ = StructFieldValue("UUID", 123)
	}, "non-struct arg should panic")

	require.Nil(t, StructFieldValue("UUID", nil))

	require.Panics(t, func() {
		_ = StructFieldValue("", p1)
	}, "invalid fieldName should panic")

	require.Panics(t, func() {
		_ = StructFieldValue("NotAField", p1)
	}, "invalid fieldName should panic")

	nickname := StructFieldValue("Nickname", p1)
	require.Nil(t, nickname)

	nickname = StructFieldValue("Nickname", p2)
	require.NotNil(t, nickname)
	require.EqualValues(t, nickname, p2.Nickname)

	iSlice := []any{p1, p2}
	iVals := SliceFieldValues("UUID", iSlice)
	require.Len(t, iVals, 2)
	require.Equal(t, p1.UUID, iVals[0])

	personPtrSlice := []*person{p1, p2}
	iVals2 := SliceFieldValues("UUID", personPtrSlice)
	require.Len(t, iVals2, 2)
	require.EqualValues(t, iVals, iVals2)

	personSlice := []person{*p1, *p2}
	iVals3 := SliceFieldValues("UUID", personSlice)
	require.Len(t, iVals2, 2)
	require.EqualValues(t, iVals, iVals3)

	require.Panics(t, func() {
		_ = SliceFieldValues("UUID", p1)
	}, "non-slice arg should panic")

	m1 := SliceFieldKeyValues("UUID", "Age", iSlice)
	require.Len(t, m1, 2)

	require.Equal(t, m1[p1.UUID], p1.Age)
	require.Equal(t, m1[p2.UUID], p2.Age)
}

func TestInterfaceSlice(t *testing.T) {
	stringSlice := []string{"hello", "world"}
	iSlice := AnySlice(stringSlice)
	require.Equal(t, len(stringSlice), len(iSlice))
	require.Equal(t, stringSlice[0], iSlice[0])

	iSlice = AnySlice(nil)
	require.Nil(t, iSlice)

	require.Panics(t, func() {
		_ = AnySlice(42)
	}, "should panic for non-slice arg")
}

func TestTempDir(t *testing.T) {
	log := lgt.New(t)
	log.Debug("huzzxah")

	td1 := TempDir(t)
	t.Logf("td1: %s", td1)
	require.NotEmpty(t, td1)
	require.DirExists(t, td1)

	td2 := TempDir(t)
	t.Logf("td2: %s", td2)

	require.NotEqual(t, td1, td2)

	td3 := TempDir(t, "foo", "bar")
	t.Logf("td3: %s", td3)
	require.True(t, strings.HasSuffix(td3, filepath.Join("foo", "bar")))
}
