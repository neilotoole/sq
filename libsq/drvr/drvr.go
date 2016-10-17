package drvr

import (
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/libsq/util"
)

type Driver interface {
	// Type returns the driver type.
	Type() Type

	// ConnURI returns the driver-specific connection string.
	ConnURI(source *Source) (string, error)

	// Open returns a database handle.
	Open(source *Source) (*sql.DB, error)

	// Ping verifies that the source is reachable, or returns an error if not.
	// The exact behavior of Ping() is driver-dependent.
	Ping(source *Source) error
	// ValidateSource verifies that the source is valid for this driver. It
	// may transform the source into a canonical form, which is returned in
	// the "src" return value (the original source is not changed). An error
	// is returned if the source is invalid.
	ValidateSource(source *Source) (src *Source, err error)

	// Release instructs the driver instance to release any held resources,
	// effectively shutting down the instance.
	// TODO: do we need Release() and the application-wide shutdown mechanism?
	Release() error

	// Metadata returns metadata about the provided datasource.
	Metadata(src *Source) (*SourceMetadata, error)
}

var registeredDrivers = make(map[Type]Driver)

func Register(driver Driver) {
	registeredDrivers[driver.Type()] = driver
}

// For returns a driver for the supplied data source.
func For(source *Source) (Driver, error) {

	drv, ok := registeredDrivers[source.Type]
	if !ok {
		return nil, util.Errorf("unknown driver type %q for data source %q", source.Type, source.Handle)
	}

	lg.Debugf("returning driver %q for data source %q", drv.Type(), source.Handle)
	return drv, nil
}
