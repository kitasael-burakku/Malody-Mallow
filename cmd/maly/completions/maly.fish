# Completado dinámico de maly (Malody Mallow) — instalar con:
#   maly completions fish > ~/.config/fish/completions/maly.fish

function __maly_complete
    # tokens previos ya tokenizados + el token parcial bajo el cursor;
    # [2..] descarta el propio "maly" (o la ruta con que se invocó)
    set -l tokens (commandline -opc) (commandline -ct)
    maly __complete $tokens[2..] 2>/dev/null
end

# -f: sin completado de archivos por defecto (los candidatos vienen de maly)
complete -c maly -f -a '(__maly_complete)'

# scan y add sí aceptan rutas: rehabilitar archivos además de los candidatos
complete -c maly -n '__fish_seen_subcommand_from scan add' --force-files
