# Completado dinámico de maly (Malody Mallow) — instalar con:
#   maly completions bash > ~/.local/share/bash-completion/completions/maly
# (requiere el paquete bash-completion; sin él, agregar a ~/.bashrc:
#   source <(maly completions bash))

_maly() {
    # tokens tras "maly" incluyendo la palabra parcial; si el cursor está
    # sobre una palabra nueva, COMP_WORDS no trae el elemento vacío: se añade
    local -a ctx=("${COMP_WORDS[@]:1:COMP_CWORD}")
    ((${#ctx[@]} < COMP_CWORD)) && ctx+=("")
    # los tokens llegan como se teclearon, con los espacios escapados de una
    # inserción previa (Proporción\ Áurea): quitar las barras para la consulta
    ctx=("${ctx[@]//\\/}")
    local cur=${ctx[-1]}

    local -a lines
    mapfile -t lines < <(maly __complete "${ctx[@]}" 2>/dev/null)

    local -a vals descs
    local line
    for line in "${lines[@]}"; do
        vals+=("${line%%$'\t'*}")
        if [[ $line == *$'\t'* ]]; then descs+=("${line#*$'\t'}"); else descs+=(""); fi
    done

    # readline por sí solo únicamente inserta el prefijo común de COMPREPLY
    # (con candidatos dispares no escribe nada) y jamás muestra descripciones:
    # la inserción se decide aquí, estilo kubectl/cobra.
    COMPREPLY=()
    if ((${#vals[@]} == 1)); then
        # candidato único: insertarlo entero; %q escapa espacios y acentos
        # ("aurea" → Proporción\ Áurea) y readline cierra con espacio solo
        COMPREPLY=("$(printf '%q' "${vals[0]}")")
    elif ((${#vals[@]} > 1)); then
        # prefijo común de los valores
        local lcd=${vals[0]} v
        for v in "${vals[@]:1}"; do
            while [[ $v != "$lcd"* ]]; do lcd=${lcd%?}; done
        done
        if ((${#lcd} > ${#cur})); then
            # hay progreso: insertar solo el prefijo, sin espacio de cierre
            compopt -o nospace
            COMPREPLY=("$(printf '%q' "$lcd")")
        else
            # sin progreso: listar los candidatos con su descripción, en el
            # orden en que maly los emite (nosort)
            compopt -o nosort 2>/dev/null
            local i
            for i in "${!vals[@]}"; do
                if [[ -n ${descs[i]} ]]; then
                    COMPREPLY+=("${vals[i]}  (${descs[i]})")
                else
                    COMPREPLY+=("${vals[i]}")
                fi
            done
            # candidato vacío de sacrificio: anula el prefijo común de la
            # lista para que readline no reescriba (y des-escape) la palabra
            COMPREPLY+=("")
        fi
    fi

    # scan y add aceptan rutas (y playlist import/export un archivo .m3u):
    # caer al completado de archivos de readline cuando maly no devuelve
    # candidatos
    case ${COMP_WORDS[1]} in
        scan | add) compopt -o default ;;
        playlist)
            case ${COMP_WORDS[2]} in
                import) ((COMP_CWORD >= 3)) && compopt -o default ;;
                export) ((COMP_CWORD >= 4)) && compopt -o default ;;
            esac ;;
    esac
}

complete -F _maly maly
