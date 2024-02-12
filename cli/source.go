package cli

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
)

// determineSources figures out what the active source is
// from any combination of stdin, flags or cfg. It will
// mutate ru.Config.Collection as necessary. If requireActive
// is true, an error is returned if there's no active source.
func determineSources(ctx context.Context, ru *run.Run, requireActive bool) error {
	cmd, coll := ru.Cmd, ru.Config.Collection
	activeSrc, err := activeSrcAndSchemaFromFlagsOrConfig(ru)
	if err != nil {
		return err
	}
	// Note: ^ activeSrc could still be nil

	// check if there's input on stdin
	stdinSrc, err := checkStdinSource(ctx, ru)
	if err != nil {
		return err
	}

	if stdinSrc != nil {
		// We have a valid source on stdin.

		// Append the stdin source to coll.
		err = coll.Add(stdinSrc)
		if err != nil {
			return err
		}

		if !cmdFlagChanged(cmd, flag.ActiveSrc) {
			// If the user has not explicitly set an active
			// source via flag, then we set the stdin pipe data
			// source as the active source.
			// We do this because the @stdin src is commonly the
			// only data source the user cares about in a pipe
			// situation.
			_, err = coll.SetActive(stdinSrc.Handle, false)
			if err != nil {
				return err
			}
			activeSrc = stdinSrc
		}
	}

	if activeSrc == nil && requireActive {
		return errz.New(msgNoActiveSrc)
	}

	return nil
}

// activeSrcAndSchemaFromFlagsOrConfig gets the active source, either
// from [flag.ActiveSrc] or from srcs.Active. An error is returned
// if the flag src is not found: if the flag src is found,
// it is set as the active src on coll. If the flag was not
// set and there is no active src in coll, (nil, nil) is
// returned.
//
// This source also checks [flag.ActiveSchema], and changes the catalog/schema
// of the source if the flag is set.
//
// See also: processFlagActiveSchema, verifySourceCatalogSchema.
func activeSrcAndSchemaFromFlagsOrConfig(ru *run.Run) (*source.Source, error) {
	cmd, coll := ru.Cmd, ru.Config.Collection
	var activeSrc *source.Source

	if cmdFlagChanged(cmd, flag.ActiveSrc) {
		// The user explicitly wants to set an active source
		// just for this query.

		handle, _ := cmd.Flags().GetString(flag.ActiveSrc)
		s, err := coll.Get(handle)
		if err != nil {
			return nil, errz.Wrapf(err, "flag --%s", flag.ActiveSrc)
		}

		activeSrc, err = coll.SetActive(s.Handle, false)
		if err != nil {
			return nil, err
		}
	} else {
		activeSrc = coll.Active()
	}

	srcModified, err := processFlagActiveSchema(cmd, activeSrc)
	if err != nil {
		return nil, err
	}

	if srcModified {
		if err = verifySourceCatalogSchema(ru.Cmd.Context(), ru, activeSrc); err != nil {
			return nil, err
		}
	}

	return activeSrc, nil
}

// verifySourceCatalogSchema verifies that src's non-empty [source.Source.Catalog]
// and [source.Source.Schema] are valid, in that they are referenceable in src's
// DB. If both fields are empty, this is a no-op. This function is typically
// used when the source's catalog or schema are modified, e.g. via [flag.ActiveSchema].
//
// See also: processFlagActiveSchema.
func verifySourceCatalogSchema(ctx context.Context, ru *run.Run, src *source.Source) error {
	if src.Catalog == "" && src.Schema == "" {
		return nil
	}

	db, drvr, err := ru.DB(ctx, src)
	if err != nil {
		return err
	}

	var exists bool
	if src.Catalog != "" {
		if exists, err = drvr.CatalogExists(ctx, db, src.Catalog); err != nil {
			return err
		}
		if !exists {
			return errz.Errorf("%s: catalog {%s} doesn't exist or not referenceable", src.Handle, src.Catalog)
		}
	}

	if src.Schema != "" {
		if exists, err = drvr.SchemaExists(ctx, db, src.Schema); err != nil {
			return err
		}
		if !exists {
			return errz.Errorf("%s: schema {%s} doesn't exist or not referenceable", src.Handle, src.Schema)
		}
	}

	return nil
}

// getCmdSource gets the source specified in args[0], or if args is empty, the
// active source, and calls applySourceOptions. If [flag.ActiveSchema] is set,
// the [source.Source.Catalog] and [source.Source.Schema] fields are configured
// (and validated) as appropriate.
//
// See: applySourceOptions, processFlagActiveSchema, verifySourceCatalogSchema.
func getCmdSource(cmd *cobra.Command, args []string) (*source.Source, error) {
	ru := run.FromContext(cmd.Context())

	var src *source.Source
	var err error
	if len(args) == 0 {
		if src = ru.Config.Collection.Active(); src == nil {
			return nil, errz.New("no active source")
		}
	} else {
		src, err = ru.Config.Collection.Get(args[0])
		if err != nil {
			return nil, err
		}
	}

	if !cmdFlagChanged(cmd, flag.ActiveSchema) {
		return src, nil
	}

	// Handle flag.ActiveSchema (--src.schema=CATALOG.SCHEMA). This func may
	// mutate src's Catalog and Schema fields if appropriate.
	var srcModified bool
	if srcModified, err = processFlagActiveSchema(cmd, src); err != nil {
		return nil, err
	}

	if err = applySourceOptions(cmd, src); err != nil {
		return nil, err
	}

	if srcModified {
		if err = verifySourceCatalogSchema(cmd.Context(), ru, src); err != nil {
			return nil, err
		}
	}

	return src, nil
}

