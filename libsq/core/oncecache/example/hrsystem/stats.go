package hrsystem

import (
	"fmt"
	"log/slog"
	"sync/atomic"
)

// Stats tracks how many times a method has been invoked.
type Stats struct {
	getOrg        *atomic.Int64
	getDepartment *atomic.Int64
	getEmployee   *atomic.Int64
}

// NewStats returns a new [Stats] instance.
func NewStats() *Stats {
	return &Stats{
		getOrg:        &atomic.Int64{},
		getDepartment: &atomic.Int64{},
		getEmployee:   &atomic.Int64{},
	}
}

// GetOrg returns the number of times GetOrg has been called.
func (s *Stats) GetOrg() int {
	return int(s.getOrg.Load())
}

// GetDepartment returns the number of times GetDepartment has been called.
func (s *Stats) GetDepartment() int {
	return int(s.getDepartment.Load())
}

// GetEmployee returns the number of times GetEmployee has been called.
func (s *Stats) GetEmployee() int {
	return int(s.getEmployee.Load())
}

// LogValue implements [slog.LogValuer].
func (s *Stats) LogValue() slog.Value {
	if s == nil {
		return slog.Value{}
	}

	return slog.GroupValue(
		slog.Int("GetOrg", int(s.getOrg.Load())),
		slog.Int("GetDepartment", int(s.getDepartment.Load())),
		slog.Int("GetEmployee", int(s.getEmployee.Load())),
	)
}

func (s *Stats) String() string {
	if s == nil {
		return ""
	}

	return fmt.Sprintf(
		"GetOrg: %d, GetDepartment: %d, GetEmployee: %d",
		s.GetOrg(), s.GetDepartment(), s.GetEmployee(),
	)
}
