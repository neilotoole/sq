// Package libsq provides a high-level interface for executing SLQ and traditional
// database-native SQL.
package libsq

import (
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/engine"
)

// Execute constructs a plan from the SLQ input, executes the plan, and writes the results to writer.
func Execute(srcs drvr.SourceSet, slq string, writer engine.RecordWriter) error {
	plan, err := engine.BuildPlan(&srcs, slq)
	if err != nil {
		return err
	}
	return engine.New(srcs, plan, writer).Execute()
}

// ExecuteSQL executes a database-native SQL query against the data source, and writes the results to writer.
func ExecuteSQL(src drvr.Source, sql string, writer engine.RecordWriter) error {
	db, err := engine.NewDatabase(&src)
	if err != nil {
		return err
	}
	err = db.Query(sql, writer)
	return err
}
