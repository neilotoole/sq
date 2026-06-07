package cli

// This file exports package constructs for testing.

var (
	DoCompleteAddLocationFile = locCompListFiles
	PreprocessFlagArgVars     = preprocessFlagArgVars
	LastHandlePart            = lastHandlePart
	GetVersionFromBrewFormula = getVersionFromBrewFormula
	FetchBrewVersion          = fetchBrewVersion
	RenderSQLSupportsFormat   = renderSQLSupportsFormat
	ErrBinaryFormatToTerminal = errBinaryFormatToTerminal
)

// B15: the legacy parsedLoc/plocStage parser was removed in B14
// (rewrite of completeAddLocation on top of LocationShape walker).
// DoTestParseLocStage and the PlocStage type alias / Ploc* constants
// formerly defined here are deleted along with TestParseLoc_stage.
// Walker stage equivalents are exercised by libsq/driver TestWalk.
