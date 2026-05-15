// Package portsakila translates the SQLite sakila SQL files into a form
// that DuckDB can ingest.
package portsakila

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// topoOrder is the canonical creation/insertion order for sakila tables in
// the DuckDB port. It is a topological sort of the FK dependency graph after
// the store→staff cycle-breaker (fk_store_staff) has been removed:
//
//  1. actor      (no FKs)
//  2. language   (no FKs)
//  3. category   (no FKs)
//  4. country    (no FKs)
//  5. city       → country
//  6. address    → city
//  7. store      → address  (FK to staff stripped; back-edge of cycle)
//  8. staff      → store, address
//  9. customer   → store, address
//  10. film       → language
//  11. film_text  (no FKs in source)
//  12. film_actor → actor, film
//  13. film_category → category, film
//  14. inventory  → store, film
//  15. rental     → staff, inventory, customer
//  16. payment    → rental, customer, staff
//
// The store/staff cycle is unavoidable: both staff.store_id and
// store.manager_staff_id are NOT NULL and the seed data is genuinely
// cyclic (staff 1 lives in store 1; store 1 is managed by staff 1).
// DuckDB has no FK-deferral mechanism (DEFERRABLE INITIALLY DEFERRED
// is accepted but silently ignored; SET foreign_keys = off doesn't
// exist; ALTER TABLE ADD/DROP CONSTRAINT is unimplemented), so at
// least one of the two cycle FKs must be dropped at port time. We
// drop fk_store_staff (store→staff) because that lets us INSERT
// store rows before staff rows exist, which matches the natural
// "make container, fill container" data-loading flow.
//
// Result: 21 of 22 sakila FK constraints are preserved.
var topoOrder = []string{
	"actor", "language", "category", "country", "city", "address",
	"store", "staff", "customer", "film", "film_text",
	"film_actor", "film_category", "inventory", "rental", "payment",
}

// PortSchema translates the SQLite sakila CREATE TABLE script into a
// DuckDB-compatible form. It strips triggers and AUTOINCREMENT, replaces
// Firebird's BLOB SUB_TYPE TEXT with TEXT, strips ON DELETE/ON UPDATE
// action clauses (DuckDB only supports the default NO ACTION), drops the
// fk_store_staff cycle-breaker constraint, and reorders the CREATE TABLE
// blocks so every FK references a table that already exists at parse time.
func PortSchema(in string) (string, error) {
	out := stripTriggers(in)
	out = stripAutoincrement(out)
	out = stripFKActionClauses(out)
	out = stripCycleBreakerFK(out)
	out = replaceBlobSubTypeText(out)
	return reorderSchemaBlocks(out, topoOrder)
}

// PortSchemaWithoutFKs translates the SQLite sakila CREATE TABLE script
// into a DuckDB-compatible form WITHOUT any FK constraints. This variant
// exists for the sakila-whitespace fixture, which ALTERs actor and
// film_actor — DuckDB rejects ALTER on tables with dependent constraints,
// so we strip FKs entirely for that build path. The main sakila fixture
// uses PortSchema, which preserves 21 of 22 FKs.
func PortSchemaWithoutFKs(in string) (string, error) {
	out := stripTriggers(in)
	out = stripAutoincrement(out)
	out = stripAllFKConstraintLines(out)
	out = replaceBlobSubTypeText(out)
	return out, nil
}

// PortInsertData translates the SQLite sakila INSERT-data script into a
// DuckDB-compatible form. It applies the generic transforms and then
// reorders the per-table INSERT chunks so child rows always follow their
// parents, satisfying DuckDB's at-INSERT FK enforcement.
func PortInsertData(in string) (string, error) {
	out := stripTriggers(in)
	out = stripAutoincrement(out)
	out = replaceBlobSubTypeText(out)
	return reorderInsertChunks(out, topoOrder)
}

// PortDropObjects translates the SQLite sakila DROP-objects script into a
// DuckDB-compatible form. With FK constraints in place, parent tables can
// only be dropped after all their dependents — so we reorder DROP TABLE
// statements in the reverse of topoOrder.
func PortDropObjects(in string) (string, error) {
	out := stripTriggers(in)
	out = stripAutoincrement(out)
	out = replaceBlobSubTypeText(out)
	return reorderDropTables(out, topoOrder)
}

