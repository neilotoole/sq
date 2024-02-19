package oncecache_test

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"strconv"
	"strings"
	"sync/atomic"
)

type Company struct {
	Name        string
	Departments []*Department
}

type Department struct {
	Name      string
	Employees []*Employee
}

type Employee struct {
	ID   int
	Name string
	Role string
}

type DB interface {
	GetCompany(ctx context.Context, company string) (*Company, error)
	GetDepartment(ctx context.Context, company, dept string) (*Department, error)
	GetEmployee(ctx context.Context, company string, ID int) (*Employee, error)
}

var _ DB = (*InMemoryDB)(nil)

type InMemoryDB struct {
	getCompanyCounter    *atomic.Int64
	getEmployeeCounter   *atomic.Int64
	getDepartmentCounter *atomic.Int64
	companies            map[string]*Company
}

func NewInMemoryDB(companies ...*Company) *InMemoryDB {
	db := &InMemoryDB{
		getCompanyCounter:    &atomic.Int64{},
		getEmployeeCounter:   &atomic.Int64{},
		getDepartmentCounter: &atomic.Int64{},
		companies:            make(map[string]*Company),
	}

	for _, comp := range companies {
		db.companies[comp.Name] = comp
	}

	return db
}

func (db *InMemoryDB) GetCompany(_ context.Context, company string) (*Company, error) {
	db.getCompanyCounter.Add(1)
	comp, ok := db.companies[company]
	if !ok {
		return nil, fmt.Errorf("db: not found: company {%s}", company)
	}
	return comp, nil
}

func (db *InMemoryDB) GetDepartment(ctx context.Context, company, dept string) (*Department, error) {
	db.getDepartmentCounter.Add(1)
	comp, err := db.GetCompany(ctx, company)
	if err != nil {
		return nil, err
	}

	for i := range comp.Departments {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if comp.Departments[i].Name == dept {
			return comp.Departments[i], nil
		}
	}

	return nil, fmt.Errorf("db: not found: department {%s: %s}", company, dept)
}

func (db *InMemoryDB) GetEmployee(ctx context.Context, company string, ID int) (*Employee, error) {
	db.getEmployeeCounter.Add(1)
	comp, err := db.GetCompany(ctx, company)
	if err != nil {
		return nil, err
	}

	for _, dept := range comp.Departments {
		for _, emp := range dept.Employees {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			if emp.ID == ID {
				return emp, nil
			}
		}
	}

	return nil, fmt.Errorf("db: not found: employee {%s: %d}", company, ID)
}

var _ DB = (*AppCache)(nil)

func NewAppCache(db DB) *AppCache {
	return &AppCache{
		db:                   db,
		getCompanyCounter:    &atomic.Int64{},
		getEmployeeCounter:   &atomic.Int64{},
		getDepartmentCounter: &atomic.Int64{},
		companies:            oncecache.New[string, *Company](db.GetCompany),
		departments: oncecache.New[string, *Department](func(ctx context.Context, key string) (val *Department, err error) {
			company, dept, found := strings.Cut(key, ".")
			if !found {
				return nil, fmt.Errorf("invalid key: %s", key)
			}
			return db.GetDepartment(ctx, company, dept)
		}),
		employees: oncecache.New[string, *Employee](func(ctx context.Context, key string) (val *Employee, err error) {
			company, empIDstr, found := strings.Cut(key, ".")
			if !found {
				return nil, fmt.Errorf("invalid key: %s", key)
			}

			empID, err := strconv.Atoi(empIDstr)
			if err != nil {
				return nil, fmt.Errorf("invalid key: %s", key)
			}

			return db.GetEmployee(ctx, company, empID)
		}),
	}
}

type AppCache struct {
	db                   DB
	getCompanyCounter    *atomic.Int64
	getEmployeeCounter   *atomic.Int64
	getDepartmentCounter *atomic.Int64
	companies            *oncecache.Cache[string, *Company]
	departments          *oncecache.Cache[string, *Department]
	employees            *oncecache.Cache[string, *Employee]
}

func (a *AppCache) GetCompany(ctx context.Context, company string) (*Company, error) {
	a.getCompanyCounter.Add(1)
	return a.companies.Get(ctx, company)
}

func (a *AppCache) GetDepartment(ctx context.Context, company, dept string) (*Department, error) {
	a.getDepartmentCounter.Add(1)
	return a.departments.Get(ctx, company+"."+dept)
}

func (a *AppCache) GetEmployee(ctx context.Context, company string, ID int) (*Employee, error) {
	a.getEmployeeCounter.Add(1)
	return a.employees.Get(ctx, company+"."+strconv.Itoa(ID))
}