// processFlagActiveSchema processes the --src.schema flag, setting appropriate
// [source.Source.Catalog] and [source.Source.Schema] values on activeSrc. If
// the src is modified by this function, modified returns true. If
// [flag.ActiveSchema] is not set, this is no-op. If activeSrc is nil, an error
// is returned.
//
// See also: verifySourceCatalogSchema.
func processFlagActiveSchema(cmd *cobra.Command, activeSrc *source.Source) (modified bool, err error) {
	ru := run.FromContext(cmd.Context())
	if !cmdFlagChanged(cmd, flag.ActiveSchema) {
		// Nothing to do here
		return false, nil
	}
	if activeSrc == nil {
		return false, errz.Errorf("active catalog/schema specified via --%s, but active source is nil",
			flag.ActiveSchema)
	}

	val, _ := cmd.Flags().GetString(flag.ActiveSchema)
	if val = strings.TrimSpace(val); val == "" {
		return false, errz.Errorf("active catalog/schema specified via --%s, but schema is empty",
			flag.ActiveSchema)
	}

	catalog, schema, err := ast.ParseCatalogSchema(val)
	if err != nil {
		return false, errz.Wrapf(err, "invalid active schema specified via --%s",
			flag.ActiveSchema)
	}

	if catalog != activeSrc.Catalog || schema != activeSrc.Schema {
		modified = true
	}

	drvr, err := ru.DriverRegistry.SQLDriverFor(activeSrc.Type)
	if err != nil {
		return false, err
	}

	if catalog != "" {
		if !drvr.Dialect().Catalog {
			return false, errz.Errorf("driver {%s} does not support catalog", activeSrc.Type)
		}

		activeSrc.Catalog = catalog
	}

	if schema != "" {
		activeSrc.Schema = schema
	}

	return modified, nil
}

// checkStdinSource checks if there's stdin data (on pipe/redirect).
// If there is, that pipe is inspected, and if it has recognizable
// input, a new source instance with handle @stdin is constructed
// and returned. If the pipe has no data (size is zero),
// then (nil,nil) is returned.
func checkStdinSource(ctx context.Context, ru *run.Run) (*source.Source, error) {
	f := ru.Stdin
	fi, err := f.Stat()
	if err != nil {
		return nil, errz.Wrap(err, "failed to get stat on stdin")
	}

	mode := fi.Mode()
	log := lg.FromContext(ctx).With(lga.File, fi.Name(), lga.Size, fi.Size(), "mode", mode.String())
	switch {
	case os.ModeNamedPipe&mode > 0:
		log.Info("Detected stdin pipe via os.ModeNamedPipe")
	case fi.Size() > 0:
		log.Info("Detected stdin redirect via size > 0")
	default:
		log.Info("No stdin data detected")
		return nil, nil //nolint:nilnil
	}

	// If we got this far, we have input from pipe or redirect.

	typ := drivertype.None
	if ru.Cmd.Flags().Changed(flag.IngestDriver) {
		val, _ := ru.Cmd.Flags().GetString(flag.IngestDriver)
		typ = drivertype.Type(val)
		if ru.DriverRegistry.ProviderFor(typ) == nil {
			return nil, errz.Errorf("unknown driver type: %s", typ)
		}
	}

	if err = ru.Files.AddStdin(ctx, f); err != nil {
		return nil, err
	}

	if typ == drivertype.None {
		if typ, err = ru.Files.DetectStdinType(ctx); err != nil {
			return nil, err
		}
		if typ == drivertype.None {
			return nil, errz.New("unable to detect type of stdin: use flag --driver")
		}
	}

	return newSource(
		ctx,
		ru.DriverRegistry,
		typ,
		source.StdinHandle,
		source.StdinHandle,
		options.Options{},
	)
}

// newSource creates a new Source instance where the
// driver type is known. Opts may be nil.
func newSource(ctx context.Context, dp driver.Provider, typ drivertype.Type, handle, loc string,
	opts options.Options,
) (*source.Source, error) {
	log := lg.FromContext(ctx)

	if opts == nil {
		log.Debug("Create new data source",
			lga.Handle, handle,
			lga.Driver, typ,
			lga.Loc, location.Redact(loc),
		)
	} else {
		log.Debug("Create new data source with opts",
			lga.Handle, handle,
			lga.Driver, typ,
			lga.Loc, location.Redact(loc),
		)
	}

	err := source.ValidHandle(handle)
	if err != nil {
		return nil, err
	}

	drvr, err := dp.DriverFor(typ)
	if err != nil {
		return nil, err
	}

	src := &source.Source{Handle: handle, Location: loc, Type: typ, Options: opts}

	canonicalSrc, err := drvr.ValidateSource(src)
	if err != nil {
		return nil, err
	}
	return canonicalSrc, nil
}
