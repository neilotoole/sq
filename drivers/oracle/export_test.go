package oracle

// Symbols exported for use by the external oracle_test package.

// GetSourceMetadata exposes getSourceMetadata so that external tests can cover
// the noSchema=true branch, which the exported grip.SourceMetadata path does
// not reach.
var GetSourceMetadata = getSourceMetadata
