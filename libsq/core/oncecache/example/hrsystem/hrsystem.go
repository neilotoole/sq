package hrsystem

import (
	"context"
	"log/slog"
)

type HRSystem interface {
	GetOrg(ctx context.Context, org string) (*Org, error)
	GetDepartment(ctx context.Context, dept string) (*Department, error)
	GetEmployee(ctx context.Context, ID int) (*Employee, error)
}

type Org struct {
	Name        string        `json:"name"`
	Departments []*Department `json:"departments"`
}

// LogValue implements [slog.LogValuer].
func (o *Org) LogValue() slog.Value {
	if o == nil {
		return slog.Value{}
	}
	return slog.GroupValue(
		slog.String("name", o.Name),
		slog.Int("depts", len(o.Departments)),
	)
}

func (o *Org) String() string {
	if o == nil {
		return ""
	}
	return o.Name
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
