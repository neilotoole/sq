package oracle

// Symbols exported for use by the external oracle_test package.
//
// Integration tests live in the external oracle_test package so they can use
// the testh harness. Because testh imports this driver, a test in package
// oracle cannot import testh (that would be an import cycle); the few tests
// that need unexported symbols therefore stay internal, and any internal
// behavior that also needs a live DB is reached from the external package
// through the seam below.

// GetSourceMetadata exposes getSourceMetadata so that external tests can cover
// the noSchema=true branch, which the exported grip.SourceMetadata path does
// not reach.
var GetSourceMetadata = getSourceMetadata
