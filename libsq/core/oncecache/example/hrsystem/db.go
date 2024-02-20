package hrsystem

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

var _ HRSystem = (*HRDatabase)(nil)

// HRDatabase pretends to be a database, but it's really just an in-memory
// structure loaded from a JSON file. It implements [HRSystem].
type HRDatabase struct {
	org   *Org
	log   *slog.Logger
	stats *Stats
}

// NewHRDatabase returns a new HRDatabase instance loaded from json datafile.
func NewHRDatabase(log *slog.Logger, datafile string) (*HRDatabase, error) {
	var err error
	data, err := os.ReadFile(datafile)
	if err != nil {
		return nil, err
	}

	var org *Org
	if err = json.Unmarshal(data, &org); err != nil {
		return nil, err
	}

	db := &HRDatabase{log: log, org: org, stats: NewStats()}
	return db, nil
}

// GetOrg implements [HRSystem].
func (db *HRDatabase) GetOrg(_ context.Context, org string) (*Org, error) {
	db.stats.getOrg.Add(1)

	if db.org.Name != org {
		err := fmt.Errorf("db: not found: org {%s}", org)
		db.log.Error("GetOrg", "org", org, "error", err.Error())
		return nil, err
	}

	db.log.Info("GetOrg", "org", db.org.Name)
	return db.org, nil
}

// GetDepartment implements [HRSystem].
func (db *HRDatabase) GetDepartment(ctx context.Context, dept string) (*Department, error) {
	db.stats.getDepartment.Add(1)
	for _, d := range db.org.Departments {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if d.Name == dept {
			db.log.Info("GetDepartment", "dept", d.Name)
			return d, nil
		}
	}

	err := fmt.Errorf("db: not found: department {%s}", dept)
	db.log.Error("GetDepartment", "dept", dept, "error", err.Error())
	return nil, err
}

// GetEmployee implements [HRSystem].
func (db *HRDatabase) GetEmployee(ctx context.Context, id int) (*Employee, error) {
	db.stats.getEmployee.Add(1)
	for _, dept := range db.org.Departments {
		for _, emp := range dept.Staff {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			if emp.ID == id {
				db.log.Info("GetEmployee", "id", emp.ID, "name", emp.Name)
				return emp, nil
			}
		}
	}

	err := fmt.Errorf("db: not found: employee {%d}", id)
	db.log.Error("GetEmployee", "id", id, "error", err.Error())
	return nil, err
}

// Stats returns database invocation stats.
func (db *HRDatabase) Stats() *Stats {
	return db.stats
}
