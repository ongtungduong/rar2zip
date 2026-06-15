# bash completion for rar2zip
# Install: source this file, or copy to /etc/bash_completion.d/ (or
# $(brew --prefix)/etc/bash_completion.d/ on Homebrew).

_rar2zip() {
    local cur flags
    cur="${COMP_WORDS[COMP_CWORD]}"
    flags="-o --output --out-dir -f --force -q --quiet --password \
--jobs --store --level --verify --json --allow-fallback \
--max-size --max-entries --list --skip-existing --verbose \
--version -h --help"

    if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return
    fi
    # Otherwise complete .rar files and directories.
    COMPREPLY=( $(compgen -f -X '!*.rar' -- "$cur") $(compgen -d -- "$cur") )
}
complete -F _rar2zip rar2zip
