package cmd

import (
	"fmt"

	"sync"
	"time"

	"strconv"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/out"
	"github.com/neilotoole/sq/lib/ql"
	"github.com/spf13/cobra"
)

// pingCmd represents the ping command
var pingCmd = &cobra.Command{
	Use:   "ping [DATASOURCE]",
	Short: "Ping data source",
	Long: `Ping the datasource to verify that the connection is healthy. If DATASOURCE is
not specified, the currently active data source is pinged.

Examples:
    # ping the active data source
    sq ping
    # ping @mydb1
    sq ping @mydb1
    sq ping --all
`,
	RunE: execPing,
}

func init() {
	preprocessCmd(pingCmd)
	pingCmd.Flags().BoolP(FlagPingAll, FlagPingAllShort, false, FlagPingAllUsage)
	RootCmd.AddCommand(pingCmd)

}

func execPing(cmd *cobra.Command, args []string) error {

	lg.Debugf("starting")

	if len(args) > 1 {
		return fmt.Errorf("invalid arguments")
	}

	var srcs []*driver.Source

	if cmd.Flags().Changed(FlagPingAll) {
		srcs = config.Default().SourceSet.Items
	} else {
		var err error
		var src *driver.Source
		if len(args) == 0 {
			ok := false
			src, ok = config.Default().SourceSet.Active()
			if !ok {
				return fmt.Errorf("can't get active data source")
			}
		} else {

			src, err = config.Default().SourceSet.Get(args[0])
			if err != nil {
				return err
			}
		}

		srcs = []*driver.Source{src}
	}

	lg.Debugf("got srcs: %d", len(srcs))
	doPing(srcs)
	return nil
}

func doPing(srcs []*driver.Source) {

	//timeout := 5
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	// maxLen is max length of the datasource name
	maxNameLen := 0
	unfinishedSrcs := hashset.New()
	for _, src := range srcs {
		unfinishedSrcs.Add(src)
		if len(src.Ref) > maxNameLen {
			maxNameLen = len(src.Ref)
		}
	}

	wg.Add(len(srcs))
	for _, src := range srcs {

		go doPingOne(src, maxNameLen, unfinishedSrcs, mu, wg)

	}

	//wg.Wait()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timeout := config.Default().Options.Timeout
	lg.Debugf("using ping timeout: %s", timeout)

	select {
	case <-done:
	// All done!
	case <-time.After(timeout):
		// Hit timeout.
		//fmt.Printf("hit the timeout yo\n")
	}

	//fmt.Printf("Num unfinished: %d\n", unfinishedSrcs.Size())
	for _, val := range unfinishedSrcs.Values() {
		src := val.(*driver.Source)

		out.Color.Number.Printf("%-"+strconv.Itoa(maxNameLen)+"s", src.Ref)
		//color.Set(out.Attrs.Number)
		//fmt.Printf("%-"+strconv.Itoa(maxNameLen)+"s", src.Ref)
		//color.Unset()
		fmt.Printf("      -    ")

		out.Color.Error.Printf("no pong!")

		//color.Set(out.Attrs.Error)
		//fmt.Printf("no pong!")
		//color.Unset()
		fmt.Printf(" exceeded timeout of %s", timeout)
		fmt.Printf("\n")
	}
}

func doPingOne(src *driver.Source, maxNameLen int, unfinishedSrcs *hashset.Set, mu *sync.Mutex, wg *sync.WaitGroup) {
	lg.Debugf("starting...")
	defer wg.Done()
	start := time.Now()

	var err error
	var database *ql.Database
	database, err = ql.NewDatabase(src)
	if err == nil {
		err = database.Ping()
	}

	finish := time.Now()
	duration := finish.Sub(start)

	mu.Lock()
	defer mu.Unlock()

	unfinishedSrcs.Remove(src)

	out.Color.Number.Printf("%-"+strconv.Itoa(maxNameLen)+"s", src.Ref)

	fmt.Printf(" %4dms    ", duration/time.Millisecond)

	if err != nil {

		out.Color.Error.Printf("no pong!")
		fmt.Printf(" %s", err)
		fmt.Printf("\n")
		return
	}

	out.Color.Success.Printf("pong!")
	fmt.Printf("\n")
}
