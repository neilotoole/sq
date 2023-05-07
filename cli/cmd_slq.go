package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/drivers/csv"
	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

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
		RunE:              execSLQ,
		ValidArgsFunction: completeSLQ,
	}

	addQueryCmdFlags(cmd)

	cmd.Flags().StringArray(flag.Arg, nil, flag.ArgUsage)

	// Explicitly add flagVersion because people like to do "sq --version"
	// as much as "sq version".
	cmd.Flags().Bool(flag.Version, false, flag.VersionUsage)

	return cmd
}

// execSLQ is sq's core command.
func execSLQ(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		msg := "no query"
		if cmdFlagChanged(cmd, flag.Arg) {
			msg += fmt.Sprintf(": maybe check flag --%s usage", flag.Arg)
		}

		return errz.New(msg)
	}

	ctx := cmd.Context()
	rc := RunContextFrom(ctx)
	coll := rc.Config.Collection

	// check if there's input on stdin
	src, err := checkStdinSource(ctx, rc)
	if err != nil {
		return err
	}

	if src != nil {
		// We have a valid source on stdin.

		// Add the source to the set.
		if err = coll.Add(src); err != nil {
			return err
		}

		// Collection the stdin pipe data source as the active source,
		// as it's commonly the only data source the user is acting upon.
		if _, err = coll.SetActive(src.Handle, false); err != nil {
			return err
		}
	} else {
		// No source on stdin, so we're using the collection.
		src = coll.Active()
		if src == nil {
			// TODO: Should sq be modified to support executing queries
			// 	even when there's no active data source. Probably.
			return errz.New(msgNoActiveSrc)
		}
	}

	mArgs, err := extractFlagArgsValues(cmd)
	if err != nil {
		return err
	}

	if err = applyCollectionOptions(cmd, rc.Config.Collection); err != nil {
		return err
	}

	if !cmdFlagChanged(cmd, flag.Insert) {
		// The user didn't specify the --insert=@src.tbl flag,
		// so we just want to print the records.
		return execSLQPrint(ctx, rc, mArgs)
	}

	// Instead of printing the records, they will be
	// written to another database
	insertTo, _ := cmd.Flags().GetString(flag.Insert)
	if insertTo == "" {
		return errz.Errorf("invalid --%s value: empty", flag.Insert)
	}

	destHandle, destTbl, err := source.ParseTableHandle(insertTo)
	if err != nil {
		return errz.Wrapf(err, "invalid --%s value", flag.Insert)
	}

	if destTbl == "" {
		return errz.Errorf("invalid value for --%s: must be @HANDLE.TABLE", flag.Insert)
	}

	destSrc, err := coll.Get(destHandle)
	if err != nil {
		return err
	}

	return execSLQInsert(ctx, rc, mArgs, destSrc, destTbl)
}

// execSQLInsert executes the SLQ and inserts resulting records
// into destTbl in destSrc.
func execSLQInsert(ctx context.Context, rc *RunContext, mArgs map[string]string,
	destSrc *source.Source, destTbl string,
) error {
	args, coll, dbases := rc.Args, rc.Config.Collection, rc.databases

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
		destDB,
		destTbl,
		driver.OptTuningRecChanSize.Get(destSrc.Options),
		libsq.DBWriterCreateTableIfNotExistsHook(destTbl),
	)

	qc := &libsq.QueryContext{
		Collection:   coll,
		DBOpener:     rc.databases,
		JoinDBOpener: rc.databases,
		Args:         mArgs,
	}

	execErr := libsq.ExecuteSLQ(ctx, qc, slq, inserter)
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
func execSLQPrint(ctx context.Context, rc *RunContext, mArgs map[string]string) error {
	slq, err := preprocessUserSLQ(ctx, rc, rc.Args)
	if err != nil {
		return err
	}

	qc := &libsq.QueryContext{
		Collection:   rc.Config.Collection,
		DBOpener:     rc.databases,
		JoinDBOpener: rc.databases,
		Args:         mArgs,
	}

	recw := output.NewRecordWriterAdapter(rc.writers.recordw)
	execErr := libsq.ExecuteSLQ(ctx, qc, slq, recw)
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
	log, reg, dbases, coll := lg.FromContext(ctx), rc.driverReg, rc.databases, rc.Config.Collection
	activeSrc := coll.Active()

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
			defer lg.WarnIfCloseError(log, lgm.CloseDB, dbase)

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
			return "", errz.Errorf("invalid data source: %s", handle)
		}

		// Check that the handle actual exists
		_, err := coll.Get(handle)
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

	log.Debug("The query didn't start with @handle, so the active src was prepended",
		lga.Query, query)

	return query, nil
}

