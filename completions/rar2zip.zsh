#compdef rar2zip
# zsh completion for rar2zip
# Install: place this file as _rar2zip in a directory on your $fpath
# (e.g. ~/.zsh/completions), then ensure `autoload -U compinit && compinit`.

_rar2zip() {
  _arguments -s \
    '(-o --output)'{-o,--output}'[write the single output to this file or directory]:path:_files' \
    '--out-dir[write all outputs into this directory]:dir:_files -/' \
    '(-f --force)'{-f,--force}'[overwrite outputs that already exist]' \
    '(-q --quiet)'{-q,--quiet}'[suppress progress output]' \
    '--password[password for encrypted archives]:password:' \
    '--jobs[number of archives to convert concurrently]:jobs:' \
    '--store[store entries without compression]' \
    '--level[Deflate compression level 1..9]:level:(1 2 3 4 5 6 7 8 9)' \
    '--verify[reopen each output ZIP and validate it after writing]' \
    '--json[emit a machine-readable JSON summary on stdout]' \
    '--allow-fallback[use system unrar/7z when the pure-Go decoder fails]' \
    '--max-size[cap total uncompressed size (e.g. 10M, 2G)]:size:' \
    '--max-entries[cap number of entries per archive]:count:' \
    '--list[preview archive contents without converting]' \
    '--skip-existing[skip inputs whose output already exists]' \
    '--verbose[print extra diagnostics to stderr]' \
    '--version[print version and exit]' \
    '(-h --help)'{-h,--help}'[print usage and exit]' \
    '*:rar file:_files -g "*.rar"'
}

_rar2zip "$@"
