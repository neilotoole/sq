package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		RunE:  execCompletion,
		Long: `To load completions:

Bash:

$ source <(sq completion bash)

# To load completions for each session, execute once:
Linux:
  $ sq completion bash > /etc/bash_completion.d/sq
MacOS:
  $ sq completion bash > /usr/local/etc/bash_completion.d/sq

Zsh:

# If shell completion is not already enabled in your environment you will need
# to enable it.  You can execute the following once:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

# To load completions for each session, execute once:
$ sq completion zsh > "${fpath[1]}/_sq"

# You will need to start a new shell for this setup to take effect.

Fish:

$ sq completion fish | source

# To load completions for each session, execute once:
$ sq completion fish > ~/.config/fish/completions/sq.fish

Powershell:

PS> sq completion powershell | Out-String | Invoke-Expression

# To load completions for every new session, run:
PS> sq completion powershell > sq.ps1
# and source this file from your powershell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	}

	return cmd
}

func execCompletion(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	switch args[0] {
	case "bash":
		return cmd.Root().GenBashCompletion(ru.Out)
	case "zsh":
		return cmd.Root().GenZshCompletion(ru.Out)
	case "fish":
		return cmd.Root().GenFishCompletion(ru.Out, true)
	case "powershell":
		return cmd.Root().GenPowerShellCompletion(ru.Out)
	default:
		return errz.Errorf("invalid arg: %s", args[0])
	}
}
