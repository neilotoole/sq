package cmd

const (
	bash_completion_func = `

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
)
