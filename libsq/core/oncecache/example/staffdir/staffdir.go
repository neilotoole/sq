package staffdir

import (
	"context"
	"log/slog"
	"sync/atomic"
)

type Company struct {
	Name        string        `json:"name"`
	Departments []*Department `json:"departments"`
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

// LogValue implements slog.LogValuer.
func (e *Department) LogValue() slog.Value {
	if e == nil {
		return slog.Value{}
	}
	return slog.GroupValue(slog.String("name", e.Name), slog.Int("staff", len(e.Staff)))
}

type Employee struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func (e *Employee) String() string {
	if e == nil {
		return ""
	}
	return e.Name
}

// LogValue implements slog.LogValuer.
func (e *Employee) LogValue() slog.Value {
	if e == nil {
		return slog.Value{}
	}
	return slog.GroupValue(slog.Int("id", e.ID), slog.String("name", e.Name), slog.String("role", e.Role))
}

type DB interface {
	GetCompany(ctx context.Context) (*Company, error)
	ListDepartments(ctx context.Context) ([]*Department, error)
	GetDepartment(ctx context.Context, dept string) (*Department, error)
	ListEmployees(ctx context.Context) ([]*Employee, error)
	GetEmployee(ctx context.Context, ID int) (*Employee, error)
}

type Stats struct {
	getCompany      *atomic.Int64
	listDepartments *atomic.Int64
	getDepartment   *atomic.Int64
	listEmployees   *atomic.Int64
	getEmployee     *atomic.Int64
}

func NewUsageStats() *Stats {
	return &Stats{
		getCompany:      &atomic.Int64{},
		listDepartments: &atomic.Int64{},
		getDepartment:   &atomic.Int64{},
		listEmployees:   &atomic.Int64{},
		getEmployee:     &atomic.Int64{},
	}
}
