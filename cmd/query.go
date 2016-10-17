package cmd

import (
	"fmt"

	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/cmd/config"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/util"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:    "query",
	Short:  "",
	RunE:   execQuery,
	Hidden: true,
}

func init() {
	preprocessCmd(queryCmd)
	addQueryCmdFlags(queryCmd)
	queryCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		RootCmd.Help()
		return nil
	})
	RootCmd.AddCommand(queryCmd)
}

func execQuery(cmd *cobra.Command, args []string) error {

	if len(args) == 0 {
		return fmt.Errorf("no arguments provided")
	}

	for i, arg := range args {
		args[i] = strings.TrimSpace(arg)
	}
	qry := strings.Join(args, " ")

	writer := getWriter(cmd)

	if getQueryMode(cmd) == config.ModeSLQ {

		lg.Debugf("using sq mode")
		_, sqQuery, err := processUserQuery(args)
		if err != nil {
			return err
		}
		//err = engine.ExecuteSQ(sqQuery, writer)
		err = libsq.Execute(*cfg.SourceSet, sqQuery, writer)
		return err
	}

	lg.Debugf("using database native SQL mode")
	// else it's a traditional database-native SQL query
	src, ok := cfg.SourceSet.Active()
	if !ok || src == nil {
		return util.Errorf("no active data source")
	}

	return libsq.ExecuteSQL(*src, qry, writer)
}

// processUserQuery does a bit of validation and munging on the SLQ input. If the query
// doesn't contain a @HANDLE, the active src handle is prepended to the query.
// Otherwise the function performs some basic sanity checking on SLQ input. On success
// the function returns the first referenced src, and the (potentially modified) SLQ string.
func processUserQuery(args []string) (src *drvr.Source, slq string, err error) {
	start := strings.TrimSpace(args[0])
	parts := strings.Split(start, " ")

	if parts[0][0] == '@' {
		// the args already start with a datasource
		dsParts := strings.Split(parts[0], ".")

		dsName := dsParts[0]
		if len(dsName) < 2 {
			// DS name is too short
			return nil, "", util.Errorf("invalid data source: %q", dsName)
		}

		// strip the leading @
		dsName = dsName[1:]

		src, err = cfg.SourceSet.Get(dsName)
		if err != nil {
			return nil, "", err
		}

		// we now know the DS to use
		slq = strings.Join(args, " ")
		return src, slq, nil
	}

	// no datasource provided as part of the args, use the active source
	src, ok := cfg.SourceSet.Active()
	if !ok {
		return nil, "", util.Errorf("no data source provided, and no active data source")
	}

	q := strings.Join(args, " ")
	slq = fmt.Sprintf("%s | %s", src.Handle, q)

	return src, slq, nil
}

// addQueryCmdFlags sets all the flags on the query command.
func addQueryCmdFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP(FlagModeSLQ, FlagModeSLQShort, false, FlagModeSLQUsage)
	cmd.Flags().BoolP(FlagModeNativeSQL, FlagModeNativeSQLShort, false, FlagModeNativeSQLUsage)

	cmd.Flags().BoolP(FlagJSON, FlagJSONShort, false, FlagJSONUsage)
	cmd.Flags().BoolP(FlagTable, FlagTableShort, false, FlagTableUsage)
	cmd.Flags().BoolP(FlagXML, FlagXMLShort, false, FlagXMLUsage)
	cmd.Flags().BoolP(FlagXLSX, FlagXLSXShort, false, FlagXLSXUsage)
	cmd.Flags().BoolP(FlagCSV, FlagCSVShort, false, FlagCSVUsage)
	cmd.Flags().BoolP(FlagTSV, FlagTSVShort, false, FlagTSVUsage)
	cmd.Flags().BoolP(FlagRaw, FlagRawShort, false, FlagRawUsage)

	cmd.Flags().BoolP(FlagHeader, FlagHeaderShort, false, FlagHeaderUsage)
	cmd.Flags().BoolP(FlagNoHeader, FlagNoHeaderShort, false, FlagNoHeaderUsage)
	cmd.Flags().BoolP(FlagMonochrome, FlagMonochromeShort, false, FlagMonochromeUsage)
}

func getQueryMode(cmd *cobra.Command) config.QueryMode {
	mode := cfg.Options.QueryMode
	if mode != config.ModeSLQ && mode != config.ModeNativeSQL {
		mode = config.ModeSLQ
	}

	if cmd.Flags().Changed(FlagModeNativeSQL) {
		mode = config.ModeNativeSQL
	}
	if cmd.Flags().Changed(FlagModeSLQ) {
		mode = config.ModeSLQ
	}

	return mode
}