// addQueryCmdFlags sets the common flags for the slq/sql commands.
func addQueryCmdFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.JSONA, flag.JSONAShort, false, flag.JSONAUsage)
	cmd.Flags().BoolP(flag.JSONL, flag.JSONLShort, false, flag.JSONLUsage)
	cmd.Flags().BoolP(flag.CSV, flag.CSVShort, false, flag.CSVUsage)
	cmd.Flags().BoolP(flag.TSV, flag.TSVShort, false, flag.TSVUsage)
	cmd.Flags().Bool(flag.HTML, false, flag.HTMLUsage)
	cmd.Flags().Bool(flag.Markdown, false, flag.MarkdownUsage)
	cmd.Flags().BoolP(flag.Raw, flag.RawShort, false, flag.RawUsage)
	cmd.Flags().BoolP(flag.XLSX, flag.XLSXShort, false, flag.XLSXUsage)
	cmd.Flags().BoolP(flag.XML, flag.XMLShort, false, flag.XMLUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)

	addTimeFormatOptsFlags(cmd)

	cmd.Flags().StringP(flag.Output, flag.OutputShort, "", flag.OutputUsage)

	cmd.Flags().String(flag.Insert, "", flag.InsertUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.Insert,
		(&handleTableCompleter{onlySQL: true, handleRequired: true}).complete))

	cmd.Flags().String(flag.ActiveSrc, "", flag.ActiveSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ActiveSrc, completeHandle(0)))

	// The driver flag can be used if data is piped to sq over stdin
	cmd.Flags().String(flag.IngestDriver, "", flag.IngestDriverUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.IngestDriver, completeDriverType))

	cmd.Flags().Bool(flag.IngestHeader, false, flag.IngestHeaderUsage)
	cmd.Flags().Bool(flag.CSVEmptyAsNull, true, flag.CSVEmptyAsNullUsage)
	cmd.Flags().String(flag.CSVDelim, flag.CSVDelimDefault, flag.CSVDelimUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.CSVDelim, completeStrings(-1, csv.NamedDelims()...)))
}

// extractFlagArgsValues returns a map {key:value} of predefined variables
// as supplied via --arg. For example:
//
//	sq --arg name TOM '.actor | .first_name == $name'
//
// See preprocessFlagArgVars.
func extractFlagArgsValues(cmd *cobra.Command) (map[string]string, error) {
	if !cmdFlagChanged(cmd, flag.Arg) {
		return nil, nil //nolint:nilnil
	}

	arr, err := cmd.Flags().GetStringArray(flag.Arg)
	if err != nil {
		return nil, errz.Err(err)
	}

	if len(arr) == 0 {
		return nil, nil //nolint:nilnil
	}

	mArgs := map[string]string{}
	for _, kv := range arr {
		k, v, ok := strings.Cut(kv, ":")
		if !ok || k == "" {
			return nil, errz.Errorf("invalid --%s value", flag.Arg)
		}

		if _, ok := mArgs[k]; ok {
			// If the key already exists, don't overwrite. This mimics jq's
			// behavior.

			log := logFrom(cmd)
			log.With("arg", k).Warn("Double use of --arg key; using first value.")

			continue
		}

		mArgs[k] = v
	}

	return mArgs, nil
}

// preprocessFlagArgVars is a hack to support the predefined
// variables "--arg" mechanism. We implement the mechanism in alignment
// with how jq does it: "--arg name value".
// See: https://stedolan.github.io/jq/manual/v1.6/
//
// For example:
//
//	sq --arg first TOM --arg last MIRANDA '.actor | .first_name == $first && .last_name == $last'
//
// However, cobra (or rather, pflag) doesn't support this type of flag input.
// So, we have a hack. In the example above, the two elements "first" and "TOM"
// are concatenated into a single flag value "first:TOM". Thus, the returned
// slice will be shorter.
//
// This function needs to be called before cobra/pflag starts processing
// the program args.
//
// Any code making use of flagArg will need to deconstruct the flag value.
// Specifically, that means extractFlagArgsValues.
func preprocessFlagArgVars(args []string) ([]string, error) {
	const flg = "--" + flag.Arg

	if len(args) == 0 {
		return args, nil
	}

	if !slices.Contains(args, flg) {
		return args, nil
	}

	rez := make([]string, 0, len(args))

	var i int
	for i = 0; i < len(args); {
		if args[i] == flg {
			val, err := extractFlagArgsSingleArg(args[i:])
			if err != nil {
				return nil, err
			}
			rez = append(rez, flg)
			rez = append(rez, val)
			i += 3
			continue
		}

		rez = append(rez, args[i])
		i++
	}

	return rez, nil
}

// args will look like ["--arg", "key", "value", "--other-flag"].
// The function will return "key:value".
// See preprocessFlagArgVars.
func extractFlagArgsSingleArg(args []string) (string, error) {
	if len(args) < 3 {
		return "", errz.Errorf("invalid %s flag: must be '--%s key value'", flag.Arg, flag.Arg)
	}

	if err := stringz.ValidIdent(args[1]); err != nil {
		return "", errz.Errorf("invalid --%s key: %s", flag.Arg, args[1])
	}

	return args[1] + ":" + args[2], nil
}
