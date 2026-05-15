package output

// SQLPayload is the structured form of a dry-run result: enough
// information to either eyeball the SQL or round-trip it back into a
// future render. Consumed by SQLWriter implementations and produced by
// the slq command's --dry-run path.
type SQLPayload struct {
	// Args is the --arg map supplied to the query, if any.
	// Values are already substituted into SQL; Args is reported so
	// consumers can re-render later with different values.
	Args map[string]string `json:"args,omitempty" yaml:"args,omitempty"`

	// SLQ is the input SLQ query (post-preprocessing).
	SLQ string `json:"slq" yaml:"slq"`

	// SQL is the rendered SQL query that would be executed.
	SQL string `json:"sql" yaml:"sql"`

	// Dialect is the lower-case dialect / driver-type name
	// (e.g. "postgres", "sqlite3", "mysql") that SQL was rendered for.
	Dialect string `json:"dialect" yaml:"dialect"`

	// Sources describes the source handles involved in the query:
	// the SQL's execution target plus the user-named inputs the SLQ
	// references. The two coincide for single-source queries; for
	// cross-source queries Target is the synthetic join DB and
	// Inputs lists the user sources whose data would be staged into
	// it. --dry-run does no staging.
	Sources SQLSources `json:"sources" yaml:"sources"`
}

// SQLSources groups the source handles involved in a --dry-run
// payload, distinguishing where the rendered SQL would execute
// (Target) from the user-named inputs the SLQ references (Inputs).
type SQLSources struct {
	// Target is the handle of the source the rendered SQL would be
	// executed against. For single-source queries this is the user
	// source itself. For cross-source queries it is the synthetic
	// join DB's handle (typically "@join_…"), distinct from any of
	// the inputs in Inputs.
	Target string `json:"target" yaml:"target"`

	// Inputs is the set of input source handles referenced by the
	// SLQ. For single-source queries it has one element equal to
	// Target. For cross-source queries it lists the user sources
	// whose data would be staged into the join DB named by Target.
	Inputs []string `json:"inputs" yaml:"inputs"`
}

// SQLWriter writes an SQLPayload in some format-specific
// representation: plain SQL text, a JSON/YAML object, etc. Used by
// the slq command's --dry-run path.
type SQLWriter interface {
	// Render writes p to the writer's output. It is expected to be
	// called exactly once per invocation.
	Render(p SQLPayload) error
}
