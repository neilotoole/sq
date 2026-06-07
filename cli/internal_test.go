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

// The legacy parsedLoc/plocStage parser was removed when
// completeAddLocation was rewritten on top of the LocationShape
// walker. The DoTestParseLocStage helper and the PlocStage type
// alias / Ploc* constants that formerly lived here are gone along
// with TestParseLoc_stage. Walker stage equivalents are exercised
// by TestWalk in libsq/driver/locshape_test.go.
