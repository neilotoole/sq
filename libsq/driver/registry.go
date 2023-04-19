package driver

import (
	"sync"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
)

// NewRegistry returns a new Registry instance that provides
// access to driver implementations. Note that Registry
// implements Provider.
func NewRegistry(log *slog.Logger) *Registry {
	return &Registry{
		log:       log,
		providers: map[source.DriverType]Provider{},
	}
}

// Registry provides access to driver implementations.
type Registry struct {
	log       *slog.Logger
	mu        sync.Mutex
	providers map[source.DriverType]Provider
	types     []source.DriverType
}

// AddProvider registers the provider for the specified driver type.
// This method has no effect if there's already a provider for typ.
func (r *Registry) AddProvider(typ source.DriverType, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existingType, ok := r.providers[typ]; ok {
		r.log.Warn("failed to add driver provider (%T) for driver type %s: provider (%T) already registered", p, typ,
			existingType)
		return
	}

	r.types = append(r.types, typ)
	r.providers[typ] = p
}

// ProviderFor returns the provider for typ, or nil if no
// registered provider.
func (r *Registry) ProviderFor(typ source.DriverType) Provider {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.providers[typ]
	return p
}

// DriverFor implements Provider.
func (r *Registry) DriverFor(typ source.DriverType) (Driver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.providers[typ]
	if !ok {
		return nil, errz.Errorf("no registered driver for {%s}", typ)
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
			r.log.Error("error getting {%s} driver: %v", typ, err)
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
			r.log.Error("Error getting {%s} driver: %v", typ, err)
			continue
		}
		drvrs = append(drvrs, drvr)
	}

	return drvrs
}
