package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func newSLQCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slq",
		Short: "Execute SLQ query",
		// This command is hidden, because it is effectively the root cmd.
		Hidden:            true,
		Args:              cobra.MaximumNArgs(1),
		RunE:              execSLQ,
		ValidArgsFunction: completeSLQ,
	}

	addQueryCmdFlags(cmd)

	// Explicitly flagVersion because people like to do "sq --version"
	// as much as "sq version".
	cmd.Flags().Bool(flagVersion, false, flagVersionUsage)

	return cmd
}

func execSLQ(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	srcs := rc.Config.Sources

	// check if there's input on stdin
	src, err := checkStdinSource(cmd.Context(), rc)
	if err != nil {
		return err
	}

	if src != nil {
		// We have a valid source on stdin.

		// Add the source to the set.
		err = srcs.Add(src)
		if err != nil {
			return err
		}

		// Set the stdin pipe data source as the active source,
		// as it's commonly the only data source the user is acting upon.
		_, err = srcs.SetActive(src.Handle)
		if err != nil {
			return err
		}
	} else {
		// No source on stdin, so we're using the source set.
		src = srcs.Active()
		if src == nil {
			// TODO: Should sq be modified to support executing queries
			// 	even when there's no active data source. Probably.
			return errz.New(msgNoActiveSrc)
		}
	}

	if !cmdFlagChanged(cmd, flagInsert) {
		// The user didn't specify the --insert=@src.tbl flag,
		// so we just want to print the records.
		return execSLQPrint(cmd.Context(), rc)
	}

	// Instead of printing the records, they will be
	// written to another database
	insertTo, _ := cmd.Flags().GetString(flagInsert)
	if insertTo == "" {
		return errz.Errorf("invalid --%s value: empty", flagInsert)
	}

	destHandle, destTbl, err := source.ParseTableHandle(insertTo)
	if err != nil {
		return errz.Wrapf(err, "invalid --%s value", flagInsert)
	}

	if destTbl == "" {
		return errz.Errorf("invalid value for --%s: must be @HANDLE.TABLE", flagInsert)
	}

	destSrc, err := srcs.Get(destHandle)
	if err != nil {
		return err
	}

	return execSLQInsert(cmd.Context(), rc, destSrc, destTbl)
}

// execSQLInsert executes the SLQ and inserts resulting records
// into destTbl in destSrc.
func execSLQInsert(ctx context.Context, rc *RunContext, destSrc *source.Source, destTbl string) error {
	args, srcs, dbases := rc.Args, rc.Config.Sources, rc.databases
	slq, err := preprocessUserSLQ(ctx, rc, args)
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	destDB, err := dbases.Open(ctx, destSrc)
	if err != nil {
		return err
	}

	// Note: We don't need to worry about closing fromConn and
	// destConn because they are closed by databases.Close, which
	// is invoked by rc.Close, and rc is closed further up the
	// stack.

	inserter := libsq.NewDBWriter(
		rc.Log,
		destDB,
		destTbl,
		driver.Tuning.RecordChSize,
		libsq.DBWriterCreateTableIfNotExistsHook(destTbl),
	)

	qc := &libsq.QueryContext{
		Sources:      srcs,
		DBOpener:     rc.databases,
		JoinDBOpener: rc.databases,
		Args:         nil,
	}

	execErr := libsq.ExecuteSLQ(ctx, rc.Log, qc, slq, inserter)
	affected, waitErr := inserter.Wait() // Wait for the writer to finish processing
	if execErr != nil {
		return errz.Wrapf(execErr, "insert %s.%s failed", destSrc.Handle, destTbl)
	}

	if waitErr != nil {
		return errz.Wrapf(waitErr, "insert %s.%s failed", destSrc.Handle, destTbl)
	}

	fmt.Fprintf(rc.Out, stringz.Plu("Inserted %d row(s) into %s.%s\n", int(affected)), affected, destSrc.Handle, destTbl)
	return nil
}

// execSLQPrint executes the SLQ query, and prints output to writer.
func execSLQPrint(ctx context.Context, rc *RunContext) error {
	slq, err := preprocessUserSLQ(ctx, rc, rc.Args)
	if err != nil {
		return err
	}

	qc := &libsq.QueryContext{
		Sources:      rc.Config.Sources,
		DBOpener:     rc.databases,
		JoinDBOpener: rc.databases,
		Args:         nil,
	}

	recw := output.NewRecordWriterAdapter(rc.writers.recordw)
	execErr := libsq.ExecuteSLQ(ctx, rc.Log, qc, slq, recw)
	_, waitErr := recw.Wait()
	if execErr != nil {
		return execErr
	}

	return waitErr
}

