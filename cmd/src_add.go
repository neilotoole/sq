package cmd

import (
	"strings"

	"net/url"

	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/util"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var srcAddCmd = &cobra.Command{
	Use: "add LOCATION @HANDLE",
	Example: `  sq add 'mysql://user:pass@tcp(localhost:3306)/mydb1' @my1
  sq add 'postgres://user:pass@localhost/pgdb1?sslmode=disable' @pg1
  sq add '/testdata/sqlite1.db' @sl1 --driver=sqlite3
  sq add /testdata/test1.xlsx @excel1 --opts='header=true'
  sq add http://neilotoole.io/sq/test/test1.xlsx @excel2
  sq add /testdata/user_comma_header.csv @csv1 --opts='header=true'`,
	Short: "Add data source",
	Long: `Add data source specified by LOCATION and identified by @HANDLE.  The
format of LOCATION varies, but is generally a DB connection string, a file path,
or a URL.

  DRIVER://CONNECTION_STRING
  /path/to/local/file.suf
  https://neilotoole.io/data/test1.xlsx

If LOCATION is a file path or URL, sq will attempt to determine the driver type
from the file suffix or URL Content-Type. If the result is ambiguous, you
must specify the driver via the --driver=X flag.

Available drivers:

  mysql        MySQL
  postgres     Postgres
  sqlite3      SQLite3
  xlsx         Microsoft Excel XLSX
  csv          Comma-Separated Values
  tsv          Tab-Separated Values

Additional help: http://neilotoole.io/sq`,
	RunE: execSrcAdd,
}

func init() {
	preprocessCmd(srcAddCmd)

	srcAddCmd.Flags().StringP(FlagDriver, "", "", FlagDriverUsage)
	srcAddCmd.Flags().StringP(FlagSrcAddOptions, "", "", FlagSrcAddOptionsUsage)
	RootCmd.AddCommand(srcAddCmd)

	// TODO: add flag --active to immediately set active
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

func execSrcAdd(cmd *cobra.Command, args []string) error {

	if len(args) == 1 {
		return util.Errorf("sorry, the @HANDLE argument is currently required, we'll fix that soon")
	}

	if len(args) != 2 {
		return util.Errorf("invalid arguments")
	}

	location := strings.TrimSpace(args[0])
	handle := strings.TrimSpace(args[1])

	err := drvr.CheckHandleValue(handle)
	if err != nil {
		return err
	}

	i, _ := cfg.SourceSet.IndexOf(handle)
	if i != -1 {
		return util.Errorf("data source handle %q already exists", handle)
	}

	driverName := ""

	if cmd.Flags().Changed(FlagDriver) {
		driverName, _ = cmd.Flags().GetString(FlagDriver)
	}

	var opts url.Values
	if cmd.Flags().Changed(FlagSrcAddOptions) {
		val, _ := cmd.Flags().GetString(FlagSrcAddOptions)

		val = strings.TrimSpace(val)

		if val != "" {
			vals, err := url.ParseQuery(val)
			if err != nil {
				return util.Errorf("unable to parse options string (should be in URL-encoded query format): %v", err)
			}
			opts = vals
		}
	}

	src, err := drvr.AddSource(handle, location, driverName, opts)
	if err != nil {
		return err
	}

	err = cfg.SourceSet.Add(src)
	if err != nil {
		return err
	}
	if len(cfg.SourceSet.Items) == 1 {
		// If this is the first DS, make it current
		cfg.SourceSet.SetActive(src.Handle)
	}

	err = saveConfig()
	if err != nil {
		return err
	}

	w := getWriter(cmd)
	//w := table.NewWriter(true)
	w.Source(src)
	return nil
}
