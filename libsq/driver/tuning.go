package driver

import "time"

// Tuning holds tuning params. Ultimately these params
// could come from user config or be dynamically calculated/adjusted?
//
// FIXME: move all of these to options.Options.
var Tuning = struct {
	// ErrgroupLimit is passed to errgroup.Group.SetLimit.
	// Note that this is the limit for any one errgroup, but
	// not a ceiling on the total number of goroutines spawned,
	// as some errgroups may themselves start an errgroup.
	ErrgroupLimit int

	// RecordChSize is the size of the buffer chan for record
	// insertion/writing.
	RecordChSize int

	// SampleSize is the number of samples that a detector should
	// take to determine type.
	SampleSize int

	// MaxRetryInterval is the maximum interval to wait between retries.
	MaxRetryInterval time.Duration
}{
	ErrgroupLimit:    16,
	RecordChSize:     1024,
	SampleSize:       1024,
	MaxRetryInterval: time.Second * 3,
}
