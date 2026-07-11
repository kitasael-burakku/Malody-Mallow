#compdef maly
# Completado dinámico de maly (Malody Mallow) — instalar como _maly en un
# directorio del fpath (antes de compinit), p. ej.:
#   maly completions zsh > ~/.local/share/zsh/site-functions/_maly
# o, sin tocar el fpath, agregar a ~/.zshrc tras compinit:
#   source <(maly completions zsh)

_maly() {
    # words[2,CURRENT] son los tokens tras "maly", incluida la palabra
    # parcial bajo el cursor (vacía si se acaba de teclear un espacio)
    local -a lines
    lines=(${(f)"$(command maly __complete "${(@)words[2,CURRENT]}" 2>/dev/null)"})

    local -a pairs
    local l val desc
    for l in $lines; do
        val=${l%%$'\t'*}
        desc=${l#*$'\t'}
        # _describe separa valor:descripción por ":": escapar los del valor
        if [[ $l == *$'\t'* ]]; then
            pairs+=("${val//:/\\:}:$desc")
        else
            pairs+=("${val//:/\\:}")
        fi
    done

    # -U: no re-filtrar por lo tecleado — maly ya filtró lo semántico y su
    # búsqueda es fold-aware ("aurea" encuentra "Proporción Áurea"; el
    # matcher default de zsh, sensible a mayúsculas y acentos, la taparía)
    local ret=1
    (($#pairs)) && _describe 'maly' pairs -U && ret=0

    # scan y add aceptan rutas además de los candidatos; playlist
    # import/export toman un archivo .m3u
    case $words[2] in
        scan | add) _files && ret=0 ;;
        playlist)
            case $words[3] in
                import) ((CURRENT >= 4)) && _files && ret=0 ;;
                export) ((CURRENT >= 5)) && _files && ret=0 ;;
            esac ;;
    esac
    return $ret
}

# con #compdef el fpath lo registra solo; el compdef explícito cubre el
# caso de `source` directo (donde la primera línea es solo un comentario)
(($+functions[compdef])) && compdef _maly maly

# autoloaded desde el fpath, el archivo ES la función: hay que invocarla;
# con `source` directo (funcstack vacío aquí) no, sería fuera de un TAB
if [[ $funcstack[1] == _maly ]]; then
    _maly "$@"
fi
