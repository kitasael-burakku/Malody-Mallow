# `internal/i18n` e `internal/update` — paquetes de soporte

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.
`internal/config`, `internal/getter` e `internal/viz` no tienen ficha propia:
sus invariantes viven en `CLAUDE.md` (config y getter) y en
`docs/architecture/tui.md` (viz).

## `internal/i18n`

`T/Tf` (idioma global) y `TL/TLf` (por petición: el cliente
manda `Request.Lang` y el demonio responde en ese idioma). `TestTableIntegrity`
valida en/es al agregar claves, y `callsites_test.go` valida los SITIOS DE
LLAMADA parseando el árbol desde `../../` con `go/ast`: que la clave exista,
que `Tf`/`TLf` pasen tantos argumentos como verbos tiene la clave, que
ningún `T`/`TL` pida una clave con verbos (el `fmt.Sprintf(i18n.T(k), …)`
que cerró A-19) y que no queden claves muertas. Hace falta porque **`go vet`
no puede ayudar acá**: reconoce printf-wrappers cuando el formato es un
PARÁMETRO, y en `Tf` el formato sale de buscar la clave en la tabla. Dos
detalles que muerden si se tocan: las cadenas literales se recogen de todo
el árbol MENOS del propio paquete `i18n` (si no, cada clave de la tabla se
usaría a sí misma y el chequeo de claves muertas sería vacuo — verificado),
y los `_test.go` quedan fuera porque usan claves inventadas a propósito. Las
claves que se ARMAN por concatenación (`"cli.preset_"+name`) van en una
lista a mano.

## `internal/update`

chequeo de releases fiel a la filosofía "coordinar
herramientas": `git ls-remote --tags` contra el repo (nada de HTTP propio),
mayor tag semver vs `version.Version`, cache 24 h en
`XDG_DATA_HOME/maly/update.json`. `maly update` (CLI y paleta) descarga el
instalador con curl a un temporal y corre `sh <tmp> --update`; la TUI
chequea en `Init` (gated por `update_check` del config) y avisa en el pie
(`updAvail`, prioridad tras `verMismatch`).
