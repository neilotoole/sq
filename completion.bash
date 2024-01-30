# bash completion for sq                                   -*- shell-script -*-

__sq_debug()
{
    if [[ -n ${BASH_COMP_DEBUG_FILE:-} ]]; then
        echo "$*" >> "${BASH_COMP_DEBUG_FILE}"
    fi
}

# Homebrew on Macs have version 1.3 of bash-completion which doesn't include
# _init_completion. This is a very minimal version of that function.
__sq_init_completion()
{
    COMPREPLY=()
    _get_comp_words_by_ref "$@" cur prev words cword
}

__sq_index_of_word()
{
    local w word=$1
    shift
    index=0
    for w in "$@"; do
        [[ $w = "$word" ]] && return
        index=$((index+1))
    done
    index=-1
}

__sq_contains_word()
{
    local w word=$1; shift
    for w in "$@"; do
        [[ $w = "$word" ]] && return
    done
    return 1
}

__sq_handle_go_custom_completion()
{
    __sq_debug "${FUNCNAME[0]}: cur is ${cur}, words[*] is ${words[*]}, #words[@] is ${#words[@]}"

    local shellCompDirectiveError=1
    local shellCompDirectiveNoSpace=2
    local shellCompDirectiveNoFileComp=4
    local shellCompDirectiveFilterFileExt=8
    local shellCompDirectiveFilterDirs=16

    local out requestComp lastParam lastChar comp directive args

    # Prepare the command to request completions for the program.
    # Calling ${words[0]} instead of directly sq allows handling aliases
    args=("${words[@]:1}")
    # Disable ActiveHelp which is not supported for bash completion v1
    requestComp="SQ_ACTIVE_HELP=0 ${words[0]} __completeNoDesc ${args[*]}"

    lastParam=${words[$((${#words[@]}-1))]}
    lastChar=${lastParam:$((${#lastParam}-1)):1}
    __sq_debug "${FUNCNAME[0]}: lastParam ${lastParam}, lastChar ${lastChar}"

    if [ -z "${cur}" ] && [ "${lastChar}" != "=" ]; then
        # If the last parameter is complete (there is a space following it)
        # We add an extra empty parameter so we can indicate this to the go method.
        __sq_debug "${FUNCNAME[0]}: Adding extra empty parameter"
        requestComp="${requestComp} \"\""
    fi

    __sq_debug "${FUNCNAME[0]}: calling ${requestComp}"
    # Use eval to handle any environment variables and such
    out=$(eval "${requestComp}" 2>/dev/null)

    # Extract the directive integer at the very end of the output following a colon (:)
    directive=${out##*:}
    # Remove the directive
    out=${out%:*}
    if [ "${directive}" = "${out}" ]; then
        # There is not directive specified
        directive=0
    fi
    __sq_debug "${FUNCNAME[0]}: the completion directive is: ${directive}"
    __sq_debug "${FUNCNAME[0]}: the completions are: ${out}"

    if [ $((directive & shellCompDirectiveError)) -ne 0 ]; then
        # Error code.  No completion.
        __sq_debug "${FUNCNAME[0]}: received error from custom completion go code"
        return
    else
        if [ $((directive & shellCompDirectiveNoSpace)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __sq_debug "${FUNCNAME[0]}: activating no space"
                compopt -o nospace
            fi
        fi
        if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __sq_debug "${FUNCNAME[0]}: activating no file completion"
                compopt +o default
            fi
        fi
    fi

    if [ $((directive & shellCompDirectiveFilterFileExt)) -ne 0 ]; then
        # File extension filtering
        local fullFilter filter filteringCmd
        # Do not use quotes around the $out variable or else newline
        # characters will be kept.
        for filter in ${out}; do
            fullFilter+="$filter|"
        done

        filteringCmd="_filedir $fullFilter"
        __sq_debug "File filtering command: $filteringCmd"
        $filteringCmd
    elif [ $((directive & shellCompDirectiveFilterDirs)) -ne 0 ]; then
        # File completion for directories only
        local subdir
        # Use printf to strip any trailing newline
        subdir=$(printf "%s" "${out}")
        if [ -n "$subdir" ]; then
            __sq_debug "Listing directories in $subdir"
            __sq_handle_subdirs_in_dir_flag "$subdir"
        else
            __sq_debug "Listing directories in ."
            _filedir -d
        fi
    else
        while IFS='' read -r comp; do
            COMPREPLY+=("$comp")
        done < <(compgen -W "${out}" -- "$cur")
    fi
}

__sq_handle_reply()
{
    __sq_debug "${FUNCNAME[0]}"
    local comp
    case $cur in
        -*)
            if [[ $(type -t compopt) = "builtin" ]]; then
                compopt -o nospace
            fi
            local allflags
            if [ ${#must_have_one_flag[@]} -ne 0 ]; then
                allflags=("${must_have_one_flag[@]}")
            else
                allflags=("${flags[*]} ${two_word_flags[*]}")
            fi
            while IFS='' read -r comp; do
                COMPREPLY+=("$comp")
            done < <(compgen -W "${allflags[*]}" -- "$cur")
            if [[ $(type -t compopt) = "builtin" ]]; then
                [[ "${COMPREPLY[0]}" == *= ]] || compopt +o nospace
            fi

            # complete after --flag=abc
            if [[ $cur == *=* ]]; then
                if [[ $(type -t compopt) = "builtin" ]]; then
                    compopt +o nospace
                fi

                local index flag
                flag="${cur%=*}"
                __sq_index_of_word "${flag}" "${flags_with_completion[@]}"
                COMPREPLY=()
                if [[ ${index} -ge 0 ]]; then
                    PREFIX=""
                    cur="${cur#*=}"
                    ${flags_completion[${index}]}
                    if [ -n "${ZSH_VERSION:-}" ]; then
                        # zsh completion needs --flag= prefix
                        eval "COMPREPLY=( \"\${COMPREPLY[@]/#/${flag}=}\" )"
                    fi
                fi
            fi

            if [[ -z "${flag_parsing_disabled}" ]]; then
                # If flag parsing is enabled, we have completed the flags and can return.
                # If flag parsing is disabled, we may not know all (or any) of the flags, so we fallthrough
                # to possibly call handle_go_custom_completion.
                return 0;
            fi
            ;;
    esac

    # check if we are handling a flag with special work handling
    local index
    __sq_index_of_word "${prev}" "${flags_with_completion[@]}"
    if [[ ${index} -ge 0 ]]; then
        ${flags_completion[${index}]}
        return
    fi

    # we are parsing a flag and don't have a special handler, no completion
    if [[ ${cur} != "${words[cword]}" ]]; then
        return
    fi

    local completions
    completions=("${commands[@]}")
    if [[ ${#must_have_one_noun[@]} -ne 0 ]]; then
        completions+=("${must_have_one_noun[@]}")
    elif [[ -n "${has_completion_function}" ]]; then
        # if a go completion function is provided, defer to that function
        __sq_handle_go_custom_completion
    fi
    if [[ ${#must_have_one_flag[@]} -ne 0 ]]; then
        completions+=("${must_have_one_flag[@]}")
    fi
    while IFS='' read -r comp; do
        COMPREPLY+=("$comp")
    done < <(compgen -W "${completions[*]}" -- "$cur")

    if [[ ${#COMPREPLY[@]} -eq 0 && ${#noun_aliases[@]} -gt 0 && ${#must_have_one_noun[@]} -ne 0 ]]; then
        while IFS='' read -r comp; do
            COMPREPLY+=("$comp")
        done < <(compgen -W "${noun_aliases[*]}" -- "$cur")
    fi

    if [[ ${#COMPREPLY[@]} -eq 0 ]]; then
        if declare -F __sq_custom_func >/dev/null; then
            # try command name qualified custom func
            __sq_custom_func
        else
            # otherwise fall back to unqualified for compatibility
            declare -F __custom_func >/dev/null && __custom_func
        fi
    fi

    # available in bash-completion >= 2, not always present on macOS
    if declare -F __ltrim_colon_completions >/dev/null; then
        __ltrim_colon_completions "$cur"
    fi

    # If there is only 1 completion and it is a flag with an = it will be completed
    # but we don't want a space after the =
    if [[ "${#COMPREPLY[@]}" -eq "1" ]] && [[ $(type -t compopt) = "builtin" ]] && [[ "${COMPREPLY[0]}" == --*= ]]; then
       compopt -o nospace
    fi
}

# The arguments should be in the form "ext1|ext2|extn"
__sq_handle_filename_extension_flag()
{
    local ext="$1"
    _filedir "@(${ext})"
}

__sq_handle_subdirs_in_dir_flag()
{
    local dir="$1"
    pushd "${dir}" >/dev/null 2>&1 && _filedir -d && popd >/dev/null 2>&1 || return
}

__sq_handle_flag()
{
    __sq_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    # if a command required a flag, and we found it, unset must_have_one_flag()
    local flagname=${words[c]}
    local flagvalue=""
    # if the word contained an =
    if [[ ${words[c]} == *"="* ]]; then
        flagvalue=${flagname#*=} # take in as flagvalue after the =
        flagname=${flagname%=*} # strip everything after the =
        flagname="${flagname}=" # but put the = back
    fi
    __sq_debug "${FUNCNAME[0]}: looking for ${flagname}"
    if __sq_contains_word "${flagname}" "${must_have_one_flag[@]}"; then
        must_have_one_flag=()
    fi

    # if you set a flag which only applies to this command, don't show subcommands
    if __sq_contains_word "${flagname}" "${local_nonpersistent_flags[@]}"; then
      commands=()
    fi

    # keep flag value with flagname as flaghash
    # flaghash variable is an associative array which is only supported in bash > 3.
    if [[ -z "${BASH_VERSION:-}" || "${BASH_VERSINFO[0]:-}" -gt 3 ]]; then
        if [ -n "${flagvalue}" ] ; then
            flaghash[${flagname}]=${flagvalue}
        elif [ -n "${words[ $((c+1)) ]}" ] ; then
            flaghash[${flagname}]=${words[ $((c+1)) ]}
        else
            flaghash[${flagname}]="true" # pad "true" for bool flag
        fi
    fi

    # skip the argument to a two word flag
    if [[ ${words[c]} != *"="* ]] && __sq_contains_word "${words[c]}" "${two_word_flags[@]}"; then
        __sq_debug "${FUNCNAME[0]}: found a flag ${words[c]}, skip the next argument"
        c=$((c+1))
        # if we are looking for a flags value, don't show commands
        if [[ $c -eq $cword ]]; then
            commands=()
        fi
    fi

    c=$((c+1))

}

__sq_handle_noun()
{
    __sq_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    if __sq_contains_word "${words[c]}" "${must_have_one_noun[@]}"; then
        must_have_one_noun=()
    elif __sq_contains_word "${words[c]}" "${noun_aliases[@]}"; then
        must_have_one_noun=()
    fi

    nouns+=("${words[c]}")
    c=$((c+1))
}

__sq_handle_command()
{
    __sq_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    local next_command
    if [[ -n ${last_command} ]]; then
        next_command="_${last_command}_${words[c]//:/__}"
    else
        if [[ $c -eq 0 ]]; then
            next_command="_sq_root_command"
        else
            next_command="_${words[c]//:/__}"
        fi
    fi
    c=$((c+1))
    __sq_debug "${FUNCNAME[0]}: looking for ${next_command}"
    declare -F "$next_command" >/dev/null && $next_command
}

__sq_handle_word()
{
    if [[ $c -ge $cword ]]; then
        __sq_handle_reply
        return
    fi
    __sq_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    if [[ "${words[c]}" == -* ]]; then
        __sq_handle_flag
    elif __sq_contains_word "${words[c]}" "${commands[@]}"; then
        __sq_handle_command
    elif [[ $c -eq 0 ]]; then
        __sq_handle_command
    elif __sq_contains_word "${words[c]}" "${command_aliases[@]}"; then
        # aliashash variable is an associative array which is only supported in bash > 3.
        if [[ -z "${BASH_VERSION:-}" || "${BASH_VERSINFO[0]:-}" -gt 3 ]]; then
            words[c]=${aliashash[${words[c]}]}
            __sq_handle_command
        else
            __sq_handle_noun
        fi
    else
        __sq_handle_noun
    fi
    __sq_handle_word
}

_sq_add()
{
    last_command="sq_add"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--driver=")
    two_word_flags+=("--driver")
    flags_with_completion+=("--driver")
    flags_completion+=("__sq_handle_go_custom_completion")
    two_word_flags+=("-d")
    flags_with_completion+=("-d")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--driver")
    local_nonpersistent_flags+=("--driver=")
    local_nonpersistent_flags+=("-d")
    flags+=("--handle=")
    two_word_flags+=("--handle")
    two_word_flags+=("-n")
    local_nonpersistent_flags+=("--handle")
    local_nonpersistent_flags+=("--handle=")
    local_nonpersistent_flags+=("-n")
    flags+=("--password")
    flags+=("-p")
    local_nonpersistent_flags+=("--password")
    local_nonpersistent_flags+=("-p")
    flags+=("--skip-verify")
    local_nonpersistent_flags+=("--skip-verify")
    flags+=("--active")
    flags+=("-a")
    local_nonpersistent_flags+=("--active")
    local_nonpersistent_flags+=("-a")
    flags+=("--ingest.header")
    local_nonpersistent_flags+=("--ingest.header")
    flags+=("--driver.csv.empty-as-null")
    local_nonpersistent_flags+=("--driver.csv.empty-as-null")
    flags+=("--driver.csv.delim=")
    two_word_flags+=("--driver.csv.delim")
    flags_with_completion+=("--driver.csv.delim")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--driver.csv.delim")
    local_nonpersistent_flags+=("--driver.csv.delim=")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_src()
{
    last_command="sq_src"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_group()
{
    last_command="sq_group"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_ls()
{
    last_command="sq_ls"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--group")
    flags+=("-g")
    local_nonpersistent_flags+=("--group")
    local_nonpersistent_flags+=("-g")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_mv()
{
    last_command="sq_mv"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_rm()
{
    last_command="sq_rm"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_inspect()
{
    last_command="sq_inspect"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--overview")
    flags+=("-O")
    local_nonpersistent_flags+=("--overview")
    local_nonpersistent_flags+=("-O")
    flags+=("--dbprops")
    flags+=("-p")
    local_nonpersistent_flags+=("--dbprops")
    local_nonpersistent_flags+=("-p")
    flags+=("--catalogs")
    flags+=("-C")
    local_nonpersistent_flags+=("--catalogs")
    local_nonpersistent_flags+=("-C")
    flags+=("--schemata")
    flags+=("-S")
    local_nonpersistent_flags+=("--schemata")
    local_nonpersistent_flags+=("-S")
    flags+=("--src.schema=")
    two_word_flags+=("--src.schema")
    flags_with_completion+=("--src.schema")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src.schema")
    local_nonpersistent_flags+=("--src.schema=")
    flags+=("--no-cache")
    local_nonpersistent_flags+=("--no-cache")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_ping()
{
    last_command="sq_ping"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--csv")
    flags+=("-C")
    local_nonpersistent_flags+=("--csv")
    local_nonpersistent_flags+=("-C")
    flags+=("--tsv")
    local_nonpersistent_flags+=("--tsv")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--timeout=")
    two_word_flags+=("--timeout")
    local_nonpersistent_flags+=("--timeout")
    local_nonpersistent_flags+=("--timeout=")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_sql()
{
    last_command="sq_sql"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--format=")
    two_word_flags+=("--format")
    flags_with_completion+=("--format")
    flags_completion+=("__sq_handle_go_custom_completion")
    two_word_flags+=("-f")
    flags_with_completion+=("-f")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format")
    local_nonpersistent_flags+=("--format=")
    local_nonpersistent_flags+=("-f")
    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--jsona")
    flags+=("-A")
    local_nonpersistent_flags+=("--jsona")
    local_nonpersistent_flags+=("-A")
    flags+=("--jsonl")
    flags+=("-J")
    local_nonpersistent_flags+=("--jsonl")
    local_nonpersistent_flags+=("-J")
    flags+=("--csv")
    flags+=("-C")
    local_nonpersistent_flags+=("--csv")
    local_nonpersistent_flags+=("-C")
    flags+=("--tsv")
    local_nonpersistent_flags+=("--tsv")
    flags+=("--html")
    local_nonpersistent_flags+=("--html")
    flags+=("--markdown")
    local_nonpersistent_flags+=("--markdown")
    flags+=("--raw")
    flags+=("-r")
    local_nonpersistent_flags+=("--raw")
    local_nonpersistent_flags+=("-r")
    flags+=("--xlsx")
    flags+=("-x")
    local_nonpersistent_flags+=("--xlsx")
    local_nonpersistent_flags+=("-x")
    flags+=("--xml")
    local_nonpersistent_flags+=("--xml")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--format.datetime=")
    two_word_flags+=("--format.datetime")
    flags_with_completion+=("--format.datetime")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.datetime")
    local_nonpersistent_flags+=("--format.datetime=")
    flags+=("--format.datetime.number")
    flags_with_completion+=("--format.datetime.number")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.datetime.number")
    flags+=("--format.date=")
    two_word_flags+=("--format.date")
    flags_with_completion+=("--format.date")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.date")
    local_nonpersistent_flags+=("--format.date=")
    flags+=("--format.date.number")
    flags_with_completion+=("--format.date.number")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.date.number")
    flags+=("--format.time=")
    two_word_flags+=("--format.time")
    flags_with_completion+=("--format.time")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.time")
    local_nonpersistent_flags+=("--format.time=")
    flags+=("--format.time.number")
    flags_with_completion+=("--format.time.number")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.time.number")
    flags+=("--format.excel.datetime=")
    two_word_flags+=("--format.excel.datetime")
    flags_with_completion+=("--format.excel.datetime")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.excel.datetime")
    local_nonpersistent_flags+=("--format.excel.datetime=")
    flags+=("--format.excel.date=")
    two_word_flags+=("--format.excel.date")
    flags_with_completion+=("--format.excel.date")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.excel.date")
    local_nonpersistent_flags+=("--format.excel.date=")
    flags+=("--format.excel.time=")
    two_word_flags+=("--format.excel.time")
    flags_with_completion+=("--format.excel.time")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.excel.time")
    local_nonpersistent_flags+=("--format.excel.time=")
    flags+=("--output=")
    two_word_flags+=("--output")
    two_word_flags+=("-o")
    local_nonpersistent_flags+=("--output")
    local_nonpersistent_flags+=("--output=")
    local_nonpersistent_flags+=("-o")
    flags+=("--insert=")
    two_word_flags+=("--insert")
    flags_with_completion+=("--insert")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--insert")
    local_nonpersistent_flags+=("--insert=")
    flags+=("--src=")
    two_word_flags+=("--src")
    flags_with_completion+=("--src")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src")
    local_nonpersistent_flags+=("--src=")
    flags+=("--src.schema=")
    two_word_flags+=("--src.schema")
    flags_with_completion+=("--src.schema")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src.schema")
    local_nonpersistent_flags+=("--src.schema=")
    flags+=("--ingest.driver=")
    two_word_flags+=("--ingest.driver")
    flags_with_completion+=("--ingest.driver")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--ingest.driver")
    local_nonpersistent_flags+=("--ingest.driver=")
    flags+=("--ingest.header")
    local_nonpersistent_flags+=("--ingest.header")
    flags+=("--no-cache")
    local_nonpersistent_flags+=("--no-cache")
    flags+=("--driver.csv.empty-as-null")
    local_nonpersistent_flags+=("--driver.csv.empty-as-null")
    flags+=("--driver.csv.delim=")
    two_word_flags+=("--driver.csv.delim")
    flags_with_completion+=("--driver.csv.delim")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--driver.csv.delim")
    local_nonpersistent_flags+=("--driver.csv.delim=")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_tbl_copy()
{
    last_command="sq_tbl_copy"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--data")
    local_nonpersistent_flags+=("--data")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_tbl_truncate()
{
    last_command="sq_tbl_truncate"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_tbl_drop()
{
    last_command="sq_tbl_drop"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_tbl()
{
    last_command="sq_tbl"

    command_aliases=()

    commands=()
    commands+=("copy")
    commands+=("truncate")
    commands+=("drop")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_diff()
{
    last_command="sq_diff"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--unified=")
    two_word_flags+=("--unified")
    two_word_flags+=("-U")
    local_nonpersistent_flags+=("--unified")
    local_nonpersistent_flags+=("--unified=")
    local_nonpersistent_flags+=("-U")
    flags+=("--format=")
    two_word_flags+=("--format")
    flags_with_completion+=("--format")
    flags_completion+=("__sq_handle_go_custom_completion")
    two_word_flags+=("-f")
    flags_with_completion+=("-f")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format")
    local_nonpersistent_flags+=("--format=")
    local_nonpersistent_flags+=("-f")
    flags+=("--overview")
    flags+=("-O")
    local_nonpersistent_flags+=("--overview")
    local_nonpersistent_flags+=("-O")
    flags+=("--dbprops")
    flags+=("-B")
    local_nonpersistent_flags+=("--dbprops")
    local_nonpersistent_flags+=("-B")
    flags+=("--schema")
    flags+=("-S")
    local_nonpersistent_flags+=("--schema")
    local_nonpersistent_flags+=("-S")
    flags+=("--counts")
    flags+=("-N")
    local_nonpersistent_flags+=("--counts")
    local_nonpersistent_flags+=("-N")
    flags+=("--data")
    flags+=("-d")
    local_nonpersistent_flags+=("--data")
    local_nonpersistent_flags+=("-d")
    flags+=("--all")
    flags+=("-a")
    local_nonpersistent_flags+=("--all")
    local_nonpersistent_flags+=("-a")
    flags+=("--no-cache")
    local_nonpersistent_flags+=("--no-cache")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_driver_ls()
{
    last_command="sq_driver_ls"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_driver()
{
    last_command="sq_driver"

    command_aliases=()

    commands=()
    commands+=("ls")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_config_ls()
{
    last_command="sq_config_ls"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--src=")
    two_word_flags+=("--src")
    flags_with_completion+=("--src")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src")
    local_nonpersistent_flags+=("--src=")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_config_get()
{
    last_command="sq_config_get"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--src=")
    two_word_flags+=("--src")
    flags_with_completion+=("--src")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src")
    local_nonpersistent_flags+=("--src=")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_config_set()
{
    last_command="sq_config_set"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--src=")
    two_word_flags+=("--src")
    flags_with_completion+=("--src")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src")
    local_nonpersistent_flags+=("--src=")
    flags+=("--delete")
    flags+=("-D")
    local_nonpersistent_flags+=("--delete")
    local_nonpersistent_flags+=("-D")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_config_location()
{
    last_command="sq_config_location"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_config_edit()
{
    last_command="sq_config_edit"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_config()
{
    last_command="sq_config"

    command_aliases=()

    commands=()
    commands+=("ls")
    commands+=("get")
    commands+=("set")
    commands+=("location")
    commands+=("edit")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_cache_location()
{
    last_command="sq_cache_location"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_cache_stat()
{
    last_command="sq_cache_stat"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_cache_enable()
{
    last_command="sq_cache_enable"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_cache_disable()
{
    last_command="sq_cache_disable"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_cache_clear()
{
    last_command="sq_cache_clear"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_sq_cache_tree()
{
    last_command="sq_cache_tree"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--size")
    flags+=("-s")
    local_nonpersistent_flags+=("--size")
    local_nonpersistent_flags+=("-s")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_cache()
{
    last_command="sq_cache"

    command_aliases=()

    commands=()
    commands+=("location")
    commands+=("stat")
    commands+=("enable")
    commands+=("disable")
    commands+=("clear")
    commands+=("tree")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_completion()
{
    last_command="sq_completion"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    must_have_one_noun+=("bash")
    must_have_one_noun+=("fish")
    must_have_one_noun+=("powershell")
    must_have_one_noun+=("zsh")
    noun_aliases=()
}

_sq_version()
{
    last_command="sq_version"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--help")
    local_nonpersistent_flags+=("--help")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_help()
{
    last_command="sq_help"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--help")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_sq_root_command()
{
    last_command="sq"

    command_aliases=()

    commands=()
    commands+=("add")
    commands+=("src")
    commands+=("group")
    commands+=("ls")
    commands+=("mv")
    commands+=("rm")
    commands+=("inspect")
    commands+=("ping")
    commands+=("sql")
    commands+=("tbl")
    commands+=("diff")
    commands+=("driver")
    commands+=("config")
    commands+=("cache")
    commands+=("completion")
    commands+=("version")
    commands+=("help")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--format=")
    two_word_flags+=("--format")
    flags_with_completion+=("--format")
    flags_completion+=("__sq_handle_go_custom_completion")
    two_word_flags+=("-f")
    flags_with_completion+=("-f")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format")
    local_nonpersistent_flags+=("--format=")
    local_nonpersistent_flags+=("-f")
    flags+=("--text")
    flags+=("-t")
    local_nonpersistent_flags+=("--text")
    local_nonpersistent_flags+=("-t")
    flags+=("--header")
    flags+=("-h")
    local_nonpersistent_flags+=("--header")
    local_nonpersistent_flags+=("-h")
    flags+=("--no-header")
    flags+=("-H")
    local_nonpersistent_flags+=("--no-header")
    local_nonpersistent_flags+=("-H")
    flags+=("--help")
    flags+=("--json")
    flags+=("-j")
    local_nonpersistent_flags+=("--json")
    local_nonpersistent_flags+=("-j")
    flags+=("--jsona")
    flags+=("-A")
    local_nonpersistent_flags+=("--jsona")
    local_nonpersistent_flags+=("-A")
    flags+=("--jsonl")
    flags+=("-J")
    local_nonpersistent_flags+=("--jsonl")
    local_nonpersistent_flags+=("-J")
    flags+=("--csv")
    flags+=("-C")
    local_nonpersistent_flags+=("--csv")
    local_nonpersistent_flags+=("-C")
    flags+=("--tsv")
    local_nonpersistent_flags+=("--tsv")
    flags+=("--html")
    local_nonpersistent_flags+=("--html")
    flags+=("--markdown")
    local_nonpersistent_flags+=("--markdown")
    flags+=("--raw")
    flags+=("-r")
    local_nonpersistent_flags+=("--raw")
    local_nonpersistent_flags+=("-r")
    flags+=("--xlsx")
    flags+=("-x")
    local_nonpersistent_flags+=("--xlsx")
    local_nonpersistent_flags+=("-x")
    flags+=("--xml")
    local_nonpersistent_flags+=("--xml")
    flags+=("--yaml")
    flags+=("-y")
    local_nonpersistent_flags+=("--yaml")
    local_nonpersistent_flags+=("-y")
    flags+=("--compact")
    flags+=("-c")
    local_nonpersistent_flags+=("--compact")
    local_nonpersistent_flags+=("-c")
    flags+=("--format.datetime=")
    two_word_flags+=("--format.datetime")
    flags_with_completion+=("--format.datetime")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.datetime")
    local_nonpersistent_flags+=("--format.datetime=")
    flags+=("--format.datetime.number")
    flags_with_completion+=("--format.datetime.number")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.datetime.number")
    flags+=("--format.date=")
    two_word_flags+=("--format.date")
    flags_with_completion+=("--format.date")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.date")
    local_nonpersistent_flags+=("--format.date=")
    flags+=("--format.date.number")
    flags_with_completion+=("--format.date.number")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.date.number")
    flags+=("--format.time=")
    two_word_flags+=("--format.time")
    flags_with_completion+=("--format.time")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.time")
    local_nonpersistent_flags+=("--format.time=")
    flags+=("--format.time.number")
    flags_with_completion+=("--format.time.number")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.time.number")
    flags+=("--format.excel.datetime=")
    two_word_flags+=("--format.excel.datetime")
    flags_with_completion+=("--format.excel.datetime")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.excel.datetime")
    local_nonpersistent_flags+=("--format.excel.datetime=")
    flags+=("--format.excel.date=")
    two_word_flags+=("--format.excel.date")
    flags_with_completion+=("--format.excel.date")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.excel.date")
    local_nonpersistent_flags+=("--format.excel.date=")
    flags+=("--format.excel.time=")
    two_word_flags+=("--format.excel.time")
    flags_with_completion+=("--format.excel.time")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--format.excel.time")
    local_nonpersistent_flags+=("--format.excel.time=")
    flags+=("--output=")
    two_word_flags+=("--output")
    two_word_flags+=("-o")
    local_nonpersistent_flags+=("--output")
    local_nonpersistent_flags+=("--output=")
    local_nonpersistent_flags+=("-o")
    flags+=("--insert=")
    two_word_flags+=("--insert")
    flags_with_completion+=("--insert")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--insert")
    local_nonpersistent_flags+=("--insert=")
    flags+=("--src=")
    two_word_flags+=("--src")
    flags_with_completion+=("--src")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src")
    local_nonpersistent_flags+=("--src=")
    flags+=("--src.schema=")
    two_word_flags+=("--src.schema")
    flags_with_completion+=("--src.schema")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--src.schema")
    local_nonpersistent_flags+=("--src.schema=")
    flags+=("--ingest.driver=")
    two_word_flags+=("--ingest.driver")
    flags_with_completion+=("--ingest.driver")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--ingest.driver")
    local_nonpersistent_flags+=("--ingest.driver=")
    flags+=("--ingest.header")
    local_nonpersistent_flags+=("--ingest.header")
    flags+=("--no-cache")
    local_nonpersistent_flags+=("--no-cache")
    flags+=("--driver.csv.empty-as-null")
    local_nonpersistent_flags+=("--driver.csv.empty-as-null")
    flags+=("--driver.csv.delim=")
    two_word_flags+=("--driver.csv.delim")
    flags_with_completion+=("--driver.csv.delim")
    flags_completion+=("__sq_handle_go_custom_completion")
    local_nonpersistent_flags+=("--driver.csv.delim")
    local_nonpersistent_flags+=("--driver.csv.delim=")
    flags+=("--version")
    local_nonpersistent_flags+=("--version")
    flags+=("--monochrome")
    flags+=("-M")
    flags+=("--no-progress")
    flags+=("--verbose")
    flags+=("-v")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--log")
    flags_with_completion+=("--log")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.file=")
    two_word_flags+=("--log.file")
    flags+=("--log.level=")
    two_word_flags+=("--log.level")
    flags_with_completion+=("--log.level")
    flags_completion+=("__sq_handle_go_custom_completion")
    flags+=("--log.format=")
    two_word_flags+=("--log.format")
    flags_with_completion+=("--log.format")
    flags_completion+=("__sq_handle_go_custom_completion")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

__start_sq()
{
    local cur prev words cword split
    declare -A flaghash 2>/dev/null || :
    declare -A aliashash 2>/dev/null || :
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion -s || return
    else
        __sq_init_completion -n "=" || return
    fi

    local c=0
    local flag_parsing_disabled=
    local flags=()
    local two_word_flags=()
    local local_nonpersistent_flags=()
    local flags_with_completion=()
    local flags_completion=()
    local commands=("sq")
    local command_aliases=()
    local must_have_one_flag=()
    local must_have_one_noun=()
    local has_completion_function=""
    local last_command=""
    local nouns=()
    local noun_aliases=()

    __sq_handle_word
}

if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_sq sq
else
    complete -o default -o nospace -F __start_sq sq
fi

# ex: ts=4 sw=4 et filetype=sh
