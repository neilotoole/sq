package driver

import (
	"context"
	"database/sql"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/options"
)

// ConfigureDB configures DB using o. It is no-op if o is nil.
func ConfigureDB(ctx context.Context, db *sql.DB, o options.Options) {
	o2 := options.Effective(o, OptConnMaxOpen, OptConnMaxIdle, OptConnMaxIdleTime, OptConnMaxLifetime)

	lg.FromContext(ctx).Debug("Setting config on DB conn", "config", o2)

	db.SetMaxOpenConns(OptConnMaxOpen.Get(o2))
	db.SetMaxIdleConns(OptConnMaxIdle.Get(o2))
	db.SetConnMaxIdleTime(OptConnMaxIdleTime.Get(o2))
	db.SetConnMaxLifetime(OptConnMaxLifetime.Get(o2))
}

var (
	// OptConnMaxOpen controls sql.DB.SetMaxOpenConn.
	OptConnMaxOpen = options.NewInt(
		"conn.max-open",
		"",
		0,
		0,
		"Max open connections to DB",
		`Maximum number of open connections to the database.
A value of zero indicates no limit.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxIdle controls sql.DB.SetMaxIdleConns.
	OptConnMaxIdle = options.NewInt(
		"conn.max-idle",
		"",
		0,
		2,
		"Max connections in idle connection pool",
		`Set the maximum number of connections in the idle connection pool.
If conn.max-open is greater than 0 but less than the new conn.max-idle,
then the new conn.max-idle will be reduced to match the conn.max-open limit.
If n <= 0, no idle connections are retained.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxIdleTime controls sql.DB.SetConnMaxIdleTime.
	OptConnMaxIdleTime = options.NewDuration(
		"conn.max-idle-time",
		"",
		0,
		time.Second*2,
		"Max connection idle time",
		`Sets the maximum amount of time a connection may be idle.
Expired connections may be closed lazily before reuse. If n <= 0,
connections are not closed due to a connection's idle time.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxLifetime controls sql.DB.SetConnMaxLifetime.
	OptConnMaxLifetime = options.NewDuration(
		"conn.max-lifetime",
		"",
		0,
		time.Minute*10,
		"Max connection lifetime",
		`Set the maximum amount of time a connection may be reused.
Expired connections may be closed lazily before reuse.
If n <= 0, connections are not closed due to a connection's age.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnOpenTimeout controls connection open timeout.
	OptConnOpenTimeout = options.NewDuration(
		"conn.open-timeout",
		"",
		0,
		time.Second*2,
		"Connection open timeout",
		"Max time to wait before a connection open timeout occurs.",
		options.TagSource,
		options.TagSQL,
	)

	// OptMaxRetryInterval is the maximum interval to wait
	// between retries.
	OptMaxRetryInterval = options.NewDuration(
		"retry.max-interval",
		"",
		0,
		time.Second*3,
		"Max interval between retries",
		`The maximum interval to wait between retries.
If an operation is retryable (for example, if the DB has too many clients),
repeated retry operations back off, typically using a Fibonacci backoff.`,
		options.TagSource,
	)

	// OptTuningErrgroupLimit controls the maximum number of goroutines that can be spawned
	// by an errgroup.
	OptTuningErrgroupLimit = options.NewInt(
		"tuning.errgroup-limit",
		"",
		0,
		16,
		"Max goroutines in any one errgroup",
		`Controls the maximum number of goroutines that can be spawned
by an errgroup. Note that this is the limit for any one errgroup, but not a
ceiling on the total number of goroutines spawned, as some errgroups may
themselves start an errgroup.

This knob is primarily for internal use. Ultimately it should go away
in favor of dynamic errgroup limit setting based on availability
of additional DB conns, etc.`,
		options.TagTuning,
	)

	// OptTuningRecChanSize is the size of the buffer chan for record
	// insertion/writing.
	OptTuningRecChanSize = options.NewInt(
		"tuning.record-buffer",
		"",
		0,
		1024,
		"Size of record buffer",
		`Controls the size of the buffer channel for record insertion/writing.`,
		options.TagTuning,
	)
)
