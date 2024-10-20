package driver

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// NewRegistry returns a new Registry instance that provides
// access to driver implementations. Note that Registry
// implements Provider.
func NewRegistry(log *slog.Logger) *Registry {
	return &Registry{
		log:       log,
		providers: map[drivertype.Type]Provider{},
	}
}

// Registry provides access to driver implementations.
type Registry struct {
	log       *slog.Logger
	providers map[drivertype.Type]Provider
	types     []drivertype.Type
	mu        sync.Mutex
}

// AddProvider registers the provider for the specified driver type.
// This method has no effect if there's already a provider for typ.
func (r *Registry) AddProvider(typ drivertype.Type, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existingType, ok := r.providers[typ]; ok {
		r.log.Warn("failed to add new driver provider for driver type: provider (%T) already registered",
			"new_driver_provider", fmt.Sprintf("%T", p),
			"driver_type", typ,
			"existing_driver_provider", fmt.Sprintf("%T", existingType))
		return
	}

	r.types = append(r.types, typ)
	r.providers[typ] = p
}

// ProviderFor returns the provider for typ, or nil if no
// registered provider.
func (r *Registry) ProviderFor(typ drivertype.Type) Provider {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.providers[typ]
	return p
}

// DriverFor implements Provider.
func (r *Registry) DriverFor(typ drivertype.Type) (Driver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.providers[typ]
	if !ok {
		return nil, errz.Errorf("no registered driver for {%s}", typ)
	}

	return p.DriverFor(typ)
}

// SQLDriverFor for is a convenience method for getting a SQLDriver.
func (r *Registry) SQLDriverFor(typ drivertype.Type) (SQLDriver, error) {
	drvr, err := r.DriverFor(typ)
	if err != nil {
		return nil, err
	}

	sqlDrvr, ok := drvr.(SQLDriver)
	if !ok {
		return nil, errz.Errorf("driver %T is not of type %T", drvr, sqlDrvr)
	}

	return sqlDrvr, nil
}

// DriversMetadata returns metadata for each registered driver type.
func (r *Registry) DriversMetadata() []Metadata {
	var md []Metadata
	for _, typ := range r.types {
		drv, err := r.DriverFor(typ)
		if err != nil {
			// Should never happen
			r.log.Error("Error getting driver", lga.Type, typ, lga.Err, err)
			continue
		}
		md = append(md, drv.DriverMetadata())
	}

	return md
}

// Drivers returns the registered drivers.
func (r *Registry) Drivers() []Driver {
	var drvrs []Driver

	for _, typ := range r.types {
		drvr, err := r.DriverFor(typ)
		if err != nil {
			// Should never happen
			r.log.Error("Error getting driver", lga.Type, typ, lga.Err, err)
			continue
		}
		drvrs = append(drvrs, drvr)
	}

	return drvrs
}
