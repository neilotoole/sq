package cli

import (
	"bufio"
	"bytes"
	"os"
	"strings"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq/core/options"

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
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return execConfigEditOptions(cmd, args)
			}

			return execConfigEditSource(cmd, args)
		},
		Short: "Edit config or source options",
		Long:  `Edit config or source options in the editor specified in envar $SQ_EDITOR or $EDITOR.`,
		Example: `  # Edit default options
  $ sq config edit

  # Edit default options, but show additional help/context.
  $ sq config edit -v

  # Edit config for source @sakila
  $ sq config edit @sakila

  # Same as above, with additional help/context.
  $ sq config edit @sakila -v

  # Use a different editor
  $ SQ_EDITOR=nano sq config edit`,
	}

	return cmd
}

// execConfigEditOptions edits the default options.
func execConfigEditOptions(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	rc, log := RunContextFrom(ctx), logFrom(cmd)
	cfg := rc.Config
	cmdOpts, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}
	verbose := OptVerbose.Get(cmdOpts)

	optsText, err := getOptionsEditableText(rc.OptionsRegistry, rc.Config.Options, verbose)
	if err != nil {
		return err
	}
	before := []byte(optsText)

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

// execConfigEditSource edits an individual source's config.
func execConfigEditSource(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	rc, log := RunContextFrom(ctx), logFrom(cmd)
	cfg := rc.Config

	cmdOpts, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}
	verbose := OptVerbose.Get(cmdOpts)

	src, err := cfg.Collection.Get(args[0])
	if err != nil {
		return err
	}

	opts := rc.OptionsRegistry.Opts()
	opts = filterOptionsForSrc(src.Type, opts...)
	srcReg := &options.Registry{}
	srcReg.Add(opts...)

	tmpSrc := src.Clone()
	tmpSrc.Options = nil
	header, err := ioz.MarshalYAML(tmpSrc)
	if err != nil {
		return err
	}

	sb := strings.Builder{}
	sb.Write(header)
	sb.WriteString("options:\n")

	optionsText, err := getOptionsEditableText(srcReg, src.Options, verbose)
	if err != nil {
		return err
	}

	// Add indentation
	sc := bufio.NewScanner(strings.NewReader(optionsText))
	var line string
	for sc.Scan() {
		line = sc.Text()
		if line != "" {
			sb.WriteString("  ") // indent
		}
		sb.WriteString(line)
		sb.WriteRune('\n')
	}

	if err = sc.Err(); err != nil {
		return errz.Err(err)
	}

	before := []byte(sb.String())
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

	src2 := &source.Source{}
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

	*src = *src2

	// TODO: if --verbose, show diff between config before and after.
	if err = rc.ConfigStore.Save(ctx, cfg); err != nil {
		return err
	}

	log.Debug("Edit source config: changes saved",
		lga.Src, src2.Handle, lga.Path, rc.ConfigStore.Location())
	return nil
}

func getOptionsEditableText(reg *options.Registry, o options.Options, verbose bool) (string, error) {
	sb := strings.Builder{}
	if verbose {
		for i, opt := range reg.Opts() {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("# ")
			sb.WriteString(strings.ReplaceAll(opt.Comment(), "\n", "\n# "))
			sb.WriteRune('\n')
			if !o.IsSet(opt) {
				sb.WriteString("#")
			}

			b, err := ioz.MarshalYAML(map[string]any{opt.Key(): opt.GetAny(o)})
			if err != nil {
				return "", err
			}

			sb.WriteString(string(b))
		}

		return sb.String(), nil
	}

	// Not verbose
	for _, opt := range reg.Opts() {
		// First we print the opts that have been set
		if !o.IsSet(opt) {
			continue
		}

		b, err := ioz.MarshalYAML(map[string]any{opt.Key(): opt.GetAny(o)})
		if err != nil {
			return "", err
		}

		sb.WriteString(string(b))
	}

	if len(o) > 0 && len(o) != len(reg.Opts()) {
		sb.WriteRune('\n')
	}

	// Now we print the unset opts
	for _, opt := range reg.Opts() {
		if o.IsSet(opt) {
			continue
		}

		sb.WriteString("#")
		b, err := ioz.MarshalYAML(map[string]any{opt.Key(): opt.GetAny(o)})
		if err != nil {
			return "", err
		}

		sb.WriteString(string(b))
	}

	return sb.String(), nil
}
