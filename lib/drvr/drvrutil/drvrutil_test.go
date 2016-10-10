package drvrutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateExcelColName(t *testing.T) {

	quantity := 704

	colNames := make([]string, quantity)

	for i := 0; i < quantity; i++ {
		colNames[i] = GenerateExcelColName(i)
	}

	items := []struct {
		index   int
		colName string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{53, "BB"},
		{77, "BZ"},
		{78, "CA"},
		{701, "ZZ"},
		{702, "AAA"},
		{703, "AAB"},
	}

	for _, item := range items {
		assert.Equal(t, item.colName, colNames[item.index])
	}
}

func TestGetSourceFile(t *testing.T) {

	location := "http://neilotoole.io/sq/test/test1.xlsx"

	file, mediatype, cleanup, err := GetSourceFile(location)
	if cleanup != nil {
		defer require.Nil(t, cleanup())
	}
	require.Nil(t, err)
	require.NotNil(t, file)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", mediatype)
	require.Nil(t, file.Close())
}
