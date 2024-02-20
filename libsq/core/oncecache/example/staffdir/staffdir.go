package staffdir

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
)

type StaffDirectory interface {
	GetCompany(ctx context.Context) (*Company, error)
	GetDepartment(ctx context.Context, dept string) (*Department, error)
	GetEmployee(ctx context.Context, ID int) (*Employee, error)
}

type Company struct {
	Name        string        `json:"name"`
	Departments []*Department `json:"departments"`
}

// LogValue implements [slog.LogValuer].
func (c *Company) LogValue() slog.Value {
	if c == nil {
		return slog.Value{}
	}
	return slog.GroupValue(
		slog.String("name", c.Name),
		slog.Int("depts", len(c.Departments)),
	)
}

func (c *Company) String() string {
	if c == nil {
		return ""
	}
	return c.Name
}

type Department struct {
	Name  string      `json:"name"`
	Staff []*Employee `json:"staff"`
}

func (d *Department) String() string {
	if d == nil {
		return ""
	}
	return d.Name
}

// LogValue implements [slog.LogValuer].
func (d *Department) LogValue() slog.Value {
	if d == nil {
		return slog.Value{}
	}
	return slog.GroupValue(slog.String("name", d.Name), slog.Int("staff", len(d.Staff)))
}

type Employee struct {
	Name string `json:"name"`
	Role string `json:"role"`
	ID   int    `json:"id"`
}

func (e *Employee) String() string {
	if e == nil {
		return ""
	}
	return e.Name
}

// LogValue implements [slog.LogValuer].
func (e *Employee) LogValue() slog.Value {
	if e == nil {
		return slog.Value{}
	}

	return slog.GroupValue(
		slog.Int("id", e.ID),
		slog.String("name", e.Name),
		slog.String("role", e.Role),
	)
}

func NewStats() *Stats {
	return &Stats{
		getCompany:    &atomic.Int64{},
		getDepartment: &atomic.Int64{},
		getEmployee:   &atomic.Int64{},
	}
}

type Stats struct {
	getCompany    *atomic.Int64
	getDepartment *atomic.Int64
	getEmployee   *atomic.Int64
}

func (s *Stats) GetCompany() int {
	return int(s.getCompany.Load())
}

func (s *Stats) GetDepartment() int {
	return int(s.getDepartment.Load())
}

func (s *Stats) GetEmployee() int {
	return int(s.getEmployee.Load())
}

func (s *Stats) LogValue() slog.Value {
	if s == nil {
		return slog.Value{}
	}

	return slog.GroupValue(
		slog.Int("GetCompany", int(s.getCompany.Load())),
		slog.Int("GetDepartment", int(s.getDepartment.Load())),
		slog.Int("GetEmployee", int(s.getEmployee.Load())),
	)
}

func (s *Stats) String() string {
	if s == nil {
		return ""
	}

	return fmt.Sprintf(
		"GetCompany: %d, GetDepartment: %d, GetEmployee: %d",
		s.GetCompany(), s.GetDepartment(), s.GetEmployee(),
	)
}
