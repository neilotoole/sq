package cli

import (
	"bytes"
	"os"
	"strings"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/shelleditor"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/errz"
)

var editorEnvs = []string{"SQ_EDITOR", "EDITOR"}

func newConfigEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "edit [@HANDLE]",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHandle(1),
		RunE:              execConfigEdit,
		Short:             "Edit config or source options",
		Long:              `Edit config or source options in the editor specified in envar $SQ_EDITOR or $EDITOR.`,
		Example: `  # Edit default options
  $ sq config edit

  # Edit config for source @sakila
  $ sq config edit @sakila

  # Use a different editor
  $ SQ_EDITOR=nano sq config edit`,
	}

	return cmd
}

func execConfigEdit(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return execConfigEditOptions(cmd, args)
	}

	return execConfigEditSource(cmd, args)
}

func execConfigEditOptions(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	rc, log := RunContextFrom(ctx), logFrom(cmd)
	cfg := rc.Config

	before, err := ioz.MarshalYAML(cfg.Options)
	if err != nil {
		return err
	}

	ed := shelleditor.NewDefaultEditor(editorEnvs...)
	after, tmpFile, err := ed.LaunchTempFile("sq", ".yml", bytes.NewReader(before))
	if tmpFile != "" {
		defer func() {
			lg.WarnIfError(log, "Delete editor temp file", errz.Err(os.Remove(tmpFile)))
		}()
	}
	if err != nil {
		return errz.Wrap(err, "edit config")
	}

	if bytes.Equal(before, after) {
		log.Debug("Edit config: no changes made")
		return nil
	}

	opts := options.Options{}
	if err = ioz.UnmarshallYAML(after, &opts); err != nil {
		return err
	}

	// TODO: if --verbose, show diff
	cfg.Options = opts
	if err = rc.ConfigStore.Save(ctx, cfg); err != nil {
		return err
	}

	log.Debug("Edit config: changes saved", lga.Path, rc.ConfigStore.Location())
	return nil
}

func execConfigEditSource(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	rc, log := RunContextFrom(ctx), logFrom(cmd)
	cfg := rc.Config

	src, err := cfg.Collection.Get(args[0])
	if err != nil {
		return err
	}

	before, err := ioz.MarshalYAML(src)
	if err != nil {
		return err
	}

	ed := shelleditor.NewDefaultEditor(editorEnvs...)
	fname := strings.ReplaceAll(src.Handle[1:], "/", "__")
	after, tmpFile, err := ed.LaunchTempFile(fname, ".yml", bytes.NewReader(before))
	if tmpFile != "" {
		defer func() {
			lg.WarnIfError(log, "Delete editor temp file", errz.Err(os.Remove(tmpFile)))
		}()
	}
	if err != nil {
		return errz.Wrapf(err, "edit config %s", src.Handle)
	}

	if bytes.Equal(before, after) {
		log.Debug("Edit source config: no changes made", lga.Src, src.Handle)
		return nil
	}

	src2 := source.Source{}
	if err = ioz.UnmarshallYAML(after, &src2); err != nil {
		return err
	}

	if src2.Handle != src.Handle {
		log.Debug("Edit source config: attempting source rename",
			lga.From, src.Handle, lga.To, src2.Handle)

		if src, err = cfg.Collection.RenameSource(src.Handle, src2.Handle); err != nil {
			return err
		}
	}

	*src = src2

	// TODO: if --verbose, show diff
	if err = rc.ConfigStore.Save(ctx, cfg); err != nil {
		return err
	}

	log.Debug("Edit source config: changes saved",
		lga.Src, src2.Handle, lga.Path, rc.ConfigStore.Location())
	return nil
}