// Port translates a SQLite sakila SQL file into a DuckDB-compatible form
// using the generic transforms only (no reordering). This entrypoint is
// used for files that don't need topological ordering: the delete-data
// script (already in reverse-topological order in the source) and the
// whitespace-alter / whitespace-restore scripts (which target specific
// pre-named tables).
func Port(in string) (string, error) {
	out := stripTriggers(in)
	out = stripAutoincrement(out)
	out = stripCycleBreakerFK(out)
	out = replaceBlobSubTypeText(out)
	return out, nil
}

// stripTriggers removes "CREATE TRIGGER ... END;" blocks. Trigger bodies
// contain semicolons, so we tokenize on the keyword pair, not on ";".
func stripTriggers(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "CREATE TRIGGER")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		// Find the matching "END;" terminator. The canonical sakila uses one
		// of three slight variations.
		rest := s[i:]
		end := strings.Index(rest, "END\n;")
		if end < 0 {
			end = strings.Index(rest, "END ;")
		}
		if end < 0 {
			end = strings.Index(rest, "END;")
		}
		if end < 0 {
			// Malformed; emit the rest verbatim.
			b.WriteString(rest)
			return b.String()
		}
		afterEnd := end
		switch {
		case strings.HasPrefix(rest[end:], "END\n;"):
			afterEnd += len("END\n;")
		case strings.HasPrefix(rest[end:], "END ;"):
			afterEnd += len("END ;")
		case strings.HasPrefix(rest[end:], "END;"):
			afterEnd += len("END;")
		}
		s = rest[afterEnd:]
	}
}

// stripAutoincrement removes the AUTOINCREMENT keyword (with surrounding spaces).
func stripAutoincrement(s string) string {
	return strings.ReplaceAll(s, " AUTOINCREMENT", "")
}

// reFKActionClause matches "ON DELETE <action>" or "ON UPDATE <action>"
// clauses on a FK constraint. DuckDB rejects every action except NO ACTION
// (the default) and RESTRICT — CASCADE, SET NULL, and SET DEFAULT produce
// "FOREIGN KEY constraints cannot use CASCADE, SET NULL or SET DEFAULT"
// at parse time. Sakila uses ON UPDATE CASCADE and ON DELETE SET NULL
// widely, so we strip all action clauses; the remaining FKs default to
// NO ACTION, which is fine for sq's read-mostly fixtures (the sakila
// data never UPDATEs primary keys or DELETEs parents during tests).
var reFKActionClause = regexp.MustCompile(
	`\s+ON\s+(?:DELETE|UPDATE)\s+(?:NO\s+ACTION|CASCADE|SET\s+NULL|SET\s+DEFAULT|RESTRICT)`,
)

// stripFKActionClauses removes "ON DELETE/UPDATE <action>" clauses from
// FK constraint definitions. See reFKActionClause for the rationale.
func stripFKActionClauses(s string) string {
	return reFKActionClause.ReplaceAllString(s, "")
}

// reAnyFKConstraint matches any FK constraint line. Used by the
// without-FKs variant for the whitespace fixture.
var reAnyFKConstraint = regexp.MustCompile(
	`(?m)^\s*CONSTRAINT\s+\S+\s+FOREIGN\s+KEY[^\n]*\n?`,
)

// stripAllFKConstraintLines removes every FK constraint line. Used by
// PortSchemaWithoutFKs for the whitespace fixture, which can't tolerate
// dependent constraints on actor/film_actor during ALTER.
func stripAllFKConstraintLines(s string) string {
	out := reAnyFKConstraint.ReplaceAllString(s, "")
	return reTrailingCommaBeforeParen.ReplaceAllString(out, "\n)")
}

// reCycleBreakerFK matches the single FK constraint that has to be
// dropped to break the store↔staff cycle: store.manager_staff_id →
// staff.staff_id (fk_store_staff). Every other FK is preserved.
var reCycleBreakerFK = regexp.MustCompile(
	`(?m)^\s*CONSTRAINT\s+fk_store_staff\s+FOREIGN\s+KEY[^\n]*\n?`,
)

