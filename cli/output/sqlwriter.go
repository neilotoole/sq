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

	// Source is the handle of the source that SQL targets, e.g.
	// "@sakila_pg". For cross-source queries this is the join DB's
	// handle.
	Source string `json:"source" yaml:"source"`

	// Multi is true if more than one source is involved in the SLQ. If
	// executed (not via --dry-run), data would be staged into the join
	// DB before the final SQL ran. --dry-run itself does no staging.
	Multi bool `json:"multi" yaml:"multi"`
}

// SQLWriter writes an SQLPayload in some format-specific
// representation: plain SQL text, a JSON/YAML object, etc. Used by
// the slq command's --dry-run path.
type SQLWriter interface {
	// Render writes p to the writer's output. It is expected to be
	// called exactly once per invocation.
	Render(p SQLPayload) error
}
