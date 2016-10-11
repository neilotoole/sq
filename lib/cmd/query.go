package cmd

import (
	"fmt"

	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/out"
	"github.com/neilotoole/sq/lib/out/csv"
	"github.com/neilotoole/sq/lib/out/json"
	"github.com/neilotoole/sq/lib/out/raw"
	"github.com/neilotoole/sq/lib/out/table"
	"github.com/neilotoole/sq/lib/out/xlsx"
	"github.com/neilotoole/sq/lib/out/xml"
	"github.com/neilotoole/sq/lib/sq"
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
	setQueryCmdOptions(queryCmd)
	queryCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		RootCmd.Help()
		//fmt.Println("help command")
		return nil
	})
	RootCmd.AddCommand(queryCmd)

}

func execQuery(cmd *cobra.Command, args []string) error {

	if len(args) == 0 {
		return fmt.Errorf("no arguments provided")
	}

	sq.SetSourceSet(cfg.SourceSet)

	for i, arg := range args {
		args[i] = strings.TrimSpace(arg)
	}
	qry := strings.Join(args, " ")

	writer := getResultWriter(cmd)

	if getQueryMode(cmd) == config.ModeSQ {

		lg.Debugf("using sq mode")
		_, sqQuery, err := getSQQueryWithDatasource(args)
		if err != nil {
			return err
		}

		err = sq.ExecuteSQ(sqQuery, writer)
		return err

	}

	lg.Debugf("using database native mode")
	// else it's a traditional database-native SQL query
	src, ok := cfg.SourceSet.Active()
	if !ok || src == nil {
		return fmt.Errorf("no active datasource")
	}

	database, err := sq.NewDatabase(src)
	if err != nil {
		return err
	}
	err = database.ExecuteAndWrite(qry, writer)
	return err
}

func getSQQueryWithDatasource(args []string) (*drvr.Source, string, error) {

	start := strings.TrimSpace(args[0])
	parts := strings.Split(start, " ")

	if parts[0][0] == '@' {
		// the args already start with a datasource
		dsParts := strings.Split(parts[0], ".")

		dsName := dsParts[0]
		if len(dsName) < 2 {
			// DS name is too short
			return nil, "", fmt.Errorf("invalid data source: %q", dsName)
		}

		// strip the leading @
		dsName = dsName[1:]

		ds, err := cfg.SourceSet.Get(dsName)
		if err != nil {
			return nil, "", err
		}

		// we now know the DS to use
		q := strings.Join(args, " ")
		return ds, q, nil
	}

	// no datasource provided as part of the args, use the active source
	src, ok := cfg.SourceSet.Active()
	if !ok {
		return nil, "", fmt.Errorf("no datasource provided")
	}

	q := strings.Join(args, " ")
	q = fmt.Sprintf("%s | %s", src.Handle, q)

	return src, q, nil
}

func setQueryCmdOptions(cmd *cobra.Command) {

	setQueryOutputOptions(cmd)

	cmd.Flags().BoolP(FlagModeSQ, FlagModeSQShort, false, FlagModeSQUsage)
	cmd.Flags().BoolP(FlagModeNativeSQL, FlagModeNativeSQLShort, false, FlagModeNativeSQLUsage)
	cmd.Flags().BoolP(FlagRaw, FlagRawShort, false, FlagRawUsage)

}

func setQueryOutputOptions(cmd *cobra.Command) {
	cmd.Flags().BoolP(FlagJSON, FlagJSONShort, false, FlagJSONUsage)
	cmd.Flags().BoolP(FlagTable, FlagTableShort, false, FlagTableUsage)
	cmd.Flags().BoolP(FlagXML, FlagXMLShort, false, FlagXMLUsage)
	cmd.Flags().BoolP(FlagXLSX, FlagXLSXShort, false, FlagXLSXUsage)
	cmd.Flags().BoolP(FlagCSV, FlagCSVShort, false, FlagCSVUsage)
	cmd.Flags().BoolP(FlagTSV, FlagTSVShort, false, FlagTSVUsage)
	cmd.Flags().BoolP(FlagHeader, FlagHeaderShort, false, FlagHeaderUsage)
	cmd.Flags().BoolP(FlagNoHeader, FlagNoHeaderShort, false, FlagNoHeaderUsage)
	cmd.Flags().BoolP(FlagMonochrome, FlagMonochromeShort, false, FlagMonochromeUsage)
}

func getQueryMode(cmd *cobra.Command) config.QueryMode {

	mode := cfg.Options.QueryMode

	if mode != config.ModeSQ && mode != config.ModeNativeSQL {
		mode = config.ModeSQ
	}

	if cmd.Flags().Changed(FlagModeNativeSQL) {
		mode = config.ModeNativeSQL
	}

	if cmd.Flags().Changed(FlagModeSQ) {
		mode = config.ModeSQ
	}

	return mode
}

func getResultWriter(cmd *cobra.Command) out.ResultWriter {

	hasHeader := cfg.Options.Header

	if cmd.Flags().Changed(FlagHeader) {
		hasHeader = true
	}
	if cmd.Flags().Changed(FlagNoHeader) {
		hasHeader = false
	}

	format := cfg.Options.Format

	if cmd.Flags().Changed(FlagTSV) {
		format = config.FormatTSV
	}

	if cmd.Flags().Changed(FlagCSV) {
		format = config.FormatCSV
	}

	if cmd.Flags().Changed(FlagXLSX) {
		format = config.FormatXLSX
	}
	if cmd.Flags().Changed(FlagXML) {
		format = config.FormatXML
	}

	if cmd.Flags().Changed(FlagRaw) {
		format = config.FormatRaw
	}

	if cmd.Flags().Changed(FlagTable) {
		format = config.FormatTable
	}

	if cmd.Flags().Changed(FlagJSON) {
		format = config.FormatJSON
	}

	switch format {
	case config.FormatTSV:
		return csv.NewWriter(hasHeader, '\t')
	case config.FormatCSV:
		return csv.NewWriter(hasHeader, ',')
	case config.FormatXML:
		return xml.NewWriter()
	case config.FormatXLSX:
		return xlsx.NewWriter(hasHeader)
	case config.FormatRaw:
		return raw.NewWriter()
	case config.FormatTable:
		return table.NewWriter(hasHeader)
	}

	return json.NewWriter()

}
