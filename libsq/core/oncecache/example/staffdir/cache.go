package staffdir

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"log/slog"
)

var _ DB = (*DirCache)(nil)

func NewDirCache(log *slog.Logger, db DB) *DirCache {
	return &DirCache{
		log:   log,
		db:    db,
		stats: NewUsageStats(),
		companies: oncecache.New[string, *Company](func(ctx context.Context, _ string) (val *Company, err error) {
			return db.GetCompany(ctx)
		}),
		departments: oncecache.New[string, *Department](db.GetDepartment),
		employees:   oncecache.New[int, *Employee](db.GetEmployee),
	}
}

type DirCache struct {
	log         *slog.Logger
	db          DB
	stats       *Stats
	companies   *oncecache.Cache[string, *Company]
	departments *oncecache.Cache[string, *Department]
	employees   *oncecache.Cache[int, *Employee]
}

func (dc *DirCache) GetCompany(ctx context.Context) (*Company, error) {
	dc.stats.getCompany.Add(1)
	got, err := dc.companies.Get(ctx, "singleton")
	if err == nil {
		dc.log.Info("GetCompany", "company", got)
	} else {
		dc.log.Error("GetCompany", "error", err.Error())
	}
	return got, err
}

func (dc *DirCache) ListDepartments(ctx context.Context) ([]*Department, error) {
	dc.stats.listDepartments.Add(1)
	got, err := dc.db.ListDepartments(ctx)
	if err == nil {
		dc.log.Info("ListDepartments", "count", len(got))
	} else {
		dc.log.Error("ListDepartments", "error", err.Error())
	}
	return got, err
}

func (dc *DirCache) GetDepartment(ctx context.Context, dept string) (*Department, error) {
	got, err := dc.departments.Get(ctx, dept)
	if err == nil {
		dc.log.Info("GetDepartment", "dept", got)
	} else {
		dc.log.Error("GetDepartment", "dept", got.Name, "error", err.Error())
	}
	return got, err
}

func (dc *DirCache) ListEmployees(ctx context.Context) ([]*Employee, error) {
	got, err := dc.db.ListEmployees(ctx)
	if err == nil {
		dc.log.Info("ListEmployees", "count", len(got))
	} else {
		dc.log.Error("ListEmployees", "error", err.Error())
	}
	return got, err
}

func (dc *DirCache) GetEmployee(ctx context.Context, id int) (*Employee, error) {
	got, err := dc.employees.Get(ctx, id)
	if err == nil {
		dc.log.Info("GetEmployee", "id", id, "name", got)
	} else {
		dc.log.Error("GetEmployee", "id", id, "error", err.Error())
	}
	return got, err
}
