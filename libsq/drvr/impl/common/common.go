// Package common contains shared driver implementation functionality.
package common

import (
	"strings"

	"strconv"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/util"
)

// OptionsHasHeader checks if src.Options has "header=true".
func OptionsHasHeader(src *drvr.Source) (header bool, ok bool, err error) {

	if len(src.Options) == 0 {
		return false, false, nil
	}

	const key = "header"
	if _, ok = src.Options[key]; !ok {
		return false, false, nil
	}
	val := src.Options.Get(key)
	if val == "" {
		return false, false, nil
	}

	header, err = strconv.ParseBool(val)
	if err != nil {
		return false, false, util.Errorf(`option %q: %v`, key, err)
	}

	return header, true, nil
}

// OptionsColNames returns column names as specified in src.Options["cols"].
func OptionsColNames(src *drvr.Source) (colNames []string, ok bool, err error) {
	if len(src.Options) == 0 {
		return nil, false, nil
	}

	const key = "cols"
	val := ""
	_, ok = src.Options[key]
	if !ok {
		return nil, false, nil
	}

	val = strings.TrimSpace(src.Options.Get(key))
	colNames = strings.Split(val, ",")
	if val == "" || len(colNames) == 0 {
		err = util.Errorf("option %q: cannot be empty", key)
		return nil, false, err
	}

	for i := range colNames {
		colNames[i] = strings.TrimSpace(colNames[i])
		if colNames[i] == "" {
			err = util.Errorf("option %q: column [%d] cannot be empty", key, i)
			return nil, false, err
		}
	}

	lg.Debugf("option %q: %v", key, colNames)
	return colNames, true, nil
}
