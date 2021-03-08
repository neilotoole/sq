package libsq

import (
	"context"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// EngineSLQ2SQL is a dedicated testing function that simulates
// execution of a SLQ query, but instead of executing the resulting
// SQL query, that ultimate SQL is returned. Effectively it is
// equivalent to libsq.ExecuteSLQ, but without the execution.
// Admittedly, this is an ugly workaround.
func EngineSLQ2SQL(ctx context.Context, log lg.Log, dbOpener driver.DatabaseOpener, joinDBOpener driver.JoinDatabaseOpener, srcs *source.Set, query string) (targetSQL string, err error) {
	var ng *engine
	ng, err = newEngine(ctx, log, dbOpener, joinDBOpener, srcs, query)
	if err != nil {
		return "", err
	}
	return ng.targetSQL, nil
}
