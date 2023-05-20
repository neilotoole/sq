package cli

import (
	"io"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// cmdFlagChanged returns true if cmd is non-nil and
// has the named flag and that flag been changed.
func cmdFlagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}

	f := cmd.Flag(name)
	if f == nil {
		return false
	}

	return f.Changed
}

// cmdFlagIsSetTrue returns true if flag name has been changed
// and the flag value is true.
// Contrast with cmdFlagIsSetTrue.
func cmdFlagIsSetTrue(cmd *cobra.Command, name string) bool {
	if !cmdFlagChanged(cmd, name) {
		return false
	}

	b, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(err) // Should never happen
	}

	return b
}

// cmdFlagIsSetTrue returns the bool value of flag name. If the flag
// has not been set, its default value is returned.
// Contrast with cmdFlagIsSetTrue.
func cmdFlagBool(cmd *cobra.Command, name string) bool { //nolint:unused
	b, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(err) // Should never happen
	}

	return b
}

// getBootstrapFlagValue parses osArgs looking for flg. The flag is always
// treated as string. This function exists because some components such
// as logging and config interrogate flags before cobra has loaded.
func getBootstrapFlagValue(flg, flgShort, flgUsage string, osArgs []string) (val string, ok bool, err error) {
	fs := pflag.NewFlagSet("bootstrap", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true
	fs.SetOutput(io.Discard)

	_ = fs.StringP(flg, flgShort, "", flgUsage)
	if err = fs.Parse(osArgs); err != nil {
		return "", false, errz.Err(err)
	}

	if !fs.Changed(flg) {
		return "", false, nil
	}

	if val, err = fs.GetString(flg); err != nil {
		return "", false, errz.Err(err)
	}

	return val, true, nil
}

func applyFlagAliases(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if f == nil {
		return pflag.NormalizedName(name)
	}
	switch name {
	case "table":
		// Legacy: flag --text was once named --table.
		name = flag.Text
	case "md":
		name = flag.Markdown
	default:
	}
	return pflag.NormalizedName(name)
}