// stripCycleBreakerFK removes the fk_store_staff constraint line. See the
// topoOrder doc comment for why we strip exactly this one.
func stripCycleBreakerFK(s string) string {
	out := reCycleBreakerFK.ReplaceAllString(s, "")
	// After stripping, the previous column or constraint may now have a
	// trailing comma before the closing paren. Fix "  ,\n)" → "\n)".
	return reTrailingCommaBeforeParen.ReplaceAllString(out, "\n)")
}

// reTrailingCommaBeforeParen matches a trailing comma left behind after
// a FK constraint line is stripped from the last column before ")".
// Used by both stripCycleBreakerFK and stripAllFKConstraintLines.
var reTrailingCommaBeforeParen = regexp.MustCompile(`(?m),\s*\n\s*\)`)

// replaceBlobSubTypeText replaces the Firebird-heritage "BLOB SUB_TYPE TEXT"
// column type with plain TEXT, which is valid in DuckDB.
func replaceBlobSubTypeText(s string) string {
	return strings.ReplaceAll(s, "BLOB SUB_TYPE TEXT", "TEXT")
}

// reCreateTable matches the start of a CREATE TABLE statement and captures
// the table name. Used to partition a schema script into per-table blocks.
var reCreateTable = regexp.MustCompile(`(?m)^CREATE TABLE\s+(\w+)`)

// reorderSchemaBlocks partitions a CREATE-TABLE-bearing script into
// per-table blocks and re-emits them in the requested topological order.
// Anything before the first CREATE TABLE is preserved as the prologue;
// anything from the first CREATE VIEW onward is preserved as the
// epilogue. CREATE INDEX statements for a table stay with their table.
//
// A table's block runs from its CREATE TABLE position to the next
// CREATE TABLE (or to the first CREATE VIEW, whichever comes first).
// The leading "-- Table structure for table X" comment that the source
// places ahead of each CREATE TABLE ends up attached to the previous
// block — that's cosmetic; the SQL is valid.
func reorderSchemaBlocks(s string, order []string) (string, error) {
	boundary := len(s)
	if i := strings.Index(s, "CREATE VIEW"); i >= 0 {
		// Walk back to the start of the preceding comment block so the
		// "-- View structure for view X" header travels with the views.
		for j := i - 1; j >= 0; j-- {
			if s[j] == '\n' && j+1 < len(s) && s[j+1] != '-' {
				boundary = j + 1
				break
			}
		}
		// Fallback: if we never found a non-comment line, snap to the
		// CREATE VIEW position itself.
		if boundary == len(s) {
			boundary = i
		}
	}

	body := s[:boundary]
	epilogue := s[boundary:]

	positions := reCreateTable.FindAllStringSubmatchIndex(body, -1)
	if len(positions) == 0 {
		return s, nil
	}

	prologue := body[:positions[0][0]]

	blocks := make(map[string]string, len(positions))
	for i, idx := range positions {
		name := body[idx[2]:idx[3]]
		start := idx[0]
		end := len(body)
		if i+1 < len(positions) {
			end = positions[i+1][0]
		}
		blocks[name] = body[start:end]
	}

	// Verify every expected table is present.
	for _, name := range order {
		if _, ok := blocks[name]; !ok {
			return "", fmt.Errorf("schema is missing CREATE TABLE %s", name)
		}
	}
	if len(blocks) != len(order) {
		var extra []string
		for name := range blocks {
			if !slices.Contains(order, name) {
				extra = append(extra, name)
			}
		}
		return "", fmt.Errorf("schema has %d CREATE TABLE statements but topoOrder lists %d (extras: %v)",
			len(blocks), len(order), extra)
	}

	var b strings.Builder
	b.WriteString(prologue)
	for _, name := range order {
		b.WriteString(blocks[name])
	}
	b.WriteString(epilogue)
	return b.String(), nil
}

// reInsertInto matches the start of an INSERT statement and captures the
// target table name. The SQLite sakila dump uses the mixed-case form
// "Insert into <table>" (capital I, lowercase i+n+t+o); we match that
// shape explicitly.
var reInsertInto = regexp.MustCompile(`(?im)^Insert into\s+(\w+)`)

