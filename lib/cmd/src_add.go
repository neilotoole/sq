package cmd

import (
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/out/table"
	"github.com/neilotoole/sq/lib/util"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var srcAddCmd = &cobra.Command{
	Use: "add LOCATION @HANDLE",
	Example: `  sq add 'mysql://user:pass@tcp(localhost:3306)/mydb1' @my1
  sq add 'postgres://user:pass@localhost/pgdb1?sslmode=disable' @pg1
  sq add 'sqlite3:///Users/neilotoole/testdata/sqlite1.db' @sl1
  sq add /Users/neilotoole/testdata/test.xlsx @excel1
  sq add http://neilotoole.io/sq/test/test1.xlsx @excel2`,
	Short: "Add data source",
	Long: `sq add LOCATION @HANDLE

Add the data source specified by LOCATION and identified by @HANDLE. The
format of LOCATION varies, but is generally of the form:

    DRIVER://CONNECTION_STRING

Available drivers:

    MySQL
    Postgres
    SQLite3
    Excel (.xlsx)
    Oracle          [BROKEN]

The format of CONNECTION_STRING is driver-dependent. See the manual for more
details: http://neilotoole.io/sq`,
	RunE: execSrcAdd,
}

func init() {
	preprocessCmd(srcAddCmd)
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
		return util.Errorf("sorry, the HANDLE argument is currently required, we'll fix that soon")
	}

	if len(args) != 2 {
		return util.Errorf("invalid arguments")
	}

	location := args[0]
	handle := args[1]

	err := driver.CheckHandleValue(handle)
	if err != nil {
		return err
	}

	//cfg := config.Default()

	//srcs := cfg.Sources()

	i, _ := cfg.SourceSet.IndexOf(handle)
	if i != -1 {
		return util.Errorf("data source already exists")
	}

	src, err := driver.NewSource(handle, location)
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

	//cfg.SourceSet = *srcs

	err = cfg.Save()
	if err != nil {
		return err
	}
	//
	w := table.NewWriter(true)
	w.Source(src)
	return nil
}
