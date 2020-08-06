package cli

import (
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

const bashCompletionFunc = `

__sq_list_sources()
{
    local sq_output out
    if sq_output=$(sq ls 2>/dev/null); then
        out=($(echo "${sq_output}" | awk 'NR > 1 {print $1}'))
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__sq_get_resource()
{
    if [[ ${#nouns[@]} -eq 0 ]]; then
        return 1
    fi
    __sq_list_sources ${nouns[${#nouns[@]} -1]}
    if [[ $? -eq 0 ]]; then
        return 0
    fi
}

__custom_func() {
    case ${last_command} in
        sq_ls | sq_src | sq_rm | sq_inspect )
            __sq_list_sources
            return
            ;;
        *)
            ;;
    esac
}
`

func newInstallBashCompletionCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:    "install-bash-completion",
		Short:  "Install bash completion script on Unix-ish systems.",
		Hidden: true,
	}

	return cmd, execInstallBashCompletion
}

func execInstallBashCompletion(rc *RunContext, cmd *cobra.Command, args []string) error {
	log := rc.Log
	var path string

	switch runtime.GOOS {
	case "windows":
		log.Warnf("skipping install bash completion on windows")
		return nil
	case "darwin":
		path = "/usr/local/etc/bash_completion.d/sq"
	default:
		// it's unixish
		path = " /etc/bash_completion.d/sq"
	}

	// TODO: only write if necessary (check for version/timestamp/checksum)
	err := cmd.Root().GenBashCompletionFile(path)
	if err != nil {
		log.Warnf("failed to write bash completion to %q: %v", path, err)
		return err
	}

	return nil
}

func newGenerateZshCompletionCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:    "gen-zsh-completion",
		Short:  "Generate zsh completion script on Unix-ish systems.",
		Hidden: true,
	}
	return cmd, execGenerateZshCompletion
}

func execGenerateZshCompletion(rc *RunContext, cmd *cobra.Command, args []string) error {
	log := rc.Log
	var path string

	switch runtime.GOOS {
	case "windows":
		log.Warnf("skipping install zsh completion on windows")
		return nil
	case "darwin":
		path = "/usr/local/etc/bash_completion.d/sq"
	default:
		// it's unixish
		path = " /etc/bash_completion.d/sq"
	}

	err := cmd.Root().GenZshCompletion(os.Stdout)
	if err != nil {
		log.Warnf("failed to write zsh completion to %q: %v", path, err)
		return err
	}
	return nil
}