// reorderInsertChunks partitions an INSERT-bearing script into per-table
// chunks and re-emits them in the requested topological order. A chunk's
// start is the line of its first "Insert into <table>" statement; its end
// is the next chunk's start (or EOF). Content before the first INSERT
// (the prologue: header, BEGIN TRANSACTION, DELETE FROM ... statements)
// is preserved verbatim; content after the last INSERT chunk (the
// epilogue: COMMIT, footer) is preserved verbatim.
//
// Source data has no INSERTs for film_text (populated by triggers in
// the original SQLite schema, which we strip), so film_text is silently
// skipped if it doesn't appear in the input.
func reorderInsertChunks(s string, order []string) (string, error) {
	positions := reInsertInto.FindAllStringSubmatchIndex(s, -1)
	if len(positions) == 0 {
		return s, nil
	}

	// Build chunks: scan the positions, group consecutive INSERTs by
	// table. A new chunk starts when the table name changes.
	type chunk struct {
		name  string
		start int
		end   int
	}
	var chunks []chunk
	curName := s[positions[0][2]:positions[0][3]]
	curStart := positions[0][0]
	for i := 1; i < len(positions); i++ {
		name := s[positions[i][2]:positions[i][3]]
		if name != curName {
			chunks = append(chunks, chunk{name: curName, start: curStart, end: positions[i][0]})
			curName = name
			curStart = positions[i][0]
		}
	}
	// Final chunk provisionally extends to EOF; we trim it back below
	// once we detect the end-of-INSERTs marker.
	chunks = append(chunks, chunk{name: curName, start: curStart, end: len(s)})

	// Detect the end of the INSERT region: scan the final chunk for a
	// transaction-terminator marker (the SQLite sakila dump ends with
	// "END TRANSACTION;" on some files and "COMMIT" on others). Anything
	// from that marker onward becomes the epilogue. Without this, if
	// topoOrder is ever permuted, a non-payment-tail table would inherit
	// the trailer's content in its chunk.
	prologue := s[:chunks[0].start]
	lastChunkStart := chunks[len(chunks)-1].start
	lastChunkBody := s[lastChunkStart:]
	terminatorIdx := -1
	for _, marker := range []string{"\nEND TRANSACTION", "\nCOMMIT"} {
		if idx := strings.Index(lastChunkBody, marker); idx >= 0 {
			if terminatorIdx < 0 || idx < terminatorIdx {
				terminatorIdx = idx
			}
		}
	}
	var epilogue string
	if terminatorIdx >= 0 {
		termAbs := lastChunkStart + terminatorIdx + 1 // +1 to skip the leading \n
		chunks[len(chunks)-1].end = termAbs
		epilogue = s[termAbs:]
	} else {
		epilogue = s[chunks[len(chunks)-1].end:]
	}

	// Build the chunk map.
	chunkByName := make(map[string]string, len(chunks))
	for _, c := range chunks {
		chunkByName[c.name] = s[c.start:c.end]
	}

	// Verify chunks for each table that has data. film_text is allowed
	// to be missing (no INSERTs in source).
	for _, name := range order {
		if name == "film_text" {
			continue
		}
		if _, ok := chunkByName[name]; !ok {
			return "", fmt.Errorf("insert-data is missing INSERT statements for %s", name)
		}
	}
	for name := range chunkByName {
		if !slices.Contains(order, name) {
			return "", fmt.Errorf("insert-data has INSERT statements for unknown table %q", name)
		}
	}

	var b strings.Builder
	b.WriteString(prologue)
	for _, name := range order {
		if c, ok := chunkByName[name]; ok {
			b.WriteString(c)
		}
	}
	b.WriteString(epilogue)
	return b.String(), nil
}

// reDropTableLine matches a single line whose content (ignoring leading
// whitespace) is "DROP TABLE [IF EXISTS] <name>". The SQLite sakila source
// uses a two-line style — the statement-terminating ";" lives on its own
// next line — so this regex matches only the DROP line itself; the
// terminator is consumed by the line-walk below.
var reDropTableLine = regexp.MustCompile(`^[ \t]*DROP[ \t]+TABLE(?:[ \t]+IF[ \t]+EXISTS)?[ \t]+(\w+)[ \t]*$`)

