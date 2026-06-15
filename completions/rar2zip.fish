# fish completion for rar2zip
# Install: copy to ~/.config/fish/completions/rar2zip.fish

complete -c rar2zip -f

complete -c rar2zip -s o -l output      -r -d 'write the single output to this file or directory'
complete -c rar2zip      -l out-dir     -r -d 'write all outputs into this directory'
complete -c rar2zip -s f -l force          -d 'overwrite outputs that already exist'
complete -c rar2zip -s q -l quiet          -d 'suppress progress output'
complete -c rar2zip      -l password    -r -d 'password for encrypted archives'
complete -c rar2zip      -l jobs        -r -d 'number of archives to convert concurrently'
complete -c rar2zip      -l store          -d 'store entries without compression'
complete -c rar2zip      -l level       -r -d 'Deflate compression level 1..9'
complete -c rar2zip      -l verify         -d 'reopen each output ZIP and validate it'
complete -c rar2zip      -l json           -d 'emit a machine-readable JSON summary'
complete -c rar2zip      -l allow-fallback -d 'use system unrar/7z when pure-Go decode fails'
complete -c rar2zip      -l max-size    -r -d 'cap total uncompressed size (e.g. 10M, 2G)'
complete -c rar2zip      -l max-entries -r -d 'cap number of entries per archive'
complete -c rar2zip      -l list           -d 'preview archive contents without converting'
complete -c rar2zip      -l skip-existing  -d 'skip inputs whose output already exists'
complete -c rar2zip      -l verbose        -d 'print extra diagnostics to stderr'
complete -c rar2zip      -l version        -d 'print version and exit'
complete -c rar2zip -s h -l help           -d 'print usage and exit'

# Positional arguments: .rar files.
complete -c rar2zip -k -a '(__fish_complete_suffix .rar)'
