package xlsx

import (
	"context"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/xuri/excelize/v2"
)

// From https://github.com/qax-os/excelize/issues/660

func convertRowDates(ctx context.Context, f *excelize.File, sheetName string, rowi int, row []string) []string {
	for x := range row {
		coords, _ := excelize.CoordinatesToCellName(x+1, rowi+1)
		style, err := f.GetCellStyle(sheetName, coords)
		if err != nil {
			lg.FromContext(ctx).Warn("Excel: couldn't get cell style; continuing",
				lga.Err, errz.Err(err))
			//_, _ = fmt.Println(err)
			continue
		}
		row[x] = convertIfDate(f, style, row[x])
	}

	return row
}

func convertDates(ctx context.Context, f *excelize.File, sheetName string, rows [][]string) [][]string {
	for y := range rows {
		for x := range rows[y] {
			coords, _ := excelize.CoordinatesToCellName(x+1, y+1)
			style, err := f.GetCellStyle(sheetName, coords)
			if err != nil {
				lg.FromContext(ctx).Warn("Excel: couldn't get cell style; continuing",
					lga.Err, errz.Err(err))
				//_, _ = fmt.Println(err)
				continue
			}
			rows[y][x] = convertIfDate(f, style, rows[y][x])
		}
	}
	return rows
}

func convertIfDate(f *excelize.File, s int, v string) string {
	if s == 0 {
		return v
	}
	styleSheet := f.Styles
	if s >= len(styleSheet.CellXfs.Xf) {
		return v
	}
	var numFmtID int
	if styleSheet.CellXfs.Xf[s].NumFmtID != nil {
		numFmtID = *styleSheet.CellXfs.Xf[s].NumFmtID
	}
	var timeFormat string
	switch numFmtID {
	case 14:
		//"mm-dd-yy"
		timeFormat = "01-02-06"
	case 15:
		//"d-mmm-yy"
		timeFormat = "02-Jan-06"
	case 16:
		//"d-mmm"
		timeFormat = "02-Jan"
	case 17:
		//"mmm-yy"
		timeFormat = "Jan-06"
	case 22:
		//"m/d/yy h:mm"
		timeFormat = "1/2/06 15:04"
	default:
		return v
	}
	t, err := time.Parse(timeFormat, v)
	if err != nil {
		return v
	}
	return t.Format(time.RFC3339)
}
