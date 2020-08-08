package cli

import (
	"strings"

	"github.com/neilotoole/lg"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/options"
	"github.com/neilotoole/sq/libsq/source"
)

// determineSources figures out what the active source is
// from any combination of stdin, flags or cfg. It will
// mutate rc.Config.Sources as necessary. If no error
// is returned, it is guaranteed that there's an active
// source on the source set.
func determineSources(rc *RunContext) error {
	cmd, srcs := rc.Cmd, rc.Config.Sources
	activeSrc, err := activeSrcFromFlagsOrConfig(cmd, srcs)
	if err != nil {
		return err
	}
	// Note: ^ activeSrc could still be nil

	// check if there's input on stdin
	stdinSrc, err := checkStdinSource(rc)
	if err != nil {
		return err
	}

	if stdinSrc != nil {
		// We have a valid source on stdin.

		// Add the stdin source to the set.
		err = srcs.Add(stdinSrc)
		if err != nil {
			return err
		}

		if !cmdFlagChanged(cmd, flagActiveSrc) {
			// If the user has not explicitly set an active
			// source via flag, then we set the stdin pipe data
			// source as the active source.
			// We do this because the @stdin src is commonly the
			// only data source the user cares about in a pipe
			// situation.
			_, err = srcs.SetActive(stdinSrc.Handle)
			if err != nil {
				return err
			}
			activeSrc = stdinSrc
		}
	}

	if activeSrc == nil {
		return errz.New(msgNoActiveSrc)
	}

	return nil
}

// activeSrcFromFlagsOrConfig gets the active source, either
// from flagActiveSrc or from srcs.Active. An error is returned
// if the flag src is not found: if the flag src is found,
// it is set as the active src on srcs. If the flag was not
// set and there is no active src in srcs, (nil, nil) is
// returned.
func activeSrcFromFlagsOrConfig(cmd *cobra.Command, srcs *source.Set) (*source.Source, error) {
	var activeSrc *source.Source

	if cmdFlagChanged(cmd, flagActiveSrc) {
		// The user explicitly wants to set an active source
		// just for this query.

		handle, _ := cmd.Flags().GetString(flagActiveSrc)
		s, err := srcs.Get(handle)
		if err != nil {
			return nil, errz.Wrapf(err, "flag --%s", flagActiveSrc)
		}

		activeSrc, err = srcs.SetActive(s.Handle)
		if err != nil {
			return nil, err
		}
	} else {
		activeSrc = srcs.Active()
	}
	return activeSrc, nil
}

// checkStdinSource checks if there's stdin data (on pipe/redirect).
// If there is, that pipe is inspected, and if it has recognizable
// input, a new source instance with handle @stdin is constructed
// and returned.
func checkStdinSource(rc *RunContext) (*source.Source, error) {
	cmd := rc.Cmd

	f := rc.Stdin
	info, err := f.Stat()
	if err != nil {
		return nil, errz.Wrap(err, "failed to get stat on stdin")
	}

	if info.Size() <= 0 {
		// Doesn't make sense to have zero-data pipe? just ignore.
		return nil, nil
	}

	// If we got this far, we have pipe input

	// It's possible the user supplied source options
	var opts options.Options
	if cmd.Flags().Changed(flagSrcOptions) {
		val, _ := cmd.Flags().GetString(flagSrcOptions)
		val = strings.TrimSpace(val)

		if val != "" {
			opts, err = options.ParseOptions(val)
			if err != nil {
				return nil, err
			}
		}
	}

	typ := source.TypeNone
	if cmd.Flags().Changed(flagDriver) {
		val, _ := cmd.Flags().GetString(flagDriver)
		typ = source.Type(val)
		if !rc.registry.HasProviderFor(typ) {
			return nil, errz.Errorf("unknown driver type: %s", typ)
		}
	}

	err = rc.files.AddStdin(f)
	if err != nil {
		return nil, err
	}

	if typ == source.TypeNone {
		typ, err = rc.files.TypeStdin(rc.Context)
		if err != nil {
			return nil, err
		}
		if typ == source.TypeNone {
			return nil, errz.New("unable to detect type of stdin: use flag --driver")
		}
	}

	return newSource(rc.Log, rc.registry, typ, source.StdinHandle, source.StdinHandle, opts)
}

// newSource creates a new Source instance where the
// driver type is known. Opts may be nil.
func newSource(log lg.Log, dp driver.Provider, typ source.Type, handle, location string, opts options.Options) (*source.Source, error) {
	if opts == nil {
		log.Debugf("create new data source %q [%s] from %q",
			handle, typ, location)
	} else {
		log.Debugf("create new data source %q [%s] from %q with opts %s",
			handle, typ, location, opts.Encode())
	}

	err := source.VerifyLegalHandle(handle)
	if err != nil {
		return nil, err
	}

	drvr, err := dp.DriverFor(typ)
	if err != nil {
		return nil, err
	}

	src := &source.Source{Handle: handle, Location: location, Type: typ, Options: opts}

	log.Debugf("validating provisional new data source: %q", src)
	canonicalSrc, err := drvr.ValidateSource(src)
	if err != nil {
		return nil, err
	}
	return canonicalSrc, nil
}
