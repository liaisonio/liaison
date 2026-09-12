# Session-local Bash >= 4.4 integration. No permanent startup files are modified.
# Remove our temporary rc before loading user configuration.
[[ ${BASH_SOURCE[0]} == */liaison-bash.* ]] && command rm -f -- "${BASH_SOURCE[0]}"
[[ -r ~/.bashrc ]] && source ~/.bashrc

# Avoid overwriting PROMPT_COMMAND hooks. Bash supports array-valued commands.
__liaison_saved_prompt=$PS1
__liaison_history_number=0
__liaison_capture_status() {
    __liaison_exit=$?
    return "$__liaison_exit"
}
# Escape OSC payload delimiters/control bytes without spawning external tools.
__liaison_escape() {
    local LC_ALL=C value=$1 char i code
    for ((i=0; i<${#value} && i<2048; i++)); do
        char=${value:i:1}
        printf -v code '%d' "'$char"
        if ((code <= 32 || code == 59 || code == 92 || code == 127)); then
            printf '\\x%02x' "$code"
        else
            printf '%s' "$char"
        fi
    done
}
__liaison_preexec() {
    local entry number command
    entry=$(HISTTIMEFORMAT= builtin history 1)
    if [[ $entry =~ ^[[:space:]]*([0-9]+)[*\ ]+[[:space:]]*(.*)$ ]]; then
        number=${BASH_REMATCH[1]}
        command=${BASH_REMATCH[2]}
        # Never reuse an old history entry when HISTCONTROL/HISTIGNORE omit input.
        if ((number > __liaison_history_number)); then
            printf '\033]633;E;'
            __liaison_escape "$command"
            printf '\007'
        fi
    fi
    printf '\033]633;C\007'
}
__liaison_prompt() {
    local entry
    printf '\033]633;D;%s\007' "${__liaison_exit:-0}"
    printf '\033]633;P;Cwd='
    __liaison_escape "$PWD"
    printf '\007'
    entry=$(HISTTIMEFORMAT= builtin history 1)
    if [[ $entry =~ ^[[:space:]]*([0-9]+) ]]; then
        __liaison_history_number=${BASH_REMATCH[1]}
    fi
    # Re-wrap a prompt changed by a user's prompt hook, without nesting markers.
    if [[ $PS1 != "$__liaison_wrapped_prompt" ]]; then
        __liaison_saved_prompt=$PS1
    fi
    __liaison_wrapped_prompt='\[\e]633;A\a\]'"$__liaison_saved_prompt"'\[\e]633;B\a\]'
    PS1=$__liaison_wrapped_prompt
}
PROMPT_COMMAND=(__liaison_capture_status "${PROMPT_COMMAND[@]}" __liaison_prompt)
PS0='$(__liaison_preexec)'"$PS0"
