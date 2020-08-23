package notify

import (
	"time"

	uuid "github.com/satori/go.uuid"

	"github.com/neilotoole/sq/libsq/core/stringz"
)

// State is the job state, one of Created, Running, Completed or Failed.
type State string

// Possible values of State.
const (
	Created   = State("CREATED")
	Running   = State("RUNNING")
	Completed = State("COMPLETED")
	Canceled  = State("CANCELED")
	Failed    = State("FAILED")
)

// Job represents a single libsq engine workflow instance.
type Job struct {
	ID      string     `yaml:"id" json:"id"`
	Started *time.Time `yaml:"started,omitempty" json:"started,omitempty"`
	Ended   *time.Time `yaml:"ended,omitempty" json:"ended,omitempty"`

	// Stmt is the SLQ/SQL statement/query this job is executing.
	Stmt   string  `yaml:"stmt" json:"stmt"`
	State  State   `yaml:"state" json:"state"`
	Errors []error `yaml:"errors,omitempty" json:"errors,omitempty"`
}

// New returns a new Job, with a generated ID, and State set to Created.
// The Started and Ended fields are both nil.
func New(stmt string) *Job {
	return &Job{ID: uuid.NewV4().String(), State: Created, Stmt: stmt}
}

func (j *Job) String() string {
	return stringz.SprintJSON(j)
}

// Start sets the job.Started timestamp, and job.State to Running.
func (j *Job) Start() *Job {
	now := time.Now()
	j.Started = &now
	j.State = Running
	return j
}

// Complete sets the job.Ended timestamp, and job.State to Completed.
func (j *Job) Complete() *Job {
	now := time.Now()
	j.Ended = &now
	j.State = Completed
	return j
}

// Fail sets the job.Ended timestamp, job.State to Failed,
// and adds the provided errors to job.Errors
func (j *Job) Fail(errs ...error) *Job {
	now := time.Now()
	j.Ended = &now
	j.State = Failed
	j.Errors = append(j.Errors, errs...)
	return j
}
