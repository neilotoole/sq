package hrsystem

import (
	"context"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/oncecache"
)

var _ HRSystem = (*HRCache)(nil)

// HRCache is a caching layer for any [HRSystem] implementation. HRCache is a
// composite cache, using a [oncecache.Cache] instance for each of the [Org],
// [Department], and [Employee] entity types. Using the [oncecache] event
// mechanism, a cache entry fill of one cache is propagated to other caches.
// For example, a call to [HRCache.GetOrg] populates not only that single cache
// entry, but propagates values to the [Department] cache, which in turn
// propagates values to the [Employee] cache.
type HRCache struct {
	log       *slog.Logger
	db        HRSystem
	stats     *Stats
	orgs      *oncecache.Cache[string, *Org]
	depts     *oncecache.Cache[string, *Department]
	employees *oncecache.Cache[int, *Employee]
}

// NewHRCache wraps db with a caching layer.
func NewHRCache(log *slog.Logger, db HRSystem) *HRCache {
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

// GetOrg implements [HRSystem].
func (c *HRCache) GetOrg(ctx context.Context, orgName string) (*Org, error) {
	c.stats.getOrg.Add(1)

	org, err := c.orgs.Get(ctx, orgName)
	if err == nil {
		c.log.Info("GetOrg", "org", org)
	} else {
		c.log.Error("GetOrg", "error", err.Error())
	}

	return org, err
}

// GetDepartment implements [HRSystem].
func (c *HRCache) GetDepartment(ctx context.Context, deptName string) (*Department, error) {
	c.stats.getDepartment.Add(1)

	dept, err := c.depts.Get(ctx, deptName)
	if err == nil {
		c.log.Info("GetDepartment", "dept", dept)
	} else {
		c.log.Error("GetDepartment", "dept", dept.Name, "error", err.Error())
	}

	return dept, err
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

// onFillOrg is invoked by HRCache.orgs when that cache fills an [Org] value from
// the DB. This handler propagates values from the returned [Org] to the
// HRCache.depts cache.
func (c *HRCache) onFillOrg(ctx context.Context, _ string, org *Org, err error) {
	if err != nil {
		return
	}

	for _, dept := range org.Departments {
		// Filling a dept entry should in turn propagate to the employees cache.
		c.depts.MaybeSet(ctx, dept.Name, dept, nil)
	}
}

// onFillDept is invoked by HRCache.depts when that cache fills a [Department]
// value from the DB. This handler propagates [Employee] values from the
// returned [Department] to the HRCache.employees cache.
func (c *HRCache) onFillDept(ctx context.Context, _ string, dept *Department, err error) {
	if err != nil {
		return
	}

	for _, emp := range dept.Staff {
		c.employees.MaybeSet(ctx, emp.ID, emp, nil)
	}
}
