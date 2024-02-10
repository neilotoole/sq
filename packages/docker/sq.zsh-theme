# git theming
ZSH_THEME_GIT_PROMPT_PREFIX="%{$reset_color%}%{$fg_no_bold[yellow]%}"
ZSH_THEME_GIT_PROMPT_SUFFIX="%b%{$fg_bold[gray]%}%{$reset_color%}"
ZSH_THEME_GIT_PROMPT_CLEAN=" "
ZSH_THEME_GIT_PROMPT_DIRTY="%{$fg_bold[red]%}✱ "

git_prompt_info () {
	if ! __git_prompt_git rev-parse --git-dir &> /dev/null || [[ "$(__git_prompt_git config --get oh-my-zsh.hide-info 2>/dev/null)" == 1 ]]
	then
		return 0
	fi
	local ref
	ref=$(__git_prompt_git symbolic-ref --short HEAD 2> /dev/null)  || ref=$(__git_prompt_git describe --tags --exact-match HEAD 2> /dev/null)  || ref=$(__git_prompt_git rev-parse --short HEAD 2> /dev/null)  || return 0

  if [[ ${#ref} -gt 24 ]]; then
    ref=$(printf "%.24s…" $ref)
  fi

	local upstream
	if (( ${+ZSH_THEME_GIT_SHOW_UPSTREAM} ))
	then
		upstream=$(__git_prompt_git rev-parse --abbrev-ref --symbolic-full-name "@{upstream}" 2>/dev/null)  && upstream=" -> ${upstream}"
	fi

	echo "${ZSH_THEME_GIT_PROMPT_PREFIX}${ref:gs/%/%%}${upstream:gs/%/%%}$(parse_git_dirty)${ZSH_THEME_GIT_PROMPT_SUFFIX}"
}

# sq_prompt returns the handle of sq's active source, or empty string.
function sq_prompt() {
  handle=`sq src` || true
  if [[ $handle != "" ]]; then
    echo "%{$fg_bold[green]%}$handle%{$reset_color%} "
  fi
}


function check_last_exit_code() {
  local LAST_EXIT_CODE=$?
  if [[ $LAST_EXIT_CODE -ne 0 ]]; then
    local EXIT_CODE_PROMPT=''
    EXIT_CODE_PROMPT+="%{$reset_color%}"
    EXIT_CODE_PROMPT+="%{$bg[red]$fg[border-circle]$fg_bold[white]%} $LAST_EXIT_CODE %{$reset_color%}"
    EXIT_CODE_PROMPT+="%{$reset_color%}"
    echo "$EXIT_CODE_PROMPT "
  fi
}

PROMPT='$(check_last_exit_code)%{$fg[cyan]%}%D{%H:%M:%S}%{$reset_color%} $(git_prompt_info)$(sq_prompt)%{$fg[cyan]%}%24<…<%~%<<%{$reset_color%} %{$fg_bold[cyan]%}%(!.#.$)%{$reset_color%} '
