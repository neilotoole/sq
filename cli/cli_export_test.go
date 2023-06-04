package cli

// This file exports package constructs for testing.

import (
	"testing"

	"github.com/neilotoole/sq/cli/run"

	"github.com/neilotoole/slogt"
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

// ToTestParseLocStage is a helper to test the
// non-exported locCompletionHelper.parseLoc method.
func DoTestParseLocStage(t testing.TB, ru *run.Run, loc string) (PlocStage, error) { //nolint:revive
	lch := &locCompleteHelper{
		ru:  ru,
		log: slogt.New(t),
	}

	ploc, err := lch.parseLoc(loc)
	if err != nil {
		return PlocInit, err
	}

	return ploc.stageDone, nil
}
