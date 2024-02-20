package staffdir

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

var _ StaffDirectory = (*InMemDB)(nil)

type InMemDB struct {
	company *Org
	log     *slog.Logger
	stats   *Stats
}

// NewInMemDB returns a new InMemDB instance loaded from json datafile.
func NewInMemDB(log *slog.Logger, datafile string) (*InMemDB, error) {
	var err error
	data, err := os.ReadFile(datafile)
	if err != nil {
		return nil, err
	}

	var company *Org
	err = json.Unmarshal(data, &company)
	if err != nil {
		return nil, err
	}

	db := &InMemDB{
		log:     log,
		company: company,
		stats:   NewStats(),
	}

	return db, nil
}

func (md *InMemDB) Stats() *Stats {
	return md.stats
}

func (md *InMemDB) GetOrg(ctx context.Context, org string) (*Org, error) {
	md.stats.getOrg.Add(1)
	md.log.Info("GetOrg", "company", md.company.Name)
	return md.company, nil
}

func (md *InMemDB) GetDepartment(ctx context.Context, dept string) (*Department, error) {
	md.stats.getDepartment.Add(1)
	for _, d := range md.company.Departments {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if d.Name == dept {
			md.log.Info("GetDepartment", "dept", d.Name)
			return d, nil
		}
	}

	err := fmt.Errorf("db: not found: department {%s}", dept)
	md.log.Error("GetDepartment", "dept", dept, "error", err.Error())
	return nil, err
}

func (md *InMemDB) GetEmployee(ctx context.Context, id int) (*Employee, error) {
	md.stats.getEmployee.Add(1)
	for _, dept := range md.company.Departments {
		for _, emp := range dept.Staff {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			if emp.ID == id {
				md.log.Info("GetEmployee", "id", emp.ID, "name", emp.Name)
				return emp, nil
			}
		}
	}

	err := fmt.Errorf("db: not found: employee {%d}", id)
	md.log.Error("GetEmployee", "id", id, "error", err.Error())
	return nil, err
}
