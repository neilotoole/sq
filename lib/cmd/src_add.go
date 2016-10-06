package cmd

import (
	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/out/table"
	"github.com/neilotoole/sq/lib/util"
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var srcAddCmd = &cobra.Command{
	Use:   "add URL NAME",
	RunE:  execSrcAdd,
	Short: "Add a data source",
	Long: `sq add URL NAME

Add the data source specified by the URL, and identified by NAME.

The format of URL is:

    DRIVER://CONNECTION_STRING

Available drivers:

    MySQL
    Postgres
    SQLite3
    Excel (.xlsx)
    Oracle          [BROKEN]


The format of CONNECTION_STRING is driver-dependent. See "More" below for
details for each supported driver.

Examples:

    # Add a MySQL data source
    sq add "mysql://user:pass@tcp(localhost:3306)/mydb1" mydb1
    # Add a Postgres data source
    sq add "postgres://pqgotest:password@localhost/pqgotest" pgdb1
    # SQLite3 datasource
    sq add 'sqlite3:///Users/neilotoole/testdata/sqlite1.db' sl1
    # Add Excel files, both local and remote (HTTP)
    sq add /Users/neilotoole/testdata/test.xlsx xl1
    sq add https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx xl2


More:

Details of driver connection strings are linked below.

    MySQL:     https://github.com/go-sql-driver/mysql
    Postgres:  https://godoc.org/github.com/lib/pq
`,
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
		return util.Errorf("sorry, the NAME argument is currently required, we'll fix that soon")
	}

	if len(args) != 2 {
		return util.Errorf("invalid arguments")
	}

	url := args[0]
	name := args[1]

	if name[0] == '@' {
		return util.Errorf("alias may not being with '@'")
	}

	cfg := config.Default()

	i, _ := cfg.SourceSet.IndexOf(name)
	if i != -1 {
		return util.Errorf("data source already exists")
	}

	src, err := driver.NewSource(name, url)
	if err != nil {
		return err
	}

	err = cfg.SourceSet.Add(src)
	if err != nil {
		return err
	}
	if len(cfg.SourceSet.Items) == 1 {
		// If this is the first DS, make it current
		cfg.SourceSet.SetActive(src.Ref)
	}
	//
	//
	//existingIndex := -1
	//
	//for i, s := range cfg.Sources {
	//	if s.Alias == alias {
	//		existingIndex = i
	//		break
	//	}
	//}
	//
	//if existingIndex >= 0 {
	//	cfg.Sources[existingIndex] = src
	//} else {
	//	cfg.Sources = append(cfg.Sources, src)
	//}
	//
	err = cfg.Save()
	if err != nil {
		return err
	}
	//
	w := table.NewWriter(true)
	w.Source(src)
	return nil
}
