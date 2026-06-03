package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
)

func newConfigExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Args:  cobra.NoArgs,
		Short: "Export config as YAML",
		Long: `Export the active sq config as YAML, including the source collection,
config options, and active source/group state. Intended for backups.

By default, output is a faithful copy of the live config: ${scheme:path}
placeholders (keyring, env, file) are written verbatim. Inline values
already present in source Locations (such as plaintext credentials in a
conn string) are dumped as-is — exactly as they appear in your config file.

With --expand, every ${scheme:path} placeholder is fetched from its
resolver (keyring, env var, or file) and the resolved value is spliced
into the exported Location. This produces a fully self-contained
snapshot suitable for transferring between machines, at the cost of
writing every referenced secret in plaintext. Resolution may fail
per-source if a keyring entry, env var, or file is missing.

When --output is used, the output file is created with mode 0600 (the
same permission sq uses for the live config file), since the export
may contain credentials regardless of whether --expand was set.`,
		RunE: execConfigExport,
		Example: `  # Export to stdout (placeholders preserved)
  $ sq config export

  # Export to a file (backup)
  $ sq config export -o sq.bak.yml

  # Self-contained export with placeholders expanded in-line
  $ sq config export --expand -o sq.bak.yml`,
	}

	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	// execConfigExport handles --output itself to enforce mode 0600
	// on the export file and mode 0700 on any parent dirs it creates,
	// since the export may contain credentials. Marking the command
	// stdout-plain stops preRun's generic --output handler from
	// opening the file with default perms first (which would defeat
	// the tighter perms and, on Windows, leave an open fd that breaks
	// atomic replace).
	cmdMarkPlainStdout(cmd)
	return cmd
}

func execConfigExport(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	expand := cmdFlagIsSetTrue(cmd, flag.Expand)

	cfg := ru.Config
	if expand {
		cloned, err := exportExpandConfig(ctx, ru, cfg)
		if err != nil {
			return err
		}
		cfg = cloned

		lg.FromContext(ctx).Warn(
			"sq config export --expand: resolved secrets are written in plaintext")
	}

	// yamlw renders YAML with the same encoder sq uses for `config ls -y`
	// and friends, honoring the configured Printing (color-on-terminal,
	// monochrome when --output is set or stdout is not a TTY).
	rendered, err := yamlw.MarshalToString(ru.Writers.PrOut, cfg)
	if err != nil {
		return errz.Wrap(err, "config export")
	}
	data := []byte(rendered)

	if !cmdFlagChanged(cmd, flag.FileOutput) {
		// Write through ru.Out (colorable-wrapped) so ANSI color codes
		// render correctly on Windows consoles too.
		if _, err = ru.Out.Write(data); err != nil {
			return errz.Wrap(err, "config export: write")
		}
		return nil
	}

	fpath, err := cmd.Flags().GetString(flag.FileOutput)
	if err != nil {
		return errz.Err(err)
	}
	if fpath = strings.TrimSpace(fpath); fpath == "" {
		return errz.Errorf("config export: --%s is specified, but empty", flag.FileOutput)
	}

	// Create parent dirs at 0o700 (not os.ModePerm) because the export
	// may contain credentials — restrict new dirs to the current user.
	// Existing parent dirs keep their permissions; MkdirAll only sets
	// mode on dirs it creates.
	if err = os.MkdirAll(filepath.Dir(fpath), 0o700); err != nil {
		return errz.Wrap(err, "config export: create parent dir")
	}

	if err = ioz.WriteFileAtomic(fpath, data, ioz.RWPerms); err != nil {
		return errz.Wrap(err, "config export: write")
	}
	return nil
}

// exportExpandConfig returns a copy of cfg with every source's Location
// passed through ru.SecretRegistry.Expand. Collection is deep-cloned;
// Options and Ext are shared with the input cfg (safe because only
// Collection sources are mutated). The input cfg is not mutated.
// Resolution errors are wrapped with the source handle so the user knows
// which source's placeholder failed.
func exportExpandConfig(ctx context.Context, ru *run.Run, cfg *config.Config) (*config.Config, error) {
	clone := &config.Config{
		Version:    cfg.Version,
		Options:    cfg.Options,
		Collection: cfg.Collection.Clone(),
		Ext:        cfg.Ext,
	}

	if clone.Collection == nil {
		return clone, nil
	}

	for _, src := range clone.Collection.Sources() {
		resolved, err := ru.SecretRegistry.Expand(ctx, src.Location)
		if err != nil {
			return nil, errz.Wrapf(err, "config export: %s", src.Handle)
		}
		src.Location = resolved
	}

	return clone, nil
}
