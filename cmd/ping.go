package cmd

import (
	"fmt"

	"sync"
	"time"

	"strconv"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/cmd/out"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/engine"
	"github.com/neilotoole/sq/libsq/util"
	"github.com/spf13/cobra"
)

// pingCmd represents the ping command
var pingCmd = &cobra.Command{
	Use: "ping [@HANDLE]",
	Example: `  # ping active data source
  sq ping

  # ping all data sources
  sq ping --all

  # ping @my1 data source
  sq ping @my1`,
	Short: "Check data source connection health",
	Long: `Ping data source to check connection health. If no arguments provided, the
active data source is pinged.`,
	RunE: execPing,
}

func init() {
	preprocessCmd(pingCmd)
	pingCmd.Flags().BoolP(FlagPingAll, FlagPingAllShort, false, FlagPingAllUsage)
	RootCmd.AddCommand(pingCmd)

}

func execPing(cmd *cobra.Command, args []string) error {

	if len(args) > 1 {
		return util.Errorf("invalid arguments")
	}

	var srcs []*drvr.Source

	if cmd.Flags().Changed(FlagPingAll) {
		srcs = cfg.SourceSet.Items
	} else {
		var err error
		var src *drvr.Source
		if len(args) == 0 {
			ok := false
			src, ok = cfg.SourceSet.Active()
			if !ok {
				return util.Errorf("can't get active data source")
			}
		} else {

			src, err = cfg.SourceSet.Get(args[0])
			if err != nil {
				return err
			}
		}

		srcs = []*drvr.Source{src}
	}

	lg.Debugf("got srcs: %d", len(srcs))
	doPing(srcs, cfg.Options.Timeout)
	return nil
}

func doPing(srcs []*drvr.Source, timeout time.Duration) {

	//timeout := 5
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	// maxLen is max length of the datasource name
	maxNameLen := 0
	unfinishedSrcs := hashset.New()
	for _, src := range srcs {
		unfinishedSrcs.Add(src)
		if len(src.Handle) > maxNameLen {
			maxNameLen = len(src.Handle)
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
		src := val.(*drvr.Source)

		out.Color.Number.Printf("%-"+strconv.Itoa(maxNameLen)+"s", src.Handle)
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
		// TODO: move to using getWriter() mechanism

	}
}

func doPingOne(src *drvr.Source, maxNameLen int, unfinishedSrcs *hashset.Set, mu *sync.Mutex, wg *sync.WaitGroup) {
	lg.Debugf("starting...")
	defer wg.Done()
	start := time.Now()

	var err error
	var database *engine.Database
	database, err = engine.NewDatabase(src)
	if err == nil {
		err = database.Ping()
	}

	finish := time.Now()
	duration := finish.Sub(start)

	mu.Lock()
	defer mu.Unlock()

	unfinishedSrcs.Remove(src)

	out.Color.Number.Printf("%-"+strconv.Itoa(maxNameLen)+"s", src.Handle)

	fmt.Printf(" %4dms    ", duration/time.Millisecond)

	if err != nil {

		out.Color.Error.Printf("no pong!")
		fmt.Printf(" %s", err)
		fmt.Printf("\n")
		return
	}

	out.Color.Success.Printf("pong!")
	fmt.Printf("\n")
	// TODO: move to using getWriter() mechanism

}
