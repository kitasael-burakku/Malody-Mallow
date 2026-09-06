# `internal/tui` — la interfaz Bubble Tea

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## `internal/tui`

Bubble Tea. Recibe estado por **suscripción push**
(`subscribe`; fallback a polling de 500 ms con reintento). **Todo el
reparto de la pantalla vive en `computeLayout` (`layout.go`), función
PURA** de (ancho, alto, `layoutOpts{viz, banner}`) — no lee el Model, y
eso es innegociable: repartida dentro de `View()`, la aritmética de tres
columnas + franja del viz + banner es justo la clase de código que se
rompe una celda a la vez y solo en ciertos tamaños, imposible de cazar a
ojo contra un terminal. La tabla de tests barre 13 anchos × 10 altos × 6
combinaciones de opciones y exige que los anchos sumen exactamente `w` y
que `bannerH + topH + vizH + nowH + 1 == h`. Tres modos por ancho
(`layoutFull` ≥120, `layoutTwoCol` ≥90, `layoutSingle` por debajo — ahí se
dibuja SOLO el panel enfocado y `tab` cicla), y un orden de sacrificios
por altura que también es decisión y no accidente: cae primero la franja
del viz (decoración), después el banner, y la fila de paneles solo cede en
el último extremo. Anchos: biblioteca FIJA (26-34; un porcentaje dejaba
dos tercios de espacio muerto para nombres de artista de 10-25 caracteres)
y "Ahora suena" fija (28-32, la manda la carátula); la COLA es la elástica
porque es la de contenido largo. La tercera columna se parte a su vez en
`npH`/`lyrH` (carátula+ficha arriba, letras abajo), con `lyrH` topeado a
`2*maxLyricDistance+3`: más allá de esa distancia las líneas ya salen en
blanco (corte de la 1.13.1), así que estirar el panel solo sumaría filas
vacías y ese sobrante vuelve a la carátula. La barra de reproducción
(`nowPanel`) es horizontal y de ancho completo en los TRES modos (decisión
del dueño: en 26 celdas de columna el progreso no se lee), y el
visualizador es una FRANJA sin panel propio. `progress.go` tiene la barra
como funciones puras: relleno con gradiente agrupado por pasos, cabeza con
octavos horizontales (`▏▎▍▌▋▊▉` — NO los del viz, que llenan por altura y
no parten una celda a lo ancho), estela `▓▒` y pista `░`; sus guardas se
escriben `!(dur > 0)` y no `dur <= 0` porque NaN es false en toda
comparación. `display.go` limpia los títulos SOLO para mostrarlos (artista
repetido + sufijos de yt-dlp): un grupo entre paréntesis se descarta solo
si TODAS sus palabras están en `noiseWords`, así "(Remix)", "(feat. …)" o
"(2016-2017)" sobreviven; no toca la DB ni `ipc`/`library`, que es lo que
mantiene a `maly search` encontrando por el título original, y por eso
tampoco vive en `internal/safetext` (que es una frontera de SEGURIDAD, no
cosmética). Paneles biblioteca/
cola + consola ctrl+p (tabla propia de comandos en `console.go`, con paridad
CLI completa: `playlist` en `console_playlist.go`, `get` vía
`tea.ExecProcess` + `internal/getter` compartido con la CLI, `controls`
aplica el preset en vivo recargando `m.keys`; `playlist delete` NO borra:
arma `conPlConfirm` y lo resuelve la línea SIGUIENTE, que se consume pase
lo que pase —igual que `confirmYesNo` de la CLI, que lee UNA línea— y es
campo aparte del `plConfirm` del panel ctrl+l para que un camino no cancele
el borrado pendiente del otro. En `get playlist`, `planGetPlaylist` toma
TODAS las decisiones previas a la descarga y `newTrackIDs` filtra lo que ya
estaba en el destino: van separadas de sus consumidores porque
`tea.ExecProcess` solo se resuelve dentro del runtime de bubbletea y al
resto solo se llega pasando por el demonio, así que sin partirlas no hay
forma de comprobar en un test qué decidió esta mitad — que es exactamente
cómo A-04 pudo pasar desapercibido) + picker
fuzzy genérico (`picker.go`, usado por ctrl+o canciones, ctrl+l playlists y
`maly select`; sus knobs `noFilter`/`emptyText`, con valor cero =
comportamiento de siempre, existen para el buscador de descargas) +
buscador de descargas ctrl+g (`get.go`: picker sobre resultados REMOTOS de
yt-dlp, de dos fases porque cada consulta cuesta ~1 s de red — elige una
URL y se la pasa a `startGet`, que es el punto único de descarga de la TUI
y NO vive acá sino en `console.go`, compartido con la consola; `get_pick.go`
es su gemelo suelto para `maly get pick`, con el patrón de `RunSelect` pero
SIN exigir demonio). **Cancelar una búsqueda y SALIR no son lo mismo**:
`abortGetSearch` (subir la generación + cancelar el contexto) alcanza mientras
la TUI siga viva, porque `cancel()` no mata —solo marca el contexto— y el kill
lo ejecuta la goroutine vigía que `os/exec` arranca en `Start()`. Si el proceso
se va enseguida, esa vigía se va con él y el yt-dlp queda huérfano hasta su
propio timeout de 20 s. Por eso la salida usa `stopGetSearch`, que cancela **y
espera** (con tope, `getKillWait`): en `RunGetPick` tras `p.Run()`, y en
`tui.Run` tras el suyo, porque `ctrl+c` corta ANTES de todo modal en
`handleKey` y nunca pasa por `closeGet` (A-15; medido: sin la espera, 5 de 5
huérfanos). Los modales tapan el footer: los flashes no se ven con un
modal abierto (el panel de playlists los dibuja bajo el modal por eso).
El árbol de la biblioteca (`tree.go`) incluye las playlists como raíces
tras los artistas (`playlistNode`, pistas hijas directas numeradas por
posición); la indentación y la búsqueda de padre usan el campo `depth`,
no el kind. Toda mutación de playlists en la TUI (plActMsg, y en la
consola `conMsg.reload`) recarga el árbol local Y manda `notifyRefresh`
(op IPC `refresh`, best-effort: el demonio sube `libGen` y las DEMÁS
TUIs recargan por el push; la CLI hace lo mismo tras sus 5 mutadores de
playlist). La doble recarga de la TUI que mutó — local + push propio —
es redundancia aceptada. El `case libraryMsg` es el PUNTO ÚNICO de
refresco: reconstruye el árbol y realimenta los pickers abiertos (ctrl+l
con `plItems` sobre las listas que la propia recarga ya trajo — nada de
reconsultar la DB; ctrl+o con `songItems`). Como `buildTree` crea nodos
nuevos, el árbol se guarda y repone con `snapshot`/`restore` (expansión,
cursor por clave de nodo, filtro y scroll) o cada scan lo colapsaría y
saltaría al tope; y los pickers usan `setItemsKeeping`, que conserva la
selección POR VALOR — con el índice pelado, algo que desaparezca más
arriba corre la lista bajo los dedos y ctrl+x borra otra playlist. La capa "Ahora suena" (ctrl+t,
`nowplaying.go`) es una vista fullscreen con carátula (interfaz
`coverRenderer`, cada renderer escala a su densidad: half-blocks ANSI en
`artrender.go`, y en kitty el protocolo gráfico vía **Unicode
placeholders** en `artkitty.go` — la imagen vive en celdas U+10EEEE con
el id en la tinta y fila/columna en diacríticos de la tabla oficial, así
el diff de bubbletea, los modales y el cierre funcionan solos; la
transmisión t=d va pegada a la fila 0 del render cacheado → una vez por
pista, no por frame; `q=2` OBLIGATORIO o kitty contesta por stdin;
detección por TERM/KITTY_WINDOW_ID y bajo tmux cae a half-blocks),
letras (resaltado sincronizado por `Status.Position` si hay `.lrc`) y la
franja del viz (`vizLines` compartido con `vizPanel`); carga carátula y
letras SIEMPRE en goroutine (`loadNowMeta`, cache por pista `npTrack` +
render invalidado en resize), y `applyStatus` relanza la carga al cambiar
la pista. `playbackKey` centraliza las teclas de reproducción compartidas
entre la vista principal y la capa. Los RELOJES de animación se
autocancelan para no gastar CPU en reposo (medido: 3,5 % → 0,7 % de un
núcleo): la onda del banner solo corre visible Y con música
(`armLogoTick`; congelada en pausa a propósito), el tick del viz muere
con el viz apagado (`armVizTick`) y con las barras decaídas respira a
500 ms en vez de 60 — NO se mata del todo: el ring del viz captura el
audio del sistema, no solo el de maly, y debe reaccionar si otra app
suena. Los rearmes viven en `applyStatus` (al instante) con red de
seguridad en el `case tickMsg`; ambos guardados por
`vizTicking`/`logoTicking` para no duplicar relojes. El comando `logo` de la consola aplica
el gradiente del banner en vivo y lo persiste (`SaveThemeLogo` → `saveKey`,
que edita claves dentro de secciones TOML sin tocar el resto). El **arte
ASCII** del banner se reemplaza con `logo.txt` junto al config
(`loadLogoArt` en `config.Load` → `Theme.LogoArt`, `toml:"-"`); a
propósito NO es clave TOML: un string multilínea rompería el parser por
líneas de `saveKey` y el escapado del `\` de figlet. La altura del panel
es dinámica (`panelH`/`minRows` del `logoModel`; el arte se recorta a
`maxLogoArt` líneas en config).
