# Completado dinámico de maly (Malody Mallow) — instalar con:
#   maly completions bash > ~/.local/share/bash-completion/completions/maly
# (requiere el paquete bash-completion; sin él, agregar a ~/.bashrc:
#   source <(maly completions bash))

_maly() {
    # tokens tras "maly" incluyendo la palabra parcial; si el cursor está
    # sobre una palabra nueva, COMP_WORDS no trae el elemento vacío: se añade
    local -a ctx=("${COMP_WORDS[@]:1:COMP_CWORD}")
    ((${#ctx[@]} < COMP_CWORD)) && ctx+=("")

    local -a lines
    mapfile -t lines < <(maly __complete "${ctx[@]}" 2>/dev/null)

    # maly ya filtró lo semántico; no se re-filtra por prefijo para que
    # readline pueda reemplazar "aurea" por "Proporción Áurea" (búsqueda
    # fold-aware). %q escapa espacios y acentos para la inserción.
    COMPREPLY=()
    local line
    for line in "${lines[@]}"; do
        COMPREPLY+=("$(printf '%q' "${line%%$'\t'*}")")
    done

    # scan y add aceptan rutas: caer al completado de archivos de readline
    # cuando maly no devuelve candidatos
    case ${COMP_WORDS[1]} in scan | add) compopt -o default ;; esac
}

complete -F _maly maly
