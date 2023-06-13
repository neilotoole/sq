package cli

// This file exports package constructs for testing.

import (
	"testing"

	"github.com/neilotoole/sq/cli/run"
)

type PlocStage = plocStage

const (
	PlocInit     = plocInit
	PlocScheme   = plocScheme
	PlocUser     = plocUser
	PlocPass     = plocPass
	PlocHostname = plocHostname
	PlocHost     = plocHost
	PlocPath     = plocPath
)

var DoCompleteAddLocationFile = locCompListFiles

// ToTestParseLocStage is a helper to test the
// non-exported locCompletionHelper.locCompParseLoc method.
func DoTestParseLocStage(t testing.TB, ru *run.Run, loc string) (PlocStage, error) { //nolint:revive
	ploc, err := locCompParseLoc(loc)
	if err != nil {
		return PlocInit, err
	}

	return ploc.stageDone, nil
}
