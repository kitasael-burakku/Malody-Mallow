# `cmd/maly` — la CLI y el arranque de la TUI

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## `cmd/maly`

CLI. `commands.go` tiene la **tabla de comandos**: fuente única de
verdad para dispatch, secciones, aliases y callbacks de completado de shell
(bash/fish/zsh vía `__complete` oculto). Nombres, argumentos localizados y
claves de descripción se comparten con la TUI desde `internal/commandmeta`;
help y completado leen esos metadatos, incluidos los subcomandos de playlist/get.
Los tests contrastan gramática esperada, dispatch real y paridad. `tui.go` decide en `startOrAttach` si la TUI embebe el
demonio o entra como cliente: ante `ErrAlreadyRunning` NO se rinde —el flock
ya dijo que hay otro— sino que espera hasta 8 s a que conteste
(`waitForDaemon`), porque un demonio arrancando tarda hasta 5 s esperando a
mpv y no contesta al ping (1.16.4). `runScan` manda la ruta ya RESUELTA y
nunca `Query: ""`: el config del demonio es de cuando arrancó, así que
resolverla él anunciaba una ruta y escaneaba otra. Al agregar un subcomando de playlist, actualizar la lista
fija de `TestCompletePlaylistSubs`. El completado de pistas pide con TOPE
(`library.SearchLimit`, `completeFetch` filas) y no con `Search`: la palabra
parcial vacía es la consulta que recorre la biblioteca entera, y un TAB no
puede materializarla para mostrar treinta líneas — ver la 1.7.3.

**Consultas en lote.** `play`, `add` y `playlist add` toman UNA consulta por
argumento (`maly add Secreto "Otra dimension"`): la shell ya respetó las
comillas, y una frase de varias palabras es un solo argumento. Antes se
unían todos en una frase, así que `maly add daft punk` pasó de «las pistas
con daft Y punk» a «las de daft más las de punk» — ruptura aceptada por el
dueño, porque la shell oculta las comillas y no hay forma de tener las dos
lecturas (ver `docs/decisions/no-hacer.md`, «CLI y consola»). Las reglas:
`resolveQueryPath` se aplica POR ARGUMENTO (unida a otra, una ruta dejaría de
existir), los argumentos en blanco no cuentan (`Search("")` es la biblioteca
entera), y en el cable `ipc.Request.SetQueries` manda una sola consulta en
`Query` —el caso de siempre funciona contra un demonio anterior— y varias en
`Queries`. Una consulta que no encuentra nada se SALTA y se nombra en la
misma línea (`ipc.WithNoMatch`): son independientes y cancelar todo por una
errata obligaría a reescribir las buenas. Si no queda ninguna pista hay
error (y salida 1); con alguna, éxito y salida 0. Un fallo real de la
biblioteca cancela el lote con su error original, antes de mutar la cola o
playlist; no se presenta como una consulta sin coincidencias. El orden es el
de los argumentos, y una pista que ya trajo un argumento anterior no se repite
dentro del lote; contra la cola o la playlist existentes se trata como
siempre (la cola admite repetidas; `playlist add` las cuenta, A-23).
`playlist add` resuelve con `library.SearchEach` y el mensaje sin pistas es
`ipc.PlaylistNoTracks`, compartidos con la consola. El completado de pistas
busca SOLO la palabra parcial: los argumentos anteriores son otras canciones.

`get.go` es el wrapper de yt-dlp
(filosofía "como lazygit usa git": maly coordina herramientas externas, no
las reimplementa): descarga MP3 con metadata/carátula embebidas a `music_dir`
y re-escanea (vía IPC si el demonio responde, directo a la DB si no);
yt-dlp/ffmpeg opcionales vía `exec.LookPath` con mensaje de instalación; el
progreso de yt-dlp pasa directo al terminal, cero parsing. Las ÚNICAS salidas
de yt-dlp que maly lee son sus interfaces de MÁQUINA, nunca el texto de
progreso: `--dump-json` (`getter.Search`, 1.15.0) y `--print-to-file
after_move:filepath` (`Opts.PathsFile` + `getter.ReadPaths`, A-07). La
distinción está detallada en la cabecera de `internal/getter/search.go` y en
la 1.15.0 del roadmap.

El **criterio de éxito de una descarga** necesita las dos señales, y ninguna
sola basta: el código de salida no ve que una búsqueda sin resultados sale 0
sin bajar nada (lección de la 1.15.0), y el diff del directorio no ve que
yt-dlp NO rebaja lo que ya existe —sale 0, no toca el disco, diff vacío— con
lo que "ya lo tenías" y "no encontré nada" colapsaban en el mismo error, con
código 1, pese a tener remedios opuestos (A-07). Con el archivo de rutas los
desenlaces quedan separados:

