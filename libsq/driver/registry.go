package driver

import (
	"sync"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
)

// NewRegistry returns a new Registry instance that provides
// access to driver implementations. Note that Registry
// implements driver.Provider.
func NewRegistry(log lg.Log) *Registry {
	return &Registry{
		log:       log,
		providers: map[source.Type]Provider{},
	}
}

// Registry provides access to driver implementations.
type Registry struct {
	mu        sync.Mutex
	log       lg.Log
	providers map[source.Type]Provider
	types     []source.Type
}

// AddProvider registers the provider for the specified driver type.
// This method has no effect if there's already a provider for typ.
func (r *Registry) AddProvider(typ source.Type, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existingType, ok := r.providers[typ]; ok {
		r.log.Warnf("failed to add driver provider (%T) for driver type %s: provider (%T) already registered", p, typ, existingType)
		return
	}

	r.types = append(r.types, typ)
	r.providers[typ] = p
}

// HasProviderFor returns true if a provider for typ exists.
func (r *Registry) HasProviderFor(typ source.Type) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.providers[typ]
	return ok
}

// DriverFor implements Provider.
func (r *Registry) DriverFor(typ source.Type) (Driver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.providers[typ]
	if !ok {
		return nil, errz.Errorf("no registered driver for %q", typ)
	}

	return p.DriverFor(typ)
}

// DriversMetadata returns metadata for each registered driver type.
func (r *Registry) DriversMetadata() []Metadata {
	var md []Metadata
	for _, typ := range r.types {
		drv, err := r.DriverFor(typ)
		if err != nil {
			// Should never happen
			r.log.Errorf("error getting %q driver: %v", typ, err)
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
			r.log.Errorf("Error getting %q driver: %v", typ, err)
			continue
		}
		drvrs = append(drvrs, drvr)
	}

	return drvrs
}
