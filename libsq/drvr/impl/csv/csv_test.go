package csv

//
//func TestGenExcelColNames(t *testing.T) {
//
//	quantity := 1000
//
//	colNames := make([]string, quantity)
//
//	for i := 0; i < quantity; i++ {
//		colNames[i] = genExcelColName(i)
//	}
//
//	items := []struct {
//		index   int
//		colName string
//	}{
//		{0, "A"},
//		{1, "B"},
//		{25, "Z"},
//		{26, "AA"},
//		{27, "AB"},
//		{51, "AZ"},
//		{52, "BA"},
//		{53, "BB"},
//		{77, "BZ"},
//		{78, "CA"},
//		{701, "ZZ"},
//		{702, "AAA"},
//		{703, "AAB"},
//	}
//
//	for _, item := range items {
//		assert.Equal(t, item.colName, colNames[item.index])
//	}
//}
