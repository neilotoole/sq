package timefmt_test

import (
	"testing"
	"time"

	"github.com/neilotoole/sq/libsq/core/timefmt"
)

var (
	denver     *time.Location
	denverTime time.Time
)

func init() { //nolint:gochecknoinits
	var err error
	if denver, err = time.LoadLocation("America/Denver"); err != nil {
		panic(err)
	}

	denverTime = time.Date(2020, 11, 12, 13, 14, 15, 12345678, denver)
}

func TestFormatFunc(t *testing.T) {
	layouts := timefmt.NamedLayouts()
	// Add some custom layouts
	layouts = append(layouts, "%Y/%m/%d", "%s")

	for _, layout := range layouts {
		fn := timefmt.FormatFunc(layout)
		got := fn(denverTime)
		t.Logf("%16s: %s", layout, got)
	}
}
