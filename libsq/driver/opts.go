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
		nil,
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
		nil,
		2,
		"Max connections in idle connection pool",
		`Set the maximum number of connections in the idle connection pool. If
conn.max-open is greater than 0 but less than the new conn.max-idle, then the
new conn.max-idle will be reduced to match the conn.max-open limit.

If n <= 0, no idle connections are retained.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxIdleTime controls sql.DB.SetConnMaxIdleTime.
	OptConnMaxIdleTime = options.NewDuration(
		"conn.max-idle-time",
		nil,
		time.Second*2,
		"Max connection idle time",
		`Sets the maximum amount of time a connection may be idle. Expired connections
may be closed lazily before reuse.

If n <= 0, connections are not closed due to a connection's idle time.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxLifetime controls sql.DB.SetConnMaxLifetime.
	OptConnMaxLifetime = options.NewDuration(
		"conn.max-lifetime",
		nil,
		time.Minute*10,
		"Max connection lifetime",
		`
Set the maximum amount of time a connection may be reused. Expired connections
may be closed lazily before reuse.

If n <= 0, connections are not closed due to a connection's age.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnOpenTimeout controls connection open timeout.
	OptConnOpenTimeout = options.NewDuration(
		"conn.open-timeout",
		nil,
		time.Second*10,
		"Connection open timeout",
		"Max time to wait before a connection open timeout occurs.",
		options.TagSource,
		options.TagSQL,
	)

	// OptMaxRetryInterval is the maximum interval to wait
	// between retries.

	// OptTuningErrgroupLimit controls the maximum number of goroutines that can be spawned
	// by an errgroup.

	// OptTuningRecChanSize is the size of the buffer chan for record
	// insertion/writing.

)
