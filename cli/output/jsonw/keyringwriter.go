package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
)

var _ output.KeyringWriter = (*keyringWriter)(nil)

// keyringWriter implements output.KeyringWriter for JSON.
type keyringWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewKeyringWriter returns a JSON output.KeyringWriter.
func NewKeyringWriter(out io.Writer, pr *output.Printing) output.KeyringWriter {
	return &keyringWriter{out: out, pr: pr}
}

// List implements output.KeyringWriter. Always emits a JSON array,
// even for the empty case (so callers that pipe to jq see "[]" rather
// than nothing).
func (w *keyringWriter) List(refs []output.KeyringRef) error {
	if refs == nil {
		refs = []output.KeyringRef{}
	}
	return writeJSON(w.out, w.pr, refs)
}

// Get implements output.KeyringWriter.
func (w *keyringWriter) Get(path, value string, revealed bool) error {
	type record struct {
		Path   string `json:"path"`
		Value  string `json:"value,omitempty"`
		Exists bool   `json:"exists"`
	}
	r := record{Path: path, Exists: true}
	if revealed {
		r.Value = value
	}
	return writeJSON(w.out, w.pr, r)
}

// Created implements output.KeyringWriter.
func (w *keyringWriter) Created(path string) error {
	return writeJSON(w.out, w.pr, map[string]any{
		"path":    path,
		"created": true,
	})
}

// Updated implements output.KeyringWriter.
func (w *keyringWriter) Updated(path string) error {
	return writeJSON(w.out, w.pr, map[string]any{
		"path":    path,
		"updated": true,
	})
}

// Rm implements output.KeyringWriter.
func (w *keyringWriter) Rm(path string) error {
	return writeJSON(w.out, w.pr, map[string]any{
		"path":    path,
		"deleted": true,
	})
}

// Migrate implements output.KeyringWriter. Emits a single JSON object
// with the dry-run flag and the row array so a consumer can tell the
// modes apart from the JSON alone.
func (w *keyringWriter) Migrate(rows []output.KeyringMigrateRow, dryRun bool) error {
	if rows == nil {
		rows = []output.KeyringMigrateRow{}
	}
	type envelope struct {
		Rows   []output.KeyringMigrateRow `json:"rows"`
		DryRun bool                       `json:"dry_run"`
	}
	return writeJSON(w.out, w.pr, envelope{DryRun: dryRun, Rows: rows})
}