// preprocessUserSLQ does a bit of validation and munging on the
// SLQ input (provided in args), returning the SLQ query. This
// function is something of a hangover from the early days of
// sq and may need to be rethought.
//
// 1. If there's piped input but no query args, the first table
// from the pipe source becomes the query. Invoked like this:
//
//	$ cat something.csv | sq
//
// The query effectively becomes:
//
//	$ cat something.csv | sq @stdin.data
//
// For non-monotable sources, the first table is used:
//
//	$ cat something.xlsx | sq @stdin.sheet1
//
// 2. If the query doesn't contain a source selector segment
// starting with @HANDLE, the active src handle is prepended
// to the query. This allows a query where the first selector
// segment is the table name.
//
//	$ sq '.person'  -->  $ sq '@active.person'
func preprocessUserSLQ(ctx context.Context, rc *RunContext, args []string) (string, error) {
	log, reg, dbases, srcs := rc.Log, rc.registry, rc.databases, rc.Config.Sources
	activeSrc := srcs.Active()

	if len(args) == 0 {
		// Special handling for the case where no args are supplied
		// but sq is receiving pipe input. Let's say the user does this:
		//
		//  $ cat something.csv | sq  # query becomes ".stdin.data"
		if activeSrc == nil {
			// Piped input would result in an active @stdin src. We don't
			// have that; we don't have any active src.
			return "", errz.New(msgEmptyQueryString)
		}

		if activeSrc.Handle != source.StdinHandle {
			// It's not piped input.
			return "", errz.New(msgEmptyQueryString)
		}

		// We know for sure that we've got pipe input
		drvr, err := reg.DriverFor(activeSrc.Type)
		if err != nil {
			return "", err
		}

		tblName := source.MonotableName

		if !drvr.DriverMetadata().Monotable {
			// This isn't a monotable src, so we can't
			// just select @stdin.data. Instead we'll select
			// the first table name, as found in the source meta.
			dbase, err := dbases.Open(ctx, activeSrc)
			if err != nil {
				return "", err
			}
			defer log.WarnIfCloseError(dbase)

			srcMeta, err := dbase.SourceMetadata(ctx)
			if err != nil {
				return "", err
			}

			if len(srcMeta.Tables) == 0 {
				return "", errz.New(msgSrcNoData)
			}

			tblName = srcMeta.Tables[0].Name
			if tblName == "" {
				return "", errz.New(msgSrcEmptyTableName)
			}

			log.Debug("Using first table name from document source metadata as table selector: ", tblName)
		}

		selector := source.StdinHandle + "." + tblName
		log.Debug("Added selector to argument-less piped query: ", selector)

		return selector, nil
	}

	// We have at least one query arg
	for i, arg := range args {
		args[i] = strings.TrimSpace(arg)
	}

	start := strings.TrimSpace(args[0])
	parts := strings.Split(start, " ")

	if parts[0][0] == '@' {
		// The query starts with a handle, e.g. sq '@my | .person'.
		// Let's perform some basic checks on it.

		// We split on . because both @my1.person and @my1 need to be checked.
		dsParts := strings.Split(parts[0], ".")

		handle := dsParts[0]
		if len(handle) < 2 {
			// handle name is too short
			return "", errz.Errorf("invalid data source: %q", handle)
		}

		// Check that the handle actual exists
		_, err := srcs.Get(handle)
		if err != nil {
			return "", err
		}

		// All is good, return the query.
		query := strings.Join(args, " ")
		return query, nil
	}

	// The query doesn't start with a handle selector; let's prepend
	// a handle selector segment.
	if activeSrc == nil {
		return "", errz.New("no data source provided, and no active data source")
	}

	query := strings.Join(args, " ")
	query = fmt.Sprintf("%s | %s", activeSrc.Handle, query)

	log.Debug("The query didn't start with @handle, so the active src was prepended: ", query)

	return query, nil
}

// addQueryCmdFlags sets the common flags for the slq/sql commands.
func addQueryCmdFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(flagOutput, flagOutputShort, "", flagOutputUsage)

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().BoolP(flagJSONA, flagJSONAShort, false, flagJSONAUsage)
	cmd.Flags().BoolP(flagJSONL, flagJSONLShort, false, flagJSONLUsage)
	cmd.Flags().BoolP(flagTable, flagTableShort, false, flagTableUsage)
	cmd.Flags().BoolP(flagXML, flagXMLShort, false, flagXMLUsage)
	cmd.Flags().BoolP(flagXLSX, flagXLSXShort, false, flagXLSXUsage)
	cmd.Flags().BoolP(flagCSV, flagCSVShort, false, flagCSVUsage)
	cmd.Flags().BoolP(flagTSV, flagTSVShort, false, flagTSVUsage)
	cmd.Flags().BoolP(flagRaw, flagRawShort, false, flagRawUsage)
	cmd.Flags().Bool(flagHTML, false, flagHTMLUsage)
	cmd.Flags().Bool(flagMarkdown, false, flagMarkdownUsage)

	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)
	cmd.Flags().BoolP(flagPretty, "", true, flagPrettyUsage)

	cmd.Flags().StringP(flagInsert, "", "", flagInsertUsage)
	_ = cmd.RegisterFlagCompletionFunc(flagInsert, (&handleTableCompleter{onlySQL: true, handleRequired: true}).complete)

	cmd.Flags().StringP(flagActiveSrc, "", "", flagActiveSrcUsage)
	_ = cmd.RegisterFlagCompletionFunc(flagActiveSrc, completeHandle(0))

	// The driver flag can be used if data is piped to sq over stdin
	cmd.Flags().StringP(flagDriver, "", "", flagQueryDriverUsage)
	_ = cmd.RegisterFlagCompletionFunc(flagDriver, completeDriverType)

	cmd.Flags().StringP(flagSrcOptions, "", "", flagQuerySrcOptionsUsage)
}