| yt-dlp | rutas | diff | maly dice |
| --- | --- | --- | --- |
| 0 | 1 | 1 nuevo | descargado |
| 0 | 1 | vacío | **ya lo tenías** (y NO es error) |
| 0 | 0 | vacío | la búsqueda no encontró nada |
| ≠0 | audio conservado | cualquiera | parcial: indexar lo obtenido, avisar del fallo y limpiar intermedios |
| ≠0 | sin audio | vacío | falló (+ `Cleanup` de intermedios) |

Que el hook dispare también para un archivo SALTADO se comprobó en la fuente
de yt-dlp (las dos ramas de `report_file_already_downloaded` no retornan
temprano, así que se llega a `run_all_pps("after_move")`) **y** en vivo contra
yt-dlp real. Un archivo de rutas ausente o vacío degrada al criterio anterior,
el diff solo.
Desde UX-07, `getter.Collect` contrasta las rutas reportadas con archivos de
audio reales bajo el destino, elimina duplicados y conserva su identidad exacta.
Si hay reporte, no incorpora archivos ajenos que aparecieron en el diff. Sin
reporte se conserva el respaldo anterior por diff, que no puede atribuir archivos
creados simultáneamente por otro proceso. Una descarga existente también pasa
por scan: estar en disco no equivale a estar indexada. Se escanea el destino
original, aunque el config cambie durante la descarga. `getter.Indexed` exige
que todas las rutas estén en SQLite antes de anunciar el resultado final.

`downloadOne` ofrece `maly add`/`maly play` por ruta con comillas de shell, sin
ejecutarlos. Los nombres con controles remiten a `maly select`, sin alterar la
ruta ni imprimir controles. Una descarga parcial ofrece los archivos indexados
y después devuelve el error de yt-dlp. `get playlist` conserva su contrato
anterior de parcial aprovechable y ofrece las rutas de la playlist creada.
El formato de rutas sigue siendo una línea por archivo; no admite nombres con
saltos de línea. No se parsea la salida humana ni se añade red propia.

`[ytdlp] cookies_from_browser` del config viaja tal cual a
`--cookies-from-browser` (passthrough sin validar, "" = sin flag; los
comentarios del template del config son estáticos en español, sin i18n);
navegadores derivados van con ruta de perfil, p. ej. Zen (base Firefox,
perfiles en `~/.config/zen/`): `firefox:/ruta/al/perfil`. mp3 a propósito:
dhowden lee sus ID3 en el scan y la miniatura APIC es justo lo que
`mpris:artUrl` extrae. Tests sin red con un yt-dlp falso en el PATH
(`get_test.go`, mismo patrón que el mpv falso de `player_test.go`).
`info.go` y `doctor.go` son el diagnóstico, y la división entre ambos es el
contrato de salida: `info` lista HECHOS (rutas, versiones, música,
biblioteca, config) y sale siempre 0; `doctor` emite VEREDICTOS y sale 1
solo si algo impide reproducir. Ninguno necesita demonio ni red — es la
condición para que sirvan de algo — y ninguno duplica detección: reusan
`getter.Tools`, `probe.Available`, `viz.CaptureBackend`,
`mpris.BusAvailable`, `MusicDirOrigin` y `update.Cached`. `libraryStats`
(en `info.go`, compartido con doctor) abre por `openLibraryIfExists`
porque `library.Open` crearía la base. Ver el detalle y sus invariantes en
la 1.7.0 del roadmap.

Dos chequeos de `doctor` le PREGUNTAN al demonio, y son los únicos: lo que
el servicio tiene en efecto no se puede deducir desde afuera. `checkDaemonConfig`
compara su config con el del disco (A-03) y `checkPlayer` pregunta si su mpv
sigue vivo (O-07) — mpv es hijo del demonio y su socket es privado, así que
antes `doctor` miraba el socket del DEMONIO, lo encontraba sano y daba todo
en verde mientras nada podía sonar. `checkPlayer` es el único de los dos que
puede ser `fail`, y con razón: el demonio no relanza mpv, así que el estado
impide reproducir y no se arregla solo. Los dos toleran un demonio viejo que
no sepa contestar (la fila simplemente no sale) y ninguno cambia el hecho de
que `doctor` funcione sin demonio.

## Guardar la cola (`playlist save`, UX-06)

CLI y consola delegan en `internal/playlist.SaveQueue`; el contrato de la
instantánea y la transacción vive en [biblioteca](library.md#guardar-una-cola-como-playlist).
La rama `save` se resuelve antes de abrir una base con `Create`: ni un nombre
vacío ni la ausencia de servicio o biblioteca fabrican estado. `play` y `save`
son los subcomandos de playlist que requieren demonio. Catálogo, ayuda, paleta
y completado anuncian `playlist save <nombre>`; los nombres existentes no se
sugieren como destino porque no se sobrescriben.