// reorderDropTables reorders DROP TABLE statements within a drop-objects
// script so that dependent tables are dropped before their parents. The
// order is the REVERSE of order. DROP VIEW statements at the start of
// the script are preserved as-is (views don't have inter-view FKs).
//
// Lines are processed sequentially. When a "DROP TABLE X" line is found,
// the immediately-following ";" line (with surrounding blank-line
// tolerance) is captured as part of the same statement and the whole
// pair is stashed under X's name. Non-DROP-TABLE lines outside the
// captured pairs are preserved verbatim. The captured pairs are
// re-emitted in reverse topological order at the position of the first
// DROP TABLE.
func reorderDropTables(s string, order []string) (string, error) {
	lines := strings.Split(s, "\n")
	drops := make(map[string]string, len(order))

	var prologue strings.Builder
	var epilogue strings.Builder
	type phase int
	const (
		phasePrologue phase = iota
		phaseDrops
		phaseEpilogue
	)
	p := phasePrologue

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := reDropTableLine.FindStringSubmatch(line)
		if m == nil {
			switch p {
			case phasePrologue:
				prologue.WriteString(line)
				prologue.WriteString("\n")
			case phaseDrops, phaseEpilogue:
				// Once we've entered the drop-block, any non-DROP-TABLE
				// line goes to the epilogue. (The captured drops carry
				// their own terminator below.)
				p = phaseEpilogue
				epilogue.WriteString(line)
				epilogue.WriteString("\n")
			}
			continue
		}
		// We found "DROP TABLE X". Consume this line plus its terminator
		// line (the next ";" line) as a single drop-statement unit.
		switch p {
		case phasePrologue:
			p = phaseDrops
		case phaseDrops:
			// Already in the drops phase — continue accumulating.
		case phaseEpilogue:
			return "", fmt.Errorf("drop-objects: DROP TABLE %q appears after end of drop-block", m[1])
		}
		name := m[1]
		var b strings.Builder
		b.WriteString(line)
		b.WriteString("\n")
		// Consume following lines until we eat a ";" line. Bound the
		// look-ahead: a well-formed DROP statement places ";" on the
		// very next non-blank line; allowing more than a handful of
		// blank lines would mean the input is malformed.
		const maxLookAhead = 5
		terminated := false
		j := i + 1
		for j < len(lines) && j-i <= maxLookAhead {
			next := strings.TrimSpace(lines[j])
			b.WriteString(lines[j])
			b.WriteString("\n")
			if next == ";" {
				terminated = true
				break
			}
			j++
		}
		if !terminated {
			return "", fmt.Errorf("drop-objects: DROP TABLE %s missing ';' terminator within %d lines", name, maxLookAhead)
		}
		drops[name] = b.String()
		i = j
	}

	// Verify all expected tables are present.
	for _, name := range order {
		if _, ok := drops[name]; !ok {
			return "", fmt.Errorf("drop-objects is missing DROP TABLE %s", name)
		}
	}
	if len(drops) != len(order) {
		var extra []string
		for name := range drops {
			if !slices.Contains(order, name) {
				extra = append(extra, name)
			}
		}
		return "", fmt.Errorf("drop-objects has %d DROP TABLE statements but topoOrder lists %d (extras: %v)",
			len(drops), len(order), extra)
	}

	// Re-emit prologue + drops (reverse topological) + epilogue.
	// Strip the trailing newlines we appended to prologue/epilogue when
	// the source didn't actually end with one (Split("…", "\n") yields a
	// trailing empty element if the input ended with "\n", which we
	// preserved as a blank line).
	prologueText := prologue.String()
	epilogueText := epilogue.String()
	// Re-balance: the original input had a final "\n" or not; we always
	// appended one to every line written. Trim exactly one trailing
	// newline from each block if the corresponding source region didn't
	// end with one. Simpler: trust the round-trip and accept that
	// every output line ends with "\n".
	var b strings.Builder
	b.WriteString(prologueText)
	for i := len(order) - 1; i >= 0; i-- {
		b.WriteString(drops[order[i]])
	}
	b.WriteString(epilogueText)
	out := b.String()
	// If the original didn't end with a newline, trim the one we added.
	if !strings.HasSuffix(s, "\n") && strings.HasSuffix(out, "\n") {
		out = out[:len(out)-1]
	}
	return out, nil
}
