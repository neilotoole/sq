package cli

import (
	"github.com/spf13/cobra"
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

// cmdFlagTrue returns true if flag name has been changed
// and the flag value is true.
func cmdFlagTrue(cmd *cobra.Command, name string) bool {
	if !cmdFlagChanged(cmd, name) {
		return false
	}

	b, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(err) // Should never happen
	}

	return b
}
