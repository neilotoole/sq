package staffdir

import (
	"context"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/oncecache"
)

var _ HRSystem = (*HRCache)(nil)

// HRCache is a caching layer for HRSystem.
type HRCache struct {
	log       *slog.Logger
	db        HRSystem
	stats     *Stats
	orgs      *oncecache.Cache[string, *Org]
	depts     *oncecache.Cache[string, *Department]
	employees *oncecache.Cache[int, *Employee]
}

// NewAppCache wraps a [HRSystem] with a caching layer.
func NewAppCache(log *slog.Logger, db HRSystem) *HRCache {
	c := &HRCache{
		log:   log,
		db:    db,
		stats: NewStats(),
	}

	c.orgs = oncecache.New[string, *Org](
		db.GetOrg,
		oncecache.OnFill(c.onFillOrg),
	)

	c.depts = oncecache.New[string, *Department](
		db.GetDepartment,
		oncecache.OnFill(c.onFillDept),
	)

	c.employees = oncecache.New[int, *Employee](
		db.GetEmployee,
	)

	return c
}

func (c *HRCache) onFillOrg(ctx context.Context, _ string, comp *Org, err error) {
	if err != nil {
		return
	}

	for _, dept := range comp.Departments {
		c.depts.Set(ctx, dept.Name, dept, nil)
	}
}

func (c *HRCache) onFillDept(ctx context.Context, _ string, dept *Department, err error) {
	if err != nil {
		return
	}

	for _, emp := range dept.Staff {
		c.employees.Set(ctx, emp.ID, emp, nil)
	}
}

// Close clears the cache.
func (c *HRCache) Close() error {
	ctx := context.Background()
	c.employees.Clear(ctx)
	c.orgs.Clear(ctx)
	c.depts.Clear(ctx)
	return nil
}

// Stats returns the cache stats.
func (c *HRCache) Stats() *Stats {
	return c.stats
}

// GetOrg implements [HRSystem].
func (c *HRCache) GetOrg(ctx context.Context, org string) (*Org, error) {
	c.stats.getOrg.Add(1)
	got, err := c.orgs.Get(ctx, "singleton")

	if err == nil {
		c.log.Info("GetOrg", "company", got)
	} else {
		c.log.Error("GetOrg", "error", err.Error())
	}

	return got, err
}

// GetDepartment implements [HRSystem].
func (c *HRCache) GetDepartment(ctx context.Context, dept string) (*Department, error) {
	c.stats.getDepartment.Add(1)
	got, err := c.depts.Get(ctx, dept)
	if err == nil {
		c.log.Info("GetDepartment", "dept", got)
	} else {
		c.log.Error("GetDepartment", "dept", got.Name, "error", err.Error())
	}
	return got, err
}

// GetEmployee implements [HRSystem].
func (c *HRCache) GetEmployee(ctx context.Context, id int) (*Employee, error) {
	c.stats.getEmployee.Add(1)
	employee, err := c.employees.Get(ctx, id)
	if err == nil {
		c.log.Info("GetEmployee", "id", id, "name", employee)
	} else {
		c.log.Error("GetEmployee", "id", id, "error", err.Error())
	}
	return employee, err
}
